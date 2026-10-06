import { describe, expect, it } from "vitest"

import {
  describePredicate,
  grantKey,
  grantsToSelection,
  prunePredicates,
  selectionToGrants,
  type Grant,
  type ScopePredicate,
} from "./grants"

// The grants JSON is NOT schema-validated: it is free JSON on the role entity,
// and the backend resolver SKIPS anything it cannot read (deliberately — kafe
// 10.53). So a wrong key here does not fail loudly; it produces a role that
// looks configured and enforces nothing, or worse, one whose row restriction
// quietly disappeared (the 10.67 measurement). These tests pin the shape.

const pages = [
  {
    page: "order-page",
    actions: [{ name: "list" }, { name: "view" }, { name: "update" }],
    tabs: [],
  },
  {
    page: "sales",
    actions: [],
    tabs: [
      { label: "Order", actions: [{ name: "list" }] },
      { label: "Customer", actions: [{ name: "list" }] },
    ],
  },
]

describe("grant selection round trip", () => {
  it("keeps an action's row scope through grants → selection → grants", () => {
    const grants: Grant[] = [
      {
        page: "order-page",
        actions: [
          {
            name: "list",
            row_scope: [
              { field: "status", op: "in", value: "paid,in_kitchen,ready,served" },
              { field: "branch_id", op: "eq", from: "session" },
            ],
          },
          { name: "view" },
        ],
      },
    ]

    const sel = grantsToSelection(grants)
    expect(sel.keys.has(grantKey("order-page", undefined, "list"))).toBe(true)
    expect(sel.scopes[grantKey("order-page", undefined, "list")]).toHaveLength(2)

    const back = selectionToGrants(pages, sel)
    const listAction = back[0].actions?.find((a) => a.name === "list")
    expect(listAction?.row_scope).toEqual([
      { field: "status", op: "in", value: "paid,in_kitchen,ready,served" },
      { field: "branch_id", op: "eq", from: "session" },
    ])
    // An action with no restriction must NOT carry an empty `row_scope` array:
    // an empty array reads like "restricted to nothing" in the manifest.
    const viewAction = back[0].actions?.find((a) => a.name === "view")
    expect(viewAction).toEqual({ name: "view" })
  })

  it("keeps tabbed pages' actions keyed by tab", () => {
    const grants: Grant[] = [{ page: "sales", tabs: [{ tab: "Customer", actions: [{ name: "list" }] }] }]
    const sel = grantsToSelection(grants)
    expect(sel.keys.has(grantKey("sales", "Customer", "list"))).toBe(true)
    expect(sel.keys.has(grantKey("sales", "Order", "list"))).toBe(false)

    const back = selectionToGrants(pages, sel)
    expect(back).toEqual([{ page: "sales", tabs: [{ tab: "Customer", actions: [{ name: "list" }] }] }])
  })

  it("drops an action whose scope became empty", () => {
    const grants: Grant[] = [
      { page: "order-page", actions: [{ name: "list", row_scope: [{ field: "status", value: "paid" }] }] },
    ]
    const sel = grantsToSelection(grants)
    const predicates: Record<string, ScopePredicate[]> = { [grantKey("order-page", undefined, "list")]: [] }
    const back = selectionToGrants(pages, sel, predicates)
    expect(back[0].actions).toEqual([{ name: "list" }])
  })

  it("emits nothing for a page with no checked action", () => {
    const back = selectionToGrants(pages, { keys: new Set(), scopes: {} })
    expect(back).toEqual([])
  })
})

describe("prunePredicates", () => {
  it("defaults the operator to eq", () => {
    const out = prunePredicates([{ field: "status", op: "", value: "paid" }])
    expect(out).toEqual([{ field: "status", op: "eq", value: "paid" }])
  })

  it("refuses a predicate with no field, and one with no value or source", () => {
    expect(prunePredicates([{ field: "   ", op: "eq", value: "paid" }])).toEqual([])
    // This is the shape that silently filters nothing, so it must never be
    // written to the manifest.
    expect(prunePredicates([{ field: "status", op: "eq" }])).toEqual([])
  })

  it("keeps a zero-like literal, which is a legitimate value", () => {
    const out = prunePredicates([{ field: "is_deleted", op: "eq", value: "0" }])
    expect(out).toEqual([{ field: "is_deleted", op: "eq", value: "0" }])
  })

  it("emits session and route sources without a literal", () => {
    expect(prunePredicates([{ field: "branch_id", op: "eq", from: "session", attr: "branch_id" }])).toEqual([
      { field: "branch_id", op: "eq", attr: "branch_id", from: "session" },
    ])
    expect(prunePredicates([{ field: "guest_token", op: "eq", from: "route", param: "token" }])).toEqual([
      { field: "guest_token", op: "eq", param: "token", from: "route" },
    ])
  })

  it("keeps a session predicate that names no attribute", () => {
    // Legitimate: the backend falls back to the entity's declared `scope.field`
    // (S5), so a bare `from: session` can resolve. The editor must not delete
    // it out from under the operator.
    expect(prunePredicates([{ field: "branch_id", op: "eq", from: "session" }])).toEqual([
      { field: "branch_id", op: "eq", from: "session" },
    ])
  })
})

describe("describePredicate", () => {
  it("reads a literal, a session value and a route value apart", () => {
    expect(describePredicate({ field: "status", op: "in", value: "paid,ready" })).toBe('status in "paid,ready"')
    expect(describePredicate({ field: "branch_id", op: "eq", from: "session", attr: "branch_id" })).toBe(
      "branch_id eq session:branch_id",
    )
    expect(describePredicate({ field: "guest_token", op: "eq", from: "route", param: "token" })).toBe(
      "guest_token eq route:token",
    )
  })
})
