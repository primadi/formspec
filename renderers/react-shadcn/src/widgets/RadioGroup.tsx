// ─── RadioGroup Widget ───
//
// For enum single-choice — alternative to `select` (opt-in via
// `widget: radio-group`). Button-based, no external dependency.

import { cn } from "@/lib/utils"

interface RadioGroupProps {
  value?: string
  onChange?: (value: string) => void
  options: string[]
  readonly?: boolean
  error?: string
}

export function RadioGroup({
  value,
  onChange,
  options,
  readonly = false,
  error,
}: RadioGroupProps) {
  if (readonly) {
    return <div className="py-1 text-sm">{value || "-"}</div>
  }

  return (
    <div
      className={cn(
        "flex flex-wrap gap-x-4 gap-y-2",
        error && "text-destructive",
      )}
      role="radiogroup"
    >
      {options.map((opt) => {
        const label =
          opt.charAt(0).toUpperCase() + opt.slice(1).replace(/_/g, " ")
        const checked = value === opt
        return (
          <label
            key={opt}
            className="flex cursor-pointer items-center gap-2 text-sm"
          >
            <button
              type="button"
              role="radio"
              aria-checked={checked}
              onClick={() => onChange?.(opt)}
              className={cn(
                "inline-flex h-4 w-4 items-center justify-center rounded-full border transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2",
                checked ? "border-primary" : "border-input",
              )}
            >
              {checked && <span className="h-2 w-2 rounded-full bg-primary" />}
            </button>
            {label}
          </label>
        )
      })}
    </div>
  )
}
