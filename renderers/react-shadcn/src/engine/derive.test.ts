// ─── Derivation Engine Tests ───
//
// Covers the column derivation fix (5.4.4 / 5.14.1): priority ordering
// (natural key → label_field → status → transaction_date → rest) and the
// guarantee that NO eligible field is ever silently dropped.
//
// Run with: npx vitest run src/engine/derive.test.ts

import { describe, it, expect } from "vitest"
import {
  deriveTable,
  deriveTableColumns,
  DERIVED_TABLE_VISIBLE_COLUMNS,
} from "./derive"
import type { EntitySchema } from "@/types/manifest"

function makeEntity(overrides: Partial<EntitySchema> = {}): EntitySchema {
  return {
    module: "clinic",
    name: "visit",
    plural: "visits",
    label_field: "number",
    fields: [],
    actions: [],
    lifecycle: "two_step_autosave",
    ...overrides,
  }
}

describe("deriveTableColumns priority ordering (5.4.4 / 5.14.1)", () => {
  it("orders natural key → label_field → status → transaction_date → rest", () => {
    const entity = makeEntity({
      label_field: "number",
      state_machine: {
        field: "doc_status",
        initial: "draft",
        states: [{ name: "draft", label: "Draft" }],
        transitions: [],
      },
      fields: [
        { name: "notes", type: "string" },
        { name: "doc_status", type: "enum", enum_values: ["draft"] },
        { name: "number", type: "string", natural_key: true },
        { name: "transaction_date", type: "date" },
        {
          name: "patient_id",
          type: "relation",
          relation: { type: "belongs_to", resource: "patient" },
        },
      ],
    })

    const cols = deriveTableColumns(entity)
    // natural key first
    expect(cols[0].field).toBe("number")
    // label_field second (number already used → next priority tier)
    expect(cols[1].field).toBe("doc_status")
    // status third
    expect(cols[2].field).toBe("transaction_date")
    // transaction_date fourth
    expect(cols[3].field).toBe("notes")
    // relation dot-path expansion
    expect(cols[4].field).toBe("patient.name")
  })

  it("never drops eligible fields — all non-child, non-computed fields present", () => {
    const fields = Array.from({ length: 20 }, (_, i) => ({
      name: `field_${i}`,
      type: "string" as const,
    }))
    const entity = makeEntity({ fields })
    const cols = deriveTableColumns(entity)
    expect(cols).toHaveLength(20)
    // All 20 fields present, none silently dropped
    for (let i = 0; i < 20; i++) {
      expect(cols.some((c) => c.field === `field_${i}`)).toBe(true)
    }
  })

  it("excludes child and computed fields", () => {
    const entity = makeEntity({
      fields: [
        { name: "name", type: "string" },
        { name: "total", type: "decimal", computed: { formula: "1+1" } },
        { name: "items", type: "child", child: { storage: "jsonb" } },
      ],
    })
    const cols = deriveTableColumns(entity)
    expect(cols.map((c) => c.field)).toEqual(["name"])
  })

  it("deriveTable keeps every column in spec.columns (renderer decides visibility)", () => {
    const fields = Array.from({ length: 15 }, (_, i) => ({
      name: `f_${i}`,
      type: "string" as const,
    }))
    const entity = makeEntity({ fields })
    const table = deriveTable(entity)
    expect(table.columns).toHaveLength(15)
    // The renderer shows the first N by default; the rest are expandable.
    expect(DERIVED_TABLE_VISIBLE_COLUMNS).toBe(8)
    expect(table.columns.length).toBeGreaterThan(DERIVED_TABLE_VISIBLE_COLUMNS)
  })
})
