// ─── Text Input Widget ───
//
// For string fields. Switches to Textarea when max_length > 120.

import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"

interface TextInputProps {
  value?: string
  onChange?: (value: string) => void
  placeholder?: string
  readonly?: boolean
  required?: boolean
  maxLength?: number
  error?: string
  /** id forwarded to the input so <label htmlFor> can target it */
  id?: string
}

export function TextInput({
  value = "",
  onChange,
  placeholder,
  readonly = false,
  required = false,
  maxLength,
  error,
  id,
}: TextInputProps) {
  const isTextarea = maxLength != null && maxLength > 120

  if (readonly) {
    return (
      <div className="py-1 text-sm">
        {value ||
          (required ? (
            <span className="text-muted-foreground italic">Empty</span>
          ) : null)}
      </div>
    )
  }

  if (isTextarea) {
    return (
      <div>
        <Textarea
          id={id}
          value={value}
          onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) =>
            onChange?.(e.target.value)
          }
          placeholder={placeholder}
          maxLength={maxLength}
          className={error ? "border-destructive" : ""}
        />
      </div>
    )
  }

  return (
    <Input
      id={id}
      value={value}
      onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
        onChange?.(e.target.value)
      }
      placeholder={placeholder}
      maxLength={maxLength}
      className={error ? "border-destructive" : ""}
    />
  )
}
