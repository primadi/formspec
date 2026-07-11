// ─── Select Widget ───
//
// For enum fields. Renders a dropdown with the allowed enum values.

import { SelectNative } from "@/components/ui/select-native"

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
    <SelectNative
      value={value ?? ""}
      onChange={(e) => onChange?.(e.target.value)}
      disabled={readonly}
      className={error ? "border-destructive" : ""}
    >
      {placeholder && <option value="">{placeholder}</option>}
      {options.map((opt) => (
        <option key={opt} value={opt}>
          {opt.charAt(0).toUpperCase() + opt.slice(1).replace(/_/g, " ")}
        </option>
      ))}
    </SelectNative>
  )
}
