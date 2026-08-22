// ─── Session Store ───
//
// Zustand store for authentication session: workspace, token, identity,
// and the `can()` permission check that drives the Permission Gate.
//
// Session lives in memory only. Reloading the page means re-authenticating.

import { create } from "zustand"
import type { KyInstance } from "ky"
import type { MeResponse } from "@/types/manifest"
import { createApiClient, fetchMe } from "@/lib/api"
import { can } from "@/engine/permissions"

// KyInstance is a generic HTTP client type from the ky library

export interface SessionState {
  /** The workspace slug from the URL (e.g. "acme") */
  workspace: string
  /** JWT token (may be empty in dev mode) */
  token: string
  /** Parsed identity from _meta/me */
  me: MeResponse | null
  /** Whether the session is fully loaded */
  loaded: boolean
  /** True when the server rejected the session with 401 (not logged in) */
  unauthenticated: boolean
  /** Optional error from boot */
  error: string | null

  // ── Actions ──
  setSession: (workspace: string, token: string) => void
  clearSession: () => void
  boot: (workspace: string, token?: string) => Promise<void>
  getClient: () => KyInstance
  /** Check if the current identity holds a permission (see engine/permissions) */
  can: (permission: string) => boolean
}

export const useSessionStore = create<SessionState>((set, get) => ({
  workspace: "",
  token: "",
  me: null,
  loaded: false,
  unauthenticated: false,
  error: null,

  setSession: (workspace: string, token: string) => {
    set({ workspace, token, loaded: true, error: null, unauthenticated: false })
  },

  clearSession: () => {
    set({
      workspace: "",
      token: "",
      me: null,
      loaded: false,
      unauthenticated: false,
      error: null,
    })
  },

  boot: async (workspace: string, token?: string) => {
    set({
      workspace,
      token: token ?? "",
      loaded: false,
      error: null,
      unauthenticated: false,
    })

    const me = await fetchMe(workspace, token)
    if (!me) {
      // Server unreachable / error — connection error screen.
      set({
        me: null,
        loaded: true,
        error: "Failed to load session",
        unauthenticated: false,
      })
      return
    }
    // _meta/me returns user_id "anonymous" when not authenticated. Treat that
    // as unauthenticated so the auth guard redirects to /login — do NOT
    // fabricate a synthetic identity (that would bypass authorization).
    if (me.user_id === "anonymous") {
      set({ me: null, loaded: true, error: null, unauthenticated: true })
      return
    }
    set({ me, loaded: true, error: null, unauthenticated: false })
  },

  getClient: () => {
    const { workspace, token } = get()
    return createApiClient({ workspace, token }) as unknown as KyInstance
  },

  can: (permission: string) => {
    const { me } = get()
    if (!me) return false
    return can(permission, me.permissions)
  },
}))
