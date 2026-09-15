// ─── PickerPanel — the UI of a child field's `picker` (S1) ───
//
// A view over the child field's *form state*: it never keeps its own copy of
// the rows. Picking writes rows back through the same `onChange` the child grid
// uses, so the grid, the panel and the submit payload can never disagree — and
// editing a row in the grid is immediately reflected here.
//
// Two placements (declared by the Form's `render.picker_panel`):
//   - `inline` (default): tiles only; picked rows show up in the child grid
//     below, which stays the editor. The picker is a convenient "add" affordance.
//   - `aside`: tiles *and* the picked-row editor (quantity, note, remove, running
//     total) in their own column; the Form skips the child grid for that field,
//     because this panel replaces it.

import { useEffect, useMemo, useState } from "react"
import { useParams } from "react-router-dom"
import { Loader2, Minus, Plus, Search, Trash2 } from "lucide-react"
import { toast } from "@/lib/ui"

import { apiList } from "@/lib/api"
import { createFormatter, moneyAmount } from "@/lib/format"
import {
  buildRows,
  clampQuantity,
  DEFAULT_MAX_QUANTITY,
  interpolateFilter,
  pickRow,
  pickedCount,
  pickedTotal,
  pickerTiles,
  priceIndex,
  removeRow,
  rowTotal,
  setNote,
  setQuantity,
  type PickedRow,
} from "@/lib/picker"
import { resolveEntityRef } from "@/engine/entityRef"
import { useMetaStore } from "@/stores/meta"
import { useSessionStore } from "@/stores/session"
import type { Field, PickerDecl } from "@/types/manifest"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { EmptyState } from "@/components/ui/empty-state"
import { cn } from "@/lib/utils"

interface Props {
  decl: PickerDecl
  /** Owning module of the entity the picker belongs to (bare refs resolve here). */
  module?: string
  /** The child field's current rows (form state). */
  value: Record<string, unknown>[] | undefined
  /** Write rows back — the same callback the child grid uses. */
  onChange: (rows: Record<string, unknown>[]) => void
  /** Resolved render context for `{token}` interpolation in filters. */
  context?: Record<string, unknown>
  /** `aside` shows the selected-row editor too; `inline` shows tiles only. */
  showSelection: boolean
  title?: string
}

/**
 * Read child rows back into the picker's own shape.
 *
 * Rows without a `ref_field` value are skipped: they were added directly in the
 * child grid, and the picker cannot express them (it addresses rows by source
 * id). The panel reports how many such rows exist instead of inventing ids.
 */
function readRows(
  value: Record<string, unknown>[] | undefined,
  decl: PickerDecl,
): PickedRow[] {
  const map = decl.map
  const rows: PickedRow[] = []
  for (const row of value ?? []) {
    const ref = String(row[map.ref_field] ?? "")
    if (!ref) continue
    rows.push({
      ref,
      name: map.name_field ? String(row[map.name_field] ?? "") : "",
      price: map.price_field ? row[map.price_field] : undefined,
      quantity: map.quantity_field ? Number(row[map.quantity_field] ?? 1) : 1,
      note: map.note_field ? String(row[map.note_field] ?? "") : undefined,
    })
  }
  return rows
}

