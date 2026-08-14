// ─── Number Input Widget ───
//
// For integer and decimal fields. Precision-aware for decimal types.

import { Input } from "@/components/ui/input"

interface NumberInputProps {
  value?: number | null
  onChange?: (value: number | null) => void
  placeholder?: string
  readonly?: boolean
  min?: number
  max?: number
  step?: number
  precision?: number
  error?: string
}

export function NumberInput({
  value,
  onChange,
  placeholder,
  readonly = false,
  min,
  max,
  step,
  error,
}: NumberInputProps) {
  if (readonly) {
    return (
      <div className="py-1 text-sm tabular-nums">
        {value != null ? value.toLocaleString() : "-"}
      </div>
    )
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
          const n = step === 1 || !step ? parseInt(v, 10) : parseFloat(v)
          onChange?.(isNaN(n) ? null : n)
        }
      }}
      placeholder={placeholder}
      min={min}
      max={max}
      step={step ?? (step === 1 ? "1" : "any")}
      className={`tabular-nums ${error ? "border-destructive" : ""}`}
    />
  )
}
