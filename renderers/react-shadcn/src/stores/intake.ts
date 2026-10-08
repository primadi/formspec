// ─── Anonymous intake gate: UI state ───
//
// Plan: docs_internal/plan/intake-challenge-pow.md.
//
// The challenge is solved during an API call (see lib/api/authHooks.ts), but it
// must be VISIBLE: a multi-second pause with no explanation reads as a broken
// page. This store is the one place that says "the surface is busy proving
// itself"; SurfaceShell renders a blocking screen while it is true, mirroring
// the `pendingContext` → SwitchContextScreen pattern (a full-surface early
// return, not a dismissible dialog — a gate the user can close is not a gate).

import { create } from "zustand"

import { solveIntakeChallenge, setIntakeSolution } from "@/lib/intake/pow"
import type { IntakeChallenge } from "@/types/manifest"

interface IntakeState {
  /** True while a challenge is being solved — the surface blocks on it. */
  solving: boolean
  /** Solve a challenge and remember the solution for the retried request. */
  solve: (challenge: IntakeChallenge) => Promise<void>
}

export const useIntakeStore = create<IntakeState>((set) => ({
  solving: false,
  solve: async (challenge) => {
    set({ solving: true })
    try {
      const solution = await solveIntakeChallenge(challenge)
      setIntakeSolution(challenge.token, solution, challenge.ttl_seconds)
    } finally {
      // Cleared in `finally`: a failed solve must not leave the surface blocked
      // forever. The request then re-challenges, which is the honest outcome.
      set({ solving: false })
    }
  },
}))
