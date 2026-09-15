// ─── Render-context permission gate ───
//
// `source: entity` context declarations pre-check a permission before fetching
// a record. The check used to ask for `{module}.{entity}.view`, but registry
// permissions are `{module}.{plural}.{action}` (internal/entity/registry.go) —
// so the requirement never matched and every entity context entry silently fell
// back for any caller without `*`. Dev seeds `*`, which is why it went
// unnoticed until a public QR page needed its guest's own table session.
//
// Run with: npx vitest run src/hooks/useRenderContext.test.ts

import { describe, it, expect } from "vitest"
import { entityViewPermission } from "./useRenderContext"
import { can } from "@/engine/permissions"

describe("entityViewPermission", () => {
  it("uses the registered {module}.{plural}.view form", () => {
    expect(entityViewPermission("cafe-order", "orders")).toBe(
      "cafe-order.orders.view",
    )
    expect(entityViewPermission("cafe-order", "table-sessions")).toBe(
      "cafe-order.table-sessions.view",
    )
  })

  it("matches what a role actually holds", () => {
    // A cashier role holding per-entity grants can read the record…
    expect(
      can(entityViewPermission("cafe-order", "orders"), [
        "cafe-order.orders.view",
      ]),
    ).toBe(true)
    expect(
      can(entityViewPermission("cafe-order", "orders"), [
        "cafe-order.orders.*",
      ]),
    ).toBe(true)
    // …but not another entity's.
    expect(
      can(entityViewPermission("cafe-order", "orders"), [
        "cafe-order.shifts.view",
      ]),
    ).toBe(false)
    // A wildcard admin (dev seed) still passes.
    expect(can(entityViewPermission("cafe-order", "orders"), ["*"])).toBe(true)
  })

  it("denies an anonymous caller — the public-surface skip is what unlocks it", () => {
    // Anonymous permissions are empty, which is exactly why a public page must
    // not run this pre-check (the server decides instead).
    expect(can(entityViewPermission("cafe-order", "table-sessions"), [])).toBe(
      false,
    )
  })

  it("is not fooled by the old singular form", () => {
    // The bug this replaced: `cafe-order.table-session.view` (singular, and a
    // 4-segment string). Pin that it does NOT match a real grant.
    expect(
      can("cafe-order.table-session.view", ["cafe-order.table-sessions.view"]),
    ).toBe(false)
  })
})