export default function PickerPanel({
  decl,
  module,
  value,
  onChange,
  context,
  showSelection,
  title,
}: Props) {
  const getClient = useSessionStore((s) => s.getClient)
  const getEntity = useMetaStore((s) => s.getEntity)
  const settings = useMetaStore((s) => s.bundle?.settings)
  const formatter = useMemo(() => createFormatter(settings), [settings])
  const { workspace = "default" } = useParams<{ workspace: string }>()

  const display = decl.display
  const map = decl.map
  const maxQuantity = map.max_quantity ?? DEFAULT_MAX_QUANTITY
  const nameField = display.name_field ?? "name"
  const [sourceModule, sourceEntity] = resolveEntityRef(
    decl.entity,
    module ?? "",
  )
  const sourceMeta = getEntity(sourceModule, sourceEntity)

  const [rows, setRows] = useState<Record<string, unknown>[]>([])
  const [prices, setPrices] = useState<Map<string, unknown> | undefined>(
    undefined,
  )
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [search, setSearch] = useState("")
  const [category, setCategory] = useState("")

  const picked = useMemo(() => readRows(value, decl), [value, decl])
  const pickedRefs = useMemo(() => new Set(picked.map((r) => r.ref)), [picked])

  // ── Source fetch (+ optional price join) ──
  const filterKey = JSON.stringify(decl.filter ?? {})
  const priceFilterKey = JSON.stringify(display.price_filter ?? {})
  const ctxKey = JSON.stringify(context ?? {})
  useEffect(() => {
    let cancelled = false
    const load = async () => {
      setLoading(true)
      setError(null)
      try {
        const client = getClient()
        const ctx = JSON.parse(ctxKey) as Record<string, unknown>
        const { items } = await apiList<Record<string, unknown>>(
          client,
          `${sourceModule}/${sourceEntity}`,
          { per_page: "200", ...interpolateFilter(JSON.parse(filterKey), ctx) },
        )
        if (cancelled) return
        setRows(items)

        if (
          display.price_entity &&
          display.price_match_field &&
          display.price_field
        ) {
          const [priceModule, priceEntity] = resolveEntityRef(
            display.price_entity,
            module ?? "",
          )
          const priceRows = await apiList<Record<string, unknown>>(
            client,
            `${priceModule}/${priceEntity}`,
            {
              per_page: "500",
              ...interpolateFilter(JSON.parse(priceFilterKey), ctx),
            },
          )
          if (cancelled) return
          setPrices(
            priceIndex(
              priceRows.items,
              display.price_match_field,
              display.price_field,
            ),
          )
        } else {
          setPrices(undefined)
        }
      } catch (err) {
        if (!cancelled)
          setError(err instanceof Error ? err.message : "Gagal memuat data")
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    load()
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sourceModule, sourceEntity, filterKey, priceFilterKey, ctxKey])

  // ── Category chips (display only) ──
  const categoryField = display.category_field
  const categoryLabel = (row: Record<string, unknown>): string => {
    if (!categoryField) return ""
    const expanded = row[categoryField.replace(/_id$/, "")] as
      | Record<string, unknown>
      | undefined
    if (expanded && typeof expanded === "object" && "name" in expanded) {
      return String(expanded.name ?? "")
    }
    const raw = row[categoryField]
    return raw == null ? "" : String(raw)
  }
  const categories = useMemo(() => {
    if (!categoryField) return []
    const seen = new Set<string>()
    for (const row of rows) {
      const label = categoryLabel(row)
      if (label) seen.add(label)
    }
    return [...seen]
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows, categoryField])

  const visible = useMemo(() => {
    const needle = search.trim().toLowerCase()
    return rows.filter((row) => {
      if (category && categoryLabel(row) !== category) return false
      if (!needle) return true
      return String(row[nameField] ?? "")
        .toLowerCase()
        .includes(needle)
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows, search, category, nameField, categoryField])

  const priceField = display.price_field ?? "price"
  const tiles = useMemo(
    () => pickerTiles({ rows: visible, nameField, priceField, prices }),
    [visible, nameField, priceField, prices],
  )

  /** Rows in the child field the picker cannot address (added manually). */
  const manualCount = (value?.length ?? 0) - picked.length

  const writeBack = (next: PickedRow[]) => onChange(buildRows(next, map))

  const imageFieldType = (sourceMeta?.fields as Field[] | undefined)?.find(
    (f) => f.name === display.image_field,
  )?.type
  const imageUrl = (row: Record<string, unknown>): string | undefined => {
    if (!display.image_field) return undefined
    const raw = row[display.image_field]
    if (raw == null || raw === "") return undefined
    if (imageFieldType === "file" || imageFieldType === "attachment") {
      // A file/attachment value is an object key served by the *source*
      // entity's file route.
      return `/${workspace}/_ui/entity/${sourceModule}/${sourceEntity}/${String(row.id)}/${display.image_field}`
    }
    return String(raw)
  }

  const showChips = Boolean(display.search || categories.length > 1)
  const columns =
    display.columns && display.columns >= 2 && display.columns <= 4
      ? display.columns
      : 3

  return (
    <div className="flex flex-col gap-4">
      {title && <h3 className="text-sm font-semibold">{title}</h3>}

      {showChips && (
        <div className="flex flex-wrap items-center gap-2">
          {display.search && (
            <div className="relative min-w-48 flex-1">
              <Search className="absolute top-1/2 left-2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Cari…"
                className="pl-8"
                aria-label="Cari"
              />
            </div>
          )}
          {categories.length > 1 && (
            <div className="flex flex-wrap gap-1">
              <Button
                type="button"
                size="sm"
                variant={category === "" ? "default" : "outline"}
                onClick={() => setCategory("")}
              >
                Semua
              </Button>
              {categories.map((c) => (
                <Button
                  key={c}
                  type="button"
                  size="sm"
                  variant={category === c ? "default" : "outline"}
                  onClick={() => setCategory(c)}
                >
                  {c}
                </Button>
              ))}
            </div>
          )}
        </div>
      )}

      {loading && (
        <div className="flex items-center gap-2 py-8 text-sm text-muted-foreground">
          <Loader2 className="size-4 animate-spin" /> Memuat…
        </div>
      )}

      {!loading && error && (
        <div
          className="rounded-md border border-destructive/40 bg-destructive/5 p-4 text-sm text-destructive"
          role="alert"
        >
          {error}
        </div>
      )}

      {!loading && !error && tiles.length === 0 && (
        <EmptyState
          title={display.empty_text ?? "Belum ada data"}
          description="Tidak ada yang bisa dipilih."
        />
      )}

      <div
        className={cn(
          "grid gap-3",
          columns === 2 && "grid-cols-2",
          columns === 3 && "grid-cols-2 md:grid-cols-3",
          columns === 4 && "grid-cols-2 md:grid-cols-4",
        )}
      >
        {tiles.map((tile) => {
          const row = visible.find((r) => String(r.id ?? "") === tile.id)
          const image = row ? imageUrl(row) : undefined
          const price = moneyAmount(tile.price)
          const pickedRow = picked.find((r) => r.ref === tile.id)
          return (
            <button
              key={tile.id}
              type="button"
              disabled={!tile.pickable}
              title={
                tile.pickable ? undefined : "Belum ada harga untuk item ini"
              }
              onClick={() => {
                if (!tile.pickable) return
                if (!map.ref_field) {
                  toast.error(
                    "Picker tidak dikonfigurasi (map.ref_field kosong)",
                  )
                  return
                }
                writeBack(
                  pickRow(
                    picked,
                    { ref: tile.id, name: tile.name, price: tile.price },
                    map,
                  ),
                )
              }}
              className="group flex flex-col overflow-hidden rounded-lg border text-left transition-colors hover:border-primary/60 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-60"
              aria-label={`Pilih ${tile.name}`}
            >
              {image ? (
                <img
                  src={image}
                  alt=""
                  className="h-28 w-full object-cover"
                  loading="lazy"
                />
              ) : null}
              <div className="flex flex-1 flex-col gap-1 p-3">
                <span className="text-sm leading-snug font-medium">
                  {tile.name}
                </span>
                {display.description_field &&
                row?.[display.description_field] ? (
                  <span className="line-clamp-2 text-xs text-muted-foreground">
                    {String(row[display.description_field])}
                  </span>
                ) : null}
                {map.price_field && (
                  <span className="mt-auto pt-1 text-sm font-semibold tabular-nums">
                    {price === undefined ? "—" : formatter.money(price)}
                  </span>
                )}
              </div>
              {pickedRow && map.quantity_field && (
                <span className="bg-primary/10 px-3 py-1 text-xs font-medium text-primary">
                  {pickedRow.quantity}× dipilih
                </span>
              )}
            </button>
          )
        })}
      </div>

      {/* Selected rows — only when this panel replaces the child grid. */}
      {showSelection && (
        <div className="flex flex-col gap-3 rounded-lg border p-4">
          <div className="flex items-center justify-between text-sm font-semibold">
            <span>Dipilih</span>
            {picked.length > 0 && (
              <span className="rounded-full bg-primary/10 px-2 py-0.5 text-xs text-primary">
                {pickedCount(picked)}
              </span>
            )}
          </div>

          {picked.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              Pilih item untuk mulai mengisi.
            </p>
          ) : (
            <ul className="flex flex-col divide-y">
              {picked.map((line) => (
                <li key={line.ref} className="flex flex-col gap-2 py-3">
                  <div className="flex items-start justify-between gap-2">
                    <span className="text-sm leading-snug font-medium">
                      {line.name}
                    </span>
                    {map.price_field && (
                      <span className="text-sm tabular-nums text-muted-foreground">
                        {rowTotal(line) === undefined
                          ? "—"
                          : formatter.money(rowTotal(line)!)}
                      </span>
                    )}
                  </div>
                  {map.quantity_field && (
                    <div className="flex items-center gap-2">
                      <Button
                        type="button"
                        size="icon"
                        variant="outline"
                        className="size-7"
                        aria-label={`Kurangi ${line.name}`}
                        onClick={() =>
                          writeBack(
                            setQuantity(
                              picked,
                              line.ref,
                              line.quantity - 1,
                              maxQuantity,
                            ),
                          )
                        }
                      >
                        <Minus className="size-3" />
                      </Button>
                      <input
                        className="w-12 rounded-md border bg-transparent py-0.5 text-center text-sm tabular-nums"
                        value={line.quantity}
                        inputMode="numeric"
                        aria-label={`Jumlah ${line.name}`}
                        onChange={(e) =>
                          writeBack(
                            setQuantity(
                              picked,
                              line.ref,
                              clampQuantity(
                                Number(e.target.value) || 1,
                                maxQuantity,
                              ),
                              maxQuantity,
                            ),
                          )
                        }
                      />
                      <Button
                        type="button"
                        size="icon"
                        variant="outline"
                        className="size-7"
                        aria-label={`Tambah ${line.name}`}
                        onClick={() =>
                          writeBack(
                            setQuantity(
                              picked,
                              line.ref,
                              line.quantity + 1,
                              maxQuantity,
                            ),
                          )
                        }
                      >
                        <Plus className="size-3" />
                      </Button>
                      <Button
                        type="button"
                        size="icon"
                        variant="ghost"
                        className="ml-auto size-7 text-muted-foreground"
                        aria-label={`Hapus ${line.name}`}
                        onClick={() => writeBack(removeRow(picked, line.ref))}
                      >
                        <Trash2 className="size-3.5" />
                      </Button>
                    </div>
                  )}
                  {map.note_field && (
                    <Input
                      value={line.note ?? ""}
                      placeholder="Catatan per baris"
                      aria-label={`Catatan ${line.name}`}
                      onChange={(e) =>
                        writeBack(setNote(picked, line.ref, e.target.value))
                      }
                    />
                  )}
                  {!map.quantity_field && (
                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      className="h-7 self-start text-muted-foreground"
                      onClick={() => writeBack(removeRow(picked, line.ref))}
                    >
                      <Trash2 className="mr-1 size-3.5" /> Hapus
                    </Button>
                  )}
                </li>
              ))}
            </ul>
          )}

          {map.price_field && picked.length > 0 && (
            <div className="flex items-center justify-between border-t pt-3 text-sm font-semibold">
              <span>Total</span>
              <span className="tabular-nums">
                {formatter.money(pickedTotal(picked))}
              </span>
            </div>
          )}

          {manualCount > 0 && (
            <p className="text-xs text-muted-foreground">
              {manualCount} baris lain diisi manual (tanpa {map.ref_field}).
            </p>
          )}
          {pickedRefs.size !== picked.length && (
            <p className="text-xs text-destructive" role="alert">
              Ada baris duplikat (ref sama) yang tidak bisa dibedakan picker
              ini.
            </p>
          )}
        </div>
      )}
    </div>
  )
}
