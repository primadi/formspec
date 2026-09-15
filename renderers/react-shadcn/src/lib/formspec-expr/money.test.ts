// ─── Money arithmetic in FormSpecExpr (S7 / gap #28) ───
//
// `money` is a first-class type whose value is the object {amount, currency}.
// These tests pin the canonical arithmetic table documented in
// docs_internal/plan/money-arithmetic-semantics.md and implemented server-side
// in internal/starlark/money.go — the two evaluators must agree.
//
// Run with: npx vitest run src/lib/formspec-expr/money.test.ts

import { describe, it, expect } from "vitest"
import { evalFormSpecExpr, evalCompute, type EvalContext } from "./index"

const idr = (amount: string) => ({ amount, currency: "IDR" })
const usd = (amount: string) => ({ amount, currency: "USD" })

type Fields = NonNullable<EvalContext["fields"]>

const ok = (expr: string, fields: Fields) => {
  const result = evalFormSpecExpr(expr, { fields })
  expect(result.warnings).toEqual([])
  expect(result.valid).toBe(true)
  return result.value
}

const fails = (expr: string, fields: Fields) => {
  const result = evalFormSpecExpr(expr, { fields })
  expect(result.valid).toBe(false)
  expect(result.warnings.length).toBeGreaterThan(0)
  return result.warnings.join("; ")
}

describe("money arithmetic — canonical form", () => {
  it("subtracts money from money (kembalian / cash difference)", () => {
    expect(
      ok("tendered - amount", {
        tendered: idr("100000"),
        amount: idr("75000"),
      }),
    ).toEqual({
      amount: "25000",
      currency: "IDR",
    })
    // A shortage is a negative money value, not a missing one.
    expect(
      ok("counted - expected", {
        counted: idr("500000"),
        expected: idr("512500"),
      }),
    ).toEqual({
      amount: "-12500",
      currency: "IDR",
    })
  })

  it("adds money to money", () => {
    expect(ok("a + b", { a: idr("1500"), b: idr("2500.50") })).toEqual({
      amount: "4000.5",
      currency: "IDR",
    })
  })

  it("multiplies money by a number, either side (line_total)", () => {
    const line = { quantity: 3, unit_price: idr("25000") }
    expect(ok("quantity * unit_price", line)).toEqual({
      amount: "75000",
      currency: "IDR",
    })
    expect(ok("unit_price * quantity", line)).toEqual({
      amount: "75000",
      currency: "IDR",
    })
  })

  it("keeps exact decimals instead of float noise", () => {
    expect(ok("a + b", { a: idr("0.1"), b: idr("0.2") })).toEqual({
      amount: "0.3",
      currency: "IDR",
    })
  })

  it("divides money by a number, and money by money as a ratio", () => {
    expect(ok("total / 2", { total: idr("25000") })).toEqual({
      amount: "12500",
      currency: "IDR",
    })
    expect(ok("gross / net", { gross: idr("15000"), net: idr("10000") })).toBe(
      1.5,
    )
  })

  it("negates money", () => {
    expect(ok("-price", { price: idr("2500") })).toEqual({
      amount: "-2500",
      currency: "IDR",
    })
  })

  it("compares money values of the same currency", () => {
    const fields = { tendered: idr("100000"), amount: idr("75000") }
    expect(ok("tendered >= amount", fields)).toBe(true)
    expect(ok("tendered < amount", fields)).toBe(false)
  })

  it("compares money for equality by value, not identity", () => {
    // Two distinct objects with the same content.
    expect(ok("a == b", { a: idr("1000"), b: idr("1000") })).toBe(true)
    expect(ok("a != b", { a: idr("1000"), b: idr("1001") })).toBe(true)
  })

  it("sums money values to money (total_amount)", () => {
    expect(
      ok("sum([a, b, c])", {
        a: idr("25000"),
        b: idr("12500"),
        c: idr("10000"),
      }),
    ).toEqual({
      amount: "47500",
      currency: "IDR",
    })
  })

  it("accepts numeric-string amounts (the wire shape)", () => {
    expect(
      ok("a - b", {
        a: { amount: 100000, currency: "IDR" },
        b: { amount: "25000", currency: "IDR" },
      }),
    ).toEqual({
      amount: "75000",
      currency: "IDR",
    })
  })
})

