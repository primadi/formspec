// ─── API Error Helpers ───
//
// ONE place that turns a failed response into a typed error. Both the explicit
// `HTTPError` path (auth.ts, meta.ts, stores/meta.ts) and the `afterResponse`
// hook path (authHooks.ts) go through `buildFormaApiError`, so the wire
// envelope `{error: {code, message, details, choices}}` is interpreted the same
// way everywhere.
//
// ── Why `error.data` and never `error.response.json()` ──
//
// ky v2 populates `HTTPError.data` with the pre-parsed error body BEFORE the
// error is thrown:
//
//   // node_modules/ky/distribution/core/Ky.js
//   httpError.data = await ky.#getResponseData(currentResponse)
//
// `#getResponseData` reads the stream via `response.body.getReader()`, so the
// body is locked/consumed by the time the caller sees the error. Reading it
// again — `error.response.clone().json()` — throws
//
//   TypeError: Failed to execute 'clone' on 'Response': Response body is
//   already used
//
// which then REPLACES the server's message at the call site (the login form
// showed that TypeError instead of "invalid username or password").
// `HTTPError.d.ts` states it plainly: "The response body is automatically
// consumed when populating `error.data`, so `error.response.json()` and other
// body methods will not work. Use `error.data` instead."

import { HTTPError } from "ky"

import {
  FormaApiError,
  type ErrorDetail,
  type ErrorResponse,
} from "@/types/manifest"

/**
 * The wire shape of an error envelope as far as this module cares. Modelled
 * loosely (every field optional) because `error.data` is `unknown` for
 * non-2xx responses that wear a JSON content-type the server did not intend
 * as an envelope.
 */
export interface ApiErrorEnvelope {
  error?: {
    code?: string
    message?: string
    details?: ErrorDetail[]
    choices?: ErrorResponse["error"]["choices"]
  }
}

/**
 * Normalize an already-parsed value into an envelope. Returns `undefined` for
 * anything that is not an object — notably a plain string, which is what ky
 * puts in `error.data` when the response is NOT `application/json`
 * (`#getResponseData` returns the raw text in that case).
 */
export function envelopeOf(data: unknown): ApiErrorEnvelope | undefined {
  if (typeof data !== "object" || data === null) return undefined
  return data as ApiErrorEnvelope
}

/**
 * Read the parsed error envelope off a ky `HTTPError` (via `error.data`).
 * Returns `undefined` for any other error type (network/timeout/abort), which
 * have no response at all.
 */
export function errorEnvelope(err: unknown): ApiErrorEnvelope | undefined {
  if (!(err instanceof HTTPError)) return undefined
  return envelopeOf(err.data)
}

/**
 * Build the typed error from a status + envelope. `statusText` is the fallback
 * message when the server sent no envelope (proxy error, empty body, non-JSON).
 */
export function buildFormaApiError(
  status: number,
  statusText: string,
  body: ApiErrorEnvelope | undefined,
): FormaApiError {
  const err = body?.error
  return new FormaApiError(
    status,
    err?.code ?? "UNKNOWN",
    err?.message ?? statusText,
    err?.details,
    err?.choices,
  )
}

/**
 * Convert any thrown value into a `FormaApiError`. Non-ky errors (network
 * failures, aborts, programming bugs) are returned unchanged so callers keep
 * `instanceof` checks meaningful.
 */
export function toFormaApiError(err: unknown): unknown {
  if (!(err instanceof HTTPError)) return err
  return buildFormaApiError(
    err.response.status,
    err.response.statusText,
    errorEnvelope(err),
  )
}
