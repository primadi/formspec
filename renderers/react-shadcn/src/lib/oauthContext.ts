// ─── OAuth context-required fragment ───
//
// When a provider round-trip ends with the caller holding SEVERAL session
// contexts, the backend refuses to pick one (backend §8.7) and redirects to the
// App's login with the choices in the fragment. The round-trip has no step where
// a choice could be collected, so the flow has to come back here, ask, and then
// resume carrying the answer.
//
// Parsing lives in its own module because the fragment is stringly-typed input
// from a redirect: it can be truncated, empty, or contain parameters meant for
// another flow. Every one of those must degrade to "no picker" rather than to a
// blank login screen.

import type { ContextChoice } from "@/types/manifest"

export interface OAuthContextResume {
  /** Provider whose flow is being resumed. */
  provider: string
  /** The contexts the caller may act in, from the repeated `c` parameters. */
  choices: ContextChoice[]
}

/**
 * Parse a location hash carrying `oauth=context_required`.
 *
 * Returns null for every other case — a missing hash, another `oauth=` value
 * (error / email_unverified / link_required), or no choices. Each of those has
 * its own handling elsewhere, and showing a picker with nothing in it would be
 * worse than showing none.
 *
 * `c` is repeated rather than delimited: a context VALUE is opaque (a branch
 * code, an outlet id), so any delimiter we chose could also appear inside one.
 */
export function parseOAuthContextHash(hash: string): OAuthContextResume | null {
  const raw = hash.replace(/^#/, "")
  if (!raw) return null
  const params = new URLSearchParams(raw)
  if (params.get("oauth") !== "context_required") return null

  const provider = params.get("provider") ?? ""
  const choices: ContextChoice[] = []
  for (const id of params.getAll("c")) {
    // The id is `<role>@<value>` (auth.Assignment.ID). Splitting on the FIRST
    // `@` is what the server does, and a role cannot contain one — so this
    // cannot mis-split a value that happens to contain `@`.
    const at = id.indexOf("@")
    if (at <= 0) continue
    choices.push({
      id,
      role: id.slice(0, at),
      // The dimension is not carried in the fragment: the picker labels a choice
      // `role · value`, and the server re-validates the id when it is used.
      dimension: "",
      value: id.slice(at + 1),
    })
  }
  if (!provider || choices.length === 0) return null
  return { provider, choices }
}

/**
 * The authorize URL that resumes a flow with the chosen context. The choice
 * travels as a query parameter, which the backend stores in the OAuth `state` —
 * so it survives the provider round-trip and is applied when the login finishes.
 */
export function oauthAuthorizeURL(
  workspace: string,
  app: string,
  provider: string,
  assignment: string,
): string {
  const q = new URLSearchParams()
  if (app) q.set("app", app)
  if (assignment) q.set("assignment", assignment)
  const qs = q.toString()
  return `/${workspace}/_ui/auth/oauth/${provider}/authorize${qs ? `?${qs}` : ""}`
}
