// ─── filterMenuItem — the `when` axis of menu visibility ───
//
// Pins the split that replaced "the client checks everything":
//   - `permissions:` is RBAC and is enforced SERVER-side (filterMenu in
//     internal/ui/meta.go), so the item never arrives and there is nothing to
//     check here. A client-side RBAC check would be bypassable and would drift
//     from the server silently — that is why filterMenuItem no longer takes a
//     permission list at all.
//   - `when:` is a BUSINESS condition and is evaluated here, because it may
//     depend on the clock (`today()`) and the bundle body is ETag-hashed, so a
//     time-dependent filter server-side would invalidate the cache continuously.
//
// Run with: npx vitest run src/hooks/useResolvedMenu.test.ts

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"

import { filterMenuItem } from "./useResolvedMenu"
import type { MeResponse } from "@/types/manifest"

function makeMe(overrides: Partial<MeResponse> = {}): MeResponse {
  return {
    user_id: "u-1",
    workspace: "demo",
    roles: ["manajer"],
    permissions: ["billing.orders.update"],
    ...overrides,
  }
}

describe("filterMenuItem — `when` business condition", () => {
  it("keeps an item with no `when`", () => {
    expect(filterMenuItem({ route: "/orders" }, makeMe())).toBe(true)
  })

  it("keeps an item whose `when` is true", () => {
    const item = { route: "/orders", when: "user.user_id == 'u-1'" }
    expect(filterMenuItem(item, makeMe())).toBe(true)
  })

  it("hides an item whose `when` is false", () => {
    const item = { route: "/orders", when: "user.user_id == 'someone-else'" }
    expect(filterMenuItem(item, makeMe())).toBe(false)
  })

  it("evaluates `when` against the signed-in identity", () => {
    const item = { route: "/settings", when: "'manajer' in user.roles" }
    expect(filterMenuItem(item, makeMe())).toBe(true)
    expect(filterMenuItem(item, makeMe({ roles: ["kasir"] }))).toBe(false)
  })

  // `today()` was added to the client evaluator for this: without it, `when`
  // could only compare identity, which is exactly the axis that belongs to
  // `permissions` instead. The server-side Starlark evaluator already had a
  // date builtin; the client did not.
  it("supports today() so `when` can express time-based conditions", () => {
    const item = { route: "/promo", when: "today() >= '2000-01-01'" }
    expect(filterMenuItem(item, makeMe())).toBe(true)

    const future = { route: "/promo", when: "today() >= '2999-01-01'" }
    expect(filterMenuItem(future, makeMe())).toBe(false)
  })

  it("treats a non-empty string result as truthy (FormSpecExpr semantics)", () => {
    const item = { route: "/x", when: "user.workspace" }
    expect(filterMenuItem(item, makeMe())).toBe(true)
  })
})

describe("filterMenuItem — failure behaviour (fail-open, deliberately)", () => {
  let errorSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    errorSpy = vi.spyOn(console, "error").mockImplementation(() => {})
  })
  afterEach(() => {
    errorSpy.mockRestore()
  })

  // An expression that cannot be evaluated shows the item and reports. Hiding
  // navigation because of a renderer bug is indistinguishable from "this item
  // legitimately does not exist", and since `when` is not a security boundary
  // (the route and its data stay protected by entity visibility +
  // required_permission), showing it costs nothing.
  it("shows the item when `when` cannot be evaluated, and reports it", () => {
    // A member call the evaluator cannot resolve — the exact shape the live
    // clinic example shipped (`when: "user.has('clinic.settings.update')"`).
    // `formspec check` now rejects it at deploy time (closed callable set), so
    // this path should be unreachable from a validated spec.
    const item = { route: "/settings", when: "user.has('x.y')" }

    expect(filterMenuItem(item, makeMe())).toBe(true)
    expect(errorSpy).toHaveBeenCalledOnce()
    const msg = errorSpy.mock.calls[0][0] as string
    expect(msg).toContain("/settings")
    expect(msg).toContain("could not be evaluated")
  })

  it("shows the item when the expression does not parse", () => {
    const item = { route: "/broken", when: "fields.status = 'open'" }
    expect(filterMenuItem(item, makeMe())).toBe(true)
    expect(errorSpy).toHaveBeenCalledOnce()
  })

  it("evaluates safely with no identity loaded", () => {
    // `me` is null before the session boot resolves. An identity-free condition
    // still decides; an identity-dependent one resolves to null → hidden, and
    // no crash.
    expect(
      filterMenuItem({ route: "/a", when: "today() >= '2000-01-01'" }, null),
    ).toBe(true)
    expect(filterMenuItem({ route: "/b", when: "user.roles" }, null)).toBe(
      false,
    )
  })
})

describe("filterMenuItem — `permissions` is NOT a client concern", () => {
  let errorSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    errorSpy = vi.spyOn(console, "error").mockImplementation(() => {})
  })
  afterEach(() => {
    errorSpy.mockRestore()
  })

  // Guards the split itself: if someone reintroduces a client-side RBAC check
  // here, this fails. The server withholds the item; the client must not have an
  // opinion about `permissions` — two opinions is how the two layers drift.
  it("ignores a `permissions` field on the item", () => {
    const item = {
      route: "/settings",
      permissions: ["billing.settings.update"],
    } as { route: string; when?: string }
    // The caller does NOT hold `billing.settings.update`; the item is still
    // kept, because that decision belongs to the server.
    expect(filterMenuItem(item, makeMe())).toBe(true)
  })
})
