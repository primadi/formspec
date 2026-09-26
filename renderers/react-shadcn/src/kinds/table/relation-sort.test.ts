// @vitest-environment jsdom
//
// ─── Relation columns cannot offer a sort the server refuses (todo 5.18.5) ───
//
// `TableColumn.sortable: true` on a relation column promises an order the API
// cannot deliver. Two independent failures, both measured:
//
//   * the column renders the target's display name (`branch.name`) while the
//     stored value is the foreign-key UUID, so ordering would be by UUID;
//   * the dot-path form is rejected outright — `?sort=branch.name` returns
//     `422 unknown field` because `checkField` (internal/api/handler.go) only
//     accepts entity fields and normative columns.
//
// Real manifests declare it: `cafe-stock/stock-level-table.yaml` has
// `{ field: branch_id, sortable: true }` and `{ field: ingredient_id, … }`, and
// the clinic visit table has `{ field: patient.name, sortable: true }`.
//
// The fix withholds the affordance (the option the item sanctions) so the header
// stops lying. Actual server-side ordering by a relation needs a JOIN in the
// PersistBackend — still open, tracked separately.

import { describe, expect, it } from "vitest"
import { readFileSync } from "node:fs"
import { fileURLToPath } from "node:url"
import { deriveTableColumns } from "@/engine/derive"
import type { EntitySchema } from "@/types/manifest"

const read = (rel: string) =>
  readFileSync(fileURLToPath(new URL(rel, import.meta.url)), "utf8")

const tableSrc = read("./TableRenderer.tsx")

const entity = {
  module: "cafe-stock",
  name: "stock-level",
  plural: "stock-levels",
  label_field: "id",
  fields: [
    {
      name: "branch_id",
      type: "relation",
      relation: { type: "belongs_to", resource: "branch" },
    },
    { name: "quantity_on_hand", type: "decimal" },
  ],
} as unknown as EntitySchema

describe("derived columns — relation sort withheld", () => {
  it("marks the relation column unsortable and leaves scalars sortable", () => {
    const cols = deriveTableColumns(entity)
    const relation = cols.find((c) => c.field === "branch.name")
    const scalar = cols.find((c) => c.field === "quantity_on_hand")
    expect(relation).toBeDefined()
    expect(scalar).toBeDefined()
    expect(relation?.sortable).toBe(false)
    // Scoped: the fix must not turn sorting off generally.
    expect(scalar?.sortable).toBe(true)
  })
})

describe("authored tables — the real bug", () => {
  // The derivation path already excluded relations, so the bug lived in tables
  // that WRITE `sortable: true` themselves. Those go through TableRenderer, so
  // the guard there is the one that matters.
  it("resolves the column's root field to decide sortability", () => {
    // `branch.name` is a relation because `branch_id`/`branch` is; checking the
    // literal string would miss both an alias and a bare `patient.name`.
    expect(tableSrc).toMatch(/function isRelationColumn/)
    expect(tableSrc).toMatch(/entity\.fields\.find/)
    expect(tableSrc).toMatch(/`\$\{root\}_id`/)
    expect(tableSrc).toMatch(/field\?\.type === "relation"/)
  })

  it("applies the guard to enableSorting", () => {
    expect(tableSrc).toMatch(
      /enableSorting:\s*\(col\.sortable \?\? true\) && !isRelationColumn\(entity, col\.field\)/,
    )
  })
})
