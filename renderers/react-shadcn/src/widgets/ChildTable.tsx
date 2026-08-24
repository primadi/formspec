// ─── Child Table Widget ───
//
// For `type: child` fields — a jsonb-backed repeatable row grid (e.g.
// visit.treatments, otc-sale.items). Renders one column per sub-field,
// add/remove row controls, and a lightweight inline editor per cell based
// on the sub-field's own type. The sequence field (child.sequence_field,
// e.g. line_number) auto-increments on add and is never user-editable.
//
// Mirrors the parent Table renderer where it makes sense for a local,
// editable grid:
//   - column rendering via the shared renderCellValue (badge/currency/date…)
//   - client-side column sorting (child data is local — no server round-trip)
//   - per-row `computed` fields (e.g. line_total = quantity * unit_price)
//   - per-row `readonly_when` (e.g. unit_price locked once menu_item_id set)

import { useMemo, useState } from "react"
import {
  Plus,
  Trash2,
  ChevronUp,
  ChevronDown,
  ChevronsUpDown,
} from "lucide-react"
import type { ChildDecl, Field } from "@/types/manifest"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select as ThemedSelect } from "@/components/ui/select"
import { RelationPicker } from "@/widgets/RelationPicker"
import { NumberInput } from "@/widgets/NumberInput"
import { renderCellValue, cellHintsForField } from "@/lib/renderCell"
import { createFormatter, type Formatter } from "@/lib/format"
import { useMetaStore } from "@/stores/meta"
import { evalCompute, evalReadonlyWhen } from "@/lib/formspec-expr"
import type { RuntimeObject } from "@/lib/formspec-expr/eval"

type Row = Record<string, unknown>

interface ChildTableProps {
  value?: Row[] | null
  onChange?: (value: Row[]) => void
  child: ChildDecl
  currentModule: string
  readonly?: boolean
  error?: string
}

