// ─── Entity-action permission helpers ───
//
// These pin the two helpers the renderer now uses to decide whether an action
// button should be shown at all (todo B, from the 5.12.4 follow-up).
//
// The bug they close: four call sites each spelled out
// `${entity.module}.${entity.plural}.${action}`, which IGNORES an action's
// declared `required_permission` — exactly the case the bundle's `permission`
// field exists to convey. An action with an explicit permission was therefore
// gated against the wrong string (usually showing the button to callers who
// could not use it, and hiding it from callers who could).
//
// Run with: npx vitest run src/engine/permissions.test.ts

import { describe, it, expect } from "vitest"
import { canDoEntityAction, entityActionPermission, can } from "./permissions"
import type { EntitySchema } from "@/types/manifest"

function makeEntity(overrides: Partial<EntitySchema> = {}): EntitySchema {
  return {
    module: "billing",
    name: "order",
    plural: "orders",
    label_field: "number",
    fields: [],
    actions: [],
    lifecycle: "two_step_manual",
    ...overrides,
  }
}

describe("entityActionPermission", () => {
  it("derives module.plural.action when the action declares no permission", () => {
    const entity = makeEntity({
      actions: [{ name: "delete", permission: "billing.orders.delete" }],
    })
    expect(entityActionPermission(entity, "delete")).toBe(
      "billing.orders.delete",
    )
  })

  it("prefers the action's declared permission over the derived form", () => {
    // An action guarded by a custom (non-derived) permission — the case the
    // hand-rolled string got wrong.
    const entity = makeEntity({
      actions: [{ name: "approve", permission: "billing.approvals.approve" }],
    })
    expect(entityActionPermission(entity, "approve")).toBe(
      "billing.approvals.approve",
    )
  })

  it("falls back to the derived form for an action not in the bundle", () => {
    // Custom transitions (DetailPage) are not always present in entity.actions;
    // the derived string is still the right guess.
    const entity = makeEntity({ actions: [] })
    expect(entityActionPermission(entity, "void-order")).toBe(
      "billing.orders.void-order",
    )
  })
})

describe("canDoEntityAction", () => {
  it("fails closed with no identity", () => {
    const entity = makeEntity()
    expect(canDoEntityAction(null, entity, "list")).toBe(false)
    expect(canDoEntityAction(undefined, entity, "list")).toBe(false)
  })

  it("honours the entity's declared permission", () => {
    const entity = makeEntity({
      actions: [{ name: "approve", permission: "billing.approvals.approve" }],
    })
    // Holding only the derived-shaped permission must NOT grant the action.
    expect(
      canDoEntityAction(
        { permissions: ["billing.orders.approve"] },
        entity,
        "approve",
      ),
    ).toBe(false)
    expect(
      canDoEntityAction(
        { permissions: ["billing.approvals.approve"] },
        entity,
        "approve",
      ),
    ).toBe(true)
  })

  it("matches Go wildcard semantics", () => {
    const entity = makeEntity()
    expect(canDoEntityAction({ permissions: ["*"] }, entity, "delete")).toBe(
      true,
    )
    expect(
      canDoEntityAction(
        { permissions: ["billing.orders.*"] },
        entity,
        "delete",
      ),
    ).toBe(true)
    expect(
      canDoEntityAction({ permissions: ["billing.*"] }, entity, "delete"),
    ).toBe(true)
    // A different entity's wildcard must not leak.
    expect(
      canDoEntityAction(
        { permissions: ["billing.customers.*"] },
        entity,
        "delete",
      ),
    ).toBe(false)
  })

  it("agrees with can() for the derived case (no behaviour change for the common path)", () => {
    const entity = makeEntity()
    const perms = ["billing.orders.list"]
    expect(canDoEntityAction({ permissions: perms }, entity, "list")).toBe(
      can(`${entity.module}.${entity.plural}.list`, perms),
    )
  })
})

// ─── authorized_actions: the server-resolved answer ───
//
// Pins the kafe 10.23/10.19 fix. The renderer used to derive EVERY CRUD route
// and action button from the entity's lifecycle, never from the caller's
// authorization: measured on the public `kafe-qr` App (grant
// `[{entity: cafe-master.menu-category, actions: [list, find]}]`), it rendered a
// working-looking "Create Menu Category" modal whose submit answered 401.

describe("canDoEntityAction with authorized_actions", () => {
  it("uses the server's set instead of the caller's permission list", () => {
    // The public-App case: a guest holds NO permissions, yet the grant lets it
    // list and create. Only the resolved set can express that.
    const entity = makeEntity({ authorized_actions: ["list", "create"] })
    const guest = { permissions: [] as string[] }
    expect(canDoEntityAction(guest, entity, "list")).toBe(true)
    expect(canDoEntityAction(guest, entity, "create")).toBe(true)
    expect(canDoEntityAction(guest, entity, "update")).toBe(false)
    expect(canDoEntityAction(guest, entity, "delete")).toBe(false)
  })

  it("treats an empty resolved set as 'nothing allowed'", () => {
    // `[]` is resolved-and-empty, which must NOT fall through to the
    // permission list — otherwise a stale grant could re-enable a button.
    const entity = makeEntity({ authorized_actions: [] })
    expect(
      canDoEntityAction(
        { permissions: ["billing.orders.create"] },
        entity,
        "create",
      ),
    ).toBe(false)
  })

  it("answers the EDIT action from the UPDATE permission", () => {
    // The UI says "edit"; the resource demands `.update`. Spelling the
    // permission from the UI word produced `.edit`, which never exists — a
    // `manajer` holding `.update` lost the Edit button the server would accept.
    const entity = makeEntity({ authorized_actions: ["list", "update"] })
    expect(canDoEntityAction({ permissions: [] }, entity, "edit")).toBe(true)
  })

  it("answers the VIEW action from the FIND permission", () => {
    const entity = makeEntity({ authorized_actions: ["list", "find"] })
    expect(canDoEntityAction({ permissions: [] }, entity, "view")).toBe(true)
  })

  it("falls back to permissions when the server did not resolve the set", () => {
    // An older bundle has no `authorized_actions` — behaviour must not change.
    const entity = makeEntity()
    expect(
      canDoEntityAction(
        { permissions: ["billing.orders.update"] },
        entity,
        "edit",
      ),
    ).toBe(true)
    expect(
      canDoEntityAction(
        { permissions: ["billing.orders.update"] },
        entity,
        "create",
      ),
    ).toBe(false)
  })
})

describe("entityActionPermission vocabulary", () => {
  it("maps the UI word edit to the resource word update", () => {
    expect(entityActionPermission(makeEntity(), "edit")).toBe(
      "billing.orders.update",
    )
  })

  it("maps the UI word view to the resource word find", () => {
    expect(entityActionPermission(makeEntity(), "view")).toBe(
      "billing.orders.find",
    )
  })

  it("still prefers an action's declared permission under either spelling", () => {
    const entity = makeEntity({
      actions: [{ name: "edit", permission: "billing.orders.custom-edit" }],
    })
    expect(entityActionPermission(entity, "edit")).toBe(
      "billing.orders.custom-edit",
    )
  })
})
