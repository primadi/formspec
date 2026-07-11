// ─── Switch Widget ───
//
// For boolean fields. Renders a switch/toggle.

interface SwitchProps {
  value?: boolean
  onChange?: (value: boolean) => void
  readonly?: boolean
  label?: string
}

export function Switch({ value = false, onChange, readonly = false, label }: SwitchProps) {
  if (readonly) {
    return (
      <div className="py-1 text-sm">
        {value ? "Yes" : "No"}
      </div>
    )
  }

  return (
    <label className="flex items-center gap-2 cursor-pointer">
      <button
        type="button"
        role="switch"
        aria-checked={value}
        onClick={() => onChange?.(!value)}
        disabled={readonly}
        className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50 ${
          value ? "bg-primary" : "bg-input"
        }`}
      >
        <span
          className={`pointer-events-none block h-4 w-4 rounded-full bg-background shadow-lg ring-0 transition-transform ${
            value ? "translate-x-4" : "translate-x-0"
          }`}
        />
      </button>
      {label && <span className="text-sm">{label}</span>}
    </label>
  )
}
