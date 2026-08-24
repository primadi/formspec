// ─── Centralized Formatting ───
//
// Single source of truth for money/date/number formatting across every
// surface. Reads the resolved global settings namespace (spec §10 — "jangan
// pernah menebak") from the meta bundle instead of hard-coding a locale or
// currency per component.
//
// Usage:
//   const fmt = createFormatter(useMetaStore.getState().getSettings())
//   fmt.money(1234.5)   // "$1,234.50" (or "Rp1.234" per settings)
//   fmt.date("2026-08-24")
//   fmt.number(1234.5)
//
// A module-level `defaultFormatter` is provided for callers that already
// read the bundle via a hook; it falls back to standard defaults when the
// bundle isn't loaded yet.

import type { Settings } from "@/types/manifest"

export interface Formatter {
  /** Format a number as money using the resolved currency + locale. */
  money: (value: number) => string
  /** Format a number with the resolved locale + decimal scale. */
  number: (value: number) => string
  /** Format a date/datetime string using the resolved locale + date format. */
  date: (value: string | Date) => string
  /** Format a datetime string using the resolved locale (with time). */
  dateTime: (value: string | Date) => string
  /** Format a value as a relative time ("5m ago"). */
  relative: (value: string | Date) => string
}

/** Rounding modes for money/decimal arithmetic (spec §10). */
export type RoundingMode = "half_even" | "half_up" | "half_down" | "up" | "down"

// ── Standard defaults (spec §10) ──
const DEFAULT_LOCALE = "en-US"
const DEFAULT_CURRENCY = "USD"
const DEFAULT_DECIMAL_PLACES = 2
const DEFAULT_DATE_FORMAT = "YYYY-MM-DD"
const DEFAULT_ROUNDING: RoundingMode = "half_even"

/**
 * Round a value to `places` decimals using the given mode (BigDecimal-style
 * semantics, spec §10):
 *   - half_even: ties to the nearest even digit (banker's, default)
 *   - half_up:   ties away from zero
 *   - half_down: ties toward zero
 *   - up:        always away from zero
 *   - down:      always toward zero
 * A small epsilon guards against binary floating-point drift (e.g. 1.005*100
 * = 100.49999…), so ties resolve to the intended digit.
 */
export function roundTo(
  value: number,
  places: number,
  mode: RoundingMode = DEFAULT_ROUNDING,
): number {
  if (!Number.isFinite(value)) return value
  const factor = Math.pow(10, places)
  // Snap the scaled value to a high precision to cancel binary floating-point
  // drift (e.g. 1.005*100 = 100.49999999999999 → 100.5) so ties resolve to
  // the intended digit instead of the nearest representable neighbor.
  const scaled = Math.round(value * factor * 1e12) / 1e12
  let r: number
  switch (mode) {
    case "up":
      r = scaled >= 0 ? Math.ceil(scaled) : Math.floor(scaled)
      break
    case "down":
      r = scaled >= 0 ? Math.floor(scaled) : Math.ceil(scaled)
      break
    case "half_up":
      r = scaled >= 0 ? Math.floor(scaled + 0.5) : Math.ceil(scaled - 0.5)
      break
    case "half_down":
      r = scaled >= 0 ? Math.ceil(scaled - 0.5) : Math.floor(scaled + 0.5)
      break
    case "half_even":
    default: {
      const floor = Math.floor(scaled)
      const diff = scaled - floor
      if (diff < 0.5) r = floor
      else if (diff > 0.5) r = floor + 1
      else r = floor % 2 === 0 ? floor : floor + 1
    }
  }
  return r / factor
}

/**
 * Build a Formatter from the resolved global settings. Falls back to the
 * standard defaults for any unset field, so behavior is consistent even when
 * the bundle isn't loaded or settings aren't declared.
 */
