// @vitest-environment node
//
// View targets (`<kind>:<name>`) — plan docs_internal/plan/print-row-action.md
//
// A TableAction can carry `view` to NAVIGATE instead of calling the entity
// action, which is how renderer builtins with no backing entity action (notably
// `print`) get a UI trigger. These pin the two rules that make it safe:
//
//   1. resolveViewTarget only resolves against the LOADED bundle — a reference
//      to a view the caller cannot see registers no route, so it must not offer
//      a button (a link to nowhere).
//   2. only `print` appends a record id (it is the only kind with a `/:id`
//      route) — a Report/Kanban target would otherwise build a dead path.

import { describe, expect, it } from "vitest"

import {
  canDoTableAction,
  canDoViewTarget,
  parseViewTarget,
  resolveViewTarget,
  viewTargetTakesId,
} from "./viewTarget"
import type {
  EntitySchema,
  Entry,
  MetaBundle,
  PrintSpec,
} from "@/types/manifest"

function bundleWith(over: Partial<MetaBundle> = {}): MetaBundle {
  return {
    app: {} as MetaBundle["app"],
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
    settings: {} as MetaBundle["settings"],
    ...over,
  }
}

const printEntry = (
  module: string,
  name: string,
  entity: string,
): Entry<PrintSpec> => ({ module, name, spec: { entity } })

const entityWith = (over: Partial<EntitySchema> = {}): EntitySchema =>
  ({
    module: "cafe-master",
    name: "dining-table",
    plural: "dining-tables",
    label_field: "code",
    fields: [],
    actions: [],
    ...over,
  }) as unknown as EntitySchema

describe("parseViewTarget", () => {
  it("splits kind and name", () => {
    expect(parseViewTarget("print:table-tent-card")).toEqual({
      kind: "print",
      name: "table-tent-card",
    })
    // Qualified name survives (name may itself contain a slash).
    expect(parseViewTarget("page:cafe-order/pos-workbench")).toEqual({
      kind: "page",
      name: "cafe-order/pos-workbench",
    })
  })

  it("rejects malformed references", () => {
    expect(parseViewTarget(undefined)).toBeUndefined()
    expect(parseViewTarget("")).toBeUndefined()
    expect(parseViewTarget("print")).toBeUndefined() // no colon
    expect(parseViewTarget(":name")).toBeUndefined() // empty kind
    expect(parseViewTarget("print:")).toBeUndefined() // empty name
  })
})

describe("viewTargetTakesId", () => {
  it("is true only for print", () => {
    expect(viewTargetTakesId("print")).toBe(true)
    expect(viewTargetTakesId("report")).toBe(false)
    expect(viewTargetTakesId("kanban")).toBe(false)
  })
})

describe("resolveViewTarget", () => {
  it("resolves a print entry and carries its entity", () => {
    const bundle = bundleWith({
      prints: [
        printEntry(
          "cafe-master",
          "table-tent-card",
          "cafe-master.dining-table",
        ),
      ],
    })
    expect(resolveViewTarget(bundle, "print:table-tent-card")).toEqual({
      kind: "print",
      name: "table-tent-card",
      module: "cafe-master",
      entityRef: "cafe-master.dining-table",
    })
  })

  it("accepts a qualified name", () => {
    const bundle = bundleWith({
      prints: [
        printEntry(
          "cafe-master",
          "table-tent-card",
          "cafe-master.dining-table",
        ),
      ],
    })
    expect(
      resolveViewTarget(bundle, "print:cafe-master/table-tent-card")?.name,
    ).toBe("table-tent-card")
  })

  it("returns undefined for an unknown kind, name, or missing bundle", () => {
    const bundle = bundleWith({
      prints: [
        printEntry(
          "cafe-master",
          "table-tent-card",
          "cafe-master.dining-table",
        ),
      ],
    })
    expect(resolveViewTarget(bundle, "nope:thing")).toBeUndefined()
    expect(resolveViewTarget(bundle, "print:missing")).toBeUndefined()
    expect(resolveViewTarget(null, "print:table-tent-card")).toBeUndefined()
  })
})

describe("canDoViewTarget", () => {
  const bundle = bundleWith({
    prints: [
      printEntry("cafe-master", "table-tent-card", "cafe-master.dining-table"),
    ],
    entities: [entityWith({ authorized_actions: ["list", "find"] })],
  })

  it("refuses a target that does not resolve (never offers a dead link)", () => {
    expect(canDoViewTarget({ permissions: [] }, bundle, "print:missing")).toBe(
      false,
    )
  })

  it("gates on the target entity's view permission", () => {
    // authorized_actions carries `find` (the resource spelling of view).
    expect(
      canDoViewTarget({ permissions: [] }, bundle, "print:table-tent-card"),
    ).toBe(true)
  })

  it("refuses when the target entity's view is not authorized", () => {
    const denied = bundleWith({
      prints: [
        printEntry(
          "cafe-master",
          "table-tent-card",
          "cafe-master.dining-table",
        ),
      ],
      entities: [entityWith({ authorized_actions: ["list"] })],
    })
    expect(
      canDoViewTarget({ permissions: [] }, denied, "print:table-tent-card"),
    ).toBe(false)
  })

  it("allows kinds with no single entity through (server decides)", () => {
    const b = bundleWith({
      approval_inboxes: [
        {
          module: "cafe-order",
          name: "supervisor-inbox",
          spec: {},
        } as Entry<never>,
      ],
    })
    expect(
      canDoViewTarget(
        { permissions: [] },
        b,
        "approval-inbox:supervisor-inbox",
      ),
    ).toBe(true)
  })
})

describe("canDoTableAction", () => {
  const bundle = bundleWith({
    prints: [
      printEntry("cafe-master", "table-tent-card", "cafe-master.dining-table"),
    ],
    entities: [entityWith({ authorized_actions: ["list", "find"] })],
  })
  const entity = entityWith({ authorized_actions: ["list", "find"] })

  it("gates a view action by its target, not the entity action name", () => {
    // `print` is not an entity action — the plain check would deny it.
    expect(
      canDoTableAction({ permissions: [] }, bundle, entity, {
        action: "print",
        view: "print:table-tent-card",
      }),
    ).toBe(true)
  })

  it("falls back to the entity action when there is no view", () => {
    expect(
      canDoTableAction({ permissions: [] }, bundle, entity, { action: "view" }),
    ).toBe(true)
    expect(
      canDoTableAction({ permissions: [] }, bundle, entity, {
        action: "delete",
      }),
    ).toBe(false)
  })
})
