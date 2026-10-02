// ─── Session Store ───
//
// Zustand store for authentication session: workspace, token, identity,
// and the `can()` permission check that drives the Permission Gate.
//
// The access + refresh tokens are persisted to sessionStorage so a browser
// refresh (F5) restores the session without re-authenticating. sessionStorage
// is per-tab and cleared when the tab closes — tokens never survive a browser
// restart.

import { create } from "zustand"
import ky, { type KyInstance } from "ky"
import {
  FormaApiError,
  type ContextChoice,
  type MeResponse,
} from "@/types/manifest"
import { createApiClient, fetchMe } from "@/lib/api"
import { onSessionExpired } from "@/lib/api/sessionEvents"
import { can } from "@/engine/permissions"

// KyInstance is a generic HTTP client type from the ky library

// ── Session persistence (sessionStorage, one slot per App) ──
//
// Login is per-App (plan app-scoped-login.md D1/D2): a session is scoped to
// exactly one App, so the storage is keyed by `(workspace, app)` instead of
// holding a single token. Switching App therefore switches to THAT App's
// session — no password re-entry while it is still alive, and never a token
// from another App (the server binds `_meta/ui` to the token's App).
//
// Tokens are short-lived (access 15m, refresh 7d). sessionStorage is per-tab
// and cleared when the tab closes — tokens never survive a browser restart.
const SESSION_STORAGE_PREFIX = "formspec-session:"

function storageKey(workspace: string, app: string): string {
  return `${SESSION_STORAGE_PREFIX}${workspace}:${app}`
}

interface StoredSession {
  workspace: string
  token: string
  refreshToken: string
  /** App scope for this session — required: login is per-App. */
  app: string
}

function readStoredSession(
  workspace: string,
  app: string,
): StoredSession | null {
  try {
    const raw = sessionStorage.getItem(storageKey(workspace, app))
    if (!raw) return null
    const parsed = JSON.parse(raw) as StoredSession
    return parsed?.token ? parsed : null
  } catch {
    return null
  }
}

function writeStoredSession(s: StoredSession): void {
  try {
    sessionStorage.setItem(storageKey(s.workspace, s.app), JSON.stringify(s))
  } catch {
    // Ignore (private mode / storage disabled).
  }
}

function clearStoredSession(workspace: string, app: string): void {
  try {
    sessionStorage.removeItem(storageKey(workspace, app))
  } catch {
    // Ignore.
  }
}

export interface SessionState {
  /** The workspace slug from the URL (e.g. "acme") */
  workspace: string
  /** App scope for this session — required: login is per-App. */
  app: string
  /** JWT access token (may be empty in dev mode) */
  token: string
  /** Refresh token used to mint a new access token when it expires. */
  refreshToken: string
  /** Parsed identity from _meta/me */
  me: MeResponse | null
  /** Whether the session is fully loaded */
  loaded: boolean
  /** True when the server rejected the session with 401 (not logged in) */
  unauthenticated: boolean
  /** Optional error from boot */
  error: string | null
  /**
   * Set when a token refresh answered 409 `CONTEXT_REQUIRED`: the session's
   * assignment was revoked (or its role deleted) and the caller must choose a
   * boundary again. Non-null means "show the picker" — the session is NOT
   * expired, so the credentials are still good (backend §8.7).
   */
  pendingContext: ContextChoice[] | null

  // ── Actions ──
  setSession: (
    workspace: string,
    token: string,
    refreshToken: string,
    app: string,
  ) => void
  clearSession: () => void
  /** Mark the session unauthenticated (401 / idle timeout) → login redirect */
  expireSession: () => void
  /**
   * Restore (or establish) the session for ONE App. An options object, not
   * positional strings: every field is a string, so a misordered call would
   * silently put the token in the `app` slot and fetch an anonymous bundle
   * (that exact bug shipped once — see plan app-scoped-login.md §Sisa).
   */
  boot: (opts: {
    workspace: string
    /** The App this session is scoped to — required (login is per-App). */
    app: string
    /** Explicit tokens (login / OAuth callback). Omitted → restore from storage. */
    token?: string
    refreshToken?: string
  }) => Promise<void>
  /** Refresh the access token (single-flight). Resolves true on success. */
  refreshSession: () => Promise<boolean>
  getClient: () => KyInstance
  /** Check if the current identity holds a permission (see engine/permissions) */
  can: (permission: string) => boolean
  /** Clear `pendingContext` once the picker has been shown / resolved. */
  clearPendingContext: () => void
}

