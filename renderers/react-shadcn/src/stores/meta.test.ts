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
