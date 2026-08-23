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
import type { MeResponse } from "@/types/manifest"
import { createApiClient, fetchMe } from "@/lib/api"
import { onSessionExpired } from "@/lib/api/sessionEvents"
import { can } from "@/engine/permissions"

// KyInstance is a generic HTTP client type from the ky library

// ── Session persistence (sessionStorage) ──
//
// Tokens are workspace-scoped and short-lived (access 15m, refresh 7d).
// Persisting them to sessionStorage lets a page refresh restore the session
// without re-authenticating, while keeping them out of localStorage (which
// would survive browser restarts).

const SESSION_STORAGE_KEY = "formspec-session"

interface StoredSession {
  workspace: string
  token: string
  refreshToken: string
}

function readStoredSession(): StoredSession | null {
  try {
    const raw = sessionStorage.getItem(SESSION_STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as StoredSession
    return parsed?.token ? parsed : null
  } catch {
    return null
  }
}

function writeStoredSession(s: StoredSession): void {
  try {
    sessionStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(s))
  } catch {
    // Ignore (private mode / storage disabled).
  }
}

function clearStoredSession(): void {
  try {
    sessionStorage.removeItem(SESSION_STORAGE_KEY)
  } catch {
    // Ignore.
  }
}

export interface SessionState {
  /** The workspace slug from the URL (e.g. "acme") */
  workspace: string
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

  // ── Actions ──
  setSession: (workspace: string, token: string, refreshToken?: string) => void
  clearSession: () => void
  /** Mark the session unauthenticated (401 / idle timeout) → login redirect */
  expireSession: () => void
  boot: (
    workspace: string,
    token?: string,
    refreshToken?: string,
  ) => Promise<void>
  /** Refresh the access token (single-flight). Resolves true on success. */
  refreshSession: () => Promise<boolean>
  getClient: () => KyInstance
  /** Check if the current identity holds a permission (see engine/permissions) */
  can: (permission: string) => boolean
}

// Single-flight guard so concurrent 401s share one refresh call.
let refreshInFlight: Promise<boolean> | null = null

export const useSessionStore = create<SessionState>((set, get) => ({
  workspace: "",
  token: "",
  refreshToken: "",
  me: null,
  loaded: false,
  unauthenticated: false,
  error: null,

  setSession: (workspace: string, token: string, refreshToken?: string) => {
    set({
      workspace,
      token,
      refreshToken: refreshToken ?? "",
      loaded: true,
      error: null,
      unauthenticated: false,
    })
    if (token) {
      writeStoredSession({ workspace, token, refreshToken: refreshToken ?? "" })
    } else if (readStoredSession()?.workspace === workspace) {
      // Anonymous (public surface) on the same workspace — drop the session.
      clearStoredSession()
    }
  },

  clearSession: () => {
    clearStoredSession()
    set({
      workspace: "",
      token: "",
      refreshToken: "",
      me: null,
      loaded: false,
      unauthenticated: false,
      error: null,
    })
  },

  expireSession: () => {
    clearStoredSession()
    set({
      token: "",
      refreshToken: "",
      me: null,
      loaded: true,
      unauthenticated: true,
      error: null,
    })
  },

  boot: async (workspace: string, token?: string, refreshToken?: string) => {
    // Restore a persisted session (same workspace) when no explicit token is
    // given — this is what survives a browser refresh.
    const stored = readStoredSession()
    const restore = stored !== null && stored.workspace === workspace
    const effectiveToken = token ?? (restore ? stored.token : "")
    const effectiveRefresh =
      refreshToken ?? (restore ? stored.refreshToken : "")

    set({
      workspace,
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
    if (!me) {
      // fetchMe returns null on 401 — invalid / expired token. Clear the
      // persisted session and treat as unauthenticated so the auth guard
      // redirects to the login page instead of showing a connection error.
      clearStoredSession()
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
      clearStoredSession()
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
      })
    }
    set({ me, loaded: true, error: null, unauthenticated: false })
  },

  refreshSession: () => {
    if (!refreshInFlight) {
      refreshInFlight = (async () => {
        const { workspace, refreshToken } = get()
        if (!refreshToken) return false
        try {
          const response = await ky.post(`/${workspace}/api/v1/auth/refresh`, {
            json: { refresh_token: refreshToken },
            retry: 0,
          })
          const body = (await response.json()) as {
            data: { access_token: string; refresh_token: string }
          }
          set({
            token: body.data.access_token,
            refreshToken: body.data.refresh_token,
          })
          writeStoredSession({
            workspace: get().workspace,
            token: body.data.access_token,
            refreshToken: body.data.refresh_token,
          })
          return true
        } catch {
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
    }) as unknown as KyInstance
  },

  can: (permission: string) => {
    const { me } = get()
    if (!me) return false
    return can(permission, me.permissions)
  },
}))

// Register the global 401 handler: any API call that returns 401 (invalid /
// expired token) marks the session unauthenticated, which the auth guard
// turns into a redirect to the login page.
onSessionExpired(() => {
  useSessionStore.getState().expireSession()
})
