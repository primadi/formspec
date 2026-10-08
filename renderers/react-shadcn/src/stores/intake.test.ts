// @vitest-environment node
//
// ─── Intake gate: UI state (plan intake-challenge-pow.md) ───
//
// `useIntakeStore.solve` is what the API hook awaits when the server answers
// 403 CHALLENGE_REQUIRED. Two properties matter and neither is cosmetic:
//
//   1. `solving` is set while the work runs — `SurfaceShell` blocks on it. If it
//      were never set the guest would see a frozen page instead of an
//      explanation.
//   2. `solving` is cleared in `finally`. A solve that throws must not leave the
//      surface blocked forever; the honest outcome is "the request re-challenges
//      or fails visibly", never a permanently stuck screen.
//
// The solver itself is mocked: this file is about the state machine around it,
// and the hash search is covered on the Go side by the verifier that actually
// decides whether a solution is acceptable.

import { afterEach, describe, expect, it, vi } from "vitest"

vi.mock("@/lib/intake/pow", async () => {
  const actual =
    await vi.importActual<typeof import("@/lib/intake/pow")>("@/lib/intake/pow")
  return {
    ...actual,
    solveIntakeChallenge: vi.fn(),
  }
})

import {
  solveIntakeChallenge,
  peekIntakeSolution,
  clearIntakeSolution,
} from "@/lib/intake/pow"
import { useIntakeStore } from "@/stores/intake"

const challenge = {
  token: "tok",
  difficulty: 12,
  alg: "sha256",
  ttl_seconds: 90,
}

describe("useIntakeStore", () => {
  afterEach(() => {
    clearIntakeSolution()
    useIntakeStore.setState({ solving: false })
    vi.mocked(solveIntakeChallenge).mockReset()
  })

  it("stores the solution and stops blocking when the solve succeeds", async () => {
    vi.mocked(solveIntakeChallenge).mockResolvedValue("42")

    const promise = useIntakeStore.getState().solve(challenge)
    // `solving` is set synchronously so the very first render after the 403 can
    // already show the challenge screen.
    expect(useIntakeStore.getState().solving).toBe(true)

    await promise
    expect(useIntakeStore.getState().solving).toBe(false)
    expect(peekIntakeSolution()).toBe("tok:42")
  })

  it("stops blocking even when the solve throws", async () => {
    vi.mocked(solveIntakeChallenge).mockRejectedValue(new Error("no worker"))

    await expect(useIntakeStore.getState().solve(challenge)).rejects.toThrow()
    expect(useIntakeStore.getState().solving).toBe(false)
    expect(peekIntakeSolution()).toBeNull()
  })
})
