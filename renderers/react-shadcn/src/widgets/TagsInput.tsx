// ─── Tags Input Widget ───
//
// Multi-select stored as a comma-separated string on a string field
// (frontend-only — no backend change). Opt-in via `widget: tags`.

import { useState, useRef, type KeyboardEvent } from "react"
import { X } from "lucide-react"
import { cn } from "@/lib/utils"

interface TagsInputProps {
  value?: string // comma-separated
  onChange?: (value: string) => void
  placeholder?: string
  readonly?: boolean
  error?: string
}

function parseTags(value?: string): string[] {
  return (value ?? "")
    .split(",")
    .map((t) => t.trim())
    .filter(Boolean)
}

export function TagsInput({
  value = "",
  onChange,
  placeholder,
  readonly = false,
  error,
}: TagsInputProps) {
  const [draft, setDraft] = useState("")
  const inputRef = useRef<HTMLInputElement>(null)
  const tags = parseTags(value)

  const commit = () => {
    const t = draft.trim()
    if (t && !tags.includes(t)) {
      onChange?.([...tags, t].join(","))
    }
    setDraft("")
  }

  const remove = (tag: string) => {
    onChange?.(tags.filter((t) => t !== tag).join(","))
  }

  if (readonly) {
    return (
      <div className="flex flex-wrap gap-1.5 py-1">
        {tags.length === 0 ? (
          <span className="text-sm text-muted-foreground italic">-</span>
        ) : (
          tags.map((t) => (
            <span
              key={t}
              className="rounded-full bg-accent px-2 py-0.5 text-xs"
            >
              {t}
            </span>
          ))
        )}
      </div>
    )
  }

  return (
    <div
      className={cn(
        "rounded-lg border border-input bg-transparent p-1.5",
        error && "border-destructive",
      )}
    >
      <div className="flex flex-wrap gap-1.5">
        {tags.map((t) => (
          <span
            key={t}
            className="inline-flex items-center gap-1 rounded-full bg-accent px-2 py-0.5 text-xs"
          >
            {t}
            <button
              type="button"
              onClick={() => remove(t)}
              className="text-muted-foreground hover:text-foreground"
            >
              <X className="size-3" />
            </button>
          </span>
        ))}
        <input
          ref={inputRef}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e: KeyboardEvent<HTMLInputElement>) => {
            if (e.key === "Enter" || e.key === ",") {
              e.preventDefault()
              commit()
            } else if (
              e.key === "Backspace" &&
              draft === "" &&
              tags.length > 0
            ) {
              remove(tags[tags.length - 1])
            }
          }}
          onBlur={commit}
          placeholder={
            tags.length === 0 ? (placeholder ?? "Type and press Enter…") : ""
          }
          className="h-6 min-w-24 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
        />
      </div>
    </div>
  )
}
