// ─── RadioGroup Widget ───
//
// Single-choice picker — alternative to `select` (opt-in via
// `widget: radio-group`). Button-based, no external dependency.
//
// Reads a field's declared choice set when it has one (`Field.options`), so a
// caption can differ from the stored value (`1` → "Senin").

import { cn } from "@/lib/utils"
import type { SelectChoice } from "@/widgets/Select"

interface RadioGroupProps {
  value?: unknown
  onChange?: (value: unknown) => void
  options: SelectChoice[]
  readonly?: boolean
  error?: string
  /** Accessible name for the radiogroup (set from the field label) */
  label?: string
}

export function RadioGroup({
  value,
  onChange,
  options,
  readonly = false,
  error,
  label,
}: RadioGroupProps) {
  const choices = options.map((c) =>
    typeof c === "string"
      ? {
          key: c,
          label: c.charAt(0).toUpperCase() + c.slice(1).replace(/_/g, " "),
          value: c as string | number | boolean,
        }
      : { key: c.key, label: c.label, value: c.value },
  )
  const current = String(value ?? "")
  const selected = choices.find((c) => c.key === current)

  if (readonly) {
    return (
      <div className="py-1 text-sm">
        {current === "" ? "-" : (selected?.label ?? current)}
      </div>
    )
  }

  return (
    <div
      className={cn(
        "flex flex-wrap gap-x-4 gap-y-2",
        error && "text-destructive",
      )}
      role="radiogroup"
      aria-label={label}
    >
      {choices.map((opt) => {
        const checked = current === opt.key
        return (
          <label
            key={opt.key}
            className="flex cursor-pointer items-center gap-2 text-sm"
          >
            <button
              type="button"
              role="radio"
              aria-checked={checked}
              // The declared scalar goes back to form state, so a numeric
              // option set keeps storing numbers.
              onClick={() => onChange?.(opt.value)}
              className={cn(
                "inline-flex h-4 w-4 items-center justify-center rounded-full border transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2",
                checked ? "border-primary" : "border-input",
              )}
            >
              {checked && <span className="h-2 w-2 rounded-full bg-primary" />}
            </button>
            {opt.label}
          </label>
        )
      })}
    </div>
  )
}
