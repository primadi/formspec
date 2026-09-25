// ─── Derived CRUD route gating ───
//
// Pins the kafe 10.23 fix: the route builder used to register every derived
// CRUD route from the entity's lifecycle alone, so a public App granted only
// `[list, find]` still served a working-looking "Create Menu Category" modal
// whose submit answered 401 (measured in the browser on `kafe-qr`).
//
// Two properties must hold, and the second is the subtle one:
//   1. An unauthorized action gets no working route.
//   2. It must NOT be swallowed by a neighbouring route either — `/new` was
//      captured by `:id` (id literally "new"), which rendered the fallback list
//      and LOOKED like it worked. An explicit not-found is the honest answer.
//
// Run with: npx vitest run src/shell/router.gating.test.tsx

import { describe, it, expect } from "vitest"
import { renderToStaticMarkup } from "react-dom/server"
import { MemoryRouter, Routes, Route } from "react-router-dom"

import { buildRoutes } from "./router"
import type { EntitySchema, MetaBundle } from "@/types/manifest"

// Renderer modules are lazy-loaded and pull in the whole kind layer; the test
// only cares WHICH route matched, so it inspects the matched path instead of
// the element tree.
function makeEntity(overrides: Partial<EntitySchema> = {}): EntitySchema {
  return {
    module: "cafe-master",
    name: "menu-category",
    plural: "menu-categories",
    label_field: "name",
    fields: [],
    actions: [],
    lifecycle: "plain_crud",
    characteristic: "master",
    ...overrides,
  }
}

function makeBundle(entity: EntitySchema): MetaBundle {
  return {
    app: { name: "kafe-qr", root_url: "/" },
    entities: [entity],
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
  } as unknown as MetaBundle
}

/** Which of the builder's routes match `path`? */
function matchedPaths(entity: EntitySchema, path: string): string[] {
  const routes = buildRoutes({ bundle: makeBundle(entity), basePath: "/kafe" })
  return routes
    .map((r) => r.path)
    .filter((p): p is string => !!p)
    .filter((p) => {
      const rx = new RegExp(
        "^" +
          p.replace(/[.*+?^${}()|[\]\\]/g, "\\$&").replace(/:id/g, "[^/]+") +
          "$",
      )
      return rx.test(path)
    })
}

describe("derived CRUD routes follow authorized_actions", () => {
  it("registers every derived route for a fully authorized caller", () => {
    const entity = makeEntity({
      authorized_actions: ["list", "find", "create", "update", "delete"],
    })
    const paths = buildRoutes({
      bundle: makeBundle(entity),
      basePath: "/kafe",
    }).map((r) => r.path)
    for (const p of [
      "/kafe/cafe-master/menu-categories",
      "/kafe/cafe-master/menu-categories/new",
      "/kafe/cafe-master/menu-categories/:id",
      "/kafe/cafe-master/menu-categories/:id/edit",
    ]) {
      expect(paths).toContain(p)
    }
  })

  it("keeps /new on its own route even when create is denied", () => {
    // The whole point: `/new` must not fall through to `:id`. If it did, the
    // matched route would be `:id` and the user would see a record-fetch
    // failure instead of "Page not found".
    const entity = makeEntity({ authorized_actions: ["list", "find"] })
    const matched = matchedPaths(
      entity,
      "/kafe/cafe-master/menu-categories/new",
    )
    expect(matched).toContain("/kafe/cafe-master/menu-categories/new")
  })

  it("still routes an authorized create to /new", () => {
    const entity = makeEntity({
      authorized_actions: ["list", "find", "create"],
    })
    const matched = matchedPaths(
      entity,
      "/kafe/cafe-master/menu-categories/new",
    )
    expect(matched).toContain("/kafe/cafe-master/menu-categories/new")
  })

  it("does not register the list route without list", () => {
    const entity = makeEntity({ authorized_actions: ["find"] })
    const paths = buildRoutes({
      bundle: makeBundle(entity),
      basePath: "/kafe",
    }).map((r) => r.path)
    expect(paths).not.toContain("/kafe/cafe-master/menu-categories")
  })

  it("renders the not-found element for an unauthorized /new", () => {
    const entity = makeEntity({ authorized_actions: ["list", "find"] })
    const routes = buildRoutes({
      bundle: makeBundle(entity),
      basePath: "/kafe",
    })
    const match = routes.find(
      (r) => r.path === "/kafe/cafe-master/menu-categories/new",
    )
    // `Component` is a render-function in these descriptors; narrow it so the
    // build's `tsc -b` stays clean (a type error here fails `make build`).
    const Component = match?.Component as unknown as (
      props: unknown,
    ) => React.ReactElement
    const html = renderToStaticMarkup(
      <MemoryRouter initialEntries={["/kafe"]}>
        <Routes>
          <Route path="/kafe/*" element={<Component />} />
        </Routes>
      </MemoryRouter>,
    )
    expect(html).toContain("Page not found")
    expect(html).not.toContain("New Entry")
  })

  it("keeps the old behaviour when the server did not resolve the set", () => {
    // No `authorized_actions` → older bundle → every route stays registered.
    const entity = makeEntity()
    const paths = buildRoutes({
      bundle: makeBundle(entity),
      basePath: "/kafe",
    }).map((r) => r.path)
    expect(paths).toContain("/kafe/cafe-master/menu-categories/new")
    expect(paths).toContain("/kafe/cafe-master/menu-categories/:id/edit")
  })
})
