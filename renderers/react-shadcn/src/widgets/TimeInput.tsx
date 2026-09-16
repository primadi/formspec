// ─── TimeInput Widget ───
//
// For `type: time` fields (time-of-day, `HH:MM:SS`) — gap #1 / item 2.14. The
// field used to render as a plain text input, so the only guidance was the field
// title; a typo became a 422 at submit.
//
// The native `time` control is used on purpose: it gives a real time picker on
// touch devices (the happy-hour promo form is filled on a tablet), it serializes
// to `HH:MM[:SS]`, and it needs no parsing of its own. `step=1` keeps seconds
// editable for fields that store them.

import { cn } from "@/lib/utils"

export interface TimeInputProps {
  value?: unknown
  onChange?: (value: unknown) => void
  readonly?: boolean
  error?: string
  /** Include seconds in the control (`HH:MM:SS`). Default: false (`HH:MM`). */
  withSeconds?: boolean
  id?: string
  disabled?: boolean
  className?: string
}

/** Trims a stored value to what an <input type="time"> accepts. */
function toControlValue(value: unknown, withSeconds: boolean): string {
  if (typeof value !== "string") return ""
  const m = /^(\d{2}):(\d{2})(?::(\d{2}))?/.exec(value.trim())
  if (!m) return ""
  return withSeconds ? `${m[1]}:${m[2]}:${m[3] ?? "00"}` : `${m[1]}:${m[2]}`
}

export function TimeInput({
  value,
  onChange,
  readonly = false,
  error,
  withSeconds = false,
  id,
  disabled,
  className,
}: TimeInputProps) {
  const current = typeof value === "string" ? value : ""

  if (readonly) {
    return (
      <span className={cn("text-sm tabular-nums", className)}>
        {current || "—"}
      </span>
    )
  }

  return (
    <div className={cn("space-y-1", className)}>
      <input
        id={id}
        type="time"
        step={withSeconds ? 1 : 60}
        value={toControlValue(current, withSeconds)}
        disabled={disabled}
        onChange={(e) => {
          const v = e.target.value
          // Empty means "no time set" — cleared, not midnight.
          onChange?.(
            v === "" ? null : withSeconds || v.length > 5 ? v : `${v}:00`,
          )
        }}
        className={cn(
          "w-full rounded-md border bg-transparent px-3 py-1.5 tabular-nums",
          error ? "border-destructive" : "border-input",
          disabled && "opacity-50",
        )}
      />
      {error && <div className="text-xs text-destructive">{error}</div>}
    </div>
  )
}
