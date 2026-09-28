// @vitest-environment node
//
// Submit-redirect resolution (kafe P3).
//
// The property that matters most here is the FAILURE direction: an unresolvable
// token must be reported, never navigated to. `interpolateTokens` leaves unknown
// tokens verbatim by design — right for a field value, wrong for an address,
// because `/menu/{response.guest_token}` looks like a working URL and lands the
// caller nowhere with no error.
//
// Run with: npx vitest run src/lib/submitRedirect.test.ts

import { describe, it, expect } from "vitest"

import { resolveSubmitRedirect } from "./submitRedirect"

describe("resolveSubmitRedirect", () => {
  it("resolves {response.*} from the service response", () => {
    const got = resolveSubmitRedirect(
      "/menu/{response.guest_token}",
      {},
      { guest_token: "tok-abc" },
    )
    expect(got).toEqual({ ok: true, target: "/menu/tok-abc" })
  })

  it("resolves a render-context token as before (no regression)", () => {
    const ctx = { route: { params: { qr_token: "JKT-A01-DEMO" } } }
    const got = resolveSubmitRedirect("/t/{route.params.qr_token}", ctx)
    expect(got).toEqual({ ok: true, target: "/t/JKT-A01-DEMO" })
  })

  it("resolves both kinds in one template", () => {
    const ctx = { session: { id: "s-1" } }
    const got = resolveSubmitRedirect(
      "/table/{session.id}/guest/{response.guest_token}",
      ctx,
      { guest_token: "tok-9" },
    )
    expect(got).toEqual({ ok: true, target: "/table/s-1/guest/tok-9" })
  })

  it("passes a token-free path through unchanged", () => {
    const got = resolveSubmitRedirect("/menu", {})
    expect(got).toEqual({ ok: true, target: "/menu" })
  })

  it("FAILS on a {response.*} token the service did not return", () => {
    // The kafe case this exists for: the service answered without the token, so
    // there is no address to go to. Reporting beats navigating to a dead end.
    const got = resolveSubmitRedirect(
      "/menu/{response.guest_token}",
      {},
      { mode: "joined" },
    )
    expect(got.ok).toBe(false)
    if (!got.ok) {
      expect(got.unresolved).toBe("{response.guest_token}")
    }
  })

  it("FAILS when there is no response at all but the template needs one", () => {
    const got = resolveSubmitRedirect("/menu/{response.guest_token}", {})
    expect(got.ok).toBe(false)
  })

  it("FAILS on an unknown render-context token", () => {
    const got = resolveSubmitRedirect("/menu/{route.params.nope}", {
      route: { params: {} },
    })
    expect(got.ok).toBe(false)
    if (!got.ok) {
      expect(got.unresolved).toBe("{route.params.nope}")
    }
  })

  it("does not let the response SHADOW a framework context slot", () => {
    // A service that returns a `route` (or `user`, or `session`) key must not be
    // able to redirect the caller somewhere it chose. The response is namespaced
    // under `response`, never spread, so `{route.params.x}` still resolves from
    // the real context.
    const ctx = { route: { params: { x: "from-context" } } }
    const got = resolveSubmitRedirect("/go/{route.params.x}", ctx, {
      route: { params: { x: "from-response" } },
    })
    expect(got).toEqual({ ok: true, target: "/go/from-context" })
  })

  it("keeps a leading zero in a numeric-looking value", () => {
    // Join codes are digit strings; `004213` must not become `4213`.
    const got = resolveSubmitRedirect(
      "/c/{response.join_code}",
      {},
      {
        join_code: "004213",
      },
    )
    expect(got).toEqual({ ok: true, target: "/c/004213" })
  })
})
