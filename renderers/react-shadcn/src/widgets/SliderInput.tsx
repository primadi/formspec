// ─── Slider Input Widget ───
//
// For integer/decimal fields with min/max — native range input. Opt-in via
// `widget: slider`.

import { cn } from "@/lib/utils"

interface SliderInputProps {
  value?: number | null
  onChange?: (value: number) => void
  min?: number
  max?: number
  step?: number
  readonly?: boolean
  error?: string
}

export function SliderInput({
  value,
  onChange,
  min = 0,
  max = 100,
  step = 1,
  readonly = false,
  error,
}: SliderInputProps) {
  const v = typeof value === "number" ? value : min

  if (readonly) {
    return <div className="py-1 text-sm">{value ?? "-"}</div>
  }

  return (
    <div className={cn("flex items-center gap-3", error && "text-destructive")}>
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={v}
        disabled={readonly}
        onChange={(e) => onChange?.(Number(e.target.value))}
        className="h-2 w-full cursor-pointer appearance-none rounded-full bg-input accent-primary"
      />
      <span className="w-12 shrink-0 text-right text-sm tabular-nums">{v}</span>
    </div>
  )
}