export function createFormatter(settings?: Settings): Formatter {
  const locale = settings?.locale || DEFAULT_LOCALE
  const currencyCode = settings?.currency?.code || DEFAULT_CURRENCY
  const currencyPlaces =
    settings?.currency?.decimal_places ?? DEFAULT_DECIMAL_PLACES
  const currencySymbol = settings?.currency?.symbol
  const dateFormat = settings?.date_format || DEFAULT_DATE_FORMAT
  const decimalScale = settings?.decimal_scale ?? DEFAULT_DECIMAL_PLACES
  const rounding = settings?.rounding || DEFAULT_ROUNDING

  const money = (value: number): string => {
    const rounded = roundTo(value, currencyPlaces, rounding)
    if (currencySymbol) {
      // Explicit symbol from settings — format with the symbol + locale
      // grouping, honoring the currency's minor-unit scale.
      const grouped = new Intl.NumberFormat(locale, {
        minimumFractionDigits: currencyPlaces,
        maximumFractionDigits: currencyPlaces,
      }).format(rounded)
      return `${currencySymbol}${grouped}`
    }
    return new Intl.NumberFormat(locale, {
      style: "currency",
      currency: currencyCode,
    }).format(rounded)
  }

  const number = (value: number): string =>
    new Intl.NumberFormat(locale, {
      minimumFractionDigits: 0,
      maximumFractionDigits: decimalScale,
    }).format(roundTo(value, decimalScale, rounding))

  const date = (value: string | Date): string => {
    const d = typeof value === "string" ? new Date(value) : value
    if (Number.isNaN(d.getTime())) return String(value)
    return formatDateByPattern(d, dateFormat, locale)
  }

  const dateTime = (value: string | Date): string => {
    const d = typeof value === "string" ? new Date(value) : value
    if (Number.isNaN(d.getTime())) return String(value)
    return d.toLocaleString(locale)
  }

  const relative = (value: string | Date): string => {
    const d = typeof value === "string" ? new Date(value) : value
    if (Number.isNaN(d.getTime())) return String(value)
    const now = new Date()
    const diff = now.getTime() - d.getTime()
    const mins = Math.floor(diff / 60000)
    if (mins < 1) return "Just now"
    if (mins < 60) return `${mins}m ago`
    const hours = Math.floor(mins / 60)
    if (hours < 24) return `${hours}h ago`
    const days = Math.floor(hours / 24)
    if (days < 7) return `${days}d ago`
    return formatDateByPattern(d, dateFormat, locale)
  }

  return { money, number, date, dateTime, relative }
}

/**
 * Format a Date using a simple token pattern (YYYY, MM, DD, HH, mm, ss).
 * Falls back to the locale's date format for unknown patterns.
 */
function formatDateByPattern(d: Date, pattern: string, locale: string): string {
  const pad = (n: number) => String(n).padStart(2, "0")
  const tokens: Record<string, string> = {
    YYYY: String(d.getFullYear()),
    MM: pad(d.getMonth() + 1),
    DD: pad(d.getDate()),
    HH: pad(d.getHours()),
    mm: pad(d.getMinutes()),
    ss: pad(d.getSeconds()),
  }
  // Replace known tokens; if the pattern contains none, use locale date.
  let out = pattern
  let matched = false
  for (const [token, val] of Object.entries(tokens)) {
    if (pattern.includes(token)) {
      out = out.split(token).join(val)
      matched = true
    }
  }
  if (!matched) return d.toLocaleDateString(locale)
  return out
}

/**
 * Parse a date string per a token pattern (YYYY, MM, DD, HH, mm, ss) into the
 * value format the native input expects:
 *   - date patterns → "YYYY-MM-DD"
 *   - datetime patterns → "YYYY-MM-DDTHH:mm:ss"
 * Returns null when the input doesn't fully match the pattern or is an
 * impossible date (e.g. 31/02). Used by DateInput so users can type a date in
 * the global `date_format` instead of only using the calendar picker.
 */