export function ChildTable({
  value,
  onChange,
  child,
  currentModule,
  readonly = false,
}: ChildTableProps) {
  const rows = Array.isArray(value) ? value : []
  const fields = child.fields ?? []
  const sequenceField = child.sequence_field

  // Resolved global settings → centralized formatter (spec §10).
  const settings = useMetaStore((s) => s.bundle?.settings)
  const formatter = useMemo(() => createFormatter(settings), [settings])

  // Client-side sorting — child data is local, so no server round-trip.
  const [sort, setSort] = useState<{ field: string; desc: boolean } | null>(
    null,
  )

  const sortedRows = useMemo(() => {
    if (!sort) return rows
    const { field, desc } = sort
    const sorted = [...rows].sort((a, b) => {
      const av = a[field]
      const bv = b[field]
      if (av == null && bv == null) return 0
      if (av == null) return 1
      if (bv == null) return -1
      if (typeof av === "number" && typeof bv === "number") return av - bv
      return String(av).localeCompare(String(bv))
    })
    return desc ? sorted.reverse() : sorted
  }, [rows, sort])

  const toggleSort = (field: string) => {
    setSort((prev) =>
      prev?.field === field
        ? prev.desc
          ? null
          : { field, desc: true }
        : { field, desc: false },
    )
  }

  const updateRow = (idx: number, patch: Row) => {
    const next = rows.map((r, i) => (i === idx ? { ...r, ...patch } : r))
    onChange?.(next)
  }

  const addRow = () => {
    const nextSeq = sequenceField
      ? Math.max(0, ...rows.map((r) => Number(r[sequenceField]) || 0)) + 1
      : undefined
    const blank: Row = {}
    for (const f of fields) blank[f.name] = f.type === "boolean" ? false : ""
    if (sequenceField) blank[sequenceField] = nextSeq
    onChange?.([...rows, blank])
  }

  const removeRow = (idx: number) => {
    onChange?.(rows.filter((_, i) => i !== idx))
  }

  // Auto-fill map: source relation field name → [{ target, sourceField }].
  // Declared via `auto_fill: { from: <relation>, field: <related field> }`
  // on the target child field (e.g. unit_price auto-filled from
  // menu_item_id → price). When the source relation changes, the related
  // record is fetched (RelationPicker already has it) and its field is
  // copied into the target; clearing the relation clears the targets too.
  const autoFillMap = useMemo(() => {
    const map = new Map<string, { target: Field; sourceField: string }[]>()
    for (const f of fields) {
      if (f.auto_fill?.from && f.auto_fill.field) {
        const list = map.get(f.auto_fill.from) ?? []
        list.push({ target: f, sourceField: f.auto_fill.field })
        map.set(f.auto_fill.from, list)
      }
    }
    return map
  }, [fields])

  const isRelationField = (f: Field): boolean => {
    if (f.type === "relation") return true
    if (f.type === "uuid") {
      return !!f.rules?.find((r) => r.name === "exists")?.value
    }
    return false
  }

  // Relation selected → set id + copy auto-filled fields in one update.
  const handleRelationSelect = (
    idx: number,
    sourceField: Field,
    record: Record<string, unknown>,
  ) => {
    const fills = autoFillMap.get(sourceField.name) ?? []
    const patch: Row = { [sourceField.name]: record.id }
    for (const fill of fills) {
      const v = record[fill.sourceField]
      if (v !== undefined) patch[fill.target.name] = v
    }
    updateRow(idx, patch)
  }

  // Relation changed (incl. cleared via the X button) → update id; when
  // cleared, also clear the auto-filled targets.
  const handleRelationChange = (idx: number, sourceField: Field, v: string) => {
    const fills = autoFillMap.get(sourceField.name) ?? []
    const patch: Row = { [sourceField.name]: v }
    if (!v) {
      for (const fill of fills) patch[fill.target.name] = null
    }
    updateRow(idx, patch)
  }

  // Per-row computed values (e.g. line_total) — display-only; the server
  // remains the authority and recomputes on read.
  const rowView = (row: Row): Row => {
    const view: Row = { ...row }
    for (const f of fields) {
      if (f.computed?.formula) {
        const result = evalCompute(f.computed.formula, {
          fields: row as RuntimeObject,
        })
        if (result != null) view[f.name] = result
      }
    }
    return view
  }

  const isCellReadonly = (f: Field, row: Row): boolean => {
    if (readonly || f.immutable || f.read_only) return true
    if (f.readonly_when) {
      return evalReadonlyWhen(f.readonly_when, {
        fields: row as RuntimeObject,
      })
    }
    return false
  }

  if (rows.length === 0 && readonly) {
    return <p className="py-1 text-sm text-muted-foreground">-</p>
  }

  return (
    <div className="rounded-md border overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="border-b bg-muted/50">
            <tr>
              {fields.map((f) => {
                const active = sort?.field === f.name
                return (
                  <th
                    key={f.name}
                    className="h-9 px-2 text-left align-middle text-xs font-medium text-muted-foreground cursor-pointer select-none hover:bg-muted"
                    onClick={() => toggleSort(f.name)}
                  >
                    <span className="inline-flex items-center gap-1">
                      {f.title ?? f.name}
                      {active ? (
                        sort!.desc ? (
                          <ChevronDown className="size-3" />
                        ) : (
                          <ChevronUp className="size-3" />
                        )
                      ) : (
                        <ChevronsUpDown className="size-3 opacity-50" />
                      )}
                    </span>
                  </th>
                )
              })}
              {!readonly && <th className="w-9" />}
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 ? (
              <tr>
                <td
                  colSpan={fields.length + 1}
                  className="px-2 py-4 text-center text-xs text-muted-foreground"
                >
                  Belum ada baris
                </td>
              </tr>
            ) : (
              sortedRows.map((row, idx) => {
                const view = rowView(row)
                return (
                  <tr key={idx} className="border-b last:border-b-0">
                    {fields.map((f) => {
                      const cellReadonly = isCellReadonly(f, row)
                      const isComputed = !!f.computed?.formula
                      const isRelation = isRelationField(f)
                      return (
                        <td key={f.name} className="p-1.5 align-middle">
                          <ChildCell
                            field={f}
                            value={view[f.name]}
                            onChange={
                              isRelation
                                ? (v) =>
                                    handleRelationChange(idx, f, v as string)
                                : (v) => updateRow(idx, { [f.name]: v })
                            }
                            onSelectRecord={
                              isRelation
                                ? (record) =>
                                    handleRelationSelect(idx, f, record)
                                : undefined
                            }
                            readonly={cellReadonly}
                            currentModule={currentModule}
                            displayOnly={isComputed}
                            fmt={formatter}
                          />
                        </td>
                      )
                    })}
                    {!readonly && (
                      <td className="p-1.5 align-middle">
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="size-7 text-muted-foreground hover:text-destructive"
                          onClick={() => removeRow(idx)}
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      </td>
                    )}
                  </tr>
                )
              })
            )}
          </tbody>
        </table>
      </div>
      {!readonly && (
        <div className="border-t p-1.5">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="gap-1.5"
            onClick={addRow}
          >
            <Plus className="size-3.5" />
            Tambah Baris
          </Button>
        </div>
      )}
    </div>
  )
}

