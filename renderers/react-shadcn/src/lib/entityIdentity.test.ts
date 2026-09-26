import { describe, expect, it } from "vitest"
import type { EntitySchema } from "@/types/manifest"
import {
  getEntityRouteIdentifier,
  getEntityRouteSegment,
} from "@/lib/entityIdentity"

const promo = {
  module: "cafe-master",
  name: "promo",
  plural: "promos",
  label_field: "name",
  fields: [
    { name: "code", type: "string", natural_key: true },
    { name: "name", type: "string" },
  ],
} as EntitySchema

const entityWithoutNaturalKey = {
  ...promo,
  fields: [{ name: "name", type: "string" }],
} as EntitySchema

describe("entity route identity", () => {
  it("prefers the declared natural key", () => {
    expect(
      getEntityRouteIdentifier(promo, {
        id: "01uuid",
        code: "HAPPY-HOUR-20",
      }),
    ).toBe("HAPPY-HOUR-20")
  })

  it("falls back to the UUID primary key", () => {
    expect(
      getEntityRouteIdentifier(entityWithoutNaturalKey, { id: "01uuid" }),
    ).toBe("01uuid")
    expect(getEntityRouteIdentifier(promo, { id: "01uuid", code: "" })).toBe(
      "01uuid",
    )
  })

  it("encodes the selected identifier as one URL segment", () => {
    expect(
      getEntityRouteSegment(promo, {
        id: "01uuid",
        code: "HAPPY HOUR/20",
      }),
    ).toBe("HAPPY%20HOUR%2F20")
  })
})
