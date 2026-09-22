// ─── Password Input Widget ───
//
// For string fields with sensitive values — masked input with a reveal
// toggle. Opt-in via `widget: password`.

import { useState } from "react"
import { Eye, EyeOff } from "lucide-react"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

interface PasswordInputProps {
  value?: string
  onChange?: (value: string) => void
  placeholder?: string
  readonly?: boolean
  error?: string
  /** id forwarded to the input so <label htmlFor> can target it */
  id?: string
  /** name forwarded to the input — password managers use it as a hint */
  name?: string
  /** autocomplete token (Chromium password-form guidance) */
  autoComplete?: string
}

export function PasswordInput({
  value = "",
  onChange,
  placeholder,
  readonly = false,
  error,
  id,
  name,
  autoComplete,
}: PasswordInputProps) {
  const [show, setShow] = useState(false)

  if (readonly) {
    return <div className="py-1 text-sm">{value ? "••••••••" : "-"}</div>
  }

  return (
    <div className="relative">
      <Input
        id={id}
        name={name}
        autoComplete={autoComplete}
        type={show ? "text" : "password"}
        value={value}
        onChange={(e) => onChange?.(e.target.value)}
        placeholder={placeholder}
        className={cn("pr-9", error && "border-destructive")}
      />
      <button
        type="button"
        onClick={() => setShow((s) => !s)}
        title={show ? "Hide" : "Show"}
        aria-label={show ? "Hide password" : "Show password"}
        aria-pressed={show}
        className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
      >
        {show ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
      </button>
    </div>
  )
}
