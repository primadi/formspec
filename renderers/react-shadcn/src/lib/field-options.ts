// ─── Field options — the declared choice set behind the pickers ───
//
// A field's `options:` (backend 05-field-types.md §1.1.1) is a closed set of
// values *with captions*, and `multiple` says **how many** of them the field
// holds. Both live on the Entity, because "which values are legal" and "one or
// many" are properties of the **data** — a Form's `widget:` only renders the
// decision, it does not make it. `enum_values` remains the value-only contract
// for `enum` (no captions, and a database CHECK constraint behind it).
//
// Everything that needs to agree on this list lives here, in one place:
//
//   1. `fieldOptions()`   — resolve `options`, falling back to `enum_values`
//                           (value-only, humanised caption);
//   2. `optionKey()`      — the canonical identity of a value, matching
//                           `spec.OptionValueKey` on the Go side, so `1` and
//                           `"1"` are the same choice and a stored value can be
//                           matched against a declared option;
//   3. `isMultiValue()` / `optionValueShape()` — the Entity's cardinality,
//                           mirrored 1:1 from `spec.FieldIsMultiple`;
//   4. `parseOptionValue` / `serializeOptionValue` (set) and
//                           `parseSingleOptionValue` / `serializeSingleOptionValue`
//                           (one value) — the value shape contract.
//
// Why one module: this is the same "one vocabulary, several call sites" shape
// that produced earlier gaps (a value rendered raw in one place and formatted in
// another). Duplicating the key rule would let the picker and the read-only
// renderer disagree about whether a stored value is "known"; duplicating the
// cardinality rule would let a form and a table disagree about whether the same
// field holds one value or a set.

import type { Field, FieldOption } from "@/types/manifest"
import { humanizeFieldName, isMultiValue } from "@/engine/derive"

export { isMultiValue }

/** The value shape a field holds. `array` for a `json` set, `string` for a
 *  comma-separated set, `scalar` for a single declared value. */
export type OptionValueShape = "array" | "string" | "scalar"

/** A resolved option: the stored value plus the caption to show. */
export interface ResolvedOption {
  value: string | number | boolean
  label: string
  /** Canonical key — compare with `optionKey()`, never with `===`. */
  key: string
}

/**
 * Canonical identity of an option value. Mirrors `spec.OptionValueKey` in
 * `pkg/spec/entity.go`: numbers, strings, and booleans all reduce to their
 * text form so `1` (YAML int) and `"1"` (JSON round-trip) are one choice.
 */
export function optionKey(value: unknown): string {
  if (value === null || value === undefined) return ""
  if (typeof value === "boolean") return value ? "true" : "false"
  return String(value)
}

/** Humanise a bare option value into a caption: `in_progress` → `In Progress`. */
function labelFor(value: string | number | boolean): string {
  if (typeof value === "string") return humanizeFieldName(value)
  return String(value)
}

/**
 * Resolve the declared choices for a field.
 *
 * `options` wins (it carries captions); `enum_values` is the value-only
 * fallback so a field that already declares an enum-like set keeps working
 * without restating it. A value appearing in both is listed once.
 */
export function fieldOptions(field: Field | undefined): ResolvedOption[] {
  if (!field) return []

  const out: ResolvedOption[] = []
  const seen = new Set<string>()

  const push = (value: string | number | boolean, label?: string) => {
    const key = optionKey(value)
    if (key === "" || seen.has(key)) return
    seen.add(key)
    out.push({ value, label: label?.trim() || labelFor(value), key })
  }

  for (const o of field.options ?? []) {
    if (o && o.value !== undefined && o.value !== null) push(o.value, o.label)
  }
  for (const v of field.enum_values ?? []) push(v)

  return out
}

/** True when the field declares a choice set at all. */
export function hasFieldOptions(field: Field | undefined): boolean {
  return fieldOptions(field).length > 0
}

/** Look up the declared option for a value, by canonical key. */
export function findOption(
  options: ResolvedOption[],
  value: unknown,
): ResolvedOption | undefined {
  const key = optionKey(value)
  return options.find((o) => o.key === key)
}

/** The caption for a value: its declared label, else the raw value humanised. */
export function optionLabel(options: ResolvedOption[], value: unknown): string {
  const found = findOption(options, value)
  if (found) return found.label
  // Unknown value (legacy data, or the declaration changed): show it rather
  // than blanking it — a dropped value reads as "the field never had one".
  if (typeof value === "boolean") return value ? "true" : "false"
  const raw = optionKey(value)
  return raw === "" ? "" : labelFor(raw)
}

/** The value shape a field's widget must preserve (see `isMultiValue`). */
export function optionValueShape(field: Field | undefined): OptionValueShape {
  if (!isMultiValue(field)) return "scalar"
  return field?.type === "json" ? "array" : "string"
}

