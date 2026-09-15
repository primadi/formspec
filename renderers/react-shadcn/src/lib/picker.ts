// ─── Child-field picker — pure logic (S1 generalized) ───
//
// Behind the `picker` declared on a child field (pkg/spec/picker.go): choose
// records from a source entity, carry a quantity, snapshot what was picked.
// Rows are ordinary child rows of the form's own state, so the Form owns
// submit — this module never builds a submit payload.
//
// Split out from the renderer so the row rules and the source→row mapping are
// unit-testable: the mapping is the contract with the entity (which row field
// receives what).

import { moneyAmount } from "@/lib/format"
import type { PickerMap } from "@/types/manifest"

/** One picked row, keyed by the row field names the picker maps onto. */
export interface PickedRow {
  /** Source record id — written into `map.ref_field`. */
  ref: string
  /** Snapshot of the source display name — written into `map.name_field`. */
  name: string
  /** Snapshot of the source price — written into `map.price_field`. */
  price?: unknown
  /** How many — written into `map.quantity_field` when declared. */
  quantity: number
  /** Free-text note — written into `map.note_field` when declared. */
  note?: string
}

/** Default per-row ceiling when `map.max_quantity` is not declared. */
export const DEFAULT_MAX_QUANTITY = 99

/** Clamp a quantity into `1..max`. */
export function clampQuantity(
  quantity: number,
  max = DEFAULT_MAX_QUANTITY,
): number {
  const ceiling = max > 0 ? max : DEFAULT_MAX_QUANTITY
  if (!Number.isFinite(quantity)) return 1
  return Math.min(Math.max(Math.trunc(quantity), 1), ceiling)
}

/**
 * Pick a source record: increment the existing row when the same record is
 * already picked (a second tap means "one more", not a duplicate row) — unless
 * the picker declares no quantity field, where each pick is its own row.
 */
export function pickRow(
  rows: PickedRow[],
  source: { ref: string; name: string; price?: unknown },
  map: PickerMap,
): PickedRow[] {
  const existing = rows.find((r) => r.ref === source.ref)
  if (existing && map.quantity_field) {
    const max = map.max_quantity ?? DEFAULT_MAX_QUANTITY
    return rows.map((r) =>
      r.ref === source.ref
        ? { ...r, quantity: clampQuantity(r.quantity + 1, max) }
        : r,
    )
  }
  return [
    ...rows,
    { ref: source.ref, name: source.name, price: source.price, quantity: 1 },
  ]
}

/** Set one row's quantity; below 1 removes the row. */
export function setQuantity(
  rows: PickedRow[],
  ref: string,
  quantity: number,
  max = DEFAULT_MAX_QUANTITY,
): PickedRow[] {
  if (quantity < 1) return removeRow(rows, ref)
  return rows.map((r) =>
    r.ref === ref ? { ...r, quantity: clampQuantity(quantity, max) } : r,
  )
}

/** Remove a picked row. */
export function removeRow(rows: PickedRow[], ref: string): PickedRow[] {
  return rows.filter((r) => r.ref !== ref)
}

/** Set (or clear) a row's note. */
export function setNote(
  rows: PickedRow[],
  ref: string,
  note: string,
): PickedRow[] {
  return rows.map((r) => (r.ref === ref ? { ...r, note } : r))
}

/** Total picked amount (sum of quantities). */
export function pickedCount(rows: PickedRow[]): number {
  return rows.reduce((n, r) => n + r.quantity, 0)
}

/** One row's total: price × quantity. Undefined for a non-money price. */
export function rowTotal(row: PickedRow): number | undefined {
  const unit = moneyAmount(row.price)
  return unit === undefined ? undefined : unit * row.quantity
}

/** Running total for display. Rows without a money price contribute nothing. */
export function pickedTotal(rows: PickedRow[]): number {
  return rows.reduce((sum, r) => sum + (rowTotal(r) ?? 0), 0)
}

// ── Source→row mapping ──

/**
 * Turn a picked row into the child record the form submits. Field names come
 * from `map`, never hardcoded — the snapshots (`name_field`, `price_field`) are
 * what keep an old record readable after the source changes.
 */
export function buildRow(
  row: PickedRow,
  map: PickerMap,
): Record<string, unknown> {
  const out: Record<string, unknown> = { [map.ref_field]: row.ref }
  if (map.quantity_field) out[map.quantity_field] = row.quantity
  if (map.price_field) out[map.price_field] = row.price
  if (map.name_field) out[map.name_field] = row.name
  if (map.note_field && row.note) out[map.note_field] = row.note
  return out
}

