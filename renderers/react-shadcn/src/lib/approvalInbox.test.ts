// @vitest-environment node
//
// Which URL does the approval inbox actually hit? (todo 5.13.6)
//
// This boots a REAL HTTP server and sends the request through ky with the SAME
// prefix shape `createApiClient` uses, because the interesting question is not
// what `approvalListPath` returns as a string — it is what ky's prefix
// resolution DOES with the `../`.
//
// The failure this guards is silent and total: if the path resolved to
// `/{ws}/workflow/approvals` (dropping `_ui`), or to `/workflow/approvals`
// (dropping the workspace slug), the inbox would 404 and render an empty queue —
// no type error, no lint error, nothing to notice until a supervisor swears
// there is a void waiting for them.
//
// Run with: npx vitest run src/lib/approvalInbox.test.ts

import { createServer, type Server } from "node:http"
import ky from "ky"
import { afterEach, describe, expect, it } from "vitest"

import { approvalDecisionPath, approvalListPath } from "./approvalInbox"

let server: Server | undefined

afterEach(() => {
  server?.close()
  server = undefined
})

/** Start a throwaway server that records the request URLs it receives. */
async function startRecorder(): Promise<{ prefix: string; seen: string[] }> {
  const seen: string[] = []
  server = createServer((req, res) => {
    seen.push(req.url ?? "")
    res.setHeader("content-type", "application/json")
    res.end(JSON.stringify({ data: [], meta: {} }))
  })
  await new Promise<void>((resolve) => server!.listen(0, "127.0.0.1", resolve))
  const { port } = server!.address() as { port: number }
  return { prefix: `http://127.0.0.1:${port}/kafe/_ui/entity`, seen }
}

describe("approvalListPath", () => {
  it("resolves against the client prefix to /{ws}/_ui/workflow/approvals", async () => {
    const { prefix, seen } = await startRecorder()
    const api = ky.create({ prefix })

    await api.get(approvalListPath("kafe-pos"))

    expect(seen).toEqual(["/kafe/_ui/workflow/approvals?app=kafe-pos"])
  })

  it("does NOT lose the _ui segment or the workspace slug", async () => {
    const { prefix, seen } = await startRecorder()
    const api = ky.create({ prefix })

    await api.get(approvalListPath())

    // Explicitly assert the absence too, so a future "simplification" to an
    // absolute path is caught with a message that says what went wrong.
    expect(seen[0]).not.toBe("/kafe/workflow/approvals")
    expect(seen[0]).not.toBe("/workflow/approvals")
    expect(seen[0]).toBe("/kafe/_ui/workflow/approvals")
  })
})

describe("approvalDecisionPath", () => {
  it("resolves to /{ws}/_ui/workflow/approvals/{id}", async () => {
    const { prefix, seen } = await startRecorder()
    const api = ky.create({ prefix })

    await api.post(approvalDecisionPath("42"), {
      json: { decision: "approve" },
    })

    expect(seen).toEqual(["/kafe/_ui/workflow/approvals/42"])
  })
})
