// @vitest-environment node
//
// ─── Anonymous intake challenge ↔ wire contract ───
//
// Plan: docs_internal/plan/intake-challenge-pow.md.
//
// Three things are pinned here, each because it is easy to get quietly wrong:
//
//   1. The 403 body shape. The challenge rides the ERROR ENVELOPE
//      (`error.challenge`) rather than a separate issue endpoint, so a drift in
//      either the Go writer or the TS type would leave the client able to see a
//      403 but unable to solve it — a surface that is gated and unusable.
//   2. `buildFormaApiError` carries the challenge through. It is the ONE place
//      that turns a wire envelope into a typed error, so if the field stops
//      there, nothing downstream can reach it.
//   3. The solution store's expiry. A solution that outlives its challenge would
//      be attached forever and every request would 403 again with no visible
//      reason — the "stale credential" failure mode.

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { buildFormaApiError } from "./errors"
import type { ErrorDetail } from "@/types/manifest"
import {
  clearIntakeSolution,
  peekIntakeSolution,
  setIntakeSolution,
} from "@/lib/intake/pow"

/**
 * A response body shaped exactly like the Go writer in
 * `internal/api/intake.go` (`writeChallengeRequired`). `satisfies` is the
 * assertion: if `ErrorDetail.challenge` / `IntakeChallenge` drift from this
 * shape, `tsc -b` fails on the literal instead of the gate failing at runtime,
 * in production, for anonymous guests only.
 */
type MeasuredChallengeBody = {
  error: ErrorDetail
  meta: { timestamp: string }
}

const measuredEnvelope = {
  error: {
    code: "CHALLENGE_REQUIRED",
    message:
      "prove this request comes from a browser: solve the challenge and resend with the X-Forma-Intake header",
    challenge: {
      token: "eyJtIjoiY2FmZS1vcmRlciJ9.ZmFrZS1zaWc",
      difficulty: 18,
      alg: "sha256",
      ttl_seconds: 90,
    },
  },
  meta: { timestamp: "2026-10-07T12:00:00Z" },
} satisfies MeasuredChallengeBody

describe("intake challenge wire contract", () => {
  it("carries the challenge into the typed error", () => {
    const err = buildFormaApiError(403, "Forbidden", measuredEnvelope)
    expect(err.code).toBe("CHALLENGE_REQUIRED")
    expect(err.challenge).toBeDefined()
    expect(err.challenge?.token).toBe(measuredEnvelope.error.challenge.token)
    expect(err.challenge?.difficulty).toBe(18)
    // `alg` and `ttl_seconds` are part of the contract, not decoration: the
    // client checks the hash name and sizes its solution's lifetime from the TTL.
    expect(err.challenge?.alg).toBe("sha256")
    expect(err.challenge?.ttl_seconds).toBe(90)
  })

  it("leaves challenge absent for every other error", () => {
    const err = buildFormaApiError(422, "Unprocessable", {
      error: { code: "VALIDATION_ERROR", message: "nope" },
    })
    expect(err.challenge).toBeUndefined()
  })
})

describe("intake solution store", () => {
  beforeEach(() => {
    vi.useFakeTimers()
    clearIntakeSolution()
  })
  afterEach(() => {
    clearIntakeSolution()
    vi.useRealTimers()
  })

  it("returns a solution while it is valid", () => {
    setIntakeSolution("tok", "12", 90)
    expect(peekIntakeSolution()).toBe("tok:12")
  })

  it("does not consume on read — a burst may reuse one solve", () => {
    setIntakeSolution("tok", "12", 90)
    expect(peekIntakeSolution()).toBe("tok:12")
    expect(peekIntakeSolution()).toBe("tok:12")
  })

  it("retires a solution once its challenge would have expired", () => {
    setIntakeSolution("tok", "12", 90)
    // The margin keeps the client from sending a token the server already
    // rejects, so expiry lands slightly before the nominal TTL.
    vi.advanceTimersByTime(90_000)
    expect(peekIntakeSolution()).toBeNull()
  })

  it("returns null when nothing has been solved", () => {
    expect(peekIntakeSolution()).toBeNull()
  })
})
