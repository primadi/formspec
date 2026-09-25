// ─── Route shapes — one test per path family ───
//
// The router registers routes from FOUR different sources, and they are easy to
// confuse because only one of them is authored:
//
//   A. `kind: Page` (authored)          → `spec.route`            — basePath + route
//   B. derived Page for Form/Table      → `/<module>/form|table/<n>` — server-made
//      (internal/ui/meta.go makeDerivedPage) and shipped in bundle.pages
//   C. derived entity CRUD              → `/<module>/<plural>[/new|/:id[/edit]]`
//   D. overlay (not a route)            → `?action=&form=&mode=` on top of A or C
//
// This file pins the SHAPE of each so a change to buildRoutes' prefixes fails
// here rather than in the browser. Jalur B is the subtle one: the client never
// invents `/<module>/form/<n>` — it renders whatever Page the server put in
// `bundle.pages`, which is why a Form missing from the bundle silently loses its
// stand-alone route while the menu (whose routeExists check looks at FORMS)
// still links to it (todo 5.22.6 ⏸️).
//
// Run with: npx vitest run src/shell/router.shapes.test.tsx

import { describe, it, expect } from "vitest"

import { buildRoutes } from "./router"
import type { EntitySchema, MetaBundle } from "@/types/manifest"

function makeEntity(overrides: Partial<EntitySchema> = {}): EntitySchema {
  return {
    module: "cafe-master",
    name: "promo",
    plural: "promos",
    label_field: "name",
    fields: [],
    actions: [],
    lifecycle: "plain_crud",
    characteristic: "master",
    authorized_actions: ["list", "find", "create", "update", "delete"],
    ...overrides,
  } as EntitySchema
}

function makeBundle(overrides: Partial<MetaBundle> = {}): MetaBundle {
  return {
    app: { name: "kafe-pos", root_url: "/app/pos" },
    entities: [],
    pages: [],
    forms: [],
    tables: [],
    dashboards: [],
    widgets: [],
    reports: [],
    wizards: [],
    kanbans: [],
    timelines: [],
    menu: [],
    prints: [],
    themes: [],
    listings: [],
    calendars: [],
    approval_inboxes: [],
    notification_centers: [],
    ...overrides,
  } as unknown as MetaBundle
}

/** All registered route paths, in registration order. */
function routePaths(bundle: MetaBundle, basePath = "/kafe/app/pos"): string[] {
  return buildRoutes({ bundle, basePath })
    .map((r) => r.path)
    .filter((p): p is string => !!p)
}

/** Does any registered route match `path` (treating :id as a segment)? */
function matches(
  bundle: MetaBundle,
  path: string,
  basePath = "/kafe/app/pos",
): boolean {
  return routePaths(bundle, basePath).some((p) => {
    const rx = new RegExp(
      "^" +
        p.replace(/[.*+?^${}()|[\]\\]/g, "\\$&").replace(/:id/g, "[^/]+") +
        "$",
    )
    return rx.test(path)
  })
}

describe("Jalur A — authored Page routes come from spec.route", () => {
  it("registers basePath + spec.route", () => {
    const bundle = makeBundle({
      pages: [
        {
          name: "pos-workbench",
          module: "cafe-order",
          spec: { route: "/pos", title: "Kasir", blocks: [] },
        },
      ] as never,
    })
    expect(routePaths(bundle)).toContain("/kafe/app/pos/pos")
  })

  it("normalizes a route written without a leading slash", () => {
    const bundle = makeBundle({
      pages: [
        {
          name: "orders",
          module: "cafe-order",
          spec: { route: "orders", title: "Orders", blocks: [] },
        },
      ] as never,
    })
    expect(routePaths(bundle)).toContain("/kafe/app/pos/orders")
  })

  it("skips the home page — it is rendered as the surface index instead", () => {
    // A "/" route would strip to an empty relative path that never matches the
    // splat remainder, so App.tsx renders the home Page as the index. Registering
    // it here too would only add a dead route.
    const bundle = makeBundle({
      pages: [
        {
          name: "home",
          module: "catalog",
          spec: { route: "/", title: "Home", blocks: [] },
        },
      ] as never,
    })
    expect(routePaths(bundle)).not.toContain("/kafe/app/pos/")
  })
})

describe("Jalur B — derived Form/Table Pages arrive as ordinary bundle pages", () => {
  // The server generates these (internal/ui/meta.go makeDerivedPage) with route
  // `/<module>/form/<name>` and `/<module>/table/<name>`, naming them
  // `<name>-page`. The client cannot tell them apart from an authored Page —
  // which is the point: it renders whatever Page the bundle carries.
  it("registers /<module>/form/<name> when the server shipped that Page", () => {
    const bundle = makeBundle({
      pages: [
        {
          name: "promo-form-page",
          module: "cafe-master",
          spec: {
            route: "/cafe-master/form/promo-form",
            title: "promo-form",
            blocks: [{ form: { ref: "promo-form" } }],
          },
        },
      ] as never,
    })
    expect(routePaths(bundle)).toContain(
      "/kafe/app/pos/cafe-master/form/promo-form",
    )
  })

  it("has NO such route when the server suppressed the derived Page", () => {
    // `public: false`, or the Form already referenced by a block elsewhere →
    // no derived Page in the bundle → no route. Pinned because the MENU filter
    // decides from the presence of the Form instead (todo 5.22.6 ⏸️), so this
    // combination produces a menu link to a 404.
    const bundle = makeBundle({ pages: [], forms: [] })
    expect(matches(bundle, "/kafe/app/pos/cafe-master/form/promo-form")).toBe(
      false,
    )
  })
})

