import { describe, it, expect } from "vitest"

import { createLookups } from "./meta"
import type { MetaBundle, Entry, WidgetSpec } from "@/types/manifest"

// Minimal bundle with two widgets that share a bare name in different modules —
// the case gap #16 / item 7.3 is about: a DashboardWidget.ref must be able to
// name the module, or the two collide.
function bundleWithWidgets(): MetaBundle {
  const widget = (module: string, name: string): Entry<WidgetSpec> => ({
    name,
    module,
    spec: { title: name, type: "metric" } as WidgetSpec,
  })
  return {
    widgets: [
      widget("cafe-report", "omzet-hari-ini"),
      widget("other-module", "omzet-hari-ini"),
    ],
    entities: [],
    pages: [],
    forms: [],
    tables: [],
    dashboards: [],
    reports: [],
    wizards: [],
    kanbans: [],
    timelines: [],
    prints: [],
    themes: [],
    listings: [],
  } as unknown as MetaBundle
}

describe("createLookups — widget refs are module-qualified (gap #16 / 7.3)", () => {
  it("resolves a widget by module/name", () => {
    const lookups = createLookups(bundleWithWidgets())
    const w = lookups.widgets.get("cafe-report/omzet-hari-ini")
    expect(w?.module).toBe("cafe-report")
  })

  it("resolves a widget by module.name", () => {
    const lookups = createLookups(bundleWithWidgets())
    const w = lookups.widgets.get("other-module.omzet-hari-ini")
    expect(w?.module).toBe("other-module")
  })

  it("still resolves a bare name (backward compatible)", () => {
    const lookups = createLookups(bundleWithWidgets())
    expect(lookups.widgets.get("omzet-hari-ini")).toBeDefined()
  })
})

// A bundle whose caller can see no entity used to arrive with `entities: null`
// (the server left the field nil, so it serialized as `null`), and
// `for (const e of bundle.entities)` threw "e.entities is not iterable" — the
// ErrorBoundary turned that into a dead panel instead of an empty list.
//
// Observed on kafe as role `dapur` opening the owner dashboard: the dashboard
// renders metrics through getWidget → createLookups, so the crash took out a
// page that had nothing to do with entities. The server now always sends `[]`
// (internal/ui/meta.go initializes Entities), and createLookups must stay alive
// even if it does not.
describe("createLookups — a null entities list must not throw", () => {
  const base = bundleWithWidgets()

  it("survives entities: null", () => {
    const bundle = { ...base, entities: null } as unknown as MetaBundle
    expect(() => createLookups(bundle)).not.toThrow()
    const lookups = createLookups(bundle)
    expect(lookups.entitiesByKey.size).toBe(0)
    expect(lookups.entitiesByPlural.size).toBe(0)
  })

  it("survives entities: undefined", () => {
    const bundle = { ...base, entities: undefined } as unknown as MetaBundle
    expect(() => createLookups(bundle)).not.toThrow()
  })

  it("still indexes entities when they are present", () => {
    const bundle = {
      ...base,
      entities: [{ module: "billing", name: "order", plural: "orders" }],
    } as unknown as MetaBundle
    const lookups = createLookups(bundle)
    expect(lookups.entitiesByPlural.get("billing/orders")).toBeTruthy()
  })
})
