// ─── Money-aware aggregation (S7 / gap #28) ───
//
// Reports and dashboard metrics aggregate rows client-side. A `money` field is
// the object {amount, currency}, so the obvious `Number(row[field])` yields NaN
// — which silently made every money report 0 (omzet, rekap kas, margin). The
// one rule this module enforces:
//
//   1. money aggregates over its `.amount` component;
//   2. anything that is neither a number nor money is an ERROR, never a 0.
//
// The server-side equivalents are `EntityStore.Aggregate` / `Window` in
// renderers/jsonb-persist/crud.go (which aggregate `$.field.amount`) and the
// `formspec check` gate in cmd/formspec/check.go — keep the three in step.

import { moneyAmount } from "@/lib/format"

/** The closed set of aggregate functions (Report totals, Table columns, Widget config). */
export type AggregateFn = "sum" | "avg" | "count" | "min" | "max"

/** Result of aggregating one field. `error` is set instead of a misleading 0. */
export interface AggregateResult {
  value: number | null
  error?: string
}

/** True when the cell holds nothing to aggregate. */
function isEmpty(value: unknown): boolean {
  return value === null || value === undefined || value === ""
}

/**
 * The number to aggregate from a cell: a bare number, a numeric string, or the
 * `.amount` of a money value.
 */
export function aggregateNumber(value: unknown): number | undefined {
  return moneyAmount(value)
}

/** A cell that is present but declares no aggregatable number. */
function isNonAggregatable(value: unknown): boolean {
  return !isEmpty(value) && aggregateNumber(value) === undefined
}

/**
 * Aggregate `field` over `items` using `fn`.
 *
 * A non-aggregatable field produces `{value: null, error}` rather than 0, so the
 * caller can say so out loud (S7: a confident 0 for a mis-declared money metric
 * is worse than showing nothing).
 */
export function aggregateRows(
  items: Record<string, unknown>[],
  fn: AggregateFn,
  field?: string,
): AggregateResult {
  if (fn === "count") {
    if (!field) return { value: items.length }
    return { value: items.filter((i) => !isEmpty(i[field])).length }
  }
  if (!field) {
    return { value: null, error: `aggregate "${fn}" needs a field` }
  }
  if (items.some((i) => isNonAggregatable(i[field]))) {
    return {
      value: null,
      error: `cannot ${fn} "${field}": the value is neither a number nor money`,
    }
  }

  const nums = items
    .map((i) => aggregateNumber(i[field]))
    .filter((n): n is number => n !== undefined)
  if (nums.length === 0) return { value: 0 }

  const sum = nums.reduce((a, b) => a + b, 0)
  switch (fn) {
    case "sum":
      return { value: sum }
    case "avg":
      return { value: sum / nums.length }
    case "min":
      return { value: Math.min(...nums) }
    case "max":
      return { value: Math.max(...nums) }
  }
}

/** A declared total/subtotal: a field plus the function to apply. */
export interface TotalDecl {
  field: string
  fn: string
}

/** Totals for a set of rows, plus any field that could not be totalled. */
export interface TotalsResult {
  values: Record<string, number>
  /** field → why it could not be aggregated (S7: loud, never a silent 0). */
  errors: Record<string, string>
}

/**
 * Compute the declared totals over a set of rows. Shared by the overall totals
 * row and the per-group subtotal rows of a Report.
 */
export function computeTotals(
  items: Record<string, unknown>[],
  totals: TotalDecl[],
): TotalsResult {
  const values: Record<string, number> = {}
  const errors: Record<string, string> = {}
  for (const total of totals) {
    const result = aggregateRows(
      items,
      (total.fn || "sum") as AggregateFn,
      total.field,
    )
    if (result.error) {
      errors[total.field] = result.error
    } else if (result.value !== null) {
      values[total.field] = result.value
    }
  }
  return { values, errors }
}
