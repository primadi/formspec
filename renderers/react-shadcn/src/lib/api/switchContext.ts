// ─── Session context switch (one path, two callers) ───
//
// `POST /_ui/auth/switch` re-issues a token pair under a different session
// context (backend §8.7). It is called from two places — the screen shown when a
// REFRESH answered 409 `CONTEXT_REQUIRED`, and the switcher in the user menu —
// and the two must behave identically, because the endpoint revokes the previous
// session as part of the switch. Two copies of this call would drift on exactly
// the parts that matter: which token is sent as proof, what happens to the
// remembered choice, and whether the surface reloads (a role switch changes what
// is visible, so a stale bundle would show the old boundary's menu).
//
// Kept out of the components deliberately: the failure modes here are about
// ORDER (revoke-then-issue), not about rendering.

import { writeContextPreference } from "@/lib/session-context"
import { useSessionStore } from "@/stores/session"

export interface SwitchContextArgs {
  workspace: string
  /** The App this session is scoped to — login is per-App. */
  app: string
  /** Proof of the session being replaced; the server revokes it. */
  refreshToken: string
  /** Target context id (`<role>@<value>`) from a `ContextChoice`. */
  assignment: string
}

/**
 * Switch the session context and reload the surface.
 *
 * The reload is not a nicety: permissions and the meta bundle are fetched per
 * session, so a role switch has to re-run boot. It also keeps the two callers
 * honest — neither can forget to refresh what it had cached.
 */
export async function switchSessionContext({
  workspace,
  app,
  refreshToken,
  assignment,
}: SwitchContextArgs): Promise<void> {
  if (!refreshToken) {
    throw new Error("This session cannot switch context — sign in again.")
  }
  const res = await fetch(`/${workspace}/_ui/auth/switch`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refreshToken, assignment, app }),
  })
  if (!res.ok) {
    // The chosen assignment was rejected too (revoked between the two calls).
    // Report it — falling back silently would leave the caller acting in a
    // boundary they did not choose.
    const body = (await res.json().catch(() => null)) as {
      error?: { message?: string }
    } | null
    throw new Error(
      body?.error?.message ?? `Could not switch context (${res.status})`,
    )
  }
  const body = (await res.json()) as {
    data: { access_token: string; refresh_token: string }
  }
  writeContextPreference(workspace, app, assignment)
  useSessionStore.getState().clearPendingContext()
  useSessionStore
    .getState()
    .setSession(workspace, body.data.access_token, body.data.refresh_token, app)
  window.location.reload()
}
