// ─── Checkbox ───
//
// Simple checkbox (button-based, no external dependency). Matches the
// shadcn-style visual language used across the shell.

import { cn } from "@/lib/utils"

interface CheckboxProps {
  checked?: boolean
  onCheckedChange?: (checked: boolean | "indeterminate") => void
  disabled?: boolean
  className?: string
}

export function Checkbox({
  checked = false,
  onCheckedChange,
  disabled,
  className,
}: CheckboxProps) {
  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={checked}
      onClick={() => onCheckedChange?.(!checked)}
      disabled={disabled}
      className={cn(
        "inline-flex h-4 w-4 shrink-0 items-center justify-center rounded border border-input transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50",
        checked
          ? "border-primary bg-primary text-primary-foreground"
          : "bg-background",
        className,
      )}
    >
      {checked && (
        <svg
          width="10"
          height="10"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="3"
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <path d="M20 6 9 17l-5-5" />
        </svg>
      )}
    </button>
  )
}
