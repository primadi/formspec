// ─── RelationPicker Widget ───
//
// Searchable combobox for belongs_to relation fields.
// Fetches candidate records from the related entity's REST API,
// debounces user input, and displays a dropdown of matching results.
//
// Design:
//   - Looks up the target EntitySchema via entityField.relation.resource
//   - Calls GET /{module}/{plural}?search=... with debounce
//   - Shows results in a floating dropdown
//   - On select → onChange(record.id)
//   - Readonly mode: display label_field value (fetched by ID if needed)

import { useState, useEffect, useRef, useMemo, useCallback } from "react"
import { Search, Loader2, X, Check } from "lucide-react"

import type { EntitySchema, Field } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { apiList, apiGet } from "@/lib/api"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

// ── Props ──

interface RelationPickerProps {
  /** Currently selected record ID */
  value?: string
  onChange?: (value: string) => void
  /** The entity field spec — must have relation.resource */
  entityField: Field
  /** Module of the entity that owns this field (for entity lookup) */
  currentModule: string
  placeholder?: string
  readonly?: boolean
  error?: string
}

interface SearchResult {
  id: string
  [key: string]: unknown
}

// ── Entity Resolution ──

/**
 * Resolve a relation resource reference to an EntitySchema.
 *
 * Supports two formats:
 *   - "patient"           → same-module reference
 *   - "clinic/visit"      → cross-module reference (module/name)
 */
function resolveRelatedEntity(
  bundle: { entities: EntitySchema[] } | null,
  resource: string | undefined,
  currentModule: string,
): EntitySchema | undefined {
  if (!bundle || !resource) return undefined

  // Cross-module: "module/name"
  if (resource.includes("/")) {
    const [mod, name] = resource.split("/", 2)
    return bundle.entities.find((e) => e.module === mod && e.name === name)
  }

  // Same module — try current module first
  let found = bundle.entities.find(
    (e) => e.module === currentModule && e.name === resource,
  )
  if (found) return found

  // Fallback: search all modules
  return bundle.entities.find((e) => e.name === resource)
}

// ── Component ──

