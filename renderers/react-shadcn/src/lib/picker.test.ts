// ─── Child-field picker — row rules, mapping, templates (S1) ───
//
// The source→row mapping is the contract with the entity: these tests pin which
// row field receives what (including the snapshots). The same rows are what the
// Form submits — the live E2E posts this shape, so a change here that is not
// reflected in the spec shows up as a 422.
//
// Run with: npx vitest run src/lib/picker.test.ts

import { describe, it, expect } from "vitest"
import type { PickerDecl, PickerMap } from "@/types/manifest"
import {
  buildRow,
  buildRows,
  clampQuantity,
  DEFAULT_MAX_QUANTITY,
  interpolateFilter,
  lookupScopeParams,
  interpolateTokens,
  pickRow,
  pickedCount,
  pickedTotal,
  pickerTiles,
  lookupIndex,
  removeRow,
  resolveDefault,
  rowTotal,
  seedDefaults,
  setNote,
  setQuantity,
  type PickedRow,
} from "./picker"

const idr = (amount: string) => ({ amount, currency: "IDR" })
const source = (ref: string, name: string, lookup?: unknown) => ({
  ref,
  name,
  lookup,
})

const map: PickerMap = {
  ref_field: "menu_item_id",
  name_field: "name_snapshot",
  lookup_field: "unit_price_snapshot",
  quantity_field: "quantity",
  note_field: "note",
  max_quantity: 20,
}

describe("picking rows", () => {
  it("adds a row with quantity 1", () => {
    expect(pickRow([], source("m1", "Kopi", idr("25000")), map)).toEqual([
      { ref: "m1", name: "Kopi", lookup: idr("25000"), quantity: 1 },
    ])
  })

  it("increments instead of duplicating when the same source is picked again", () => {
    let rows = pickRow([], source("m1", "Kopi", idr("25000")), map)
    rows = pickRow(rows, source("m1", "Kopi", idr("25000")), map)
    expect(rows).toHaveLength(1)
    expect(rows[0].quantity).toBe(2)
  })

  it("clamps to the declared ceiling", () => {
    let rows: PickedRow[] = []
    for (let i = 0; i < 30; i++)
      rows = pickRow(rows, source("m1", "Kopi", 1000), map)
    expect(rows[0].quantity).toBe(20)
    expect(clampQuantity(0)).toBe(1)
    expect(clampQuantity(-4)).toBe(1)
    expect(clampQuantity(2.7)).toBe(2)
    expect(clampQuantity(1000)).toBe(DEFAULT_MAX_QUANTITY)
  })

  it("treats each pick as its own row when the map declares no quantity field", () => {
    // A checklist is the case: two picks of the same item are two rows.
    const noQuantity: PickerMap = { ref_field: "item_id" }
    let rows = pickRow([], source("i1", "Langkah A"), noQuantity)
    rows = pickRow(rows, source("i1", "Langkah A"), noQuantity)
    expect(rows).toHaveLength(2)
    expect(rows.every((r) => r.quantity === 1)).toBe(true)
  })

  it("removes a row when its quantity drops below 1", () => {
    const rows = pickRow([], source("m1", "Kopi", 1000), map)
    expect(setQuantity(rows, "m1", 0)).toEqual([])
    expect(setQuantity(rows, "m1", -1)).toEqual([])
    expect(removeRow(rows, "m1")).toEqual([])
    // Removing something absent is a no-op, not an error.
    expect(removeRow(rows, "nope")).toHaveLength(1)
  })

  it("sets and clears a per-row note", () => {
    const rows = pickRow([], source("m1", "Kopi", 1000), map)
    expect(setNote(rows, "m1", "tanpa gula")[0].note).toBe("tanpa gula")
    expect(setNote(rows, "m1", "")[0].note).toBe("")
  })
})

describe("picked totals (display only)", () => {
  const rows: PickedRow[] = [
    { ref: "m1", name: "Kopi", lookup: idr("25000"), quantity: 2 },
    { ref: "m2", name: "Roti", lookup: idr("12500.50"), quantity: 1 },
  ]

  it("multiplies money by quantity exactly", () => {
    expect(rowTotal(rows[0])).toBe(50000)
    expect(pickedTotal(rows)).toBe(62500.5)
  })

  it("counts by quantity, not by rows", () => {
    expect(pickedCount(rows)).toBe(3)
  })

  it("does not treat a non-money price as zero-with-meaning", () => {
    expect(
      rowTotal({ ref: "x", name: "X", lookup: "bukan angka", quantity: 2 }),
    ).toBeUndefined()
    expect(
      pickedTotal([{ ref: "x", name: "X", lookup: null, quantity: 2 }]),
    ).toBe(0)
  })
})

