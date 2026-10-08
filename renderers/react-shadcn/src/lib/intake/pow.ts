// ─── Anonymous intake proof-of-work (client) ───
//
// Plan: docs_internal/plan/intake-challenge-pow.md.
//
// When an anonymous surface is under pressure the server answers an opted-in
// action with 403 `CHALLENGE_REQUIRED` and a challenge. The client searches for
// a `solution` where `sha256(token + ":" + solution)` has at least `difficulty`
// leading zero bits, then resends with `X-Forma-Intake: <token>:<solution>`.
//
// The search runs in a Web Worker so the page stays responsive: a solve can be
// seconds of pegged CPU, and freezing the main thread mid-checkout is worse
// than the gate it is paying for. There is no build step for the worker — its
// source is inlined into a Blob, so nothing has to be published as an asset or
// configured in Vite.
//
// The hash is NOT a licence to trust the client: the server verifies the
// solution itself. This code only buys the right to ask again.

import type { IntakeChallenge } from "@/types/manifest"

export const INTAKE_HEADER = "X-Forma-Intake"

/** A solution lives as long as its challenge, minus a small safety margin. */
const SOLUTION_MARGIN_MS = 5_000

interface PendingSolution {
  value: string
  expiresAt: number
}

let pending: PendingSolution | null = null

/** Remember a solved challenge so the retried request can carry it. */
export function setIntakeSolution(
  token: string,
  solution: string,
  ttlSeconds: number,
): void {
  const ttlMs = Math.max(1, ttlSeconds) * 1000 - SOLUTION_MARGIN_MS
  pending = {
    value: `${token}:${solution}`,
    expiresAt: Date.now() + Math.max(1_000, ttlMs),
  }
}

/**
 * Read the current solution, or null when there is none / it has expired.
 *
 * It is NOT consumed on read: one solve legitimately covers the burst that
 * follows it (create then list, say). Expiry is what retires it, and the
 * server refuses an expired token anyway — the failure mode of attaching a
 * stale solution is another challenge, never a bypass.
 */
export function peekIntakeSolution(): string | null {
  if (!pending) return null
  if (Date.now() > pending.expiresAt) {
    pending = null
    return null
  }
  return pending.value
}

/** Drop the current solution (tests, or a hard session reset). */
export function clearIntakeSolution(): void {
  pending = null
}

// The worker's whole source. Kept in sync with the fallback below: both must
// count leading zero bits the same way the Go verifier does (a full byte of
// zeros counts 8, then the leading zeros of the first non-zero byte).
const WORKER_SOURCE = `
function leadingZeroBits(bytes, difficulty) {
  let n = 0
  for (let idx = 0; idx < bytes.length; idx++) {
    const b = bytes[idx]
    if (b === 0) {
      n += 8
      if (n >= difficulty) return n
      continue
    }
    for (let k = 7; k >= 0; k--) {
      if ((b & (1 << k)) === 0) {
        n++
      } else {
        return n
      }
    }
    return n
  }
  return n
}

self.onmessage = async (e) => {
  const { token, difficulty } = e.data
  const enc = new TextEncoder()
  for (let i = 0; ; i++) {
    const solution = String(i)
    const digest = await crypto.subtle.digest("SHA-256", enc.encode(token + ":" + solution))
    if (leadingZeroBits(new Uint8Array(digest), difficulty) >= difficulty) {
      self.postMessage({ solution })
      return
    }
    // Yield periodically so a long search can still be interrupted by the
    // browser terminating the worker.
    if (i % 4096 === 0) await new Promise((r) => setTimeout(r, 0))
  }
}
`

/**
 * Solve a challenge, returning the solution string (without the token).
 *
 * Runs in a Worker when one can be created, and falls back to the main thread
 * otherwise (jsdom, or a environment that blocks Blob workers). The fallback
 * still awaits between batches so the caller's UI is not starved outright.
 */
export async function solveIntakeChallenge(
  challenge: IntakeChallenge,
): Promise<string> {
  const { token, difficulty } = challenge
  if (
    typeof Worker !== "undefined" &&
    typeof crypto !== "undefined" &&
    crypto.subtle
  ) {
    try {
      return await solveInWorker(token, difficulty, WORKER_SOURCE)
    } catch {
      // Fall through to the main-thread path — a worker is an optimization,
      // never a requirement for the gate to work.
    }
  }
  return solveOnMainThread(token, difficulty)
}

function solveInWorker(
  token: string,
  difficulty: number,
  source: string,
): Promise<string> {
  return new Promise((resolve, reject) => {
    const blob = new Blob([source], { type: "application/javascript" })
    const url = URL.createObjectURL(blob)
    const worker = new Worker(url)
    const cleanup = () => {
      worker.terminate()
      URL.revokeObjectURL(url)
    }
    worker.onmessage = (e: MessageEvent<{ solution: string }>) => {
      cleanup()
      resolve(e.data.solution)
    }
    worker.onerror = (err) => {
      cleanup()
      reject(err)
    }
    worker.postMessage({ token, difficulty })
  })
}

async function solveOnMainThread(
  token: string,
  difficulty: number,
): Promise<string> {
  const enc = new TextEncoder()
  for (let i = 0; ; i++) {
    const solution = String(i)
    const digest = await crypto.subtle.digest(
      "SHA-256",
      enc.encode(token + ":" + solution),
    )
    if (
      countLeadingZeroBits(new Uint8Array(digest), difficulty) >= difficulty
    ) {
      return solution
    }
    if (i % 2048 === 0) {
      await new Promise((r) => setTimeout(r, 0))
    }
  }
}

/** Mirror of the worker's counter — see WORKER_SOURCE. */
function countLeadingZeroBits(bytes: Uint8Array, difficulty: number): number {
  let n = 0
  for (let idx = 0; idx < bytes.length; idx++) {
    const b = bytes[idx]
    if (b === 0) {
      n += 8
      if (n >= difficulty) return n
      continue
    }
    for (let k = 7; k >= 0; k--) {
      if ((b & (1 << k)) === 0) {
        n++
      } else {
        return n
      }
    }
    return n
  }
  return n
}
