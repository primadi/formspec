// @vitest-environment jsdom
//
// ─── Widget catalog parity (S10 / kafe TODO 1.4) ───
//
// `widget:` is a closed set. Three things must agree, or the vocabulary is
// fiction again:
//
//   1. the JSON Schema enum (what `formspec validate` and the YAML editor see),
//      generated from pkg/spec/widget.go;
//   2. the runtime catalog (this directory's catalog.ts), used by the form
//      renderer's guard;
//   3. what the renderers actually dispatch (a `case` in FormFieldWidget, a
//      branch in renderCellValue) — and what derive.ts emits by default, since
//      a derived widget name must also be a legal explicit one.
//
// Run with: npx vitest run src/widgets/catalog.test.ts

import { describe, it, expect } from "vitest"
import { readFileSync } from "node:fs"
import { fileURLToPath } from "node:url"
import { render, cleanup } from "@testing-library/react"
import {
  FORM_WIDGETS,
  TABLE_CELL_WIDGETS,
  isFormWidget,
  isTableCellWidget,
  formWidgetAliasTarget,
  formWidgetNames,
} from "./catalog"
import { FormFieldWidget } from "@/kinds/form/FormRenderer"

const read = (rel: string) =>
  readFileSync(fileURLToPath(new URL(rel, import.meta.url)), "utf8")

const schema = JSON.parse(read("../../../../schemas/formspec.schema.json")) as {
  $defs: Record<string, { enum?: string[]; type?: string }>
}

/** Extract `case "x":` labels from a renderer source file. */
const caseLabels = (source: string): string[] => {
  const out = new Set<string>()
  for (const m of source.matchAll(/case\s+"([^"]+)":/g)) out.add(m[1])
  return [...out]
}

const formRendererSrc = read("../kinds/form/FormRenderer.tsx")
const renderCellSrc = read("../lib/renderCell.tsx")
const deriveSrc = read("../engine/derive.ts")

describe("widget catalog ↔ generated JSON Schema", () => {
  it("exposes a form widget enum that matches the runtime catalog", () => {
    const enumValues = schema.$defs.FormWidget?.enum
    expect(
      enumValues,
      "$defs/FormWidget.enum is missing from the schema",
    ).toBeDefined()
    expect([...enumValues!].sort()).toEqual([...FORM_WIDGETS].sort())
  })

  it("exposes a table cell widget enum that matches the runtime catalog", () => {
    const enumValues = schema.$defs.TableCellWidget?.enum
    expect(enumValues).toBeDefined()
    expect([...enumValues!].sort()).toEqual([...TABLE_CELL_WIDGETS].sort())
  })

  it("refs the enums from FormField.widget and TableColumn.widget", () => {
    expect(schema.$defs.FormField).toBeDefined()
    const formField = JSON.parse(
      read("../../../../schemas/formspec.schema.json"),
    ) as {
      $defs: Record<string, { properties?: Record<string, { $ref?: string }> }>
    }
    expect(formField.$defs.FormField.properties?.widget?.$ref).toBe(
      "#/$defs/FormWidget",
    )
    expect(formField.$defs.TableColumn.properties?.widget?.$ref).toBe(
      "#/$defs/TableCellWidget",
    )
  })
})