function ChildCell({
  field,
  value,
  onChange,
  readonly,
  currentModule,
  displayOnly = false,
  onSelectRecord,
  fmt,
}: {
  field: Field
  value: unknown
  onChange: (value: unknown) => void
  readonly?: boolean
  currentModule: string
  displayOnly?: boolean
  onSelectRecord?: (record: Record<string, unknown>) => void
  fmt?: Formatter
}) {
  // Readonly or computed cells render like parent-table cells (badge,
  // currency, date…) instead of an input.
  if (readonly || displayOnly) {
    const hints = cellHintsForField(field)
    return (
      <span className="text-sm tabular-nums">
        {renderCellValue(value, hints.widget, hints.format, fmt)}
      </span>
    )
  }

  switch (field.type) {
    case "integer":
    case "decimal":
      // Reuse NumberInput so child fields get the same integer/decimal
      // behavior as top-level fields: integer blocks decimal input, `scale`
      // (spec 05-field-types.md §1.2) limits decimal digits, and min/max/
      // positive rules drive the spinner boundary + out-of-range red flag.
      return (
        <NumberInput
          value={(value as number | null) ?? null}
          onChange={(v) => onChange(v)}
          integer={field.type === "integer"}
          scale={field.scale}
          min={
            field.rules?.find((r) => r.name === "min")?.value as
              | number
              | undefined
          }
          max={
            field.rules?.find((r) => r.name === "max")?.value as
              | number
              | undefined
          }
          positive={!!field.rules?.some((r) => r.name === "positive")}
        />
      )

    case "boolean":
      return (
        <input
          type="checkbox"
          className="size-4 rounded border border-input"
          checked={!!value}
          onChange={(e) => onChange(e.target.checked)}
        />
      )

    case "enum":
      return (
        <ThemedSelect
          value={(value as string) ?? ""}
          onChange={onChange}
          options={field.enum_values ?? []}
        />
      )

    case "date":
    case "datetime":
      return (
        <Input
          type={field.type === "datetime" ? "datetime-local" : "date"}
          className="h-8"
          value={(value as string) ?? ""}
          onChange={(e) => onChange(e.target.value)}
        />
      )

    case "uuid": {
      // A rule shaped `{ name: "exists", value: "<resource>" }` on the
      // sub-field is the only signal a `uuid` child field carries about
      // which entity it references — the child schema has no `relation`
      // block of its own.
      const existsResource = field.rules?.find(
        (r) => r.name === "exists",
      )?.value
      if (typeof existsResource === "string" && existsResource) {
        return (
          <RelationPicker
            value={(value as string) ?? ""}
            onChange={onChange}
            onSelectRecord={onSelectRecord}
            entityField={{
              ...field,
              relation: { type: "belongs_to", resource: existsResource },
            }}
            currentModule={currentModule}
          />
        )
      }
      return (
        <Input
          className="h-8 font-mono text-xs"
          value={(value as string) ?? ""}
          onChange={(e) => onChange(e.target.value)}
        />
      )
    }

    case "relation":
      return (
        <RelationPicker
          value={(value as string) ?? ""}
          onChange={onChange}
          onSelectRecord={onSelectRecord}
          entityField={field}
          currentModule={currentModule}
        />
      )

    default:
      return (
        <Input
          className="h-8"
          value={(value as string) ?? ""}
          onChange={(e) => onChange(e.target.value)}
        />
      )
  }
}
