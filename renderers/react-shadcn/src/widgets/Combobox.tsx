// ─── Combobox Widget ───
//
// Searchable select for large enums — opt-in via `widget: combobox`.
// Custom dropdown (button + search + list), no external dependency.

import { useState, useRef, useEffect } from "react"
import { ChevronDown, Search, Check } from "lucide-react"
import { cn } from "@/lib/utils"

interface ComboboxProps {
  value?: string
  onChange?: (value: string) => void
  options: string[]
  placeholder?: string
  readonly?: boolean
  error?: string
}

export function Combobox({
  value,
  onChange,
  options,
  placeholder,
  readonly = false,
  error,
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

  const filtered = options.filter((o) =>
    o.toLowerCase().includes(query.toLowerCase()),
  )
  const selected = options.find((o) => o === value)

  if (readonly) {
    return <div className="py-1 text-sm">{value || "-"}</div>
  }

  return (
    <div ref={containerRef} className="relative">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className={cn(
          "flex h-8 w-full items-center justify-between rounded-lg border border-input bg-transparent px-2.5 text-sm focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
          error && "border-destructive",
        )}
      >
        <span className={cn("truncate", !selected && "text-muted-foreground")}>
          {selected
            ? selected.charAt(0).toUpperCase() +
              selected.slice(1).replace(/_/g, " ")
            : (placeholder ?? "Select…")}
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
              const label =
                opt.charAt(0).toUpperCase() + opt.slice(1).replace(/_/g, " ")
              const isSelected = opt === value
              return (
                <button
                  key={opt}
                  type="button"
                  onClick={() => {
                    onChange?.(opt)
                    setOpen(false)
                    setQuery("")
                  }}
                  className={cn(
                    "flex w-full items-center justify-between rounded px-2 py-1.5 text-left text-sm hover:bg-accent",
                    isSelected && "bg-accent",
                  )}
                >
                  {label}
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
