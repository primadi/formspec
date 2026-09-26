// ─── Auth API ───
//
// Username/password login against the UI surface (/_ui/auth/login) — the
// always-available surface for UI sessions. /api/v1/auth is opt-in for
// programmatic clients (EnableAPIAuth). Returns the JWT access token used to
// authenticate the UI session.

import ky, { HTTPError } from "ky"

import { buildFormaApiError, errorEnvelope } from "./errors"

export interface LoginResult {
  accessToken: string
  refreshToken: string
}

/**
 * Log in with username/password and return the JWT token pair.
 * `app` scopes the session to one App (role management is per-App); empty =
 * workspace-level session (e.g. the _admin surface).
 *
 * `assignment` is the session context (`<role>@<value>`) the caller wants to
 * act in. Omit it on the first attempt: the server picks it automatically when
 * the principal has exactly one, and answers 409 `CONTEXT_REQUIRED` with the
 * `choices` list when there are several (backend §8.7). A caller holding a
 * remembered choice sends it right away.
 *
 * Throws a `FormaApiError` carrying the server's `code`, `message`, `details`
 * and (for 409) `choices` — never a raw ky error, so callers can tell "wrong
 * password" from "choose a context" without touching ky types.
 */
export async function loginWithPassword(
  workspace: string,
  username: string,
  password: string,
  app?: string,
  assignment?: string,
): Promise<LoginResult> {
  try {
    const response = await ky.post(`/${workspace}/_ui/auth/login`, {
      json: {
        username,
        password,
        ...(app ? { app } : {}),
        ...(assignment ? { assignment } : {}),
      },
      retry: 0,
    })
    const body = (await response.json()) as {
      data: { access_token: string; refresh_token: string }
    }
    return {
      accessToken: body.data.access_token,
      refreshToken: body.data.refresh_token,
    }
  } catch (err) {
    if (err instanceof HTTPError) {
      // Read the pre-parsed envelope off `error.data` — ky has already
      // consumed the body to populate it, so `err.response.clone().json()`
      // throws "Response body is already used" and masks the server message.
      throw buildFormaApiError(
        err.response.status,
        err.response.statusText,
        errorEnvelope(err),
      )
    }
    throw err
  }
}
