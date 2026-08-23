// ─── Session Expiry Events ───
//
// Tiny module-level event bus so the API layer (lib/api) can notify the
// session store (stores/session) that the server rejected the session with
// 401 (invalid / expired token) — without a circular import, since
// stores/session already imports lib/api.
//
// The session store registers the single handler; any 401 observed by a
// client (entity CRUD or meta) marks the session unauthenticated, which the
// auth guard turns into a redirect to the login page.

type SessionExpiredHandler = () => void

let handler: SessionExpiredHandler | null = null

/** Register the handler that reacts to a 401 (session expired). */
export function onSessionExpired(fn: SessionExpiredHandler): void {
  handler = fn
}

/** Called by API clients when the server returns 401. */
export function notifySessionExpired(): void {
  handler?.()
}

/**
 * Thrown by the auth hooks when a refresh attempt fails and the session must
 * be expired. Lets callers (e.g. fetchMe) distinguish "session is gone" from
 * a generic network/connection error.
 */
export class SessionExpiredError extends Error {
  constructor() {
    super("Session expired")
    this.name = "SessionExpiredError"
  }
}
