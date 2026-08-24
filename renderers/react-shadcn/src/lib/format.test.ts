// ─── Centralized Formatting Tests ───
//
// Verifies the global-settings-driven formatter (spec §10 — "jangan pernah
// menebak"): money/date/number formatting follows the resolved settings
// namespace, and falls back to standard defaults when unset.
//
// Run with: npx vitest run src/lib/format.test.ts

import { describe, it, expect } from "vitest"
import { createFormatter, parseDateByPattern, formatDateInput } from "./format"

describe("createFormatter — defaults (no settings)", () => {
  const fmt = createFormatter()

  it("formats money with default USD + en-US", () => {
    expect(fmt.money(1234.5)).toBe("$1,234.50")
  })

  it("formats numbers with default locale + scale", () => {
    expect(fmt.number(1234.567)).toBe("1,234.57")
  })

  it("formats dates with default ISO pattern", () => {
    expect(fmt.date("2026-08-24T00:00:00Z")).toBe("2026-08-24")
  })

  it("formats datetimes with locale", () => {
    expect(fmt.dateTime("2026-08-24T12:30:00Z")).toContain("2026")
  })

  it("formats relative times", () => {
    const past = new Date(Date.now() - 5 * 60000).toISOString()
    expect(fmt.relative(past)).toBe("5m ago")
  })
})

describe("createFormatter — with global settings", () => {
  const fmt = createFormatter({
    currency: { code: "IDR", decimal_places: 0, symbol: "Rp" },
    locale: "id-ID",
    timezone: "Asia/Jakarta",
    date_format: "DD/MM/YYYY",
    decimal_scale: 2,
    rounding: "half_even",
  })

  it("formats money with explicit symbol + scale", () => {
    expect(fmt.money(1234)).toBe("Rp1.234")
  })

  it("formats money with decimals when scale > 0", () => {
    const fmt2 = createFormatter({
      currency: { code: "IDR", decimal_places: 2, symbol: "Rp" },
      locale: "id-ID",
    })
    expect(fmt2.money(1234.5)).toBe("Rp1.234,50")
  })

  it("formats numbers with id-ID locale", () => {
    expect(fmt.number(1234.5)).toBe("1.234,5")
  })

  it("formats dates with DD/MM/YYYY pattern", () => {
    expect(fmt.date("2026-08-24T00:00:00Z")).toBe("24/08/2026")
  })
})

describe("createFormatter — partial settings overlay", () => {
  it("keeps defaults for unset fields", () => {
    const fmt = createFormatter({ locale: "id-ID" })
    // currency not set → default USD code, but locale id-ID drives grouping
    expect(fmt.money(10)).toBe("US$10,00")
    // date_format not set → default ISO
    expect(fmt.date("2026-08-24T00:00:00Z")).toBe("2026-08-24")
  })
})

describe("parseDateByPattern", () => {
  it("parses DD/MM/YYYY", () => {
    expect(parseDateByPattern("24/08/2026", "DD/MM/YYYY")).toBe("2026-08-24")
  })

  it("parses YYYY-MM-DD", () => {
    expect(parseDateByPattern("2026-08-24", "YYYY-MM-DD")).toBe("2026-08-24")
  })

  it("parses DD-MM-YYYY", () => {
    expect(parseDateByPattern("24-08-2026", "DD-MM-YYYY")).toBe("2026-08-24")
  })

  it("parses datetime patterns", () => {
    expect(parseDateByPattern("24/08/2026 14:30", "DD/MM/YYYY HH:mm")).toBe(
      "2026-08-24T14:30:00",
    )
  })

  it("rejects incomplete input", () => {
    expect(parseDateByPattern("24/08", "DD/MM/YYYY")).toBeNull()
  })

  it("rejects impossible dates", () => {
    expect(parseDateByPattern("31/02/2026", "DD/MM/YYYY")).toBeNull()
  })

  it("rejects non-matching input", () => {
    expect(parseDateByPattern("hello", "DD/MM/YYYY")).toBeNull()
  })
})

describe("formatDateInput — auto-insert separators while typing", () => {
  it("formats 24082026 → 24/08/2026 for DD/MM/YYYY", () => {
    expect(formatDateInput("24082026", "DD/MM/YYYY")).toBe("24/08/2026")
  })

  it("formats progressively while typing", () => {
    const p = "DD/MM/YYYY"
    expect(formatDateInput("2", p)).toBe("2")
    expect(formatDateInput("24", p)).toBe("24")
    expect(formatDateInput("240", p)).toBe("24/0")
    expect(formatDateInput("2408", p)).toBe("24/08")
    expect(formatDateInput("24082", p)).toBe("24/08/2")
    expect(formatDateInput("24082026", p)).toBe("24/08/2026")
  })

  it("handles already-separated input", () => {
    expect(formatDateInput("24/08/2026", "DD/MM/YYYY")).toBe("24/08/2026")
  })

  it("formats YYYY-MM-DD", () => {
    expect(formatDateInput("20260824", "YYYY-MM-DD")).toBe("2026-08-24")
  })

  it("formats datetime patterns", () => {
    expect(formatDateInput("240820261430", "DD/MM/YYYY HH:mm")).toBe(
      "24/08/2026 14:30",
    )
  })

  it("returns empty for empty input", () => {
    expect(formatDateInput("", "DD/MM/YYYY")).toBe("")
  })
})
