// ─── Number Input Widget ───
//
// For integer and decimal fields. Precision-aware for decimal types.

import { Input } from "@/components/ui/input"
import { createFormatter } from "@/lib/format"
import { useMetaStore } from "@/stores/meta"

interface NumberInputProps {
  value?: number | null
  onChange?: (value: number | null) => void
  placeholder?: string
  readonly?: boolean
  min?: number
  max?: number
  step?: number
  /** Max digits after the decimal point (spec `scale`, 05-field-types.md §1.2).
   *  When set, the input is constrained to `scale` decimal places — extra
   *  digits are blocked on keydown and rounded away on change/paste. */
  scale?: number
  error?: string
  /** true = value must be > 0 (spec `positive` rule). */
  positive?: boolean
  /** true = integer field (parseInt, block decimal/exponent input).
   *  false/undefined = decimal field (parseFloat). Do NOT infer from `step`
   *  — callers pass `step` inconsistently (decimal often passes undefined). */
  integer?: boolean
}

export function NumberInput({
  value,
  onChange,
  placeholder,
  readonly = false,
  min,
  max,
  step,
  scale,
  error,
  integer = false,
  positive = false,
}: NumberInputProps) {
  if (readonly) {
    const settings = useMetaStore.getState().bundle?.settings
    const formatter = createFormatter(settings)
    return (
      <div className="py-1 text-sm tabular-nums">
        {value != null ? formatter.number(value) : "-"}
      </div>
    )
  }

  // Range validation — flag out-of-range values (red border + red text +
  // tooltip) instead of silently clamping or ignoring them, so the user
  // knows why the value is invalid and can correct it. Only applies to real
  // numbers (empty/null values are not errors).
  const rangeError =
    typeof value === "number" &&
    ((positive && value <= 0) ||
      (min != null && value < min) ||
      (max != null && value > max))
      ? positive && value <= 0
        ? "Harus lebih dari 0"
        : min != null && max != null
          ? `Nilai antara ${min} dan ${max}`
          : min != null
            ? `Minimal ${min}`
            : `Maksimal ${max}`
      : undefined

  const effectiveError = error ?? rangeError

  // Spinner boundary: `positive` stops at the smallest positive step (so the
  // up/down arrows never go to 0 or negative); explicit min/max are respected
  // by the native number input.
  const positiveMin = integer
    ? 1
    : scale != null
      ? 1 / Math.pow(10, scale)
      : 0.01
  const inputMin = positive ? Math.max(min ?? 0, positiveMin) : min

  // Block keys that would violate the field's numeric constraints.
  // NOTE: only the integer case blocks here — `type="number"` inputs do not
  // expose selectionStart/End (they're null), so a scale-based keydown block
  // can't tell "typing at the end" from "select-all + replace" and would
  // wrongly block the latter. The scale constraint is enforced by the
  // sanitize-on-change below (toFixed), which handles every edit path.
  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (integer) {
      if (e.key === "." || e.key === "," || e.key === "e" || e.key === "E") {
        e.preventDefault()
      }
    }
  }

  return (
    <Input
      type="number"
      value={value ?? ""}
      onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
        const v = e.target.value
        if (v === "") {
          onChange?.(null)
        } else {
          let n = integer ? parseInt(v, 10) : parseFloat(v)
          if (!isNaN(n) && !integer && scale != null) {
            // Constrain to `scale` decimal places (rounds; also covers pasted
            // values like "1.234" → 1.23 with scale=2).
            n = Number(n.toFixed(scale))
          }
          onChange?.(isNaN(n) ? null : n)
        }
      }}
      onKeyDown={handleKeyDown}
      placeholder={placeholder}
      min={inputMin}
      max={max}
      step={
        step ?? (integer ? 1 : scale != null ? 1 / Math.pow(10, scale) : "any")
      }
      className={`tabular-nums ${
        effectiveError ? "border-destructive text-destructive" : ""
      }`}
      title={effectiveError}
    />
  )
}
