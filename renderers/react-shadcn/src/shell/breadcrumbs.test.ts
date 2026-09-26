import { describe, expect, it } from "vitest"
import type { EntitySchema } from "@/types/manifest"
import { buildBreadcrumbs } from "@/shell/breadcrumbs"

const promo = {
  module: "cafe-master",
  name: "promo",
  plural: "promos",
  label_field: "name",
  fields: [{ name: "code", type: "string", natural_key: true }],
} as EntitySchema

describe("buildBreadcrumbs", () => {
  it("uses the loaded natural key for a UUID detail route", () => {
    const breadcrumbs = buildBreadcrumbs(
      "/kafe/app/pos/cafe-master/promos/01uuid",
      "kafe",
      [promo],
      "HAPPY-HOUR-20",
    )

    expect(breadcrumbs.at(-1)).toEqual({
      label: "HAPPY-HOUR-20",
      href: "/kafe/app/pos/cafe-master/promos/01uuid",
      isLast: true,
    })
  })

  it("keeps the URL identifier while loading or on non-detail routes", () => {
    const loading = buildBreadcrumbs(
      "/kafe/app/pos/cafe-master/promos/01uuid",
      "kafe",
      [promo],
    )
    const list = buildBreadcrumbs(
      "/kafe/app/pos/cafe-master/promos",
      "kafe",
      [promo],
      "SHOULD-NOT-BE-USED",
    )

    expect(loading.at(-1)?.label).toBe("01uuid")
    expect(list.at(-1)?.label).toBe("Promos")
  })
})
