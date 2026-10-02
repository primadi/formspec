// @vitest-environment jsdom
//
// Landing selection — which entity the shell may land on when the App has no
// authored home page and no resolved menu (kafe 10.20).

import { describe, it, expect } from "vitest"
import { canLandOnList, pickLandingEntity } from "./landing"
import type { EntitySchema } from "@/types/manifest"

const ent = (o: Partial<EntitySchema>): EntitySchema =>
  ({ module: "m", name: "n", plural: "ns", ...o }) as EntitySchema

describe("canLandOnList", () => {
  it("is false when the caller only holds `find`", () => {
    // The kafe-qr case: dining-table ships with `find` only, so its list route
    // is never registered — landing there reads "Page not found".
    expect(canLandOnList(ent({ authorized_actions: ["find"] }))).toBe(false)
  })

  it("is true when the caller holds `list`", () => {
    expect(canLandOnList(ent({ authorized_actions: ["list", "find"] }))).toBe(
      true,
    )
  })

  it("keeps legacy behaviour when authorized_actions is unresolved", () => {
    expect(canLandOnList(ent({ authorized_actions: undefined }))).toBe(true)
  })

  it("never lands on a summary entity", () => {
    expect(
      canLandOnList(
        ent({ characteristic: "summary", authorized_actions: ["list"] }),
      ),
    ).toBe(false)
  })

  it("never lands on an entity the App does not expose (routable: false)", () => {
    // registered_views (plan registered-views.md): a non-routable entity has no
    // derived list route, so landing there reads "Page not found".
    expect(
      canLandOnList(ent({ authorized_actions: ["list"], routable: false })),
    ).toBe(false)
  })

  it("keeps legacy behaviour when routable is unresolved", () => {
    expect(canLandOnList(ent({ authorized_actions: ["list"] }))).toBe(true)
  })
})

describe("pickLandingEntity", () => {
  it("skips a find-only entity and returns the first listable one", () => {
    const entities = [
      ent({ name: "dining-table", authorized_actions: ["find"] }),
      ent({ name: "menu-category", authorized_actions: ["list", "find"] }),
    ]
    expect(pickLandingEntity(entities)?.name).toBe("menu-category")
  })

  it("returns undefined when no entity is listable", () => {
    const entities = [ent({ authorized_actions: ["find"] })]
    expect(pickLandingEntity(entities)).toBeUndefined()
  })

  it("handles an empty/absent list", () => {
    expect(pickLandingEntity([])).toBeUndefined()
    expect(pickLandingEntity(undefined)).toBeUndefined()
  })
})
