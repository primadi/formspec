// ── ApprovalInbox display-field formatting ──
//
// The approver is shown the values a workflow step declares as
// `display_fields` and asked to decide on them. Two ways that promise broke,
// both measured in the browser on kafe's void queue:
//
//   1. a money value printed as "[object Object]" (kafe Total Amount =
//      `{amount, currency}`), so the approver could not read the number they
//      were approving a refund for; and
//   2. a field the requester had filled in but the record did not yet carry
//      (an intercepted transition writes nothing until approval completes)
//      rendered as empty — that half is fixed server-side by falling back to
//      the approval row's `params`, which is why the type/label have to travel
//      WITH the value.
//
// These tests pin the client half: format by the DECLARED type, never by
// guessing.

import { describe, expect, it } from "vitest"

import {
  formatApprovalField,
  type ApprovalField,
} from "./ApprovalInboxRenderer"
import { createFormatter } from "@/lib/format"

const formatter = createFormatter({
  locale: "id-ID",
  currency: { code: "IDR", symbol: "Rp", decimal_places: 0 },
})

const field = (f: Partial<ApprovalField>): ApprovalField => ({
  field: "x",
  // `value` is required on ApprovalField (`unknown`), so a partial override that
  // omits it must still produce the key.
  value: undefined,
  ...f,
})

describe("formatApprovalField", () => {
  it("formats a money value instead of printing [object Object]", () => {
    expect(
      formatApprovalField(
        field({ type: "money", value: { amount: "125000", currency: "IDR" } }),
        formatter,
      ),
    ).toBe("Rp125.000")
  })

  it("does not format a plain number as money", () => {
    // `decimal`/`integer`/`number` keep their own formatting: printing a count
    // as a currency would be a lie about what the field holds.
    expect(
      formatApprovalField(field({ type: "integer", value: 125000 }), formatter),
    ).toBe("125.000")
  })

  it("renders a currency object with no usable amount as — , not a blob", () => {
    expect(
      formatApprovalField(field({ type: "money", value: {} }), formatter),
    ).toBe("—")
    expect(
      formatApprovalField(
        field({ type: "money", value: { amount: "abc" } }),
        formatter,
      ),
    ).toBe("—")
  })

  it("renders an object with no declared type as — (never [object Object])", () => {
    expect(formatApprovalField(field({ value: { a: 1 } }), formatter)).toBe("—")
  })

  it("renders empty values as —", () => {
    expect(
      formatApprovalField(field({ type: "string", value: null }), formatter),
    ).toBe("—")
    expect(
      formatApprovalField(field({ type: "string", value: "" }), formatter),
    ).toBe("—")
  })

  it("passes a string through untouched", () => {
    expect(
      formatApprovalField(
        field({ type: "string", value: "salah input" }),
        formatter,
      ),
    ).toBe("salah input")
  })

  it("formats dates by their declared type", () => {
    // Locale-independent assertion: it must not equal the raw ISO string.
    const out = formatApprovalField(
      field({ type: "datetime", value: "2026-10-04T09:28:46.616296031Z" }),
      formatter,
    )
    expect(out).not.toBe("2026-10-04T09:28:46.616296031Z")
    expect(out).toContain("2026")
  })
})
