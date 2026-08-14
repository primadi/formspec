// ─── useSelectFilterOptions ───
//
// Derives the options for a `select` filter from the entity field definition,
// independent of the currently loaded records:
//   - relation field (`belongs_to`) → fetch the related entity's records
//     (id + label field), so the options stay valid even when the board/table
//     is scoped to an empty date range.
//   - enum field → the field's enum_values.
//   - otherwise → empty (the caller may fall back to record-derived values).
//
// Shared by Table and Kanban so select filters behave identically.

import { useEffect, useState } from "react"
import type { EntitySchema, FilterSpec, MetaBundle } from "@/types/manifest"

export interface SelectOption {
  value: string
  label: string
}

export function useSelectFilterOptions(
  filter: FilterSpec,
  entity: EntitySchema | undefined,
  metaBundle: MetaBundle | null,
  getClient: () => import("ky").KyInstance,
): SelectOption[] {
  const fieldDef = entity?.fields.find((f) => f.name === filter.field)
  const isRelation = fieldDef?.type === "relation" && fieldDef?.relation != null

  const [relationOptions, setRelationOptions] = useState<SelectOption[]>([])

  useEffect(() => {
    if (!isRelation || !metaBundle || !fieldDef?.relation) return
    const resource = fieldDef.relation.resource
    const relatedEntity = metaBundle.entities.find((e) => e.name === resource)
    if (!relatedEntity) return
    const client = getClient()
    const labelField = relatedEntity.label_field || "name"
    client
      .get(`${relatedEntity.module}/${resource}`, {
        searchParams: { per_page: "500" },
      })
      .json<{ data: Record<string, unknown>[] }>()
      .then((body) => {
        setRelationOptions(
          (body.data ?? []).map((item) => ({
            value: String(item.id ?? ""),
            label: String(item[labelField] ?? item.id ?? ""),
          })),
        )
      })
      .catch(() => {
        // Silently fail — the filter just shows "All" only
      })
  }, [isRelation, metaBundle, fieldDef, getClient])

  if (isRelation) return relationOptions
  if (fieldDef?.enum_values?.length) {
    return fieldDef.enum_values.map((o) => ({ value: o, label: o }))
  }
  return []
}
