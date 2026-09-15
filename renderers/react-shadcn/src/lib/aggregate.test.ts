// ─── Money-aware aggregation tests (S7 / gap #28) ───
//
// The bug this prevents: a money column is the object {amount, currency}, so
// `Number(row[field])` is NaN and every money total silently came out 0
// (omzet harian, rekap kas, margin per menu).
//
// Run with: npx vitest run src/lib/aggregate.test.ts

import { describe, it, expect } from "vitest"
import { aggregateRows, computeTotals } from "./aggregate"

const idr = (amount: string) => ({ amount, currency: "IDR" })

describe("aggregateRows over money", () => {
  const rows = [
    { total: idr("25000"), qty: 2, note: "a" },
    { total: idr("12500"), qty: 3, note: "b" },
    { total: idr("10000"), qty: 1, note: "c" },
  ]

  it("sums the amount, not the object", () => {
    expect(aggregateRows(rows, "sum", "total")).toEqual({ value: 47500 })
  })

  it("averages, mins and maxes the amount", () => {
    expect(aggregateRows(rows, "avg", "total").value).toBeCloseTo(47500 / 3)
    expect(aggregateRows(rows, "min", "total")).toEqual({ value: 10000 })
    expect(aggregateRows(rows, "max", "total")).toEqual({ value: 25000 })
  })

  it("accepts a bare number or a numeric string too", () => {
    const mixed = [
      { total: 25000 },
      { total: "12500" },
      { total: idr("10000") },
    ]
    expect(aggregateRows(mixed, "sum", "total")).toEqual({ value: 47500 })
  })

  it("counts rows that carry a value", () => {
    expect(aggregateRows(rows, "count", "total")).toEqual({ value: 3 })
    // count() with no field counts every row.
    expect(aggregateRows(rows, "count")).toEqual({ value: 3 })
  })

  it("ignores rows where the field is absent", () => {
    const sparse = [{ total: idr("25000") }, {}, { total: null }]
    expect(aggregateRows(sparse, "sum", "total")).toEqual({ value: 25000 })
  })
})

describe("aggregateRows rejects what it cannot aggregate", () => {
  it("refuses to sum a text column (was a silent 0)", () => {
    const rows = [{ note: "a" }, { note: "b" }]
    const result = aggregateRows(rows, "sum", "note")
    expect(result.value).toBeNull()
    expect(result.error).toContain("neither a number nor money")
  })

  it("refuses a plain object that is not money", () => {
    const rows = [{ address: { city: "Bandung" } }]
    expect(aggregateRows(rows, "sum", "address").value).toBeNull()
  })

  it("refuses a non-currency aggregate without a field", () => {
    const result = aggregateRows([{ a: 1 }], "sum")
    expect(result.value).toBeNull()
    expect(result.error).toContain("needs a field")
  })
})

describe("computeTotals", () => {
  it("reports money totals and per-field errors side by side", () => {
    const rows = [
      { total: idr("25000"), note: "a" },
      { total: idr("12500"), note: "b" },
    ]
    const totals = computeTotals(rows, [
      { field: "total", fn: "sum" },
      { field: "note", fn: "sum" },
    ])
    expect(totals.values).toEqual({ total: 37500 })
    expect(totals.errors.note).toContain("cannot sum")
  })

  it("treats a missing fn as sum", () => {
    expect(
      computeTotals([{ a: 1 }, { a: 2 }], [{ field: "a", fn: "" }]).values,
    ).toEqual({ a: 3 })
  })
})