describe("money accessors", () => {
  it("extracts the amount as a plain number", () => {
    expect(ok("amount(price)", { price: idr("25000") })).toBe(25000)
    expect(ok("amount(amount)", { amount: idr("25000") })).toBe(25000)
  })

  it("extracts the currency", () => {
    expect(ok("currency(price)", { price: idr("25000") })).toBe("IDR")
  })

  it("lets money meet a plain number without inventing a unit", () => {
    expect(
      ok("amount(total) / item_count", { total: idr("25000"), item_count: 4 }),
    ).toBe(6250)
  })
})

describe("money errors are loud — never a silent 0", () => {
  it("rejects a currency mismatch", () => {
    expect(fails("a - b", { a: idr("100000"), b: usd("10") })).toContain(
      "currency mismatch",
    )
  })

  it("rejects money combined with a plain number", () => {
    expect(fails("total - 5", { total: idr("100000") })).toContain(
      "not defined",
    )
  })

  it("rejects money times money", () => {
    expect(fails("a * b", { a: idr("100"), b: idr("2") })).toContain(
      "not defined",
    )
  })

  it("rejects division by zero", () => {
    expect(fails("a / 0", { a: idr("100") })).toContain("division by zero")
    expect(fails("a / b", { a: idr("100"), b: idr("0") })).toContain(
      "division by zero",
    )
  })

  it("rejects a non-numeric operand instead of coercing it to 0", () => {
    const msg = fails("total - note", { total: idr("100"), note: "not money" })
    expect(msg).toContain("non-numeric string")
    // …and a plain object is rejected too.
    expect(
      fails("total - address", {
        total: idr("100"),
        address: { city: "Bandung" },
      }),
    ).toContain("an object")
  })

  it("rejects comparing money with a plain number", () => {
    expect(fails("total > 100000", { total: idr("100") })).toContain(
      "both operands must be money",
    )
  })

  it("rejects a sum that mixes money and numbers", () => {
    expect(fails("sum([a, 5])", { a: idr("100") })).toContain(
      "cannot mix money",
    )
  })

  it("rejects a sum across currencies", () => {
    expect(fails("sum([a, b])", { a: idr("100"), b: usd("1") })).toContain(
      "currency mismatch",
    )
  })
})

describe("money in computed fields", () => {
  it("evalCompute returns the money object (never null)", () => {
    const value = evalCompute("tendered - amount", {
      fields: { tendered: idr("100000"), amount: idr("75000") },
    })
    expect(value).toEqual({ amount: "25000", currency: "IDR" })
  })

  it("evalCompute returns null only when the formula is genuinely wrong", () => {
    expect(
      evalCompute("tendered - amount", {
        fields: { tendered: idr("100000"), amount: usd("10") },
      }),
    ).toBeNull()
  })
})

describe("non-money values are unaffected", () => {
  it("still does plain scalar arithmetic", () => {
    expect(ok("1 + 2 * 3", {})).toBe(7)
    expect(ok("quantity * price", { quantity: 2, price: 5000 })).toBe(10000)
  })

  it("still sums plain numbers", () => {
    expect(ok("sum([1, 2, 3])", {})).toBe(6)
  })

  it("keeps a plain object as an object (not money)", () => {
    expect(ok("address.city", { address: { city: "Bandung" } })).toBe("Bandung")
    // and arithmetic on it is an error, not 0
    expect(fails("address - 1", { address: { city: "Bandung" } })).toContain(
      "an object",
    )
  })
})