describe("row mapping — the entity contract", () => {
  it("writes the declared row fields, snapshots included", () => {
    const row: PickedRow = {
      ref: "menu-1",
      name: "Kopi Susu",
      lookup: idr("25000"),
      quantity: 2,
      note: "tanpa gula",
    }
    expect(buildRow(row, map)).toEqual({
      menu_item_id: "menu-1",
      quantity: 2,
      unit_price_snapshot: idr("25000"),
      name_snapshot: "Kopi Susu",
      note: "tanpa gula",
    })
  })

  it("omits fields the map does not declare", () => {
    const minimal: PickerMap = { ref_field: "item_id" }
    const row = { ref: "i1", name: "Langkah", quantity: 1 } as PickedRow
    expect(buildRow(row, minimal)).toEqual({ item_id: "i1" })
  })

  it("keeps an empty note out of the payload (no empty-string field)", () => {
    const row: PickedRow = { ref: "m", name: "Kopi", quantity: 1, note: "" }
    expect(buildRow(row, map)).not.toHaveProperty("note")
  })

  it("maps a whole pick list", () => {
    const rows = [pickRow([], source("m1", "Kopi", idr("1000")), map)].flat()
    expect(buildRows(rows, map)).toEqual([
      {
        menu_item_id: "m1",
        quantity: 1,
        unit_price_snapshot: idr("1000"),
        name_snapshot: "Kopi",
      },
    ])
  })

  it("never invents totals — they belong to the entity's computed fields", () => {
    const rows = pickRow([], source("m1", "Kopi", idr("1000")), map)
    const built = buildRow(rows[0], map)
    expect(built).not.toHaveProperty("line_total")
    expect(built).not.toHaveProperty("subtotal")
  })
})

describe("templates — default_from and filters", () => {
  const ctx = {
    session: { id: "sess-1", branch_id: "br-1" },
    route: { params: { id: "tok" } },
  }

  it("resolves dotted paths from the render context", () => {
    expect(interpolateTokens("{session.branch_id}", ctx)).toBe("br-1")
    expect(interpolateTokens("cabang-{route.params.id}", ctx)).toBe(
      "cabang-tok",
    )
  })

  it("leaves an unresolvable token verbatim — visible, not silently empty", () => {
    expect(interpolateTokens("{session.missing}", ctx)).toBe(
      "{session.missing}",
    )
    expect(interpolateTokens("{nope}", ctx)).toBe("{nope}")
  })

  it("resolves default_from, including the block-local clock tokens", () => {
    const now = new Date("2026-09-15T07:30:00.000Z")
    expect(resolveDefault("qr_table", ctx, now)).toBe("qr_table")
    expect(resolveDefault("{now}", ctx, now)).toBe("2026-09-15T07:30:00.000Z")
    expect(resolveDefault("{today}", ctx, now)).toBe("2026-09-15")
    expect(resolveDefault("{session.id}", ctx, now)).toBe("sess-1")
    expect(resolveDefault(undefined, ctx, now)).toBeUndefined()
  })

  it("seeds only the fields that declare default_from", () => {
    const now = new Date("2026-09-15T07:30:00.000Z")
    const seeds = seedDefaults(
      [
        { name: "channel", default_from: "qr_table" },
        { name: "branch_id", default_from: "{session.branch_id}" },
        { name: "transaction_date", default_from: "{now}" },
        { name: "guest_note" },
      ],
      ctx,
      now,
    )
    expect(seeds).toEqual({
      channel: "qr_table",
      branch_id: "br-1",
      transaction_date: "2026-09-15T07:30:00.000Z",
    })
  })

  it("interpolates filter values, leaving unresolved ones visible", () => {
    expect(
      interpolateFilter(
        { branch_id: "{session.branch_id}", is_available: "true" },
        ctx,
      ),
    ).toEqual({ branch_id: "br-1", is_available: "true" })
    // An empty value would silently broaden the query; a literal token makes the
    // missing context obvious in the request.
    expect(interpolateFilter({ branch_id: "{session.branch_id}" }, {})).toEqual(
      {
        branch_id: "{session.branch_id}",
      },
    )
    expect(interpolateFilter(undefined, ctx)).toEqual({})
  })

  // A SERVER-enforced lookup scope sends the REFERENCE the server resolves the
  // value from — e.g. the session id already in the route — rather than a
  // client-computed dimension value. The caller states which referenced record
  // it is, never which dimension value it may see.
  describe("lookupScopeParams", () => {
    const routeCtx = { route: { params: { session_id: "sess-42" } } }

    it("sends the route parameter the scope names", () => {
      expect(
        lookupScopeParams(
          [
            {
              field: "branch_id",
              from: "route",
              param: "session_id",
              via: "cafe-order.table-session",
              via_field: "branch_id",
            },
          ],
          routeCtx,
        ),
      ).toEqual({ session_id: "sess-42" })
    })

    it("falls back to the field name when param is omitted", () => {
      expect(
        lookupScopeParams([{ field: "session_id", from: "route" }], routeCtx),
      ).toEqual({ session_id: "sess-42" })
    })

    it("sends nothing when the scope is absent or not route-sourced", () => {
      expect(lookupScopeParams(undefined, routeCtx)).toEqual({})
      expect(lookupScopeParams([], routeCtx)).toEqual({})
      // Literal and session-sourced entries are resolved entirely on the
      // server; the client has no parameter to send for them.
      expect(
        lookupScopeParams(
          [
            { field: "branch_id", from: "session" },
            { field: "is_active", value: "true" },
          ],
          routeCtx,
        ),
      ).toEqual({})
    })

    // An absent parameter must NOT be sent as a literal `{...}` template: the
    // server would then report a reference that cannot resolve, which sends the
    // reader looking for a missing record instead of a missing declaration.
    it("sends nothing when the route parameter is missing", () => {
      expect(
        lookupScopeParams(
          [{ field: "branch_id", from: "route", param: "session_id" }],
          { route: { params: {} } },
        ),
      ).toEqual({})
      expect(
        lookupScopeParams(
          [{ field: "branch_id", from: "route", param: "session_id" }],
          {},
        ),
      ).toEqual({})
    })
  })
})

