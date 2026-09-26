// ─── Combobox Widget ───
//
// Searchable select for large enums — opt-in via `widget: combobox`.
// Custom dropdown (button + search + list), no external dependency.

import { useState, useRef, useEffect } from "react"
import { ChevronDown, Search, Check } from "lucide-react"
import { cn } from "@/lib/utils"
import { normalizeChoice, type SelectChoice } from "@/widgets/Select"

interface ComboboxProps {
  /** The stored value (any declared scalar, or `null`/`""` for none). */
  value?: unknown
  onChange?: (value: unknown) => void
  options: SelectChoice[]
  placeholder?: string
  readonly?: boolean
  error?: string
  /** Accessible name for the trigger button (set from the field label) */
  label?: string
}

export function Combobox({
  value,
  onChange,
  options,
  placeholder,
  readonly = false,
  error,
  label,
}: ComboboxProps) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState("")
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const onClick = (e: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(e.target as Node)
      ) {
        setOpen(false)
      }
    }
    document.addEventListener("mousedown", onClick)
    return () => document.removeEventListener("mousedown", onClick)
  }, [])

  // Captions come from the declaration when there is one (`options`), so a
  // `1`-valued option reads "Senin" and search matches the caption, not the key.
  const choices = options.map(normalizeChoice)
  const current = value === null || value === undefined ? "" : String(value)
  const selected = choices.find((o) => o.key === current)

  const filtered = choices.filter((o) =>
    o.label.toLowerCase().includes(query.toLowerCase()),
  )

  if (readonly) {
    return (
      <div className="py-1 text-sm">
        {current === "" ? "-" : (selected?.label ?? current)}
      </div>
    )
  }

  return (
    <div ref={containerRef} className="relative">
      <button
        type="button"
        aria-label={label}
        onClick={() => setOpen((o) => !o)}
        className={cn(
          "flex h-8 w-full items-center justify-between rounded-lg border border-input bg-transparent px-2.5 text-sm focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
          error && "border-destructive",
        )}
      >
        <span className={cn("truncate", !selected && "text-muted-foreground")}>
          {selected ? selected.label : (placeholder ?? "Select…")}
        </span>
        <ChevronDown className="size-4 shrink-0 text-muted-foreground" />
      </button>

      {open && (
        <div className="absolute z-50 mt-1 w-full rounded-lg border border-border bg-popover p-1 shadow-lg">
          <div className="flex items-center gap-1.5 border-b border-border px-2 py-1">
            <Search className="size-3.5 text-muted-foreground" />
            <input
              autoFocus
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search…"
              className="h-6 w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground"
            />
          </div>
          <div className="max-h-48 overflow-auto py-1">
            {filtered.length === 0 && (
              <div className="px-2 py-1.5 text-sm text-muted-foreground">
                No results
              </div>
            )}
            {filtered.map((opt) => {
              const isSelected = opt.key === current
              return (
                <button
                  key={opt.key}
                  type="button"
                  onClick={() => {
                    // The declared scalar, not the key — a numeric option set
                    // must keep storing numbers.
                    onChange?.(opt.value)
                    setOpen(false)
                    setQuery("")
                  }}
                  className={cn(
                    "flex w-full items-center justify-between rounded px-2 py-1.5 text-left text-sm hover:bg-accent",
                    isSelected && "bg-accent",
                  )}
                >
                  {opt.label}
                  {isSelected && <Check className="size-4 text-primary" />}
                </button>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
