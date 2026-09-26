// ─── Session Context Picker ───
//
// Shown when login answers 409 `CONTEXT_REQUIRED`: the principal holds more
// than one assignment (role × branch) and the server refuses to pick one for
// them (backend §8.7 — "Server tidak pernah memilih boundary atas nama
// pemanggil"). The caller picks, and login is retried with that id.
//
// Prefilled from the last choice on this device, falling back to the first
// option — one click to continue, but the boundary is still something the
// caller stated, so the audit answer ("as which role, in which branch")
// remains true. Submitting is never automatic.

import { useEffect, useRef, useState } from "react"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import type { ContextChoice } from "@/types/manifest"

interface ContextPickerProps {
  choices: ContextChoice[]
  /** Id to preselect (remembered choice, else the first). */
  defaultId?: string
  /** Retry login with the chosen assignment. */
  onSubmit: (assignment: string) => void
  /** Disables the form while the retried login is in flight. */
  busy?: boolean
}

export function ContextPicker({
  choices,
  defaultId,
  onSubmit,
  busy = false,
}: ContextPickerProps) {
  const [selected, setSelected] = useState(defaultId ?? choices[0]?.id ?? "")
  // Focus the group on mount so keyboard users land on the choice rather than
  // having to tab through the whole login form again.
  const firstRef = useRef<HTMLInputElement>(null)
  useEffect(() => {
    firstRef.current?.focus()
  }, [])

  if (choices.length === 0) return null

  const label = (c: ContextChoice) => `${c.role} · ${c.value}`

  return (
    <form
      className="space-y-4"
      onSubmit={(e) => {
        e.preventDefault()
        if (selected) onSubmit(selected)
      }}
    >
      <p className="text-sm text-muted-foreground">
        You can act in more than one context. Choose the one to sign in as —
        permissions and data are scoped to it.
      </p>

      <div role="radiogroup" aria-label="Session context" className="space-y-2">
        {choices.map((choice, i) => (
          <label
            key={choice.id}
            className={cn(
              "flex cursor-pointer items-center gap-3 rounded-lg border border-border px-3 py-2.5 text-sm transition-colors",
              selected === choice.id
                ? "border-foreground/30 bg-muted/60"
                : "hover:bg-muted/40",
            )}
          >
            <input
              ref={i === 0 ? firstRef : undefined}
              type="radio"
              name="assignment"
              value={choice.id}
              checked={selected === choice.id}
              onChange={() => setSelected(choice.id)}
              disabled={busy}
              className="size-4 shrink-0 accent-foreground"
            />
            <span className="flex min-w-0 flex-col">
              <span className="font-medium">{label(choice)}</span>
              <span className="truncate text-xs text-muted-foreground">
                {choice.dimension} = {choice.value}
              </span>
            </span>
          </label>
        ))}
      </div>

      <Button type="submit" className="w-full" disabled={busy || !selected}>
        {busy ? "Signing in…" : "Continue"}
      </Button>
    </form>
  )
}
