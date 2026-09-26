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
  entityFieldHelp,
  entityFieldLabel,
  humanizeFieldName,
  resolveForm,
  resolveTable,
  withEntityColumnLabels,
  withEntityFieldDefaults,
} from "./derive"
import type { EntitySchema, Entry, FormSpec, TableSpec } from "@/types/manifest"

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

describe("deriveTableColumns format — money renders as an amount, not JSON", () => {
  // The runtime symptom this locks down: a derived column for a `money` field
  // got no `format`, so `renderCellValue` fell through to `JSON.stringify` and
  // the cell showed `{"amount":"50000","currency":"IDR"}`.
  it("gives a money field `format: currency`", () => {
    const entity = makeEntity({
      fields: [
        { name: "reason", type: "string" },
        { name: "amount", type: "money" },
      ],
    })
    const cols = deriveTableColumns(entity)
    expect(cols.find((c) => c.field === "amount")?.format).toBe("currency")
  })

  // `rules: [min: 0]` is an ordinary lower bound, not a currency declaration —
  // `gl-balance.opening_balance` and `clinic.setting.tax_percent` carry it
  // without being money (05-field-types.md §2 forbids guessing a currency).
  it("leaves a plain decimal unformatted even with a min rule", () => {
    const entity = makeEntity({
      fields: [
        { name: "balance", type: "decimal" },
        {
          name: "tax_percent",
          type: "decimal",
          rules: [{ name: "min", value: 0 }],
        },
      ],
    })
    const cols = deriveTableColumns(entity)
    expect(cols.find((c) => c.field === "balance")?.format).toBeUndefined()
    expect(cols.find((c) => c.field === "tax_percent")?.format).toBeUndefined()
  })

  it("keeps the other format hints intact", () => {
    const entity = makeEntity({
      fields: [
        { name: "at", type: "datetime" },
        { name: "on", type: "date" },
        { name: "ratio", type: "percent" },
      ],
    })
    const cols = deriveTableColumns(entity)
    expect(cols.find((c) => c.field === "at")?.format).toBe("relative")
    expect(cols.find((c) => c.field === "on")?.format).toBe("date")
    expect(cols.find((c) => c.field === "ratio")?.format).toBe("percent")
  })
})

// ─── Field caption fallback: label → entity title → humanised name ───
//
// Regression lock for the kafe `promo-form` symptom: declaring `kind: Form`
// (for field order, sections or `visible_when` — not for labels) silently
// demoted every caption from the entity's `title` ("Minimum Belanja") to the
// raw field name ("min_purchase"). 111 of 161 authored form fields in
// `examples/` declare no `label`, so the fallback is the common path.

describe("entityFieldLabel — caption precedence", () => {
  const entity = makeEntity({
    fields: [
      { name: "min_purchase", type: "money", title: "Minimum Belanja" },
      { name: "menu_item_id", type: "relation", title: "Menu Spesifik" },
      { name: "plain_field", type: "string" },
    ],
  })

  it("prefers an explicit label over the entity title", () => {
    expect(entityFieldLabel(entity, "min_purchase", "Belanja Min.")).toBe(
      "Belanja Min.",
    )
  })

  it("falls back to the entity field title", () => {
    expect(entityFieldLabel(entity, "min_purchase")).toBe("Minimum Belanja")
    expect(entityFieldLabel(entity, "menu_item_id")).toBe("Menu Spesifik")
  })

  it("humanises the name when the field has no title", () => {
    expect(entityFieldLabel(entity, "plain_field")).toBe("Plain Field")
  })

  it("resolves a relation dot-path column via the root or its _id field", () => {
    // Derived columns use `menu_item.name`; authored YAML may use either.
    expect(entityFieldLabel(entity, "menu_item.name")).toBe("Menu Spesifik")
  })

  it("humanises rather than crashing for an unknown field or missing entity", () => {
    expect(entityFieldLabel(entity, "not_a_field")).toBe("Not A Field")
    expect(entityFieldLabel(undefined, "min_purchase")).toBe("Min Purchase")
  })

  it("humanizeFieldName capitalises words and upper-cases a bare id", () => {
    expect(humanizeFieldName("max_uses_per_member")).toBe("Max Uses Per Member")
    expect(humanizeFieldName("id")).toBe("ID")
  })
})

