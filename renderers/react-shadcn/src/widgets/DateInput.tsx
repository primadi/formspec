// ─── Date Input Widget ───
//
// For date/datetime fields. Native browser date/datetime-local picker —
// no extra dependency, works with keyboard entry and a calendar popup.

import { Input } from "@/components/ui/input"

interface DateInputProps {
  value?: string
  onChange?: (value: string) => void
  readonly?: boolean
  withTime?: boolean
  error?: string
}

export function DateInput({
  value = "",
  onChange,
  readonly = false,
  withTime = false,
  error,
}: DateInputProps) {
  if (readonly) {
    return (
      <div className="py-1 text-sm">
        {value ? (withTime ? new Date(value).toLocaleString() : new Date(value).toLocaleDateString()) : "-"}
      </div>
    )
  }

  return (
    <Input
      type={withTime ? "datetime-local" : "date"}
      value={value}
      onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange?.(e.target.value)}
      className={error ? "border-destructive" : ""}
    />
  )
}
