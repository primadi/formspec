// ─── Submit redirect resolution ───
//
// Resolving a Form's `submit.redirect` into a navigation target.
//
// Two things make this worth its own module rather than a line in the renderer:
//
//  1. With `submit.call` set, the redirect needs `{response.*}` — the SERVICE
//     RESPONSE. That is the only way to address a value the server decided: on
//     the kafe join flow the token the guest must carry is the EXISTING
//     session's, which the client never knew and could not have guessed.
//  2. An unresolved token must FAIL rather than navigate.
//
// Point 2 is the whole reason for the explicit result type. `interpolateTokens`
// deliberately leaves unknown tokens verbatim — right for a field value (a
// visible placeholder beats a silent blank, which is how "why is branch_id
// empty" becomes a 20-minute hunt) and WRONG for an address. Navigating to
// `/menu/{response.guest_token}` looks like it worked, lands the caller
// nowhere, and reports nothing. The guest sees an empty page instead of an
// error, which is the failure mode this codebase keeps paying for.

import { interpolateTokens } from "@/lib/picker"

export type SubmitRedirectResult =
  | { ok: true; target: string }
  | { ok: false; unresolved: string }

/**
 * Resolve a Form `submit.redirect` template into a navigation target.
 *
 * @param redirect  The authored template, e.g. `/menu/{response.guest_token}`.
 * @param ctx       The form's render context (`route`, `user`, `session`, …).
 * @param response  The SERVICE response, when the submit was a `submit.call`.
 *                  Exposed as `{response.*}` — deliberately NOT spread into
 *                  `ctx`, so a service response cannot shadow a framework slot
 *                  and redirect the caller somewhere it chose.
 * @returns `{ok: true, target}` or `{ok: false, unresolved}` naming the first
 *          token that could not be resolved.
 */
export function resolveSubmitRedirect(
  redirect: string,
  ctx: Record<string, unknown>,
  response?: Record<string, unknown>,
): SubmitRedirectResult {
  const scope: Record<string, unknown> = { ...ctx, response: response ?? {} }
  const target = interpolateTokens(redirect, scope)

  const unresolved = target.match(/\{[\w.]+\}/)
  if (unresolved) {
    return { ok: false, unresolved: unresolved[0] }
  }
  return { ok: true, target }
}
