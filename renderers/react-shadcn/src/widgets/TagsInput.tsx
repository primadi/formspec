// ─── Tags Input Widget ───
//
// Multi-select widget that accepts two value shapes:
//   - string  → stored as a comma-separated string on a string field
//   - string[] → stored as a JSON array (entity field type: json, e.g. roles)
// The value kind is preserved: array in → array out, string in → string out.
// Opt-in via `widget: tags`.

import { useState, useRef, type KeyboardEvent } from "react"
import { X } from "lucide-react"
import { cn } from "@/lib/utils"

type TagsValue = string | string[]

interface TagsInputProps {
  value?: TagsValue
  onChange?: (value: TagsValue) => void
  placeholder?: string
  readonly?: boolean
  error?: string
  /** id forwarded to the tag input so <label htmlFor> can target it */
  id?: string
}

/** Normalize any incoming value shape to a tag list. Never throws —
 * unsupported shapes (json object, null) degrade to an empty list. */
function parseTags(value?: TagsValue): string[] {
  if (value == null) return []
  if (Array.isArray(value)) {
    return value.map((t) => String(t).trim()).filter(Boolean)
  }
  if (typeof value === "string") {
    return value
      .split(",")
      .map((t) => t.trim())
      .filter(Boolean)
  }
  return []
}

/** Serialize a tag list back into the value shape given on input.
 * `isArray` must be decided by the caller's schema (json → array),
 * not by the current emptiness of the value. */
function serializeTags(tags: string[], isArray: boolean): TagsValue {
  if (isArray) return tags
  return tags.join(",")
}

export function TagsInput({
  value = "",
  onChange,
  placeholder,
  readonly = false,
  error,
  id,
}: TagsInputProps) {
  const [draft, setDraft] = useState("")
  const inputRef = useRef<HTMLInputElement>(null)
  const tags = parseTags(value)
  const isArray = Array.isArray(value)

  const emit = (next: string[]) => {
    onChange?.(serializeTags(next, isArray))
  }

  const commit = () => {
    const t = draft.trim()
    if (t && !tags.includes(t)) {
      emit([...tags, t])
    }
    setDraft("")
  }

  const remove = (tag: string) => {
    emit(tags.filter((t) => t !== tag))
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
          id={id}
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
