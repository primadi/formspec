// ─── Auth Hooks (shared) ───
//
// Shared ky hooks that implement the refresh-token flow for both the entity
// CRUD client (lib/api/client.ts) and the meta client (lib/api/meta.ts):
//
//   - beforeRequest: attach the current access token (read live via getToken,
//     so a refreshed token is picked up on retries).
//   - afterResponse: a 401 on the first attempt forces a retry (ky.retry());
//     a 401 on the retry (fresh token still rejected) expires the session.
//   - beforeRetry: refresh the access token (single-flight via onUnauthorized).
//     If refresh fails, the session is expired and the retry is aborted.
//
// Only forced retries (ky.retry()) are allowed — `shouldRetry` returns false
// so network/5xx errors are NOT retried (preserves the previous `retry: 0`).

import ky from "ky"
import { notifySessionExpired, SessionExpiredError } from "./sessionEvents"
import { buildFormaApiError, type ApiErrorEnvelope } from "./errors"
import { INTAKE_HEADER } from "@/lib/intake/pow"
import type { IntakeChallenge } from "@/types/manifest"

export interface AuthHooksOptions {
  /** Read the current access token (live from the session store). */
  getToken: () => string
  /** Refresh the access token; resolves true when a fresh token is available. */
  onUnauthorized: () => Promise<boolean>
  /**
   * True when the last refresh failure was a 409 `CONTEXT_REQUIRED` rather
   * than a dead session: the credentials are still valid, but the session's
   * assignment (role × branch) was revoked, so the caller must pick a new
   * context (backend §8.7). Expiring the session in that case would throw
   * away a working refresh token and force a full re-login.
   */
  needsContext?: () => boolean
  /**
   * Read the current anonymous intake solution ("<token>:<solution>"), or null
   * when there is none / it has expired. Attached as `X-Forma-Intake`
   * (plan docs_internal/plan/intake-challenge-pow.md). Optional: clients that
   * never touch an anonymous surface do not need it.
   */
  getIntakeSolution?: () => string | null
  /**
   * Called when the server demands a proof-of-work for this request. Solving
   * is asynchronous — it may take seconds of CPU — and the request retries
   * once a solution is available. Optional: without it a 403
   * `CHALLENGE_REQUIRED` surfaces as a normal error, which is the right
   * behaviour for a non-browser client that cannot solve at all.
   */
  onChallengeRequired?: (challenge: IntakeChallenge) => Promise<void>
}

export function createAuthHooks(opts: AuthHooksOptions) {
  return {
    hooks: {
      beforeRequest: [
        ({ request }: { request: Request }) => {
          const token = opts.getToken()
          if (token) {
            request.headers.set("Authorization", `Bearer ${token}`)
          }
          const intake = opts.getIntakeSolution?.()
          if (intake) {
            request.headers.set(INTAKE_HEADER, intake)
          }
        },
      ],
      beforeRetry: [
        async ({ request }: { request: Request }) => {
          // Attach the intake solution BEFORE the refresh branch below: the
          // retry forced by a 403 CHALLENGE_REQUIRED must not be routed through
          // token refresh, which would abort it as a dead session.
          const intake = opts.getIntakeSolution?.()
          if (intake) {
            request.headers.set(INTAKE_HEADER, intake)
            return
          }
          const ok = await opts.onUnauthorized()
          if (!ok) {
            // A 409 CONTEXT_REQUIRED is not an expired session — the caller
            // picks a new boundary and the surface renders that picker from
            // `pendingContext`. Abort the retry either way.
            if (!opts.needsContext?.()) {
              // Refresh genuinely failed — the session is gone. Expire it
              // (login redirect) and abort the retry.
              notifySessionExpired()
            }
            throw new SessionExpiredError()
          }
          const token = opts.getToken()
          if (token) {
            request.headers.set("Authorization", `Bearer ${token}`)
          }
        },
      ],
      afterResponse: [
        async ({
          response,
          retryCount,
        }: {
          response: Response
          retryCount: number
        }) => {
          if (!response.ok) {
            // ky hands this hook a freshly cloned response, so its body is
            // still unread here — unlike `HTTPError.response`, whose body ky
            // consumes to populate `error.data` before throwing (reading that
            // one would throw "Response body is already used").
            const body = (await response.json().catch(() => undefined)) as
              | ApiErrorEnvelope
              | undefined
            if (response.status === 401 && retryCount === 0) {
              // First 401: force a retry — beforeRetry refreshes the token.
              return ky.retry()
            }
            if (response.status === 401) {
              // Retry with a fresh token still rejected — session is gone.
              notifySessionExpired()
            }
            // Anonymous intake gate: the server asks for proof-of-work on a
            // request under pressure. Solve it and retry ONCE — `beforeRetry`
            // puts the solution on the wire. A second challenge (retryCount > 0)
            // falls through to the error rather than looping.
            if (
              response.status === 403 &&
              retryCount === 0 &&
              body?.error?.code === "CHALLENGE_REQUIRED" &&
              body.error.challenge &&
              opts.onChallengeRequired
            ) {
              await opts.onChallengeRequired(body.error.challenge)
              return ky.retry()
            }
            throw buildFormaApiError(response.status, response.statusText, body)
          }
        },
      ],
    },
    retry: { limit: 1, shouldRetry: () => false },
  }
}
