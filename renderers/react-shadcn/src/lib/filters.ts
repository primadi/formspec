// ─── Filter helpers (shared) ───
//
// Generic filter model for data kinds (Table, Kanban, Listing, …):
//   - `filters`      → user-adjustable controls, pre-seeded from `default`
//   - `fixed_filters`→ immutable, always merged into the list request, never
//                      rendered as a control and not user-clearable
//
// Values are sent to the list API as `field[op]=value` (operator syntax), so
// they flow straight through buildListParams / apiList.

import type { FilterSpec } from "@/types/manifest"

/** Default caption for a select filter's "All" (clear) option. */
export const DEFAULT_ALL_LABEL = "(ALL)"

/** Whether a select filter shows the "All" (clear) option. Default true. */
export function shouldShowAll(spec: FilterSpec): boolean {
  return spec.show_all !== false
}

/** Caption of a select filter's "All" (clear) option. Default "(ALL)". */
export function allLabel(spec: FilterSpec): string {
  return spec.all_label || DEFAULT_ALL_LABEL
}

/** Server's current date (YYYY-MM-DD, UTC). Anchors `today` / `today()` in
 *  filter defaults — same convention as widget query translation: server
 *  timestamps are RFC3339 UTC, so UTC today == server today. */
export function serverToday(): string {
  return new Date().toISOString().slice(0, 10)
}

/** Resolves a single filter value. Supports `today` / `today()` → the server's
 *  current date; any other value is passed through unchanged. */
export function resolveFilterValue(value?: string): string {
  if (!value) return ""
  const trimmed = value.trim()
  if (trimmed === "today()" || trimmed === "today") return serverToday()
  return trimmed
}

/** Builds URLSearchParams-ready entries (`field[op]=value`) from the
 *  manifest's fixed_filters — immutable, always applied. */
export function buildFixedFilterParams(fixed?: FilterSpec[]): Record<string, string> {
  const q: Record<string, string> = {}
  for (const f of fixed ?? []) {
    const v = resolveFilterValue(f.default)
    if (!v) continue
    q[`${f.field}[${f.op || "eq"}]`] = v
  }
  return q
}

/** Builds URLSearchParams-ready entries (`field[op]=value`) from the user's
 *  active filter values, resolving each field's op from its FilterSpec. */
export function buildUserFilterParams(
  specs: FilterSpec[],
  values: Record<string, string>,
): Record<string, string> {
  const q: Record<string, string> = {}
  const opByField = new Map(specs.map((s) => [s.field, s.op || "eq"]))
  for (const [field, value] of Object.entries(values)) {
    if (!value) continue
    q[`${field}[${opByField.get(field) || "eq"}]`] = value
  }
  return q
}