describe("withEntityFieldDefaults — authored forms inherit entity titles", () => {
  const entity = makeEntity({
    fields: [
      { name: "min_purchase", type: "money", title: "Minimum Belanja" },
      { name: "menu_item_id", type: "relation", title: "Menu Spesifik" },
      { name: "time_from", type: "time", title: "Jam Mulai" },
    ],
  })

  it("fills only the fields that declare no label", () => {
    const spec: FormSpec = {
      entity: "clinic.visit",
      sections: [
        {
          title: "Main",
          fields: [
            { name: "min_purchase" },
            { name: "menu_item_id", label: "Menu" },
            { name: "time_from", label: "Jam Mulai" },
          ],
        },
      ],
    }
    const out = withEntityFieldDefaults(spec, entity)
    const labels = out.sections[0].fields.map((f) => f.label)
    expect(labels).toEqual(["Minimum Belanja", "Menu", "Jam Mulai"])
  })

  it("does not mutate the input spec (bundle entries are shared)", () => {
    const spec: FormSpec = {
      entity: "clinic.visit",
      sections: [{ title: "Main", fields: [{ name: "min_purchase" }] }],
    }
    withEntityFieldDefaults(spec, entity)
    expect(spec.sections[0].fields[0].label).toBeUndefined()
  })

  it("resolveForm enriches an authored form instead of returning it raw", () => {
    const authored = new Map<string, Entry<FormSpec>>([
      [
        "promo-form",
        {
          module: "cafe-master",
          name: "promo-form",
          spec: {
            entity: "cafe-master.promo",
            sections: [{ title: "Nilai", fields: [{ name: "min_purchase" }] }],
          },
        } as unknown as Entry<FormSpec>,
      ],
    ])
    const resolved = resolveForm(entity, "create", authored, "promo-form")
    expect(resolved.sections[0].fields[0].label).toBe("Minimum Belanja")
  })

  it("resolveForm still derives when no authored form matches", () => {
    const resolved = resolveForm(entity, "create", new Map())
    expect(resolved.sections[0].fields[0].label).toBe("Minimum Belanja")
  })
})

// ─── help inheritance: Entity `description` → FormField `help` ───
//
// Two vocabularies for one thing: `Field.description` (Entity) and
// `FormField.help` (Form). Only `deriveForm()` used to read the former, so
// declaring `kind: Form` — usually for ordering/sections, NOT for help —
// silently dropped every description. Authors copied them by hand instead:
// `promo-form.yaml` and `promo/entity.yaml` carried the same sentence
// verbatim. These tests pin the shared rule the same way the label tests
// above pin caption precedence.
describe("withEntityFieldDefaults — authored forms inherit entity help", () => {
  const entity = makeEntity({
    description: "ATURAN promo, bukan pemakaian",
    fields: [
      {
        name: "branch_id",
        type: "relation",
        title: "Cabang",
        description: "Kosong = berlaku di semua cabang",
      },
      { name: "priority", type: "integer", title: "Prioritas" },
    ],
  })

  const authored = (fields: FormSpec["sections"][0]["fields"]): FormSpec => ({
    entity: "cafe-master.promo",
    sections: [{ title: "Identitas", fields }],
  })

  it("fills help from the entity field's description", () => {
    const out = withEntityFieldDefaults(authored([{ name: "branch_id" }]), entity)
    expect(out.sections[0].fields[0].help).toBe(
      "Kosong = berlaku di semua cabang",
    )
  })

  it("leaves an explicit help declaration untouched", () => {
    const out = withEntityFieldDefaults(
      authored([{ name: "branch_id", help: "Ditulis sendiri" }]),
      entity,
    )
    expect(out.sections[0].fields[0].help).toBe("Ditulis sendiri")
  })

  it("adds no help when the entity field has no description", () => {
    // `undefined`, not "" — a field with no description must not gain an
    // empty element that the renderer would draw as a blank line.
    const out = withEntityFieldDefaults(authored([{ name: "priority" }]), entity)
    expect(out.sections[0].fields[0].help).toBeUndefined()
  })

  it("fills the first section description from the entity description", () => {
    // This is the drawer/dialog subtitle OverlayHost reads; without it an
    // authored form degrades to "Fill in the details for this promo." even
    // though the entity is described.
    const out = withEntityFieldDefaults(authored([{ name: "priority" }]), entity)
    expect(out.sections[0].description).toBe("ATURAN promo, bukan pemakaian")
  })

  it("does not overwrite a section's own description, nor later sections", () => {
    const spec: FormSpec = {
      entity: "cafe-master.promo",
      sections: [
        { title: "A", description: "Sendiri", fields: [{ name: "priority" }] },
        { title: "B", fields: [{ name: "priority" }] },
      ],
    }
    const out = withEntityFieldDefaults(spec, entity)
    expect(out.sections[0].description).toBe("Sendiri")
    expect(out.sections[1].description).toBeUndefined()
  })

  it("does not mutate the input spec (bundle entries are shared)", () => {
    const spec = authored([{ name: "branch_id" }])
    withEntityFieldDefaults(spec, entity)
    expect(spec.sections[0].fields[0].help).toBeUndefined()
    expect(spec.sections[0].description).toBeUndefined()
  })

  it("keeps an authored field referentially equal when it declares both", () => {
    // resolveForm() runs in a useMemo keyed on the entity; allocating new
    // objects for already-complete fields would churn every render.
    const field = { name: "branch_id", label: "Cabang", help: "Ditulis sendiri" }
    const out = withEntityFieldDefaults(authored([field]), entity)
    expect(out.sections[0].fields[0]).toBe(field)
  })

  it("resolveForm carries help through the authored path", () => {
    const authoredForms = new Map<string, Entry<FormSpec>>([
      [
        "promo-form",
        {
          module: "cafe-master",
          name: "promo-form",
          spec: {
            entity: "cafe-master.promo",
            sections: [{ title: "Nilai", fields: [{ name: "branch_id" }] }],
          },
        } as unknown as Entry<FormSpec>,
      ],
    ])
    const resolved = resolveForm(entity, "create", authoredForms, "promo-form")
    expect(resolved.sections[0].fields[0].help).toBe(
      "Kosong = berlaku di semua cabang",
    )
  })

  it("the derived path applies the same rule (no second spelling)", () => {
    const resolved = resolveForm(entity, "create", new Map())
    const field = resolved.sections[0].fields.find(
      (f) => f.name === "branch_id",
    )
    expect(field?.help).toBe("Kosong = berlaku di semua cabang")
  })
})

