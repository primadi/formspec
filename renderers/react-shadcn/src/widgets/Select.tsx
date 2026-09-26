// ─── Select Widget ───
//
// Single-value picker. Reads a field's declared choice set when it has one
// (`Field.options` — values *with captions*, so `1` shows as "Senin"), and
// plain `enum_values` otherwise. The value keeps the declared scalar type: a
// `value: 1` option stores the number 1, not "1".

import { Select as ThemedSelect } from "@/components/ui/select"
import type { ResolvedOption } from "@/lib/field-options"

/** A choice this widget can render. A bare string is an `enum_values` entry
 *  (its caption is the value humanised); a `ResolvedOption` carries a declared
 *  caption and the scalar value to store. */
export type SelectChoice = string | ResolvedOption

/** Normalise both choice spellings into one shape the pickers can render. */
export function normalizeChoice(c: SelectChoice): {
  /** Canonical key — comparisons use this, never `===` on the value. */
  key: string
  label: string
  /** The scalar to hand back to form state (`1`, not `"1"`). */
  value: string | number | boolean
} {
  if (typeof c === "string") {
    return {
      key: c,
      label: c.charAt(0).toUpperCase() + c.slice(1).replace(/_/g, " "),
      value: c,
    }
  }
  return { key: c.key, label: c.label, value: c.value }
}

interface SelectProps {
  /** The stored value (any declared scalar, or `null`/`""` for none). */
  value?: unknown
  onChange?: (value: unknown) => void
  options: SelectChoice[]
  placeholder?: string
  readonly?: boolean
  error?: string
  id?: string
  /** Accessible name for the trigger button (set from the field label) */
  ariaLabel?: string
}

export function Select({
  value,
  onChange,
  options,
  placeholder,
  readonly = false,
  error,
  id,
  ariaLabel,
}: SelectProps) {
  // Compares by canonical key so `1` (YAML) and `"1"` (JSON round-trip) resolve
  // to the same choice — the rule `spec.OptionValueKey` / `optionKey` define.
  const choices = options.map(normalizeChoice)
  const current = value === null || value === undefined ? "" : String(value)
  const selected = choices.find((c) => c.key === current)

  if (readonly) {
    return (
      <div className="py-1 text-sm">
        {current === "" ? "-" : (selected?.label ?? current)}
      </div>
    )
  }

  return (
    <ThemedSelect
      value={selected?.key ?? current}
      // Hand back the declared scalar, not the stringified key: a numeric
      // option set must keep storing numbers (the shape `FieldOption.Value`
      // declares). Clearing yields `null` — an explicit "cleared" the server can
      // apply, rather than `""` stored as a string on a numeric field.
      onChange={(key) => {
        const picked = choices.find((c) => c.key === key)
        onChange?.(picked ? picked.value : key === "" ? null : key)
      }}
      options={choices.map((c) => ({ value: c.key, label: c.label }))}
      placeholder={placeholder}
      disabled={readonly}
      error={!!error}
      id={id}
      ariaLabel={ariaLabel}
    />
  )
}
