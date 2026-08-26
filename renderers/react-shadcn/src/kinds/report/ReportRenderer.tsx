// ─── Report Renderer ───
//
// Parameterized tabular report with client-side grouping,
// totals, and CSV export.
//
// Design doc §5.5 Report kind (F5)

import { useState, useMemo, useCallback, Fragment } from "react"
import { Download, Loader2, FileSpreadsheet } from "lucide-react"
import { toast } from "@/lib/ui"

import type {
  Entry,
  ReportSpec,
  ReportParam,
  ReportColumn,
  ReportTotal,
  Field,
  ListResponseMeta,
} from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { resolveEntityRef } from "@/engine/entityRef"
import { apiList, buildListParams } from "@/lib/api"
import { createFormatter, type Formatter } from "@/lib/format"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { RelationPicker } from "@/widgets/RelationPicker"

interface ReportRendererProps {
  entry: Entry<ReportSpec>
}

export default function ReportRenderer({ entry }: ReportRendererProps) {
  const getClient = useSessionStore((s) => s.getClient)
  const getEntity = useMetaStore((s) => s.getEntity)
  const settings = useMetaStore((s) => s.bundle?.settings)
  const formatter = useMemo(() => createFormatter(settings), [settings])

  const [data, setData] = useState<Record<string, unknown>[]>([])
  const [loading, setLoading] = useState(false)
  const [params, setParams] = useState<Record<string, string>>({})
  const [executed, setExecuted] = useState(false)
  const [meta, setMeta] = useState<ListResponseMeta | null>(null)

  const hasParams = (entry.spec.parameters?.length ?? 0) > 0

  const fetchReport = useCallback(async () => {
    setLoading(true)
    try {
      const client = getClient()
      const listParams: Record<string, string> = {}

      // Add parameter values as filters
      for (const [key, value] of Object.entries(params)) {
        if (value) listParams[key] = value
      }

      const [module, name] = resolveEntityRef(entry.spec.entity, entry.module)
      const schema = getEntity(module, name)
      if (!schema) {
        throw new Error(`entity ${module}.${name} not found`)
      }

      const result = await apiList<Record<string, unknown>>(
        client,
        `${schema.module}/${schema.name}`,
        buildListParams({
          per_page: 1000,
          ...listParams,
        } as any),
      )
      setData(result.items)
      setMeta(result.meta)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to load report")
      setData([])
    } finally {
      setLoading(false)
      setExecuted(true)
    }
  }, [entry.spec.entity, entry.module, params, getClient, getEntity])

  // Group data
  const groups = useMemo(() => {
    if (!entry.spec.groups?.length) return null
    const grouped = new Map<string, Record<string, unknown>[]>()
    for (const item of data) {
      const key = entry.spec
        .groups!.map((g) => String(item[g.field] ?? ""))
        .join(" · ")
      const list = grouped.get(key) ?? []
      list.push(item)
      grouped.set(key, list)
    }
    return grouped
  }, [data, entry.spec.groups])

  // Calculate totals for a set of items — shared by the overall totals row
  // and per-group subtotal rows (5.13.1).
  const totals = useMemo(() => {
    if (!entry.spec.totals?.length) return null
    return computeTotals(data, entry.spec.totals)
  }, [data, entry.spec.totals])

  // CSV export
  const exportCSV = useCallback(() => {
    const cols = entry.spec.columns
    const header = cols.map((c) => `"${c.label || c.field}"`).join(",")
    const rows = data.map((item) =>
      cols
        .map((c) => `"${String(item[c.field] ?? "").replace(/"/g, '""')}"`)
        .join(","),
    )
    const csv = [header, ...rows].join("\n")

    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" })
    const url = URL.createObjectURL(blob)
    const a = document.createElement("a")
    a.href = url
    a.download = `${entry.spec.title.replace(/\s+/g, "-").toLowerCase()}.csv`
    a.click()
    URL.revokeObjectURL(url)
    toast.success("CSV exported")
  }, [data, entry.spec.columns, entry.spec.title])

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            {entry.spec.title}
          </h1>
          {meta && (
            <p className="text-sm text-muted-foreground">{meta.total} rows</p>
          )}
        </div>

        {data.length > 0 && entry.spec.export?.includes("csv") && (
          <div className="flex items-center gap-2">
            <Button variant="outline" onClick={exportCSV}>
              <Download className="size-4 mr-1" />
              CSV
            </Button>
          </div>
        )}
      </div>

      {/* Parameters */}
      {hasParams && (
        <div className="rounded-md border p-4 space-y-3">
          <h3 className="text-sm font-medium">Parameters</h3>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            {(entry.spec.parameters ?? []).map((param) => (
              <div key={param.field} className="space-y-1">
                <label className="text-xs font-medium">{param.label}</label>
                <ReportParamInput
                  param={param}
                  value={params[param.field] ?? ""}
                  module={entry.module}
                  onChange={(v) =>
                    setParams((p) => ({
                      ...p,
                      [param.field]: v,
                    }))
                  }
                />
              </div>
            ))}
          </div>
          <Button onClick={fetchReport} disabled={loading}>
            {loading ? <Loader2 className="size-4 mr-1 animate-spin" /> : null}
            {executed ? "Refresh" : "Generate Report"}
          </Button>
        </div>
      )}

      {/* Auto-execute if no params */}
      {!hasParams && !executed && !loading && (
        <div className="text-center py-8">
          <Button onClick={fetchReport} size="lg">
            <FileSpreadsheet className="size-5 mr-2" />
            Generate Report
          </Button>
        </div>
      )}

      {/* Results */}
      {executed && !loading && data.length === 0 && (
        <div className="text-center py-12 text-sm text-muted-foreground">
          No data matches the report criteria.
        </div>
      )}

      {data.length > 0 && (
        <div className="rounded-md border overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full caption-bottom text-sm">
              <thead className="border-b bg-muted/50">
                <tr>
                  {entry.spec.columns.map((col) => (
                    <th
                      key={col.field}
                      className="h-10 px-3 text-left align-middle font-medium text-muted-foreground"
                    >
                      {col.label || col.field}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {groups ? (
                  <>
                    {Array.from(groups.entries()).map(([group, items]) => (
                      <Fragment key={group}>
                        <tr className="bg-muted/30">
                          <td
                            colSpan={entry.spec.columns.length}
                            className="px-3 py-2 text-xs font-medium text-muted-foreground"
                          >
                            {group}
                          </td>
                        </tr>
                        {items.map((item, idx) => (
                          <tr
                            key={idx}
                            className="border-b transition-colors hover:bg-muted/50"
                          >
                            {entry.spec.columns.map((col) => (
                              <td key={col.field} className="p-3 align-middle">
                                {formatReportValue(
                                  item[col.field],
                                  col.format,
                                  col.aggregate,
                                  formatter,
                                )}
                              </td>
                            ))}
                          </tr>
                        ))}
                        {/* Per-group subtotal row (5.13.1) */}
                        {totals && (
                          <TotalsRow
                            totals={computeTotals(items, entry.spec.totals!)}
                            columns={entry.spec.columns}
                            totalsDef={entry.spec.totals!}
                            formatter={formatter}
                            label="Subtotal"
                          />
                        )}
                      </Fragment>
                    ))}
                  </>
                ) : (
                  data.map((item, idx) => (
                    <tr
                      key={idx}
                      className="border-b transition-colors hover:bg-muted/50"
                    >
                      {entry.spec.columns.map((col) => (
                        <td key={col.field} className="p-3 align-middle">
                          {formatReportValue(
                            item[col.field],
                            col.format,
                            col.aggregate,
                            formatter,
                          )}
                        </td>
                      ))}
                    </tr>
                  ))
                )}

                {/* Overall totals row (5.13.1) — values placed in the
                    matching columns, not an empty cell */}
                {totals && (
                  <TotalsRow
                    totals={totals}
                    columns={entry.spec.columns}
                    totalsDef={entry.spec.totals!}
                    formatter={formatter}
                    label="Total"
                  />
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}

// ── Report parameter input ──
//
// `type: date` gets a native date-picker. `type: select` on a field named
// `*_id` is treated as a relation to the same-named entity (by convention —
// ReportParam carries no explicit target, so the `_id` suffix is the only
// signal available) and gets the same RelationPicker used by form relation
// fields. Anything else falls back to plain text.
function ReportParamInput({
  param,
  value,
  module,
  onChange,
}: {
  param: ReportParam
  value: string
  module: string
  onChange: (value: string) => void
}) {
  if (param.type === "date" || param.type === "datetime") {
    return (
      <Input
        type={param.type === "datetime" ? "datetime-local" : "date"}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
    )
  }

  if (param.type === "select" && param.field.endsWith("_id")) {
    const resource = param.field.slice(0, -"_id".length)
    const syntheticField: Field = {
      name: param.field,
      type: "relation",
      relation: { type: "belongs_to", resource },
    }
    return (
      <RelationPicker
        value={value}
        onChange={onChange}
        entityField={syntheticField}
        currentModule={module}
        placeholder={param.label}
      />
    )
  }

  return (
    <Input
      placeholder={param.label}
      value={value}
      onChange={(e) => onChange(e.target.value)}
    />
  )
}

function formatReportValue(
  value: unknown,
  format?: string,
  _aggregate?: string,
  fmt?: Formatter,
) {
  if (value == null) return <span className="text-muted-foreground">-</span>

  const formatter = fmt ?? createFormatter()

  if (format === "currency" && typeof value === "number") {
    return formatter.money(value)
  }

  if (format === "date" && typeof value === "string") {
    return formatter.date(value)
  }

  if (format === "datetime" && typeof value === "string") {
    return formatter.dateTime(value)
  }

  if (format === "percentage" && typeof value === "number") {
    return `${(value * 100).toFixed(1)}%`
  }

  return String(value)
}

// ── Totals / subtotals (5.13.1) ──

/**
 * Compute aggregate values over a set of rows for the declared `totals`.
 * Shared by the overall totals row and per-group subtotal rows.
 */
function computeTotals(
  items: Record<string, unknown>[],
  totals: ReportTotal[],
): Record<string, number> {
  const result: Record<string, number> = {}
  for (const total of totals) {
    let sum = 0
    let count = 0
    for (const item of items) {
      const val = Number(item[total.field])
      if (!isNaN(val)) {
        sum += val
        count++
      }
    }
    switch (total.fn) {
      case "sum":
        result[total.field] = sum
        break
      case "avg":
        result[total.field] = count > 0 ? sum / count : 0
        break
      case "count":
        result[total.field] = count
        break
      case "min": {
        let min = Infinity
        for (const item of items) {
          const val = Number(item[total.field])
          if (!isNaN(val) && val < min) min = val
        }
        result[total.field] = min === Infinity ? 0 : min
        break
      }
      case "max": {
        let max = -Infinity
        for (const item of items) {
          const val = Number(item[total.field])
          if (!isNaN(val) && val > max) max = val
        }
        result[total.field] = max === -Infinity ? 0 : max
        break
      }
    }
  }
  return result
}

/**
 * Render one totals/subtotal row. The first column carries the label; each
 * subsequent column whose field has a computed aggregate shows its value
 * (formatted like the data column), the rest stay empty. Fixes the previous
 * bug where the row computed values but rendered an empty `<td>`.
 */
function TotalsRow({
  totals,
  columns,
  totalsDef,
  formatter,
  label,
}: {
  totals: Record<string, number>
  columns: ReportColumn[]
  totalsDef: ReportTotal[]
  formatter: Formatter
  label: string
}) {
  return (
    <tr className="border-t-2 font-medium bg-muted/30">
      {columns.map((col, idx) => {
        const totalDef = totalsDef.find((t) => t.field === col.field)
        const value = totals[col.field]
        if (idx === 0) {
          return (
            <td key={col.field} className="p-3 text-sm">
              {label}
            </td>
          )
        }
        if (totalDef && value !== undefined) {
          return (
            <td key={col.field} className="p-3 text-sm">
              {formatReportValue(value, col.format, col.aggregate, formatter)}
            </td>
          )
        }
        return <td key={col.field} className="p-3 text-sm" />
      })}
    </tr>
  )
}