/**
 * Order values by their position in the declaration, with undeclared values
 * last (keeping their relative order).
 *
 * One function, used by both the form widget and the read-only cell/detail
 * renderers: an ordered set (`Senin..Jumat`) must read the same in a table
 * column as it does in the form. Two orderings of the same value would be the
 * "one vocabulary, several call sites" split this module exists to prevent.
 *
 * This is a **display** order only — the stored array keeps the order values
 * were entered in, so opening and saving a form never rewrites the record.
 */
export function orderByDeclaration(
  values: (string | number | boolean)[],
  options: ResolvedOption[],
): (string | number | boolean)[] {
  const rank = new Map(options.map((o, i) => [o.key, i]))
  return [...values].sort((a, b) => {
    const ra = rank.get(optionKey(a))
    const rb = rank.get(optionKey(b))
    if (ra === undefined && rb === undefined) return 0
    if (ra === undefined) return 1
    if (rb === undefined) return -1
    return ra - rb
  })
}

/**
 * Normalise a stored value into the selected option values, keeping the
 * declared scalar type where the value is known (so a `json` field saves `[1]`,
 * not `["1"]`) and the raw form where it is not.
 *
 * Non-list values (a bare scalar, an object, `null`) degrade to `[]` — but a
 * non-list *is* a real difference from an empty list to a consumer, so callers
 * that can show an error should check `isListValue()` first.
 */
export function parseOptionValue(
  value: unknown,
  options: ResolvedOption[],
): (string | number | boolean)[] {
  let raw: unknown[]
  if (Array.isArray(value)) {
    raw = value
  } else if (typeof value === "string") {
    if (value.trim() === "") return []
    raw = value
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean)
  } else {
    return []
  }

  const out: (string | number | boolean)[] = []
  const seen = new Set<string>()
  for (const item of raw) {
    const declared = findOption(options, item)
    // A known value keeps its declared type; an unknown one is kept as-is when
    // scalar (the user must be able to see and remove it) and dropped when it
    // could not be shown as a chip.
    const kept =
      declared?.value ??
      (typeof item === "string" ||
      typeof item === "number" ||
      typeof item === "boolean"
        ? item
        : undefined)
    if (kept === undefined) continue
    const key = optionKey(kept)
    if (key === "" || seen.has(key)) continue
    seen.add(key)
    out.push(kept)
  }
  return out
}

/**
 * True when the stored value is a list shape this widget can edit (an array, or
 * a comma-separated string). A bare object/number is a *different* value, not
 * an empty list — the widget surfaces that instead of silently replacing it.
 */
export function isListValue(value: unknown): boolean {
  return Array.isArray(value) || typeof value === "string" || value == null
}

/** Serialize selected values back into the field's declared shape. */
export function serializeOptionValue(
  values: (string | number | boolean)[],
  shape: OptionValueShape,
): unknown {
  if (shape === "array") return values
  return values.map((v) => String(v)).join(",")
}

/**
 * Normalise a stored single value against the declared options, keeping the
 * declared scalar type where the value is known — the single-value twin of
 * `parseOptionValue`, so `1` from a `value: 1` declaration is stored as the
 * number 1 rather than "1".
 *
 * An empty/absent value stays `undefined` (the caller renders the placeholder);
 * an unknown value is kept as-is when scalar, because a legacy record or a
 * shrunk declaration must remain visible and removable rather than silently
 * blanked on the next save (the policy `select-multi-tag` already follows).
 */
export function parseSingleOptionValue(
  value: unknown,
  options: ResolvedOption[],
): string | number | boolean | undefined {
  if (value === null || value === undefined || value === "") return undefined
  // A set value on a field declared single is a different value, not a choice:
  // surfacing it as no selection would hide the mismatch behind an empty
  // control, and saving would drop it.
  if (typeof value === "object") return undefined
  const declared = findOption(options, value)
  if (declared) return declared.value
  return typeof value === "string" ||
    typeof value === "number" ||
    typeof value === "boolean"
    ? value
    : undefined
}

/**
 * Serialize a single selection back into the field's shape: the declared scalar
 * as-is, the empty selection as `null` (an explicit "cleared" the server can
 * apply, rather than `""` which would be stored as a string on a numeric
 * field).
 */
export function serializeSingleOptionValue(
  value: string | number | boolean | undefined,
): unknown {
  return value === undefined ? null : value
}

/** Type guard for the manifest's `FieldOption` array (used by tests). */
export function isFieldOption(v: unknown): v is FieldOption {
  return (
    typeof v === "object" &&
    v !== null &&
    "value" in (v as Record<string, unknown>)
  )
}
