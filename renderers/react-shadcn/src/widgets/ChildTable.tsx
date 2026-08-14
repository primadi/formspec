// ─── Child Table Widget ───
//
// For `type: child` fields — a jsonb-backed repeatable row grid (e.g.
// visit.treatments, otc-sale.items). Renders one column per sub-field,
// add/remove row controls, and a lightweight inline editor per cell based
// on the sub-field's own type. The sequence field (child.sequence_field,
// e.g. line_number) auto-increments on add and is never user-editable.

import { Plus, Trash2 } from "lucide-react"
import type { ChildDecl, Field } from "@/types/manifest"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select as ThemedSelect } from "@/components/ui/select"
import { RelationPicker } from "@/widgets/RelationPicker"

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

  if (rows.length === 0 && readonly) {
    return <p className="py-1 text-sm text-muted-foreground">-</p>
  }

  return (
    <div className="rounded-md border overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="border-b bg-muted/50">
            <tr>
              {fields.map((f) => (
                <th key={f.name} className="h-9 px-2 text-left align-middle text-xs font-medium text-muted-foreground">
                  {f.title ?? f.name}
                </th>
              ))}
              {!readonly && <th className="w-9" />}
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 ? (
              <tr>
                <td colSpan={fields.length + 1} className="px-2 py-4 text-center text-xs text-muted-foreground">
                  Belum ada baris
                </td>
              </tr>
            ) : (
              rows.map((row, idx) => (
                <tr key={idx} className="border-b last:border-b-0">
                  {fields.map((f) => (
                    <td key={f.name} className="p-1.5 align-middle">
                      <ChildCell
                        field={f}
                        value={row[f.name]}
                        onChange={(v) => updateRow(idx, { [f.name]: v })}
                        readonly={readonly || f.immutable}
                        currentModule={currentModule}
                      />
                    </td>
                  ))}
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
              ))
            )}
          </tbody>
        </table>
      </div>
      {!readonly && (
        <div className="border-t p-1.5">
          <Button type="button" variant="ghost" size="sm" className="gap-1.5" onClick={addRow}>
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
}: {
  field: Field
  value: unknown
  onChange: (value: unknown) => void
  readonly?: boolean
  currentModule: string
}) {
  if (readonly) {
    return <span className="text-sm tabular-nums">{value == null || value === "" ? "-" : String(value)}</span>
  }

  switch (field.type) {
    case "integer":
    case "decimal":
      return (
        <Input
          type="number"
          className="h-8"
          value={(value as number | string | undefined) ?? ""}
          onChange={(e) => {
            const v = e.target.value
            onChange(v === "" ? null : field.type === "integer" ? parseInt(v, 10) : parseFloat(v))
          }}
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
      const existsResource = field.rules?.find((r) => r.name === "exists")?.value
      if (typeof existsResource === "string" && existsResource) {
        return (
          <RelationPicker
            value={(value as string) ?? ""}
            onChange={onChange}
            entityField={{ ...field, relation: { type: "belongs_to", resource: existsResource } }}
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
