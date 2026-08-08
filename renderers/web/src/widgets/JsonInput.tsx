// ─── JSON Input Widget ───
//
// For `type: json` fields (freeform JSON blob — e.g. patient.allergies).
// A textarea holding pretty-printed JSON, parsed live with inline error
// feedback. Not a full tree/code editor — just enough to author/inspect
// small JSON values without hand-typing them into a single-line text input.

import { useEffect, useState } from "react"
import { Textarea } from "@/components/ui/textarea"

interface JsonInputProps {
  value?: unknown
  onChange?: (value: unknown) => void
  readonly?: boolean
  placeholder?: string
  error?: string
}

export function JsonInput({
  value,
  onChange,
  readonly = false,
  placeholder,
  error,
}: JsonInputProps) {
  const [text, setText] = useState(() => stringify(value))
  const [parseError, setParseError] = useState<string | null>(null)

  // Re-sync when the underlying value changes for reasons other than typing
  // in this widget (record load, form reset).
  useEffect(() => {
    setText(stringify(value))
    setParseError(null)
  }, [value])

  if (readonly) {
    return (
      <pre className="py-1 text-xs font-mono whitespace-pre-wrap break-words text-muted-foreground">
        {text || "-"}
      </pre>
    )
  }

  return (
    <div>
      <Textarea
        value={text}
        onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => {
          const next = e.target.value
          setText(next)
          if (next.trim() === "") {
            setParseError(null)
            onChange?.(undefined)
            return
          }
          try {
            const parsed = JSON.parse(next)
            setParseError(null)
            onChange?.(parsed)
          } catch {
            setParseError("JSON tidak valid")
          }
        }}
        placeholder={placeholder ?? "[]"}
        rows={4}
        className={`font-mono text-xs ${error || parseError ? "border-destructive" : ""}`}
      />
      {parseError && <p className="text-xs text-destructive mt-1">{parseError}</p>}
    </div>
  )
}

function stringify(value: unknown): string {
  if (value == null) return ""
  if (typeof value === "string") {
    // Some persistence layers hand back an already-serialized JSON string —
    // pretty-print it if it parses, otherwise show it verbatim.
    try {
      return JSON.stringify(JSON.parse(value), null, 2)
    } catch {
      return value
    }
  }
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}
