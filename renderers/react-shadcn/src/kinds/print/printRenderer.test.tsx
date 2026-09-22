// ── Print value rendering: QR payload + money (item 2.6) ──
//
// The `kind: Print` body could not carry a QR at all, and money values were
// stringified — a receipt printed `{"amount":"62500","currency":"IDR"}` while
// every other surface showed `Rp62.500`. These tests pin the two rules that
// close that gap on the client (`format: html`) pipeline.

import { describe, expect, it } from "vitest"

import {
  formatValue,
  isMoneyObject,
  qrPixelSize,
  resolveQrPayload,
} from "./PrintRenderer"
import { createFormatter } from "@/lib/format"

describe("resolveQrPayload", () => {
  const ctx = { guest_token: "TOK-123", code: "A-01" }

  it("interpolates the payload from the record", () => {
    expect(resolveQrPayload({ payload: "{guest_token}" }, ctx, "https://kafe.example")).toBe(
      "TOK-123",
    )
  })

  it("prepends the origin when `absolute` is set", () => {
    expect(
      resolveQrPayload(
        { payload: "/status/{guest_token}", absolute: true },
        ctx,
        "https://kafe.example",
      ),
    ).toBe("https://kafe.example/status/TOK-123")
  })

  it("does not double the slash when the origin ends with one", () => {
    expect(
      resolveQrPayload({ payload: "/x", absolute: true }, ctx, "https://kafe.example/"),
    ).toBe("https://kafe.example/x")
  })

  it("adds a leading slash to a relative non-slash payload", () => {
    expect(
      resolveQrPayload({ payload: "status/{guest_token}", absolute: true }, ctx, "https://kafe.example"),
    ).toBe("https://kafe.example/status/TOK-123")
  })

  it("leaves an already-absolute URL alone (idempotent)", () => {
    const qr = { payload: "https://other.example/status/TOK-123", absolute: true }
    expect(resolveQrPayload(qr, ctx, "https://kafe.example")).toBe(
      "https://other.example/status/TOK-123",
    )
  })

  it("returns empty when the payload cannot be resolved — never a broken code", () => {
    expect(resolveQrPayload({ payload: "{missing}", absolute: true }, ctx, "https://kafe.example")).toBe(
      "",
    )
  })

  it("returns empty when only part of the payload resolves (dangling path)", () => {
    expect(
      resolveQrPayload(
        { payload: "/status/{missing}", absolute: true },
        ctx,
        "https://kafe.example",
      ),
    ).toBe("")
  })
})

describe("formatValue", () => {
  const formatter = createFormatter({
    locale: "id-ID",
    currency: { code: "IDR", symbol: "Rp", decimal_places: 0 },
  })

  it("formats a money object with the shared formatter", () => {
    expect(formatValue({ amount: "62500", currency: "IDR" }, formatter)).toBe("Rp62.500")
  })

  it("does not treat a plain number as money", () => {
    expect(isMoneyObject(2)).toBe(false)
    expect(formatValue(2, formatter)).toBe("2")
  })

  it("does not treat a currency-less object as money", () => {
    expect(isMoneyObject({ amount: "2" })).toBe(false)
    expect(formatValue({ amount: "2" }, formatter)).toBe("[object Object]")
  })

  it("returns empty for missing values and passes strings through", () => {
    expect(formatValue(null, formatter)).toBe("")
    expect(formatValue("Kopi Susu", formatter)).toBe("Kopi Susu")
  })
})

describe("qrPixelSize", () => {
  it("maps millimetres to CSS pixels at 96dpi", () => {
    expect(qrPixelSize(30)).toBe(113)
    expect(qrPixelSize(undefined)).toBe(113) // default 30mm
    expect(qrPixelSize(0)).toBe(113)
    expect(qrPixelSize(50)).toBe(189)
  })
})
