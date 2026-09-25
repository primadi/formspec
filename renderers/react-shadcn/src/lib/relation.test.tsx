// @vitest-environment jsdom
//
// ─── Relation cell display (kafe: "mengapa cabang dan bahan masih uuid?") ───
//
// A `belongs_to` relation is stored as a foreign key, but the API also resolves
// the related record onto the row under an alias:
//
//   "branch_id": "01a0bf3d-…", "branch": { "id": "…", "name": "Kafe Senayan" }
//
// Three call sites read that alias and one did not, so an AUTHORED table naming
// the FK (`field: branch_id`) printed the UUID while the name sat unused on the
// same row. These tests pin the resolution and the two spellings that must both
// keep working — a second renderer implementing its own lookup is exactly how
// the omission happened.

import { describe, expect, it } from "vitest"
import { readPath, relationAlias, relationDisplay } from "./relation"
import { resolveColumnCell } from "./renderCell"
import type { EntitySchema } from "@/types/manifest"

const branch: EntitySchema = {
  module: "cafe-master",
  name: "branch",
  plural: "branches",
  label_field: "name",
  lifecycle: "plain_crud",
  fields: [{ name: "name", type: "string" }],
  actions: [],
}

// `label_field` points at a field the record does not carry, so the helper
// must fall back to `name` — the case a bare `label_field: "name"` would hide.
const ingredient: EntitySchema = {
  module: "cafe-stock",
  name: "ingredient",
  plural: "ingredients",
  label_field: "display_name",
  lifecycle: "plain_crud",
  fields: [{ name: "name", type: "string" }],
  actions: [],
}

const stockLevel: EntitySchema = {
  module: "cafe-stock",
  name: "stock-level",
  plural: "stock-levels",
  label_field: "branch_id",
  lifecycle: "plain_crud",
  fields: [
    {
      name: "branch_id",
      type: "relation",
      relation: { type: "belongs_to", resource: "cafe-master.branch" },
    },
    {
      name: "ingredient_id",
      type: "relation",
      relation: { type: "belongs_to", resource: "cafe-stock.ingredient" },
    },
    { name: "quantity_on_hand", type: "decimal", scale: 3 },
  ],
  actions: [],
}

const findEntity = (m: string, n: string) =>
  [branch, ingredient].find((e) => e.module === m && e.name === n)

const row = {
  id: "01a0c859-6b4f-77da-b19c-0631384d66e2",
  branch_id: "01a0bf3d-9ee2-7019-9051-7517d6194be5",
  branch: { id: "01a0bf3d-…", name: "Kafe Senayan", code: "KFE-JKT-01" },
  ingredient_id: "01a0c84f-56d1-7575-a97a-9014219ee23d",
  ingredient: { id: "01a0c84f-…", name: "Beras Putih" },
  quantity_on_hand: 20000,
}

describe("relationAlias", () => {
  it("mirrors the server's rule: strip _id, else use the resource name", () => {
    expect(
      relationAlias({
        name: "branch_id",
        relation: { type: "belongs_to", resource: "cafe-master.branch" },
      } as never),
    ).toBe("branch")
    // A relation field NOT named `_id` uses the resource's own name.
    expect(
      relationAlias({
        name: "parent",
        relation: { type: "belongs_to", resource: "category" },
      } as never),
    ).toBe("category")
  })
})

describe("readPath", () => {
  it("reads a nested dot-path", () => {
    expect(readPath(row, "branch.name")).toBe("Kafe Senayan")
    expect(readPath(row, "missing.deep")).toBeUndefined()
  })
})

describe("relationDisplay", () => {
  it("resolves a _id column to the related record's label", () => {
    // The report: "mengapa cabang dan bahan masih uuid?"
    expect(relationDisplay(row, "branch_id", stockLevel, findEntity)).toBe(
      "Kafe Senayan",
    )
    // label_field unset → falls back to `name`, still not the UUID.
    expect(relationDisplay(row, "ingredient_id", stockLevel, findEntity)).toBe(
      "Beras Putih",
    )
  })

  it("accepts the dot-path spelling too", () => {
    expect(relationDisplay(row, "branch.name", stockLevel, findEntity)).toBe(
      "Kafe Senayan",
    )
  })

  it("returns null for a non-relation column, so callers render normally", () => {
    expect(
      relationDisplay(row, "quantity_on_hand", stockLevel, findEntity),
    ).toBeNull()
  })

  it("falls back to the raw value when the API did not enrich the row", () => {
    // No `branch` object — showing the key beats showing nothing, and it is
    // what the cell did before this fix.
    const bare = { branch_id: "01a0bf3d-9ee2" }
    expect(relationDisplay(bare, "branch_id", stockLevel, findEntity)).toBe(
      "01a0bf3d-9ee2",
    )
  })

  it("prefers label_field, then name/title/code — never an opaque id first", () => {
    // A related record with only a code still shows something actionable.
    const withCode = { id: "x", code: "BHN-01" }
    expect(
      relationDisplay(
        { category_id: "x", category: withCode },
        "category_id",
        {
          ...stockLevel,
          fields: [
            {
              name: "category_id",
              type: "relation",
              relation: { type: "belongs_to", resource: "catalog.category" },
            },
          ],
        } as never,
        () => undefined,
      ),
    ).toBe("BHN-01")
  })
})

describe("resolveColumnCell", () => {
  it("returns the related label for a relation column", () => {
    expect(
      resolveColumnCell(row, "branch_id", stockLevel, findEntity).value,
    ).toBe("Kafe Senayan")
  })

  it("returns the raw value plus the field's own scale for a scalar column", () => {
    // The scale travels with the value so `format: number` can honour it
    // instead of printing the global decimal_scale.
    const cell = resolveColumnCell(
      row,
      "quantity_on_hand",
      stockLevel,
      findEntity,
    )
    expect(cell.value).toBe(20000)
    expect(cell.scale).toBe(3)
  })
})

