// ─── Date Input Widget ───
//
// For date/datetime fields. Displays the value in the global `date_format`
// (spec §10 — settings.date_format) while supporting BOTH input methods:
//
//   - TYPE: the text input is editable — type a date in the global format
//     (e.g. "24/08/2026" for DD/MM/YYYY); it is parsed and stored as ISO on
//     the fly. On blur the field re-formats to the canonical display.
//   - PICK: the calendar icon (or Space/Enter) calls `showPicker()` on a
//     hidden native <input type="date|datetime-local">, opening the browser
//     calendar; its onChange updates the ISO value.
//
// This keeps input consistent with the global format instead of the browser
// locale (which native date inputs always follow).

import { useRef, useState } from "react"
import { CalendarIcon } from "lucide-react"
import { Input } from "@/components/ui/input"
import {
  createFormatter,
  formatDateInput,
  parseDateByPattern,
} from "@/lib/format"
import { useMetaStore } from "@/stores/meta"
import { cn } from "@/lib/utils"

interface DateInputProps {
  value?: string
  onChange?: (value: string) => void
  readonly?: boolean
  withTime?: boolean
  error?: string
  /** Extra classes for the visible text input (sizing, etc.). */
  className?: string
}

export function DateInput({
  value = "",
  onChange,
  readonly = false,
  withTime = false,
  error,
  className,
}: DateInputProps) {
  const settings = useMetaStore((s) => s.bundle?.settings)
  const formatter = createFormatter(settings)
  const nativeRef = useRef<HTMLInputElement>(null)
  // While the user is typing, hold the raw text so it isn't re-formatted
  // under their cursor; null = show the canonical formatted value.
  const [text, setText] = useState<string | null>(null)

  if (readonly) {
    return (
      <div className="py-1 text-sm">
        {value
          ? withTime
            ? formatter.dateTime(value)
            : formatter.date(value)
          : "-"}
      </div>
    )
  }

  const displayValue =
    text ??
    (value
      ? withTime
        ? formatter.dateTime(value)
        : formatter.date(value)
      : "")
  const placeholder = settings?.date_format || "YYYY-MM-DD"

  const openPicker = () => {
    const native = nativeRef.current
    if (!native) return
    try {
      native.showPicker?.()
    } catch {
      // Fallback for browsers without showPicker(): focus the native input,
      // which opens the picker on focus for date inputs.
      native.focus()
      native.click()
    }
  }

  const handleChange = (raw: string) => {
    // Auto-insert separators as the user types: "24082026" → "24/08/2026".
    const formatted = formatDateInput(raw, placeholder)
    setText(formatted)
    // Parse a complete date in the global format → update the ISO value.
    const parsed = parseDateByPattern(formatted, placeholder)
    if (parsed) onChange?.(parsed)
  }

  const handleBlur = () => {
    // Revert to the canonical formatted display (drop partial typing).
    setText(null)
  }

  return (
    <div className="relative">
      <Input
        type="text"
        value={displayValue}
        placeholder={placeholder}
        onChange={(e) => handleChange(e.target.value)}
        onBlur={handleBlur}
        onKeyDown={(e) => {
          if (e.key === " " || e.key === "Enter") {
            e.preventDefault()
            openPicker()
          }
        }}
        className={cn(error ? "border-destructive" : "", className, "pr-8")}
      />
      <button
        type="button"
        onClick={openPicker}
        aria-label={withTime ? "Pilih tanggal dan waktu" : "Pilih tanggal"}
        className="absolute right-1.5 top-1/2 -translate-y-1/2 rounded p-1 text-muted-foreground hover:text-foreground"
      >
        <CalendarIcon className="size-4" />
      </button>
      {/* Hidden native picker — showPicker() opens its calendar. Kept rendered
          (sr-only, not display:none) so showPicker() can anchor to it. */}
      <input
        ref={nativeRef}
        type={withTime ? "datetime-local" : "date"}
        value={value}
        onChange={(e) => {
          onChange?.(e.target.value)
          setText(null)
        }}
        aria-hidden
        tabIndex={-1}
        className="sr-only"
      />
    </div>
  )
}
