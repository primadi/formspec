// ─── Textarea Input Widget ───
//
// For `type: text` (multi-line) fields and explicit `widget: textarea`.
// A dedicated multi-line input — distinct from TextInput, which only
// switches to a textarea when max_length > 120.

import { Textarea } from "@/components/ui/textarea"

interface TextareaInputProps {
  value?: string
  onChange?: (value: string) => void
  placeholder?: string
  readonly?: boolean
  maxLength?: number
  error?: string
  rows?: number
}

export function TextareaInput({
  value = "",
  onChange,
  placeholder,
  readonly = false,
  maxLength,
  error,
  rows = 4,
}: TextareaInputProps) {
  if (readonly) {
    return (
      <div className="py-1 text-sm whitespace-pre-wrap">{value || "-"}</div>
    )
  }

  return (
    <Textarea
      value={value}
      onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) =>
        onChange?.(e.target.value)
      }
      placeholder={placeholder}
      maxLength={maxLength}
      rows={rows}
      className={error ? "border-destructive" : ""}
    />
  )
}