export function RelationPicker({
  value,
  onChange,
  entityField,
  currentModule,
  placeholder,
  readonly = false,
  error,
}: RelationPickerProps) {
  const getClient = useSessionStore((s) => s.getClient)
  const bundle = useMetaStore((s) => s.bundle)
  const [isOpen, setIsOpen] = useState(false)
  const [query, setQuery] = useState("")
  const [results, setResults] = useState<SearchResult[]>([])
  const [loading, setLoading] = useState(false)
  const [selectedLabel, setSelectedLabel] = useState<string>("")
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(undefined)
  const inputRef = useRef<HTMLInputElement>(null)
  const dropdownRef = useRef<HTMLDivElement>(null)

  // Resolve the related entity schema
  const relatedEntity = useMemo(
    () => resolveRelatedEntity(bundle, entityField.relation?.resource, currentModule),
    [bundle, entityField.relation?.resource, currentModule],
  )

  const moduleName = relatedEntity?.module ?? currentModule
  // API endpoints use the singular entity name (entity.Name), not plural.
  const entityName = relatedEntity?.name ?? entityField.relation?.resource ?? ""
  const labelField = relatedEntity?.label_field ?? "id"
  // Secondary identifier shown alongside label_field in search results to
  // distinguish records with identical display names (e.g. same-name patients).
  // Uses the first unique non-label_field, or common identifier field names.
  const secondaryField = useMemo(() => {
    if (!relatedEntity) return undefined
    const candidates = ["nik", "code", "phone", "email", "number", "license_number"]
    // Prefer the first unique field that isn't the label_field
    const uniqueField = relatedEntity.fields.find(
      (f) => f.unique && f.name !== labelField && f.name !== "id",
    )
    if (uniqueField) return uniqueField.name
    // Fallback to common identifier names
    return candidates.find((c) =>
      relatedEntity.fields.some((f) => f.name === c && f.name !== labelField),
    )
  }, [relatedEntity, labelField])

  /** Build a display label for a search result item, optionally including secondary info. */
  const formatLabel = useCallback(
    (item: SearchResult): string => {
      const primary = String(item[labelField] ?? item.id)
      const secondary =
        secondaryField && item[secondaryField]
          ? String(item[secondaryField])
          : undefined
      return secondary ? `${primary} (${secondary})` : primary
    },
    [labelField, secondaryField],
  )

  // ── Load label for current value (on mount / value change) ──
  useEffect(() => {
    if (!value || !relatedEntity || !entityName) {
      setSelectedLabel("")
      return
    }

    // If we already have a matching result cached, use it
    const cached = results.find((r) => r.id === value)
    if (cached) {
      setSelectedLabel(formatLabel(cached))
      return
    }

    // Fetch the record by ID
    let cancelled = false
    const fetchLabel = async () => {
      try {
        const client = getClient()
        const record = await apiGet<SearchResult>(
          client,
          `${moduleName}/${entityName}/${value}`,
        )
        if (!cancelled) {
          setSelectedLabel(formatLabel(record as SearchResult))
        }
      } catch {
        if (!cancelled) setSelectedLabel(value)
      }
    }
    fetchLabel()
    return () => { cancelled = true }
  }, [value, relatedEntity, moduleName, entityName, labelField, secondaryField, getClient, results, formatLabel])

  // ── Search ──
  const doSearch = useCallback(
    async (q: string) => {
      if (!q.trim()) {
        setResults([])
        return
      }

      setLoading(true)
      try {
        const client = getClient()
        const { items } = await apiList<SearchResult>(client, `${moduleName}/${entityName}`, {
          search: q.trim(),
          per_page: "10",
        })
        setResults(items ?? [])
      } catch {
        setResults([])
      } finally {
        setLoading(false)
      }
    },
    [moduleName, entityName, getClient],
  )

  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current)
    if (!isOpen || readonly) return
    debounceRef.current = setTimeout(() => doSearch(query), 300)
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
    }
  }, [query, isOpen, readonly, doSearch])

  // ── Auto-focus input when opening ──
  // Without this, the shadcn Input's blockAutofill mechanism leaves the
  // field readOnly until focused, preventing the user from typing directly
  // after clicking the label display.
  useEffect(() => {
    if (isOpen) {
      requestAnimationFrame(() => inputRef.current?.focus())
    }
  }, [isOpen])

  // ── Click outside to close ──
  useEffect(() => {
    if (!isOpen) return
    const handleClickOutside = (e: MouseEvent) => {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(e.target as Node) &&
        inputRef.current &&
        !inputRef.current.contains(e.target as Node)
      ) {
        setIsOpen(false)
      }
    }
    document.addEventListener("mousedown", handleClickOutside)
    return () => document.removeEventListener("mousedown", handleClickOutside)
  }, [isOpen])

  // ── Select ──
  const handleSelect = (item: SearchResult) => {
    setSelectedLabel(formatLabel(item))
    setQuery("")
    setResults([])
    setIsOpen(false)
    onChange?.(item.id)
  }

  // ── Clear ──
  const handleClear = () => {
    setSelectedLabel("")
    setQuery("")
    setResults([])
    onChange?.("")
    inputRef.current?.focus()
  }

  // ── Render: readonly mode ──
  if (readonly) {
    return (
      <div className="py-1 text-sm">
        {selectedLabel || "-"}
      </div>
    )
  }

  return (
    <div className="relative">
      {/* Input area: show label if selected, otherwise show search input */}
      <div className="relative">
        {selectedLabel && !isOpen ? (
          <div
            role="combobox"
            aria-haspopup="listbox"
            aria-expanded={false}
            tabIndex={0}
            className={cn(
              "flex h-8 w-full items-center rounded-lg border border-input bg-transparent px-2.5 text-sm",
              "cursor-pointer transition-colors hover:border-ring",
              "focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring",
              error && "border-destructive",
            )}
            onClick={() => {
              setIsOpen(true)
              setQuery("")
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault()
                setIsOpen(true)
                setQuery("")
              }
            }}
          >
            <Search className="mr-2 size-3.5 shrink-0 text-muted-foreground" />
            <span className="flex-1 truncate">{selectedLabel}</span>
            <button
              type="button"
              className="ml-1 rounded p-0.5 text-muted-foreground hover:bg-accent hover:text-foreground"
              onClick={(e) => {
                e.stopPropagation()
                handleClear()
              }}
              tabIndex={-1}
            >
              <X className="size-3.5" />
            </button>
          </div>
        ) : (
          <div className="relative">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
              ref={inputRef}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onFocus={() => setIsOpen(true)}
              onBlur={(e) => {
                // Tab/Shift+Tab away doesn't fire the click-outside listener
                // below, so without this the dropdown is left open, covering
                // whatever field comes next. Skip closing when focus is
                // landing on the dropdown itself (e.g. a result button).
                const next = e.relatedTarget as Node | null
                if (next && dropdownRef.current?.contains(next)) return
                setIsOpen(false)
              }}
              placeholder={selectedLabel || placeholder || "Cari..."}
              className={cn("pl-8", error && "border-destructive")}
            />
            {loading && (
              <Loader2 className="absolute right-2.5 top-1/2 size-4 -translate-y-1/2 animate-spin text-muted-foreground" />
            )}
          </div>
        )}
      </div>

      {/* Dropdown */}
      {isOpen && (
        <div
          ref={dropdownRef}
          className={cn(
            "absolute z-50 mt-1 w-full rounded-lg border border-border bg-popover shadow-md",
            "max-h-60 overflow-auto",
          )}
        >
          {loading && results.length === 0 && (
            <div className="flex items-center justify-center gap-2 px-3 py-6 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" />
              Mencari...
            </div>
          )}

          {!loading && query.trim() && results.length === 0 && (
            <div className="px-3 py-6 text-center text-sm text-muted-foreground">
              Tidak ditemukan
            </div>
          )}

          {!query.trim() && results.length === 0 && !loading && (
            <div className="px-3 py-6 text-center text-sm text-muted-foreground">
              Ketik untuk mencari
            </div>
          )}

          {results.map((item) => {
            const primaryLabel = String(item[labelField] ?? item.id)
            const secondaryLabel =
              secondaryField && item[secondaryField]
                ? String(item[secondaryField])
                : undefined

            return (
              <button
                key={item.id}
                type="button"
                className={cn(
                  "flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors",
                  "hover:bg-accent hover:text-accent-foreground",
                  item.id === value && "bg-accent/50 font-medium",
                )}
                onClick={() => handleSelect(item)}
              >
                <span className="flex-1 truncate">
                  <span>{primaryLabel}</span>
                  {secondaryLabel && (
                    <span className="ml-2 text-xs text-muted-foreground">
                      {secondaryLabel}
                    </span>
                  )}
                </span>
                {item.id === value && (
                  <Check className="size-3.5 shrink-0 text-primary" />
                )}
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}