export function parseDateByPattern(
  input: string,
  pattern: string,
): string | null {
  const tokenRe: Record<string, string> = {
    YYYY: "(\\d{4})",
    MM: "(\\d{1,2})",
    DD: "(\\d{1,2})",
    HH: "(\\d{1,2})",
    mm: "(\\d{1,2})",
    ss: "(\\d{1,2})",
  }

  // Build a regex from the pattern: tokens → capture groups, other chars escaped.
  let re = ""
  let i = 0
  while (i < pattern.length) {
    let matched = false
    for (const len of [4, 2]) {
      const tok = pattern.slice(i, i + len)
      if (tokenRe[tok]) {
        re += tokenRe[tok]
        i += len
        matched = true
        break
      }
    }
    if (!matched) {
      re += pattern[i].replace(/[.*+?^${}()|[\]\\]/g, "\\$&")
      i++
    }
  }
  const m = input.trim().match(new RegExp("^" + re + "$"))
  if (!m) return null

  // Extract values in token order.
  const vals: Record<string, number> = {}
  let gi = 1
  i = 0
  while (i < pattern.length) {
    let matched = false
    for (const len of [4, 2]) {
      const tok = pattern.slice(i, i + len)
      if (tokenRe[tok]) {
        vals[tok] = parseInt(m[gi], 10)
        gi++
        i += len
        matched = true
        break
      }
    }
    if (!matched) i++
  }

  const year = vals["YYYY"] ?? new Date().getFullYear()
  const month = (vals["MM"] ?? 1) - 1
  const day = vals["DD"] ?? 1
  const hour = vals["HH"] ?? 0
  const min = vals["mm"] ?? 0
  const sec = vals["ss"] ?? 0
  const d = new Date(year, month, day, hour, min, sec)
  if (Number.isNaN(d.getTime())) return null
  // Reject impossible dates (e.g. 31/02) via round-trip.
  if (
    d.getFullYear() !== year ||
    d.getMonth() !== month ||
    d.getDate() !== day
  ) {
    return null
  }

  const pad = (n: number) => String(n).padStart(2, "0")
  const datePart = `${year}-${pad(month + 1)}-${pad(day)}`
  const hasTime =
    pattern.includes("HH") || pattern.includes("mm") || pattern.includes("ss")
  if (hasTime) {
    return `${datePart}T${pad(hour)}:${pad(min)}:${pad(sec)}`
  }
  return datePart
}

/**
 * Auto-format a date string as the user types: strips non-digits and inserts
 * the pattern's separators. E.g. "24082026" → "24/08/2026" for DD/MM/YYYY,
 * "240820261430" → "24/08/2026 14:30" for DD/MM/YYYY HH:mm. Returns the
 * formatted partial string (may be incomplete while typing).
 */
export function formatDateInput(raw: string, pattern: string): string {
  const tokenLens: Record<string, number> = {
    YYYY: 4,
    MM: 2,
    DD: 2,
    HH: 2,
    mm: 2,
    ss: 2,
  }

  // Parse the pattern into segments: each token group carries the separator
  // that follows it (e.g. DD/MM/YYYY → [{2,"/"},{2,"/"},{4,""}]).
  const segments: { digits: number; sep: string }[] = []
  let i = 0
  while (i < pattern.length) {
    let matched = false
    for (const len of [4, 2]) {
      const tok = pattern.slice(i, i + len)
      if (tokenLens[tok]) {
        segments.push({ digits: tokenLens[tok], sep: "" })
        i += len
        matched = true
        break
      }
    }
    if (!matched) {
      if (segments.length > 0) {
        segments[segments.length - 1].sep += pattern[i]
      }
      i++
    }
  }

  const digits = raw.replace(/\D/g, "")
  let out = ""
  let di = 0
  for (const seg of segments) {
    const take = Math.min(seg.digits, digits.length - di)
    if (take > 0) {
      out += digits.slice(di, di + take)
      di += take
    }
    // Insert the separator only when this group is full and more digits remain.
    if (take === seg.digits && di < digits.length) {
      out += seg.sep
    }
  }
  return out
}
