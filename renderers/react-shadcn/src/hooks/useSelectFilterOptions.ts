// ─── useSelectFilterOptions ───
//
// Derives the options for a `select` filter from the entity field definition,
// independent of the currently loaded records:
//   - relation field (`belongs_to`) → fetch the related entity's records
//     (id + label field), so the options stay valid even when the board/table
//     is scoped to an empty date range.
//   - a field with a declared choice set (`options`, or `enum_values`) → those
//     values, with `options` donating the captions. One vocabulary: the same
//     declaration the form widget and the table cell already read, so a filter
//     cannot offer a different caption (or a different set) than the column it
//     filters.
//   - otherwise → empty (the caller may fall back to record-derived values).
//
// Shared by Table and Kanban so select filters behave identically.

import { useEffect, useState } from "react"
import type { EntitySchema, FilterSpec, MetaBundle } from "@/types/manifest"
import { resolveEntityRef } from "@/engine/entityRef"
import { fieldOptions } from "@/lib/field-options"

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
  // Only a `select` filter renders options. The guard lives HERE, not at the
  // call site, because callers must now call this hook unconditionally
  // (a hook inside a `switch` case violates the Rules of Hooks — see
  // TableRenderer.FilterControl). Folding the requirement into the hook keeps
  // the "no fetch for a date/text filter" behaviour that a hoisted call would
  // otherwise lose.
  const isSelect = (filter.type ?? "select") === "select"
  const fieldDef = isSelect
    ? entity?.fields.find((f) => f.name === filter.field)
    : undefined
  const isRelation = fieldDef?.type === "relation" && fieldDef?.relation != null

  const [relationOptions, setRelationOptions] = useState<SelectOption[]>([])

  useEffect(() => {
    if (!isRelation || !metaBundle || !fieldDef?.relation) return
    const resource = fieldDef.relation.resource
    // relation.resource can be "entity" (same module) or "module.entity"
    // (cross-module) — resolve relative to the owning entity's module.
    const [relModule, relName] = resolveEntityRef(
      resource,
      entity?.module ?? "",
    )
    const relatedEntity = metaBundle.entities.find(
      (e) => e.module === relModule && e.name === relName,
    )
    if (!relatedEntity) return
    const client = getClient()
    const labelField = relatedEntity.label_field || "name"
    client
      .get(`${relatedEntity.module}/${relatedEntity.name}`, {
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
    // `entity?.module` is read (via resolveEntityRef) but a dep on the
    // optional chain would re-run this on every render; the entity identity
    // covers it — a module change always arrives as a different entity.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isRelation, metaBundle, fieldDef, getClient])

  if (isRelation) return relationOptions

  // `options` wins (it carries captions), `enum_values` is the value-only
  // fallback — the same resolution `fieldOptions()` performs for the form
  // widget, the table cell, and the detail page. Before this the hook read
  // `enum_values` directly, so a field that declared `options` (with human
  // captions) produced an empty filter.
  return fieldOptions(fieldDef).map((o) => ({
    value: String(o.value),
    label: o.label,
  }))
}
