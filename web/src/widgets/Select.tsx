// ─── Select Widget ───
//
// For enum fields. Renders a themed dropdown with the allowed enum values.

import { Select as ThemedSelect } from "@/components/ui/select"

interface SelectProps {
  value?: string
  onChange?: (value: string) => void
  options: string[]
  placeholder?: string
  readonly?: boolean
  error?: string
}

export function Select({
  value,
  onChange,
  options,
  placeholder,
  readonly = false,
  error,
}: SelectProps) {
  if (readonly) {
    return (
      <div className="py-1 text-sm">
        {value || "-"}
      </div>
    )
  }

  return (
    <ThemedSelect
      value={value ?? ""}
      onChange={(v) => onChange?.(v)}
      options={options}
      placeholder={placeholder}
      disabled={readonly}
      error={!!error}
    />
  )
}
