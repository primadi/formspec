// ─── Dashboard Renderer ───
//
// Renders kind: Dashboard — a canvas of stat/chart/list widgets.
// Supports customizable layouts (drag-and-drop, Fase 4.F6).
//
// Design doc §5.5 Dashboard kind (F4)

import { useEffect, useMemo, useState } from "react"
import { Loader2 } from "lucide-react"
import type { Entry, DashboardSpec, WidgetLayout, WidgetSpec, FilterOpValue } from "@/types/manifest"
import { useMetaStore } from "@/stores/meta"
import { useSessionStore } from "@/stores/session"
import { can as checkPermission } from "@/engine/permissions"
import { resolveEntityRef } from "@/engine/entityRef"
import { apiList, buildListParams } from "@/lib/api"
import { useRealtime } from "@/hooks/useRealtime"
import { Badge } from "@/widgets/Badge"

interface DashboardRendererProps {
  entry: Entry<DashboardSpec>
}

export default function DashboardRenderer({ entry }: DashboardRendererProps) {
  const me = useSessionStore((s) => s.me)
  const getWidget = useMetaStore((s) => s.getWidget)
  const getEntity = useMetaStore((s) => s.getEntity)

  const widgets = useMemo(
    () =>
      entry.spec.widgets
        .map((w) => {
          const meta = getWidget(w.ref)
          return { placement: w, meta }
        })
        .filter((w) => {
          if (!w.meta?.spec.entity) return true
          if (!me) return false
          const [module, name] = resolveEntityRef(w.meta.spec.entity, w.meta.module)
          const perm = `${module}.${getEntity(module, name)?.plural ?? "list"}`
          return checkPermission(perm, me.permissions)
        }),
    [entry.spec.widgets, getWidget, me, getEntity],
  )

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{entry.spec.title}</h1>
        {entry.spec.description && (
          <p className="text-sm text-muted-foreground">{entry.spec.description}</p>
        )}
      </div>

      {entry.spec.customizable && (
        <p className="text-xs text-muted-foreground">
          This dashboard is customizable. Drag widgets to rearrange.
        </p>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {widgets.map((w, idx) => (
          <DashboardWidgetCard
            key={`${w.placement.ref}-${idx}`}
            placement={w.placement}
            meta={w.meta}
            realtime={!!entry.spec.realtime}
          />
        ))}
      </div>
    </div>
  )
}

function DashboardWidgetCard({
  placement,
  meta,
  realtime,
}: {
  placement: { ref: string; layout: WidgetLayout; config?: Record<string, unknown> }
  meta?: import("@/types/manifest").Entry<import("@/types/manifest").WidgetSpec>
  realtime?: boolean
}) {
  const spec = meta?.spec

  return (
    <div
      className="rounded-md border bg-card"
      style={{
        gridColumn: `span ${Math.min(placement.layout.w, 3)}`,
      }}
    >
      <div className="border-b px-4 py-2 flex items-center justify-between">
        <h3 className="text-sm font-medium">{spec?.title ?? placement.ref}</h3>
        {spec?.type && (
          <Badge value={spec.type} />
        )}
      </div>
      <div className="p-4">
        <WidgetBody spec={spec} module={meta?.module} realtime={realtime} />
      </div>
    </div>
  )
}

function WidgetBody({
  spec,
  module,
  realtime,
}: {
  spec?: WidgetSpec
  module?: string
  realtime?: boolean
}) {
  if (!spec) {
    return (
      <p className="text-sm text-muted-foreground text-center py-4">
        Widget definition not found
      </p>
    )
  }

  switch (spec.type) {
    case "metric":
      return <MetricWidget spec={spec} module={module ?? ""} realtime={realtime} />
    case "chart":
      return <ChartWidget spec={spec} module={module ?? ""} realtime={realtime} />
    case "list":
      return <ListWidget spec={spec} />
    case "table":
      return <p className="text-sm text-muted-foreground">Table widget (Fase 4.F6)</p>
    default:
      return <p className="text-sm text-muted-foreground">Unknown widget type: {spec.type}</p>
  }
}

// ── Metric widget ──
//
// Fetches spec.entity's records, applies spec.query (a small subset of
// FormaExpr — see applySimpleQuery below), and aggregates spec.config.field
// per spec.config.aggregate. Re-fetches every spec.refresh_secs.
function MetricWidget({ spec, module, realtime }: { spec: WidgetSpec; module: string; realtime?: boolean }) {
  const getClient = useSessionStore((s) => s.getClient)
  const [value, setValue] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)

  // Realtime: refetch on matching entity events / reconnect (non-durable).
  const realtimeTick = useRealtime(
    realtime && spec.entity ? resolveEntityRef(spec.entity, module).join("/") : "",
  )

  useEffect(() => {
    if (!spec.entity) {
      setLoading(false)
      return
    }
    let cancelled = false
    const load = async () => {
      try {
        const client = getClient()
        const [mod, name] = resolveEntityRef(spec.entity!, module)
        // Push spec.query to the server list endpoint as filters when possible
        // (DB pre-filters before rows hit the wire); applySimpleQuery remains
        // the final client-side safety net for any untranslatable clause.
        const filters = translateWidgetQuery(spec.query)
        const search: Record<string, string> = { per_page: "1000" }
        if (filters) Object.assign(search, buildListParams({ filters }))
        const { items } = await apiList<Record<string, unknown>>(client, `${mod}/${name}`, search)
        if (cancelled) return
        setValue(aggregate(applySimpleQuery(items, spec.query), spec.config))
      } catch {
        if (!cancelled) setValue(null)
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    load()
    if (!spec.refresh_secs) return
    const interval = setInterval(load, spec.refresh_secs * 1000)
    return () => {
      cancelled = true
      clearInterval(interval)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [spec.entity, spec.query, spec.refresh_secs, JSON.stringify(spec.config ?? {}), module, getClient, realtimeTick])

  const formatted = useMemo(() => formatMetric(value, spec.config), [value, spec.config])

  return (
    <div className="text-center">
      <p className="text-3xl font-bold tabular-nums">{loading ? "…" : formatted}</p>
      <p className="text-xs text-muted-foreground mt-1">
        {spec.entity ? `${spec.entity}` : "No data source"}
      </p>
    </div>
  )
}

function aggregate(items: Record<string, unknown>[], config?: Record<string, unknown>): number {
  const fn = String(config?.aggregate ?? "count")
  if (fn === "count") return items.length

  const field = config?.field ? String(config.field) : undefined
  if (!field) return items.length
  const nums = items.map((it) => Number(it[field])).filter((n) => !Number.isNaN(n))

  switch (fn) {
    case "sum": return nums.reduce((a, b) => a + b, 0)
    case "avg": return nums.length ? nums.reduce((a, b) => a + b, 0) / nums.length : 0
    case "min": return nums.length ? Math.min(...nums) : 0
    case "max": return nums.length ? Math.max(...nums) : 0
    default: return items.length
  }
}

function formatMetric(value: number | null, config?: Record<string, unknown>): string {
  if (value == null) return "--"
  const format = config?.format ? String(config.format) : undefined
  if (format === "currency") {
    return new Intl.NumberFormat("id-ID", {
      style: "currency",
      currency: String(config?.currency ?? "IDR"),
      maximumFractionDigits: 0,
    }).format(value)
  }
  if (format === "percentage") return `${(value * 100).toFixed(1)}%`
  return new Intl.NumberFormat("id-ID").format(value)
}

/**
 * Evaluates the narrow subset of FormaExpr actually used by Widget.spec.query:
 * `field = today()`, `field in [...]`, and simple equality/inequality
 * (`field = 'x'`, `field == 'x'`, `field != 'x'`); multiple conditions may be
 * joined with ` and `. Falls back to no filtering for anything else — a full
 * FormaExpr-to-client-filter compiler is out of scope here (queries beyond
 * this are expected to be pre-filtered by the entity's own list endpoint via
 * spec.entity semantics — see translateWidgetQuery below).
 */
function applySimpleQuery(items: Record<string, unknown>[], query?: string): Record<string, unknown>[] {
  if (!query) return items

  // Support compound queries like `transaction_date = today() and status != 'cancelled'`
  // by splitting on ` and ` and applying each predicate in sequence.
  const clauses = query.split(/\s+and\s+/i).map((s) => s.trim()).filter(Boolean)
  if (clauses.length > 1) {
    return clauses.reduce((acc, clause) => applySimpleQuery(acc, clause), items)
  }
  const q = clauses[0]

  const todayMatch = q.match(/^(\w+)\s*=\s*today\(\)$/)
  if (todayMatch) {
    const [, field] = todayMatch
    return items.filter((it) => String(it[field] ?? "").slice(0, 10) === serverToday())
  }

  const inMatch = q.match(/^(\w+)\s+in\s+\[(.+)\]$/)
  if (inMatch) {
    const [, field, list] = inMatch
    const values = list.split(",").map((s) => s.trim().replace(/^['"]|['"]$/g, ""))
    return items.filter((it) => values.includes(String(it[field] ?? "")))
  }

  // `!=` must be tested before the generic `=`/`==` match below.
  const neMatch = q.match(/^(\w+)\s*!=\s*['"]?([\w.-]+)['"]?$/)
  if (neMatch) {
    const [, field, val] = neMatch
    return items.filter((it) => String(it[field] ?? "") !== val)
  }

  // `==?` matches both `=` and `==` (FormaExpr uses `==`; showcase uses `=`).
  const eqMatch = q.match(/^(\w+)\s*==?\s*['"]?([\w.-]+)['"]?$/)
  if (eqMatch) {
    const [, field, val] = eqMatch
    return items.filter((it) => String(it[field] ?? "") === val)
  }

  return items
}

/**
 * Returns the server/business date as a UTC YYYY-MM-DD string.
 *
 * `today()` in a widget query must match the stored date fields, which are
 * business dates aligned to the server's clock (all server timestamps are
 * UTC). Using the browser's wall-clock is wrong: a viewer in a timezone ahead
 * of UTC (e.g. WIB, UTC+7) sees a different calendar day near midnight than
 * the server, which silently zeroed the "Kunjungan Hari Ini" metric (the
 * browser read 2026-08-05 while the visit was still transaction_date
 * 2026-08-04). Server timestamps are RFC3339 UTC, so UTC today == server today.
 */
function serverToday(): string {
  return new Date().toISOString().slice(0, 10)
}

/**
 * Translates a widget `query` (the narrow FormaExpr subset above) into
 * server-side list filters (`field[op]=value`) so the DB can pre-filter
 * before rows hit the wire. Handles `field = today()` → eq, `field in [...]`
 * → in, `field != 'v'` → neq, `field = 'v'`/`field == 'v'` → eq; compound
 * clauses joined with ` and `. Returns undefined if any clause is not
 * translatable — the caller then falls back to fetching all rows and relying
 * on applySimpleQuery client-side.
 */
function translateWidgetQuery(query?: string): Record<string, string | FilterOpValue> | undefined {
  if (!query) return undefined

  const clauses = query.split(/\s+and\s+/i).map((s) => s.trim()).filter(Boolean)
  const filters: Record<string, string | FilterOpValue> = {}
  for (const clause of clauses) {
    const todayMatch = clause.match(/^(\w+)\s*=\s*today\(\)$/)
    if (todayMatch) {
      const [, field] = todayMatch
      filters[field] = { op: "eq", value: serverToday() }
      continue
    }
    const inMatch = clause.match(/^(\w+)\s+in\s+\[(.+)\]$/)
    if (inMatch) {
      const [, field, list] = inMatch
      filters[field] = { op: "in", value: list.split(",").map((s) => s.trim().replace(/^['"]|['"]$/g, "")).join(",") }
      continue
    }
    const neMatch = clause.match(/^(\w+)\s*!=\s*['"]?([\w.-]+)['"]?$/)
    if (neMatch) {
      const [, field, val] = neMatch
      filters[field] = { op: "neq", value: val }
      continue
    }
    const eqMatch = clause.match(/^(\w+)\s*==?\s*['"]?([\w.-]+)['"]?$/)
    if (eqMatch) {
      const [, field, val] = eqMatch
      filters[field] = { op: "eq", value: val }
      continue
    }
    return undefined
  }
  return filters
}

// ── Chart widget ──
//
// Fetches spec.entity's records and renders a lightweight inline-SVG line
// chart (no charting library dependency) — one series per config.group_by
// value, x = config.x field, y = config.y field.
function ChartWidget({ spec, module, realtime }: { spec: WidgetSpec; module: string; realtime?: boolean }) {
  const getClient = useSessionStore((s) => s.getClient)
  const [rows, setRows] = useState<Record<string, unknown>[]>([])
  const [loading, setLoading] = useState(true)

  // Realtime: refetch on matching entity events / reconnect (non-durable).
  const realtimeTick = useRealtime(
    realtime && spec.entity ? resolveEntityRef(spec.entity, module).join("/") : "",
  )

  useEffect(() => {
    if (!spec.entity) {
      setLoading(false)
      return
    }
    let cancelled = false
    const load = async () => {
      try {
        const client = getClient()
        const [mod, name] = resolveEntityRef(spec.entity!, module)
        // Server-side pre-filter (see translateWidgetQuery) + client-side
        // applySimpleQuery as final safety net.
        const filters = translateWidgetQuery(spec.query)
        const search: Record<string, string> = { per_page: "1000" }
        if (filters) Object.assign(search, buildListParams({ filters }))
        const { items } = await apiList<Record<string, unknown>>(client, `${mod}/${name}`, search)
        if (!cancelled) setRows(applySimpleQuery(items, spec.query))
      } catch {
        if (!cancelled) setRows([])
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    load()
    if (!spec.refresh_secs) return
    const interval = setInterval(load, spec.refresh_secs * 1000)
    return () => {
      cancelled = true
      clearInterval(interval)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [spec.entity, spec.query, spec.refresh_secs, module, getClient, realtimeTick])

  const config = spec.config ?? {}
  const xField = config.x ? String(config.x) : undefined
  const yField = config.y ? String(config.y) : undefined
  // No `y` (or `y: count`) → count rows per bucket instead of summing a field.
  const countMode = !yField || yField === "count"
  const groupBy = config.group_by ? String(config.group_by) : undefined

  const series = useMemo(() => {
    if (!xField || rows.length === 0) return []
    // Bucket per series (groupBy) then per x. Count mode counts rows; otherwise
    // sums the `y` field value per bucket. Aggregating per-x collapses multiple
    // rows sharing the same x for a series into one point — required when
    // charting a live transaction entity (e.g. counting visits per day).
    const byKey = new Map<string, { label: string; points: Map<string, number> }>()
    for (const row of rows) {
      const key = groupBy ? String(row[groupBy] ?? "-") : "all"
      // Relation fields ("polyclinic_id") are expanded by the backend under
      // the base name ("polyclinic") alongside the raw id — prefer its
      // display name for the legend when available.
      const expanded = groupBy ? (row[groupBy.replace(/_id$/, "")] as Record<string, unknown> | undefined) : undefined
      const label = groupBy ? String(expanded?.name ?? key) : spec.title
      if (!byKey.has(key)) byKey.set(key, { label, points: new Map() })
      const pts = byKey.get(key)!.points
      const x = String(row[xField] ?? "")
      const y = countMode ? 1 : Number(row[yField]) || 0
      pts.set(x, (pts.get(x) ?? 0) + y)
    }
    return Array.from(byKey.values()).map(({ label, points }) => ({
      label,
      points: Array.from(points.entries())
        .map(([x, y]) => ({ x, y }))
        .sort((a, b) => a.x.localeCompare(b.x)),
    }))
  }, [rows, xField, yField, countMode, groupBy, spec.title])

  if (loading) {
    return (
      <div className="flex items-center justify-center h-32">
        <Loader2 className="size-5 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (series.length === 0) {
    return (
      <div className="flex items-center justify-center h-32">
        <p className="text-sm text-muted-foreground">No data</p>
      </div>
    )
  }

  return <LineChart series={series} />
}

const CHART_COLORS = ["#2563eb", "#16a34a", "#d97706", "#dc2626", "#7c3aed", "#0891b2"]

function LineChart({ series }: { series: { label: string; points: { x: string; y: number }[] }[] }) {
  const width = 400
  const height = 160
  const padding = 20

  const allXs = Array.from(new Set(series.flatMap((s) => s.points.map((p) => p.x)))).sort()
  const maxY = Math.max(1, ...series.flatMap((s) => s.points.map((p) => p.y)))

  const xScale = (x: string) => {
    const idx = allXs.indexOf(x)
    return allXs.length > 1 ? padding + (idx / (allXs.length - 1)) * (width - padding * 2) : width / 2
  }
  const yScale = (y: number) => height - padding - (y / maxY) * (height - padding * 2)

  return (
    <div>
      <svg viewBox={`0 0 ${width} ${height}`} className="w-full h-32 text-border">
        <line x1={padding} y1={height - padding} x2={width - padding} y2={height - padding} stroke="currentColor" />
        {series.map((s, i) => (
          <polyline
            key={s.label}
            fill="none"
            stroke={CHART_COLORS[i % CHART_COLORS.length]}
            strokeWidth={2}
            points={s.points.map((p) => `${xScale(p.x)},${yScale(p.y)}`).join(" ")}
          />
        ))}
      </svg>
      <div className="flex flex-wrap gap-3 mt-2">
        {series.map((s, i) => (
          <div key={s.label} className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <span className="size-2 rounded-full" style={{ background: CHART_COLORS[i % CHART_COLORS.length] }} />
            {s.label}
          </div>
        ))}
      </div>
    </div>
  )
}

function ListWidget({ spec }: { spec: import("@/types/manifest").WidgetSpec }) {
  return (
    <div className="space-y-2">
      <p className="text-sm text-muted-foreground">Recent {spec.entity}</p>
      <div className="text-center py-4">
        <p className="text-xs text-muted-foreground">List widget (Fase 4.F6)</p>
      </div>
    </div>
  )
}