describe("Jalur C — derived entity CRUD routes", () => {
  const entity = makeEntity()

  it("registers list, new, detail, and edit under /<module>/<plural>", () => {
    const bundle = makeBundle({ entities: [entity] })
    const paths = routePaths(bundle)
    expect(paths).toContain("/kafe/app/pos/cafe-master/promos")
    expect(paths).toContain("/kafe/app/pos/cafe-master/promos/new")
    expect(paths).toContain("/kafe/app/pos/cafe-master/promos/:id")
    expect(paths).toContain("/kafe/app/pos/cafe-master/promos/:id/edit")
  })

  it("builds the route from the entity's PLURAL, not its name", () => {
    // entity `promo`, plural `promos` → /promos. The menu route
    // (module.yaml `route: /cafe-master/promos`) must match this, and it does
    // because both sides use the plural.
    const bundle = makeBundle({ entities: [entity] })
    expect(routePaths(bundle)).not.toContain("/kafe/app/pos/cafe-master/promo")
  })

  it("omits the list route when list is not authorized", () => {
    const bundle = makeBundle({
      entities: [makeEntity({ authorized_actions: ["find"] })],
    })
    expect(routePaths(bundle)).not.toContain("/kafe/app/pos/cafe-master/promos")
  })

  it("keeps /new registered (as a not-found) when create is denied", () => {
    // `/new` must not be swallowed by `:id` — that would fetch a record
    // literally named "new" and render something plausible.
    const bundle = makeBundle({
      entities: [makeEntity({ authorized_actions: ["list", "find"] })],
    })
    expect(matches(bundle, "/kafe/app/pos/cafe-master/promos/new")).toBe(true)
  })
})

describe("Jalur D — the overlay is a query string, not a route", () => {
  it("adds no route for an overlay action", () => {
    // `?action=create&form=promo-form&mode=drawer` is navigated to by setting
    // search params on the CURRENT route (TableRenderer), and OverlayHost reads
    // them. If this ever becomes a real route, `:id` would have to be given up
    // and the list page would remount on every open.
    const bundle = makeBundle({ entities: [makeEntity()] })
    const paths = routePaths(bundle)
    // The overlay URL is the list route plus a query string — the path itself is
    // unchanged, and nothing named action/mode/form exists as a route.
    expect(paths).toContain("/kafe/app/pos/cafe-master/promos")
    expect(paths.filter((p) => /action|mode|overlay|drawer/.test(p))).toEqual(
      [],
    )
  })
})

describe("route families for the navigation kinds", () => {
  it("registers one route per navigation kind under its prefix", () => {
    const bundle = makeBundle({
      dashboards: [{ name: "d", module: "m" }] as never,
      widgets: [{ name: "w", module: "m" }] as never,
      wizards: [{ name: "z", module: "m" }] as never,
      kanbans: [{ name: "k", module: "m" }] as never,
      timelines: [{ name: "t", module: "m" }] as never,
      reports: [{ name: "r", module: "m" }] as never,
      prints: [{ name: "p", module: "m" }] as never,
      listings: [{ name: "l", module: "m" }] as never,
      calendars: [{ name: "c", module: "m" }] as never,
      approval_inboxes: [{ name: "a", module: "m" }] as never,
      notification_centers: [{ name: "n", module: "m" }] as never,
    })
    const paths = routePaths(bundle)
    for (const p of [
      "/kafe/app/pos/dashboard/d",
      "/kafe/app/pos/widget/w",
      "/kafe/app/pos/wizard/z",
      "/kafe/app/pos/kanban/k",
      "/kafe/app/pos/timeline/t",
      "/kafe/app/pos/report/r",
      "/kafe/app/pos/print/p",
      "/kafe/app/pos/listing/l",
      "/kafe/app/pos/calendar/c",
      "/kafe/app/pos/approval-inbox/a",
      "/kafe/app/pos/notification-center/n",
    ]) {
      expect(paths, `missing route ${p}`).toContain(p)
    }
  })

  it("registers Print twice — with and without an id", () => {
    // PrintRenderer handles both (browse the template with no data, or print a
    // record), so the id-less form is not a leftover.
    const bundle = makeBundle({
      prints: [{ name: "receipt", module: "m" }] as never,
    })
    const paths = routePaths(bundle)
    expect(paths).toContain("/kafe/app/pos/print/receipt")
    expect(paths).toContain("/kafe/app/pos/print/receipt/:id")
  })
})

describe("route resolution vs the menu's expectations", () => {
  it("every route buildRoutes emits is absolute under basePath", () => {
    // The menu concatenates `basePath + item.route`, and `useSurface().surfacePath`
    // builds the same prefix for the app surface. A route that is not absolute
    // under basePath would be unreachable through the sidebar.
    const bundle = makeBundle({
      entities: [makeEntity()],
      pages: [
        {
          name: "home",
          module: "catalog",
          spec: { route: "/pos", title: "POS", blocks: [] },
        },
      ] as never,
    })
    for (const p of routePaths(bundle)) {
      expect(p.startsWith("/kafe/app/pos/")).toBe(true)
    }
  })
})
