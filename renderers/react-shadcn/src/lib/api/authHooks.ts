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
import { FormaApiError, type ErrorResponse } from "@/types/manifest"

export interface AuthHooksOptions {
  /** Read the current access token (live from the session store). */
  getToken: () => string
  /** Refresh the access token; resolves true when a fresh token is available. */
  onUnauthorized: () => Promise<boolean>
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
        },
      ],
      beforeRetry: [
        async ({ request }: { request: Request }) => {
          const ok = await opts.onUnauthorized()
          if (!ok) {
            // Refresh failed — the session is gone. Expire it (login
            // redirect) and abort the retry.
            notifySessionExpired()
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
            if (response.status === 401 && retryCount === 0) {
              // First 401: force a retry — beforeRetry refreshes the token.
              return ky.retry()
            }
            if (response.status === 401) {
              // Retry with a fresh token still rejected — session is gone.
              notifySessionExpired()
            }
            const body = (await response
              .clone()
              .json()
              .catch(() => ({}))) as ErrorResponse
            const err = body?.error
            throw new FormaApiError(
              response.status,
              err?.code ?? "UNKNOWN",
              err?.message ?? response.statusText,
              err?.details,
            )
          }
        },
      ],
    },
    retry: { limit: 1, shouldRetry: () => false },
  }
}
