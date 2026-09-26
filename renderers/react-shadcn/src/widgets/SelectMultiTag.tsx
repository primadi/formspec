// ─── SelectMultiTag Widget ───
//
// Tag input whose values come from a *declared* set, not from typing. Opt-in via
// `widget: select-multi-tag` on a multi-value field (`json` array or
// comma-separated `string`).
//
// Why it exists next to `tags`: `tags` accepts whatever is typed, so a set the
// spec defines (`1=Senin .. 7=Minggu`) could not be enforced — `9` was storable.
// Here every chip comes from `Field.options` (falling back to `enum_values`):
//
//   - a value already selected is **no longer offered** in the picker, so the
//     same choice cannot be added twice;
//   - chips are ordered by **declaration order**, not click order, so an ordered
//     set (`Senin..Jumat`) reads naturally;
//   - a stored value that is **not** in the declaration (legacy data, or the
//     spec changed) is still shown, marked as unknown — never dropped on save;
//   - a non-list value (an object in a `json` field) shows an **error** instead
//     of being replaced by an empty list.
//
// The value shape is preserved: `json` in → array out, `string` in →
// comma-separated string out (`lib/field-options.ts` owns that contract).

import { useEffect, useMemo, useRef, useState } from "react"
import { ChevronDown, Plus, Search, X } from "lucide-react"
import { cn } from "@/lib/utils"
import {
  fieldOptions,
  isListValue,
  optionKey,
  optionValueShape,
  orderByDeclaration,
  parseOptionValue,
  serializeOptionValue,
  type ResolvedOption,
} from "@/lib/field-options"
import type { Field } from "@/types/manifest"

interface SelectMultiTagProps {
  value?: unknown
  onChange?: (value: unknown) => void
  /** The entity field — supplies `options`/`enum_values` and the value shape. */
  entityField: Field
  placeholder?: string
  readonly?: boolean
  error?: string
  /** id forwarded to the trigger button so <label htmlFor> can target it */
  id?: string
  /** Accessible name for the trigger button (set from the field label). The
   *  control is button-based, so it takes its name from aria-label rather than
   *  a <label for> — pointing a label at a button is an a11y error. */
  ariaLabel?: string
}

