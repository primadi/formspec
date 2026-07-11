// ─── Dashboard Renderer ───
//
// Renders kind: Dashboard — a canvas of stat/chart/list widgets.
// Supports customizable layouts (drag-and-drop, Fase 4.F6).
//
// Design doc §5.5 Dashboard kind (F4)

import { useMemo } from "react"
import type { Entry, DashboardSpec, WidgetLayout } from "@/types/manifest"
import { useMetaStore } from "@/stores/meta"
import { useSessionStore } from "@/stores/session"
import { can as checkPermission } from "@/engine/permissions"
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
          const perm = `${w.meta.module}.${(getEntity(w.meta.module, w.meta.spec.entity ?? "")?.plural) ?? "list"}`
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
          />
        ))}
      </div>
    </div>
  )
}

function DashboardWidgetCard({
  placement,
  meta,
}: {
  placement: { ref: string; layout: WidgetLayout; config?: Record<string, unknown> }
  meta?: import("@/types/manifest").Entry<import("@/types/manifest").WidgetSpec>
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
        <WidgetBody spec={spec} />
      </div>
    </div>
  )
}

function WidgetBody({
  spec,
}: {
  spec?: import("@/types/manifest").WidgetSpec
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
      return <MetricWidget spec={spec} />
    case "chart":
      return <ChartWidget spec={spec} />
    case "list":
      return <ListWidget spec={spec} />
    case "table":
      return <p className="text-sm text-muted-foreground">Table widget (Fase 4.F6)</p>
    default:
      return <p className="text-sm text-muted-foreground">Unknown widget type: {spec.type}</p>
  }
}

function MetricWidget({ spec }: { spec: import("@/types/manifest").WidgetSpec }) {
  return (
    <div className="text-center">
      <p className="text-3xl font-bold tabular-nums">--</p>
      <p className="text-xs text-muted-foreground mt-1">
        {spec.entity ? `${spec.entity}` : "No data source"}
      </p>
    </div>
  )
}

function ChartWidget({ spec }: { spec: import("@/types/manifest").WidgetSpec }) {
  return (
    <div className="flex items-center justify-center h-32">
      <p className="text-sm text-muted-foreground">
        Chart: {spec.title}
        <br />
        <span className="text-xs">Chart renderer (Fase 4.F6)</span>
      </p>
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
