// @vitest-environment node
//
// ─── Error envelope ↔ wire contract (kafe 10.64) ───
//
// The SPA declared ONE type for two different things:
//
//   ErrorResponse.error.details : ErrorDetail[]
//   ErrorDetail                 = { field?, code: string, message }
//
// The wire never looked like that. The server sends
// `{level, field?, message}` (`internal/api.ErrorDetailItem`, mirrored by
// `pkg/spec.ErrorDetail`) and never sends `code` inside a detail entry — so
// `code` was required by the type and always absent at runtime, and `level`
// was always present and invisible to the compiler.
//
// Nothing caught it because no consumer read `details` yet: every call site
// shows `err.message`. The mismatch would have surfaced at the first per-field
// error UI — precisely the feature `details` exists for — so this pins the
// shape now, while it is still cheap.
//
// Guards here are of two kinds on purpose:
//   1. `satisfies` on a literal that mirrors a REAL measured response — a
//      compile-time assertion (`tsc -b` fails if the type drifts back).
//   2. runtime assertions on `buildFormaApiError`, the single place that turns
//      a wire envelope into a typed error.
//   3. a source-level parity check against `sdk/browser`, the other client in
//      this repo, so the two cannot describe the same wire differently.

import { readFileSync } from "node:fs"

import { describe, expect, it } from "vitest"

import { buildFormaApiError } from "./errors"
import {
  FormaApiError,
  type ErrorDetail,
  type ErrorDetailItem,
} from "@/types/manifest"

/**
 * A response body measured on 2026-10-02 from the kafe dev server, after kafe
 * 10.61 made a caller fault answer 422 + `details[]` instead of 500:
 *
 *   POST /kafe/_ui/entity/cafe-order/shift   {"zzz": 1, …}
 *   → 422 {"error":{"code":"VALIDATION_ERROR","message":"shift insert: unknown
 *          field: \"zzz\"","details":[{"level":"field","field":"zzz",…}]}}
 *
 * `satisfies` is the assertion: if `ErrorDetail`/`ErrorDetailItem` drift away
 * from this shape (a required `code` on the entry, a missing `level`), `tsc -b`
 * fails on this literal rather than a form silently losing the field name.
 *
 * The envelope is typed locally because the `error` member is what this file
 * asserts about — and `satisfies` on an object literal rejects extra
 * properties, so a type that omits `meta` would reject the literal itself.
 */
type MeasuredEnvelope = {
  error: ErrorDetail
  meta: { timestamp: string }
}

/** Just the `error` member, for cases where `meta` is not the subject. */
type MeasuredEnvelopeBase = { error: ErrorDetail }

const measuredResponse = {
  error: {
    code: "VALIDATION_ERROR",
    message: 'shift insert: unknown field: "zzz"',
    details: [
      {
        level: "field",
        field: "zzz",
        message: 'shift insert: unknown field: "zzz"',
      },
    ],
  },
  meta: { timestamp: "2026-10-02T23:59:40Z" },
} satisfies MeasuredEnvelope

/** The entry type on its own — the half that was wrong. */
const measuredEntry = measuredResponse.error
  .details[0] satisfies ErrorDetailItem

describe("error detail contract — the envelope is not the entry", () => {
  it("a measured 422 becomes a typed FormaApiError with the offending field", () => {
    const err = buildFormaApiError(
      422,
      "Unprocessable Entity",
      measuredResponse,
    )

    expect(err).toBeInstanceOf(FormaApiError)
    expect(err.status).toBe(422)
    expect(err.code).toBe("VALIDATION_ERROR")
    expect(err.details).toHaveLength(1)
    // The load-bearing half: the caller can find out WHICH field was rejected
    // without parsing the message string.
    const [detail] = err.details ?? []
    expect(detail?.field).toBe("zzz")
    expect(detail?.level).toBe("field")
    expect(detail?.message).toContain("unknown field")
  })

  it("a detail entry carries no `code` — the type must not demand one", () => {
    // The previous type required `code: string` on every entry while the wire
    // never sends it. Read through a widened view so the assertion documents
    // the wire instead of re-asserting the type it just checked.
    const wireEntry = measuredEntry as Record<string, unknown>
    expect(wireEntry.code).toBeUndefined()
    expect(Object.keys(wireEntry).sort()).toEqual(["field", "level", "message"])
  })

  it("a record-level failure may omit the field", () => {
    // A required-field/record-level error has no field; the type must allow it,
    // or the client would have to invent one.
    const envelope = {
      error: {
        code: "VALIDATION_ERROR",
        message: 'order insert: required field missing: "branch_id"',
        details: [{ level: "record", message: "required field missing" }],
      },
    } satisfies MeasuredEnvelopeBase

    const err = buildFormaApiError(422, "Unprocessable Entity", envelope)
    expect(err.details?.[0]?.field).toBeUndefined()
  })
})

describe("sdk/browser parity", () => {
  // The repo ships two clients. The SPA and the SDK must describe one wire the
  // same way; a divergence would make `details` usable in one and not the
  // other, and the difference would only be noticed by whoever wrote the second
  // consumer. Read as text (not imported) because the SDK is a separate package
  // with its own tsconfig — importing across that boundary would pull it into
  // this build.
  const sdkTypes = readFileSync(
    new URL("../../../../../sdk/browser/src/types.ts", import.meta.url),
    "utf8",
  )

  it("the SDK's ErrorDetailItem declares level/field/message", () => {
    const block = sdkTypes.match(/interface ErrorDetailItem \{([^}]*)\}/)?.[1]
    expect(block, "sdk/browser must declare ErrorDetailItem").toBeTruthy()
    expect(block).toMatch(/level\??:\s*string/)
    expect(block).toMatch(/field\??:\s*string/)
    expect(block).toMatch(/message:\s*string/)
  })

  it("neither client demands a `code` on a detail entry", () => {
    // The regression this guards is exactly "someone re-adds `code` because the
    // envelope above it has one". Calibrated below: the pattern DOES match a
    // drifted snippet, so the assertion is not vacuous.
    const demandsCode = /interface ErrorDetailItem \{([^}]*)\}/
    const block = sdkTypes.match(demandsCode)?.[1] ?? ""
    expect(block).not.toMatch(/\bcode\??:\s*string/)

    const driftedSnippet = `interface ErrorDetailItem {
      field?: string;
      code: string;
      message: string;
    }`
    expect(driftedSnippet).toMatch(
      /interface ErrorDetailItem \{[\s\S]*\bcode\??:\s*string/,
    )
  })
})
