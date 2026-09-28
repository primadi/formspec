// @vitest-environment node
//
// Which URL does a `submit.call` actually hit? (kafe P3)
//
// This boots a REAL HTTP server and sends the request through ky with the SAME
// prefix shape `createApiClient` uses, because the interesting question is not
// what `serviceCallPath` returns as a string — it is what ky's prefix resolution
// DOES with the `../`.
//
// The failure this guards is silent and total: if `../service/...` resolved to
// `/{ws}/service/...` (dropping `_ui`) or to `/service/...` (dropping the
// workspace slug), every service call would 404 and the guest check-in would
// simply never work — with no type error, no lint error, and nothing to notice
// until someone clicks the button.
//
// Run with: npx vitest run src/lib/serviceCall.test.ts

import { createServer, type Server } from "node:http"
import ky from "ky"
import { afterEach, describe, expect, it } from "vitest"

import { serviceCallPath } from "./serviceCall"

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
    res.end(JSON.stringify({ data: { ok: true } }))
  })
  await new Promise<void>((resolve) => server!.listen(0, "127.0.0.1", resolve))
  const { port } = server!.address() as { port: number }
  return { prefix: `http://127.0.0.1:${port}/kafe/_ui/entity`, seen }
}

describe("serviceCallPath", () => {
  it("builds a prefix-relative path", () => {
    expect(serviceCallPath("cafe-order.table-access.open")).toBe(
      "../service/cafe-order/table-access/open",
    )
  })

  it("resolves against the client prefix to /{ws}/_ui/service/...", async () => {
    const { prefix, seen } = await startRecorder()
    const api = ky.create({ prefix })

    await api.post(serviceCallPath("cafe-order.table-access.open"), {
      json: { qr_token: "JKT-A01-DEMO" },
    })

    expect(seen).toEqual(["/kafe/_ui/service/cafe-order/table-access/open"])
  })

  it("does NOT lose the _ui segment (the silent-404 failure)", async () => {
    const { prefix, seen } = await startRecorder()
    const api = ky.create({ prefix })

    await api.post(serviceCallPath("demo.gateway.open"), { json: {} })

    // Explicitly assert the absence too, so a future "simplification" to an
    // absolute path is caught with a message that says what went wrong.
    expect(seen[0]).not.toBe("/kafe/service/demo/gateway/open")
    expect(seen[0]).not.toBe("/service/demo/gateway/open")
    expect(seen[0]).toBe("/kafe/_ui/service/demo/gateway/open")
  })
})