export function SelectMultiTag({
  value,
  onChange,
  entityField,
  placeholder,
  readonly = false,
  error,
  id,
  ariaLabel,
}: SelectMultiTagProps) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState("")
  const containerRef = useRef<HTMLDivElement>(null)

  const options = useMemo(() => fieldOptions(entityField), [entityField])
  // The shape follows the Entity's declared cardinality (`Field.multiple`) — a
  // set on `string` is comma-separated, a set on `json` an array. This widget
  // only ever renders a set (the gate in `formspec check` refuses it on a
  // single-value field), so the shape is never `scalar` here.
  const shape = optionValueShape(entityField)

  // Close on click outside — same behaviour as Combobox/Select.
  useEffect(() => {
    if (!open) return
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
  }, [open])

  // A value that is neither a list nor empty is a different value, not an empty
  // list — replacing it silently would destroy data on the next save.
  const malformed = !isListValue(value)
  const selected = useMemo(
    () => (malformed ? [] : parseOptionValue(value, options)),
    [value, options, malformed],
  )

  const isUnknown = (v: string | number | boolean) =>
    !options.some((o) => o.key === optionKey(v))

  // Declaration order wins so an ordered set reads in its own order; values not
  // in the declaration keep their stored order, after the known ones. Shares
  // `orderByDeclaration` with the read-only cell/detail renderers so the same
  // set cannot read differently in a table column than in the form.
  //
  // Deliberate: this is a *display* order only. The stored array keeps the order
  // the values were entered in, so merely opening and saving a form never
  // rewrites the record's data (a silent reorder would show up as a change in
  // every diff/audit for a field whose order carries no meaning).
  const orderedSelected = useMemo(
    () => orderByDeclaration(selected, options),
    [selected, options],
  )

  const emit = (next: (string | number | boolean)[]) => {
    onChange?.(serializeOptionValue(next, shape))
  }

  const add = (o: ResolvedOption) => {
    if (selected.some((v) => optionKey(v) === o.key)) return
    emit([...selected, o.value])
    setQuery("")
  }

  const remove = (v: string | number | boolean) => {
    emit(selected.filter((x) => optionKey(x) !== optionKey(v)))
  }

  const labelOf = (v: string | number | boolean) =>
    options.find((o) => o.key === optionKey(v))?.label ?? String(v)

  const available = options.filter(
    (o) =>
      !selected.some((v) => optionKey(v) === o.key) &&
      o.label.toLowerCase().includes(query.trim().toLowerCase()),
  )

  // ── Read-only: chips, no controls ──
  if (readonly) {
    return (
      <div className="flex flex-wrap gap-1.5 py-1">
        {orderedSelected.length === 0 ? (
          <span className="text-sm text-muted-foreground italic">-</span>
        ) : (
          orderedSelected.map((v) => (
            <span
              key={optionKey(v)}
              className={cn(
                "rounded-full bg-accent px-2 py-0.5 text-xs",
                isUnknown(v) &&
                  "border border-dashed border-muted-foreground/50 text-muted-foreground",
              )}
              title={isUnknown(v) ? "Not in the declared options" : undefined}
            >
              {labelOf(v)}
            </span>
          ))
        )}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-1">
      <div
        ref={containerRef}
        className={cn(
          "relative rounded-lg border border-input bg-transparent p-1.5",
          (error || malformed) && "border-destructive",
        )}
      >
        <div className="flex flex-wrap items-center gap-1.5">
          {orderedSelected.map((v) => (
            <span
              key={optionKey(v)}
              className={cn(
                "inline-flex items-center gap-1 rounded-full bg-accent px-2 py-0.5 text-xs",
                isUnknown(v) &&
                  "border border-dashed border-muted-foreground/50 text-muted-foreground",
              )}
              title={isUnknown(v) ? "Not in the declared options" : undefined}
            >
              {labelOf(v)}
              <button
                type="button"
                onClick={() => remove(v)}
                aria-label={`Remove ${labelOf(v)}`}
                className="cursor-pointer text-muted-foreground hover:text-foreground"
              >
                <X className="size-3" />
              </button>
            </span>
          ))}

          <button
            type="button"
            id={id}
            aria-label={ariaLabel ?? placeholder ?? "Add value"}
            aria-expanded={open}
            aria-haspopup="listbox"
            onClick={() => setOpen((o) => !o)}
            disabled={available.length === 0 && !open}
            className={cn(
              "inline-flex h-6 items-center gap-1 rounded-full px-2 text-xs text-muted-foreground hover:bg-accent",
              available.length === 0 &&
                "cursor-not-allowed opacity-50 hover:bg-transparent",
            )}
          >
            <Plus className="size-3" />
            {orderedSelected.length === 0
              ? (placeholder ?? "Pilih…")
              : "Tambah"}
            <ChevronDown className="size-3" />
          </button>
        </div>

        {open && (
          <div className="absolute z-50 mt-1 w-full rounded-lg border border-border bg-popover p-1 shadow-lg">
            <div className="flex items-center gap-1.5 border-b border-border px-2 py-1">
              <Search className="size-3.5 text-muted-foreground" />
              <input
                autoFocus
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Cari…"
                aria-label="Search options"
                className="h-6 w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground"
              />
            </div>
            <div
              className="max-h-48 overflow-auto py-1"
              role="listbox"
              aria-multiselectable="true"
            >
              {available.length === 0 && (
                <div className="px-2 py-1.5 text-sm text-muted-foreground">
                  No results
                </div>
              )}
              {available.map((o) => (
                <button
                  key={o.key}
                  type="button"
                  role="option"
                  aria-selected={false}
                  onClick={() => add(o)}
                  className="flex w-full items-center justify-between rounded px-2 py-1.5 text-left text-sm hover:bg-accent"
                >
                  {o.label}
                </button>
              ))}
            </div>
          </div>
        )}
      </div>

      {malformed && (
        <p className="text-xs text-destructive">
          Nilai field ini bukan daftar — perbaiki di editor JSON sebelum memakai
          pilihan tag.
        </p>
      )}
    </div>
  )
}

/** Read-only chip list for a multi-value field, used by DetailPage so a stored
 *  set of declared values reads as labelled chips instead of raw JSON. */
export function OptionChips({
  value,
  entityField,
}: {
  value: unknown
  entityField: Field
}) {
  const options = fieldOptions(entityField)
  const selected = orderByDeclaration(parseOptionValue(value, options), options)

  if (selected.length === 0) {
    return <span className="text-sm text-muted-foreground italic">-</span>
  }

  return (
    <div className="flex flex-wrap gap-1.5">
      {selected.map((v) => {
        const found = options.find((o) => o.key === optionKey(v))
        return (
          <span
            key={optionKey(v)}
            className={cn(
              "rounded-full bg-accent px-2 py-0.5 text-xs",
              !found &&
                "border border-dashed border-muted-foreground/50 text-muted-foreground",
            )}
            title={!found ? "Not in the declared options" : undefined}
          >
            {found?.label ?? String(v)}
          </span>
        )
      })}
    </div>
  )
}