// Single-flight guard so concurrent 401s share one refresh call.
let refreshInFlight: Promise<boolean> | null = null

// Single-flight + generation guards for boot() — see boot below. Without
// these, the AppSurface boot effect can start an anonymous boot while a
// login boot is still in flight; the anonymous one finishes last and
// overwrites the authenticated session (in dev auto-auth the tokenless
// /_meta/me returns a real identity, so the anonymous guard never fires).
// The in-flight boot is keyed by App: a boot for App A must never be reused
// for App B, or B would be handed A's session (one store, many Apps).
let bootInFlight: Promise<void> | null = null
let bootInFlightApp = ""
let bootGeneration = 0

export const useSessionStore = create<SessionState>((set, get) => ({
  workspace: "",
  app: "",
  token: "",
  refreshToken: "",
  me: null,
  loaded: false,
  unauthenticated: false,
  error: null,
  pendingContext: null,

  setSession: (
    workspace: string,
    token: string,
    refreshToken: string,
    app: string,
  ) => {
    set({
      workspace,
      app,
      token,
      refreshToken,
      loaded: true,
      error: null,
      unauthenticated: false,
    })
    if (token) {
      writeStoredSession({ workspace, token, refreshToken, app })
    } else {
      // Anonymous entry for this App — drop this App's slot only. Other Apps'
      // sessions are untouched (that is the point of per-App scoping).
      clearStoredSession(workspace, app)
    }
  },

  clearSession: () => {
    const { workspace, app } = get()
    if (workspace && app) clearStoredSession(workspace, app)
    set({
      workspace: "",
      app: "",
      token: "",
      refreshToken: "",
      me: null,
      loaded: false,
      unauthenticated: false,
      error: null,
      pendingContext: null,
    })
  },

  expireSession: () => {
    const { workspace, app } = get()
    if (workspace && app) clearStoredSession(workspace, app)
    set({
      token: "",
      refreshToken: "",
      me: null,
      loaded: true,
      unauthenticated: true,
      error: null,
      pendingContext: null,
    })
  },

  boot: ({ workspace, app, token, refreshToken }) => {
    // Single-flight: an anonymous re-entry (the surface boot effect re-running
    // while a login boot is still in flight) must not start a second
    // concurrent boot — the anonymous one would finish last and overwrite the
    // authenticated session. A boot with an explicit token (login) always
    // proceeds so it can invalidate older boots below, and a boot for a
    // DIFFERENT App always proceeds (it is a different session, not a repeat).
    if (!token && bootInFlight && bootInFlightApp === app) return bootInFlight
    const gen = ++bootGeneration
    const run = (async () => {
      // Restore THIS App's persisted session when no explicit token is given —
      // this is what survives a browser refresh, and what makes switching App
      // reuse a live session rather than demanding the password again.
      const stored = readStoredSession(workspace, app)
      const effectiveToken = token ?? stored?.token ?? ""
      const effectiveRefresh = refreshToken ?? stored?.refreshToken ?? ""

      set({
        workspace,
        app,
        token: effectiveToken,
        refreshToken: effectiveRefresh,
        loaded: false,
        error: null,
        unauthenticated: false,
      })

      let me: MeResponse | null
      try {
        me = await fetchMe(workspace, effectiveToken, {
          getToken: () => get().token,
          onUnauthorized: () => get().refreshSession(),
          needsContext: () => get().pendingContext !== null,
        })
      } catch {
        // Server unreachable / error — connection error screen. Keep the
        // persisted session so a later reload can retry.
        set({
          me: null,
          loaded: true,
          error: "Failed to load session",
          unauthenticated: false,
        })
        return
      }
      // A newer boot (e.g. login with an explicit token) superseded this one —
      // discard the stale result instead of overwriting the newer state.
      if (gen !== bootGeneration) return
      if (!me) {
        // fetchMe returns null on 401 — invalid / expired token. Clear THIS
        // App's persisted session and treat as unauthenticated so the auth
        // guard redirects to the login page instead of showing a connection
        // error. Other Apps' sessions are untouched.
        clearStoredSession(workspace, app)
        set({
          token: "",
          refreshToken: "",
          me: null,
          loaded: true,
          error: null,
          unauthenticated: true,
        })
        return
      }
      // _meta/me returns user_id "anonymous" when not authenticated. Treat that
      // as unauthenticated so the auth guard redirects to /login — do NOT
      // fabricate a synthetic identity (that would bypass authorization).
      if (me.user_id === "anonymous") {
        clearStoredSession(workspace, app)
        set({
          token: "",
          refreshToken: "",
          me: null,
          loaded: true,
          error: null,
          unauthenticated: true,
        })
        return
      }
      // Persist the restored/authenticated session for the next refresh.
      if (effectiveToken) {
        writeStoredSession({
          workspace,
          token: effectiveToken,
          refreshToken: effectiveRefresh,
          app,
        })
      }
      set({ me, loaded: true, error: null, unauthenticated: false })
    })()
    bootInFlight = run
    bootInFlightApp = app
    void run.then(
      () => {
        if (bootInFlight === run) bootInFlight = null
      },
      () => {
        if (bootInFlight === run) bootInFlight = null
      },
    )
    return run
  },

  refreshSession: () => {
    if (!refreshInFlight) {
      refreshInFlight = (async () => {
        const { workspace, refreshToken } = get()
        if (!refreshToken) return false
        try {
          const response = await ky.post(`/${workspace}/_ui/auth/refresh`, {
            json: { refresh_token: refreshToken },
            retry: 0,
          })
          const body = (await response.json()) as {
            data: { access_token: string; refresh_token: string }
          }
          set({
            token: body.data.access_token,
            refreshToken: body.data.refresh_token,
            pendingContext: null,
          })
          writeStoredSession({
            workspace: get().workspace,
            token: body.data.access_token,
            refreshToken: body.data.refresh_token,
            app: get().app,
          })
          return true
        } catch (err) {
          // 409 CONTEXT_REQUIRED: the session's assignment was revoked or its
          // role was deleted. This is NOT an expired session — the server is
          // asking which boundary to renew under, so the caller must choose
          // again (backend §8.7: "fail closed, minta pilih ulang"). Returning
          // false here would expire the session and silently discard a still
          // valid credential; instead record the choices so the picker can be
          // shown, and let the caller decide.
          if (
            err instanceof FormaApiError &&
            err.status === 409 &&
            err.code === "CONTEXT_REQUIRED" &&
            err.choices?.length
          ) {
            set({ pendingContext: err.choices })
          }
          return false
        }
      })().finally(() => {
        refreshInFlight = null
      })
    }
    return refreshInFlight
  },

  getClient: () => {
    const { workspace } = get()
    return createApiClient({
      workspace,
      getToken: () => get().token,
      onUnauthorized: () => get().refreshSession(),
      needsContext: () => get().pendingContext !== null,
    }) as unknown as KyInstance
  },

  can: (permission: string) => {
    const { me } = get()
    if (!me) return false
    return can(permission, me.permissions)
  },

  clearPendingContext: () => set({ pendingContext: null }),
}))

// Register the global 401 handler: any API call that returns 401 (invalid /
// expired token) marks the session unauthenticated, which the auth guard
// turns into a redirect to the login page.
onSessionExpired(() => {
  useSessionStore.getState().expireSession()
})