describe("entityFieldHelp", () => {
  const entity = makeEntity({
    fields: [
      { name: "promo_id", type: "uuid" },
      { name: "branch_id", type: "relation", description: "Semua cabang" },
    ],
  })

  it("resolves the relation field by its bare name and its `_id` form", () => {
    expect(entityFieldHelp(entity, "branch_id")).toBe("Semua cabang")
    expect(entityFieldHelp(entity, "branch")).toBe("Semua cabang")
  })

  it("returns undefined when there is nothing to show", () => {
    expect(entityFieldHelp(entity, "promo_id")).toBeUndefined()
    expect(entityFieldHelp(entity, "not_a_field")).toBeUndefined()
    expect(entityFieldHelp(undefined, "branch_id")).toBeUndefined()
  })
})

describe("deriveForm widget inference — declared option sets", () => {
  const widgetFor = (field: Record<string, unknown>) => {
    const entity = makeEntity({
      name: "promo",
      plural: "promos",
      fields: [field as never],
    })
    return resolveForm(entity, "create", new Map()).sections[0].fields[0].widget
  }

  it("derives select-multi-tag for a set declared on the Entity", () => {
    // Without this the author must write `widget:` just to avoid a raw JSON
    // editor on a field whose values are a declared set of days.
    expect(
      widgetFor({
        name: "days_of_week",
        type: "json",
        multiple: true,
        options: [
          { value: 1, label: "Senin" },
          { value: 2, label: "Selasa" },
        ],
      }),
    ).toBe("select-multi-tag")
  })

  it("derives a single-value select when the Entity declares one value", () => {
    // The cardinality decision lives on the Entity, so the same `options` list
    // renders a one-value picker — and the derived form and the authored form
    // agree because both call `deriveFormWidget`.
    expect(
      widgetFor({
        name: "day_of_week",
        type: "integer",
        multiple: false,
        options: [{ value: 1, label: "Senin" }],
      }),
    ).toBe("select")
  })

  it("derives a single-value select for a json field declared single", () => {
    expect(
      widgetFor({
        name: "day_of_week",
        type: "json",
        multiple: false,
        options: [{ value: 1, label: "Senin" }],
      }),
    ).toBe("select")
  })

  it("keeps the JSON editor for a json field with no options", () => {
    // No behaviour change for existing specs: free-form JSON stays free-form.
    expect(widgetFor({ name: "payload", type: "json" })).toBe("json")
  })

  it("does not hijack a json field whose options list is empty", () => {
    expect(widgetFor({ name: "payload", type: "json", options: [] })).toBe(
      "json",
    )
  })

  it("keeps the plain input for a string field with no options", () => {
    // A `string` with no choice set is free text — unless `max_length` says it
    // is prose, which stays `textarea`.
    expect(widgetFor({ name: "note", type: "string" })).toBe("input")
    expect(
      widgetFor({ name: "body", type: "string", rules: [{ name: "max_length", value: 500 }] }),
    ).toBe("textarea")
  })
})

describe("withEntityColumnLabels — authored tables inherit entity titles", () => {
  const entity = makeEntity({
    fields: [
      { name: "min_purchase", type: "money", title: "Minimum Belanja" },
      { name: "start_date", type: "date", title: "Mulai Berlaku" },
    ],
  })

  it("fills only the columns that declare no label", () => {
    const cols = withEntityColumnLabels(
      [{ field: "min_purchase" }, { field: "start_date", label: "Mulai" }],
      entity,
    )
    expect(cols.map((c) => c.label)).toEqual(["Minimum Belanja", "Mulai"])
  })

  it("resolveTable enriches authored columns", () => {
    const authored = new Map<string, Entry<TableSpec>>([
      [
        "promo",
        {
          module: "cafe-master",
          name: "promo",
          spec: {
            entity: "cafe-master.promo",
            columns: [{ field: "min_purchase" }],
          },
        } as unknown as Entry<TableSpec>,
      ],
    ])
    const resolved = resolveTable(entity, authored)
    expect(resolved.columns[0].label).toBe("Minimum Belanja")
  })
})