describe("resolveColumnCell — declared option sets (5.10.15)", () => {
  // The same "value rendered raw in one place, formatted in another" split that
  // produced 10.25/10.26/10.28: a multi-value field printed as JSON in the table
  // while the form showed labelled tags. Resolved in this one function so Table,
  // Listing, and ChildTable cannot drift apart.
  const promo: EntitySchema = {
    module: "cafe-master",
    name: "promo",
    plural: "promos",
    label_field: "name",
    lifecycle: "plain_crud",
    fields: [
      {
        name: "days_of_week",
        type: "json",
        options: [
          { value: 1, label: "Senin" },
          { value: 3, label: "Rabu" },
          { value: 5, label: "Jumat" },
        ],
      },
      { name: "payload", type: "json" },
    ],
    actions: [],
  }
  const noFind = () => undefined

  it("renders declared values as labels, not JSON", () => {
    expect(
      resolveColumnCell({ days_of_week: [1, 3] }, "days_of_week", promo, noFind)
        .value,
    ).toBe("Senin, Rabu")
  })

  it("uses declaration order, not the stored array order", () => {
    expect(
      resolveColumnCell({ days_of_week: [5, 1] }, "days_of_week", promo, noFind)
        .value,
    ).toBe("Senin, Jumat")
  })

  it("shows an undeclared stored value instead of hiding it", () => {
    // Legacy data (or a spec whose options shrank) must stay visible.
    expect(
      resolveColumnCell({ days_of_week: [1, 9] }, "days_of_week", promo, noFind)
        .value,
    ).toBe("Senin, 9")
  })

  it("leaves a json field with no declared options to the raw renderer", () => {
    // Free-form JSON keeps printing as JSON — no behaviour change.
    expect(
      resolveColumnCell({ payload: [1, 2] }, "payload", promo, noFind).value,
    ).toEqual([1, 2])
  })
})

// ── DOM: the reported symptom, end to end ──
//
// The helper tests above would still pass if a renderer forgot to call them
// (which is exactly how the UUID reached the screen). These render the real
// component against a real API payload.

describe("ListingRenderer renders labels and grouped numbers", () => {
  const listingEntity = {
    module: "cafe-stock",
    name: "stock-level",
    plural: "stock-levels",
    label_field: "quantity_on_hand",
    lifecycle: "plain_crud",
    fields: [
      {
        name: "branch_id",
        type: "relation",
        relation: { type: "belongs_to", resource: "cafe-master.branch" },
      },
      { name: "quantity_on_hand", type: "decimal", scale: 3 },
    ],
    actions: [],
    characteristic: "summary",
  }
  const relatedBranch = {
    module: "cafe-master",
    name: "branch",
    plural: "branches",
    label_field: "name",
    lifecycle: "plain_crud",
    fields: [{ name: "name", type: "string" }],
    actions: [],
  }

  it("shows the branch name and a grouped Saldo, not a UUID or 20000", async () => {
    const { useMetaStore } = await import("@/stores/meta")
    const { useSessionStore } = await import("@/stores/session")
    const { MemoryRouter } = await import("react-router-dom")
    const { render, screen } = await import("@testing-library/react")
    const { default: ListingRenderer } =
      await import("@/kinds/listing/ListingRenderer")

    const apiRow = {
      id: "1",
      branch_id: "01a0bf3d-9ee2-7019-9051-7517d6194be5",
      // The API enriches the row — this is the object the cell must use.
      branch: { id: "01a0bf3d-…", name: "Kafe Senayan" },
      quantity_on_hand: 20000,
    }

    useSessionStore.setState({
      workspace: "kafe",
      token: "t",
      me: { user_id: "u", roles: [], permissions: ["*"] } as never,
      getClient: () =>
        ({
          get: () => ({
            json: async () => ({
              data: [apiRow],
              meta: { page: 1, per_page: 20, total: 1, total_pages: 1 },
            }),
          }),
        }) as never,
    })
    useMetaStore.setState({
      bundle: {
        app: { name: "kafe-pos", root_url: "/app", chrome: {} },
        entities: [listingEntity, relatedBranch],
        pages: [],
        forms: [],
        tables: [],
        dashboards: [],
        widgets: [],
        reports: [],
        wizards: [],
        kanbans: [],
        timelines: [],
        prints: [],
        themes: [],
        listings: [],
        settings: {
          currency: { code: "IDR", decimal_places: 0, symbol: "Rp" },
          locale: "id-ID",
          decimal_scale: 2,
        },
      } as never,
    })

    render(
      <MemoryRouter>
        <ListingRenderer
          entry={
            {
              apiVersion: "formspec.dev/v1",
              kind: "Listing",
              name: "stock-level-listing",
              module: "cafe-stock",
              spec: {
                entity: "cafe-stock.stock-level",
                columns: [
                  { field: "branch_id", label: "Cabang" },
                  {
                    field: "quantity_on_hand",
                    label: "Saldo",
                    format: "number",
                  },
                ],
              },
            } as never
          }
        />
      </MemoryRouter>,
    )

    // "mengapa cabang dan bahan masih uuid?" → the label, not the FK.
    expect(await screen.findByText("Kafe Senayan")).toBeTruthy()
    expect(screen.queryByText(apiRow.branch_id)).toBeNull()
    // "mengapa saldo tidak ada pemisah ribuan?" → grouped, locale-aware.
    expect(screen.getByText("20.000")).toBeTruthy()
  })
})
