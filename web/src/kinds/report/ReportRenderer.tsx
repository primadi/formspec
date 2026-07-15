// ─── Report Renderer ───
//
// Parameterized tabular report with client-side grouping,
// totals, and CSV export.
//
// Design doc §5.5 Report kind (F5)

import { useState, useMemo, useCallback } from "react"
import { Download, Loader2, FileSpreadsheet } from "lucide-react"
import { toast } from "sonner"

import type { Entry, ReportSpec, ListResponseMeta } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { resolveEntityRef } from "@/engine/entityRef"
import { apiList, buildListParams } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"


interface ReportRendererProps {
  entry: Entry<ReportSpec>
}

export default function ReportRenderer({ entry }: ReportRendererProps) {
  const getClient = useSessionStore((s) => s.getClient)
  const getEntity = useMetaStore((s) => s.getEntity)

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
        `${schema.module}/${schema.plural}`,
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
      const key = entry.spec.groups!.map((g) => String(item[g] ?? "")).join(" · ")
      const list = grouped.get(key) ?? []
      list.push(item)
      grouped.set(key, list)
    }
    return grouped
  }, [data, entry.spec.groups])

  // Calculate totals
  const totals = useMemo(() => {
    if (!entry.spec.totals?.length) return null
    const result: Record<string, number> = {}
    for (const total of entry.spec.totals!) {
      let sum = 0
      let count = 0
      for (const item of data) {
        const val = Number(item[total.field])
        if (!isNaN(val)) {
          sum += val
          count++
        }
      }
      switch (total.fn) {
        case "sum": result[total.field] = sum; break
        case "avg": result[total.field] = count > 0 ? sum / count : 0; break
        case "count": result[total.field] = count; break
        case "min": {
          let min = Infinity
          for (const item of data) {
            const val = Number(item[total.field])
            if (!isNaN(val) && val < min) min = val
          }
          result[total.field] = min === Infinity ? 0 : min
          break
        }
        case "max": {
          let max = -Infinity
          for (const item of data) {
            const val = Number(item[total.field])
            if (!isNaN(val) && val > max) max = val
          }
          result[total.field] = max === -Infinity ? 0 : max
          break
        }
      }
    }
    return result
  }, [data, entry.spec.totals])

  // CSV export
  const exportCSV = useCallback(() => {
    const cols = entry.spec.columns
    const header = cols.map((c) => `"${c.label || c.field}"`).join(",")
    const rows = data.map((item) =>
      cols.map((c) => `"${String(item[c.field] ?? "").replace(/"/g, '""')}"`).join(","),
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
          <h1 className="text-2xl font-bold tracking-tight">{entry.spec.title}</h1>
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
              <div key={param.name} className="space-y-1">
                <label className="text-xs font-medium">{param.label}</label>
                <Input
                  placeholder={param.name}
                  value={params[param.name] ?? ""}
                  onChange={(e) =>
                    setParams((p) => ({
                      ...p,
                      [param.name]: e.target.value,
                    }))
                  }
                />
              </div>
            ))}
          </div>
          <Button onClick={fetchReport} disabled={loading}>
            {loading ? (
              <Loader2 className="size-4 mr-1 animate-spin" />
            ) : null}
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
                      <>
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
                                {formatReportValue(item[col.field], col.format, col.aggregate)}
                              </td>
                            ))}
                          </tr>
                        ))}
                      </>
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
                          {formatReportValue(item[col.field], col.format, col.aggregate)}
                        </td>
                      ))}
                    </tr>
                  ))
                )}

                {/* Totals row */}
                {totals && (
                  <tr className="border-t-2 font-medium bg-muted/30">
                    <td className="p-3 text-sm" colSpan={entry.spec.columns.length}>
                      {/* Place totals in matching columns */}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}

function formatReportValue(value: unknown, format?: string, _aggregate?: string) {
  if (value == null) return <span className="text-muted-foreground">-</span>

  if (format === "currency" && typeof value === "number") {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: "USD",
    }).format(value)
  }

  if (format === "date" && typeof value === "string") {
    return new Date(value).toLocaleDateString()
  }

  if (format === "datetime" && typeof value === "string") {
    return new Date(value).toLocaleString()
  }

  if (format === "percentage" && typeof value === "number") {
    return `${(value * 100).toFixed(1)}%`
  }

  return String(value)
}