describe("widget catalog ↔ renderer dispatch", () => {
  it("the form router implements every catalogued widget", () => {
    const handled = new Set(caseLabels(formRendererSrc))
    const missing = FORM_WIDGETS.filter((w) => !handled.has(w))
    expect(missing, `no \`case\` for: ${missing.join(", ")}`).toEqual([])
  })

  it("the cell renderer implements every catalogued table widget", () => {
    const handled = new Set(
      [...renderCellSrc.matchAll(/widget\s*===\s*"([^"]+)"/g)].map((m) => m[1]),
    )
    const missing = TABLE_CELL_WIDGETS.filter((w) => !handled.has(w))
    expect(
      missing,
      `renderCellValue has no branch for: ${missing.join(", ")}`,
    ).toEqual([])
  })

  it("derive.ts only emits catalogued widget names", () => {
    // The `formWidget()` switch: every `return "<name>"` must be a catalog
    // member, otherwise a derived form silently uses a name an author cannot
    // write — and the catalog is again fiction.
    const returned = new Set<string>()
    const fn = deriveSrc.slice(deriveSrc.indexOf("function formWidget("))
    for (const m of fn
      .slice(0, fn.indexOf("\n}"))
      .matchAll(/return\s+"([^"]+)"/g)) {
      returned.add(m[1])
    }
    expect(returned.size).toBeGreaterThan(5)
    const unknown = [...returned].filter((w) => !isFormWidget(w))
    expect(
      unknown,
      `derive.ts emits non-catalogued: ${unknown.join(", ")}`,
    ).toEqual([])
  })

  it("the table form-widget aliases are accepted by the cell renderer", () => {
    // `boolean` is the cell renderer's own name (and a form alias) — both
    // facts are intentional, so pin them.
    expect(isTableCellWidget("boolean")).toBe(true)
    expect(isFormWidget("switch")).toBe(true)
  })
})

describe("widget catalog helpers", () => {
  it("classifies names by surface", () => {
    expect(isFormWidget("relation-picker")).toBe(true)
    expect(isFormWidget("badge")).toBe(false) // table-only
    expect(isTableCellWidget("badge")).toBe(true)
    expect(isTableCellWidget("relation-picker")).toBe(false)
  })

  it("rejects typos and unimplemented widgets", () => {
    for (const bad of [
      "relaion-picker",
      "money-input",
      "textinput",
      "toggle",
    ]) {
      expect(isFormWidget(bad), bad).toBe(false)
      expect(isTableCellWidget(bad), bad).toBe(false)
    }
  })

  it("maps legacy type-name aliases to canonical widgets", () => {
    expect(formWidgetAliasTarget("relation")).toBe("relation-picker")
    expect(formWidgetAliasTarget("child")).toBe("child-grid")
    expect(formWidgetAliasTarget("relaion-picker")).toBeUndefined()
  })

  it("lists the closed set for error messages", () => {
    expect(formWidgetNames()).toContain("relation-picker")
    expect(formWidgetNames()).not.toContain("badge")
  })
})

describe("FormFieldWidget — unknown widget is loud, not silent", () => {
  const entityField = {
    name: "status",
    type: "enum",
    enum_values: ["open", "paid"],
    title: "Status",
  } as never

  const renderWidget = (widget: string) =>
    render(
      <FormFieldWidget
        field={{ name: "status", widget } as never}
        entityField={entityField}
        value={undefined}
        readonly={false}
        onChange={() => {}}
      />,
    )

  it("renders a visible error for a widget outside the catalog", () => {
    // The S10 regression: `relaion-picker` used to render a plain TextInput.
    const { container } = renderWidget("relaion-picker")
    expect(container.textContent).toContain("Unknown widget")
    expect(container.textContent).toContain("relaion-picker")
    // Allowed names are surfaced so the author can fix it without the docs.
    expect(container.querySelector("[title]")?.getAttribute("title")).toContain(
      "relation-picker",
    )
    cleanup()
  })

  it("renders a catalogued widget normally", () => {
    const { container } = renderWidget("select")
    expect(container.textContent).not.toContain("Unknown widget")
    cleanup()
  })

  it("still renders the plain input for a field type with no widget", () => {
    // No explicit `widget:` (derived `money`/`time` fields land here) — this
    // stays a text input until MoneyInput/TimeInput exist (known gap #1).
    const { container } = render(
      <FormFieldWidget
        field={{ name: "total" } as never}
        entityField={{ name: "total", type: "money" } as never}
        value={undefined}
        readonly={false}
        onChange={() => {}}
      />,
    )
    expect(container.textContent).not.toContain("Unknown widget")
    expect(container.querySelector("input")).not.toBeNull()
    cleanup()
  })
})
