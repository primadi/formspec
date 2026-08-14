// ─── Filter helpers Tests ───
//
// Covers the generic filter model shared by Table & Kanban:
//   - `resolveFilterValue`: `today` / `today()` → server date, others passthrough
//   - `buildFixedFilterParams`: immutable `field[op]=value` entries
//   - `buildUserFilterParams`: user-adjustable `field[op]=value` entries
//
// Run with: npx vitest run src/lib/filters.test.ts

import { describe, it, expect, vi, afterEach } from "vitest"
import {
  resolveFilterValue,
  buildFixedFilterParams,
  buildUserFilterParams,
  shouldShowAll,
  allLabel,
  DEFAULT_ALL_LABEL,
} from "./filters"
import type { FilterSpec } from "@/types/manifest"

afterEach(() => {
  vi.restoreAllMocks()
})

describe("resolveFilterValue", () => {
  it("resolves `today` to the server date (YYYY-MM-DD)", () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date("2026-08-07T23:30:00Z"))
    expect(resolveFilterValue("today")).toBe("2026-08-07")
  })

  it("resolves `today()` to the server date", () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date("2026-08-07T00:00:00Z"))
    expect(resolveFilterValue("today()")).toBe("2026-08-07")
  })

  it("passes through plain values unchanged", () => {
    expect(resolveFilterValue("2026-08-07")).toBe("2026-08-07")
    expect(resolveFilterValue("poly-1")).toBe("poly-1")
  })

  it("returns empty for undefined/empty", () => {
    expect(resolveFilterValue()).toBe("")
    expect(resolveFilterValue("")).toBe("")
    expect(resolveFilterValue("   ")).toBe("")
  })
})

describe("buildFixedFilterParams", () => {
  it("builds field[op]=value entries, resolving today()", () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date("2026-08-07T10:00:00Z"))
    const fixed: FilterSpec[] = [
      { field: "transaction_date", default: "today()" },
      { field: "tenant_id", default: "tenant-1" },
    ]
    expect(buildFixedFilterParams(fixed)).toEqual({
      "transaction_date[eq]": "2026-08-07",
      "tenant_id[eq]": "tenant-1",
    })
  })

  it("honors a custom op and skips empty defaults", () => {
    const fixed: FilterSpec[] = [
      { field: "total", op: "gte", default: "100" },
      { field: "archived", default: "" },
    ]
    expect(buildFixedFilterParams(fixed)).toEqual({ "total[gte]": "100" })
  })

  it("handles undefined input", () => {
    expect(buildFixedFilterParams(undefined)).toEqual({})
  })
})

describe("buildUserFilterParams", () => {
  it("maps active values to field[op]=value with per-spec ops", () => {
    const specs: FilterSpec[] = [
      { field: "transaction_date", type: "date" },
      { field: "total", op: "gte" },
    ]
    expect(
      buildUserFilterParams(specs, { transaction_date: "2026-08-07", total: "50" }),
    ).toEqual({
      "transaction_date[eq]": "2026-08-07",
      "total[gte]": "50",
    })
  })

  it("skips empty values", () => {
    const specs: FilterSpec[] = [{ field: "status", type: "select" }]
    expect(buildUserFilterParams(specs, { status: "" })).toEqual({})
  })
})

describe("shouldShowAll / allLabel", () => {
  it("defaults to showing the All option with (ALL) caption", () => {
    const spec: FilterSpec = { field: "status", type: "select" }
    expect(shouldShowAll(spec)).toBe(true)
    expect(allLabel(spec)).toBe("(ALL)")
    expect(DEFAULT_ALL_LABEL).toBe("(ALL)")
  })

  it("honors show_all: false to hide the All option", () => {
    const spec: FilterSpec = { field: "status", type: "select", show_all: false }
    expect(shouldShowAll(spec)).toBe(false)
  })

  it("customizes the All caption via all_label", () => {
    const spec: FilterSpec = { field: "status", type: "select", all_label: "Semua" }
    expect(allLabel(spec)).toBe("Semua")
  })
})
