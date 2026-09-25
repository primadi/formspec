// ─── Relation cell display ───
//
// A `belongs_to` relation is stored as a foreign key (`branch_id`), but the API
// also resolves the related record into a sibling alias object:
//
//   "branch_id": "01a0bf3d-…", "branch": { "id": "…", "name": "Kafe Senayan" }
//
// Reading history, three call sites resolved that alias and one did not:
//
//   - `engine/derive.ts` rewrites a DERIVED column to `branch.name` (dot-path),
//     so a derived table is correct;
//   - `kinds/page/DetailPage.tsx` had its own ~40-line resolver for the detail
//     page;
//   - `TableRenderer` / `ListingRenderer` did not resolve at all, so an
//     AUTHORED table that names the FK (`field: branch_id`) rendered the raw
//     UUID — even though the name was already in the row.
//
// This module is the one place both spellings resolve, so the next renderer
// cannot repeat the omission. Two forms are accepted because both are legal:
//
//   - `branch_id` (the FK field) → look up the alias object on the record;
//   - `branch.name` (dot-path)   → already nested, read it directly. Listing
//     read `row["branch.name"]` before, which is always undefined — this fixes
//     that too.
//
// The alias rule mirrors the server (`renderers/jsonb-persist/crud.go`):
// `patient_id` → `patient`, otherwise the relation's own resource name.

import type { EntitySchema, Field } from "@/types/manifest"
import { resolveEntityRef } from "@/engine/entityRef"

/** The alias an enriched relation object arrives under. */
export function relationAlias(field: Pick<Field, "name" | "relation">): string {
  const resource = field.relation?.resource ?? field.name
  return field.name.endsWith("_id") ? field.name.slice(0, -3) : resource
}

/** Read a dot-path (`branch.name`) out of a record, or undefined. */
export function readPath(
  record: Record<string, unknown>,
  path: string,
): unknown {
  const parts = path.split(".")
  let value: unknown = record
  for (const part of parts) {
    if (value == null || typeof value !== "object") return undefined
    value = (value as Record<string, unknown>)[part]
  }
  return value
}

function isPlainObject(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v)
}

/**
 * Display text for a relation cell, or `null` when this column is not a
 * relation (callers then render the value normally).
 *
 * `record` must be the row as the API returned it — the enriched alias is what
 * makes the name available without a second request.
 */
export function relationDisplay(
  record: Record<string, unknown>,
  columnField: string,
  entity: EntitySchema | undefined,
  findEntity: (module: string, name: string) => EntitySchema | undefined,
): string | null {
  // Dot-path form: the name is already nested on the record.
  if (columnField.includes(".")) {
    const v = readPath(record, columnField)
    if (v == null) return null
    if (isPlainObject(v)) return labelOf(v)
    return String(v)
  }

  const field = entity?.fields.find((f) => f.name === columnField)
  if (
    !field ||
    field.type !== "relation" ||
    field.relation?.type !== "belongs_to"
  )
    return null

  // Prefer the enriched object; fall back to the raw FK only when the alias is
  // absent. Showing the UUID is better than showing nothing, and it is what
  // this cell did before — but it now only happens when the server did not
  // resolve the relation at all (e.g. a field the caller may not read).
  const enriched = record[relationAlias(field)]
  if (isPlainObject(enriched)) {
    const [relModule, relName] = resolveEntityRef(
      field.relation.resource ?? "",
      entity?.module ?? "",
    )
    const related = findEntity(relModule, relName)
    return labelOf(enriched, related)
  }

  const raw = record[columnField]
  return raw == null ? null : String(raw)
}

/**
 * The human label of a related record: the related entity's `label_field`,
 * else the conventional `name`/`title`, else its `code`. UUIDs are the last
 * resort — a record whose label cannot be found still shows something the
 * operator can act on (e.g. a code) before it shows an opaque id.
 */
function labelOf(
  record: Record<string, unknown>,
  related?: EntitySchema,
): string {
  const labelField = related?.label_field
  const candidates = [
    ...(labelField ? [labelField] : []),
    "name",
    "title",
    "code",
  ]
  for (const key of candidates) {
    const v = record[key]
    if (typeof v === "string" && v !== "") return v
  }
  return String(record.id ?? "")
}