/** Build every picked row for submission. */
export function buildRows(
  rows: PickedRow[],
  map: PickerMap,
): Record<string, unknown>[] {
  return rows.map((r) => buildRow(r, map))
}

// ── Templates (default_from / filters) ──

/** Block-local tokens available on top of the render context. */
export function clockTokens(now: Date = new Date()): Record<string, unknown> {
  return {
    now: now.toISOString(),
    today: now.toISOString().slice(0, 10),
  }
}

/** Resolve a dotted path inside a nested object. */
function lookupPath(ctx: Record<string, unknown>, path: string): unknown {
  return path.split(".").reduce<unknown>((acc, key) => {
    if (acc == null || typeof acc !== "object") return undefined
    return (acc as Record<string, unknown>)[key]
  }, ctx)
}

/**
 * Replace `{dotted.path}` with the value found in `ctx`.
 *
 * An unresolvable token is left **verbatim** rather than becoming an empty
 * string: the payload then shows `{session.branch_id}` instead of a silent
 * blank, which is a far better clue than a mystery 422 from a missing required
 * field.
 */
export function interpolateTokens(
  value: string,
  ctx: Record<string, unknown>,
): string {
  if (!value.includes("{")) return value
  return value.replace(/\{([\w.]+)\}/g, (match, path: string) => {
    const resolved = lookupPath(ctx, path)
    return resolved === undefined ? match : String(resolved)
  })
}

/** Interpolate one `default_from` template. */
export function resolveDefault(
  template: string | undefined,
  ctx: Record<string, unknown>,
  now: Date = new Date(),
): unknown {
  if (template === undefined || template === "") return undefined
  return interpolateTokens(template, { ...ctx, ...clockTokens(now) })
}

/** Interpolate every value of a filter map (source/price filters). */
export function interpolateFilter(
  filter: Record<string, string> | undefined,
  ctx: Record<string, unknown>,
  now: Date = new Date(),
): Record<string, string> {
  if (!filter) return {}
  const scope = { ...ctx, ...clockTokens(now) }
  const out: Record<string, string> = {}
  for (const [key, value] of Object.entries(filter)) {
    out[key] = interpolateTokens(value, scope)
  }
  return out
}

/**
 * Seed values for every field declaring `default_from` — what the Form uses to
 * carry values the user does not type (branch, session, timestamp).
 */
export function seedDefaults(
  fields: { name: string; default_from?: string }[] | undefined,
  ctx: Record<string, unknown>,
  now: Date = new Date(),
): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const field of fields ?? []) {
    if (!field.default_from) continue
    const value = resolveDefault(field.default_from, ctx, now)
    if (value !== undefined) out[field.name] = value
  }
  return out
}

// ── Pricing (source rows may read prices from a separate entity) ──

/**
 * Build a lookup of source id → price from a price entity's rows.
 *
 * Per-branch price lists put the price in its own entity
 * (`menu-item-price.{branch_id, menu_item_id, price}`), so the tile price is a
 * client-side join. A source row with no matching price row has no price — and
 * therefore cannot be picked (there is nothing to sell).
 */
export function priceIndex(
  rows: Record<string, unknown>[],
  matchField: string,
  priceField: string,
): Map<string, unknown> {
  const index = new Map<string, unknown>()
  for (const row of rows) {
    const key = row[matchField]
    if (key == null || key === "") continue
    const price = row[priceField]
    if (price == null) continue
    index.set(String(key), price)
  }
  return index
}

/** A pickable tile: identity, display data, price, pickability. */
export interface PickerTile {
  id: string
  name: string
  price: unknown
  /** False when the source row has no resolvable price (cannot be picked). */
  pickable: boolean
}

/** Project source rows into tiles, resolving prices via {@link priceIndex}. */
export function pickerTiles(opts: {
  rows: Record<string, unknown>[]
  nameField: string
  priceField: string
  prices?: Map<string, unknown>
}): PickerTile[] {
  const { rows, nameField, priceField, prices } = opts
  return rows.map((row) => {
    const id = String(row.id ?? "")
    const price = prices ? prices.get(id) : row[priceField]
    return {
      id,
      name: String(row[nameField] ?? ""),
      price,
      // Only enforce a price when the picker says where one comes from.
      pickable:
        prices || priceField
          ? price != null && moneyAmount(price) !== undefined
          : true,
    }
  })
}
