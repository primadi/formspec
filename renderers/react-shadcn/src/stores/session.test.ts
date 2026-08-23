// ─── Session Expiry Tests ───
//
// Verifies the 401 → login wiring: the session store registers a handler on
// the sessionEvents bus, and any API client that observes a 401 calls
// notifySessionExpired(), which expires the session (unauthenticated=true)
// so the auth guard redirects to the login page.

import { describe, it, expect, beforeEach } from "vitest"
import { notifySessionExpired, onSessionExpired } from "@/lib/api/sessionEvents"
import { useSessionStore } from "@/stores/session"

describe("session expiry", () => {
  beforeEach(() => {
    useSessionStore.setState({
      workspace: "acme",
      token: "some-token",
      refreshToken: "some-refresh",
      me: {
        user_id: "u1",
        workspace: "acme",
        roles: ["admin"],
        permissions: ["*"],
      },
      loaded: true,
      unauthenticated: false,
      error: null,
    })
  })

  it("expireSession marks the session unauthenticated and clears the tokens", () => {
    useSessionStore.getState().expireSession()
    const s = useSessionStore.getState()
    expect(s.unauthenticated).toBe(true)
    expect(s.token).toBe("")
    expect(s.refreshToken).toBe("")
    expect(s.me).toBeNull()
    expect(s.loaded).toBe(true)
    expect(s.error).toBeNull()
  })

  it("setSession stores the refresh token", () => {
    useSessionStore.getState().setSession("acme", "tok", "ref")
    const s = useSessionStore.getState()
    expect(s.token).toBe("tok")
    expect(s.refreshToken).toBe("ref")
    expect(s.unauthenticated).toBe(false)
  })

  it("notifySessionExpired triggers the store's registered handler (401 → expireSession)", () => {
    // The session store registers its handler at module load.
    notifySessionExpired()
    const s = useSessionStore.getState()
    expect(s.unauthenticated).toBe(true)
    expect(s.token).toBe("")
    expect(s.me).toBeNull()
  })

  it("onSessionExpired registers a custom handler", () => {
    let called = 0
    onSessionExpired(() => {
      called++
    })
    notifySessionExpired()
    expect(called).toBe(1)
  })
})