describe("lookup join — a value read from a related entity", () => {
  const lookupRows = [
    { menu_item_id: "m1", price: idr("25000") },
    { menu_item_id: "m2", price: idr("12500") },
  ]

  it("indexes lookup rows by the declared key", () => {
    const index = lookupIndex(lookupRows, "menu_item_id", "price")
    expect(index.get("m1")).toEqual(idr("25000"))
    expect(index.size).toBe(2)
  })

  it("skips rows without a usable key or value", () => {
    expect(
      lookupIndex(
        [
          { menu_item_id: "", price: 1000 },
          { menu_item_id: "m3" },
          { menu_item_id: "m4", price: null },
        ],
        "menu_item_id",
        "price",
      ).size,
    ).toBe(0)
  })

  it("marks a source row without a looked-up value as not pickable", () => {
    const tiles = pickerTiles({
      rows: [
        { id: "m1", name: "Kopi Susu" },
        { id: "m9", name: "Menu Baru" },
      ],
      nameField: "name",
      valueField: "price",
      lookups: lookupIndex(lookupRows, "menu_item_id", "price"),
    })
    expect(tiles[0].pickable).toBe(true)
    expect(tiles[0].lookup).toEqual(idr("25000"))
    expect(tiles[1].pickable).toBe(false)
    expect(tiles[1].lookup).toBeUndefined()
  })

  it("reads the source row's own field when no lookup is declared", () => {
    const tiles = pickerTiles({
      rows: [{ id: "m1", name: "Kopi", price: idr("9000") }],
      nameField: "name",
      valueField: "price",
    })
    expect(tiles[0].pickable).toBe(true)
    expect(tiles[0].lookup).toEqual(idr("9000"))
  })

  it("does not restrict pickability when the picker reads no value", () => {
    // A checklist picker takes no value at all.
    const tiles = pickerTiles({
      rows: [{ id: "i1", name: "Langkah A" }],
      nameField: "name",
      valueField: "",
    })
    expect(tiles[0].pickable).toBe(true)
  })
})

describe("picker declaration shape (docs parity)", () => {
  it("the kafe order lines picker maps onto the declared child fields", () => {
    // The declaration is generic vocabulary — a source list, an optional lookup
    // that reads one value per row, and a map onto the child's own fields.
    const decl: PickerDecl = {
      entity: "cafe-master.menu-item",
      filter: { is_available: "true" },
      lookup: {
        entity: "cafe-master.menu-item-price",
        key: "menu_item_id",
        field: "price",
        filter: { is_active: "true" },
        scope: [
          {
            field: "branch_id",
            from: "route",
            param: "session_id",
            via: "cafe-order.table-session",
            via_field: "branch_id",
          },
        ],
      },
      display: {
        name_field: "name",
        image_field: "photo",
        category_field: "menu_category_id",
        columns: 3,
        search: true,
      },
      map,
    }
    const row = pickRow([], source("m1", "Kopi Susu", idr("25000")), decl.map)
    expect(buildRow(row[0], decl.map)).toMatchObject({
      menu_item_id: "m1",
      name_snapshot: "Kopi Susu",
      unit_price_snapshot: { amount: "25000", currency: "IDR" },
      quantity: 1,
    })
  })
})
