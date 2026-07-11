// ─── Widget Renderer ───
//
// Standalone widget renderer (kind: Widget).
// Same logic as DashboardWidgetCard but as a standalone page.

import type { Entry, WidgetSpec } from "@/types/manifest"
import { Badge } from "@/widgets/Badge"

interface WidgetRendererProps {
  entry: Entry<WidgetSpec>
}

export default function WidgetRenderer({ entry }: WidgetRendererProps) {
  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{entry.spec.title}</h1>
        <Badge value={entry.spec.type} />
      </div>

      <div className="rounded-md border p-8">
        <div className="text-center text-sm text-muted-foreground">
          <p>Widget type: {entry.spec.type}</p>
          {entry.spec.entity && <p>Entity: {entry.spec.entity}</p>}
          {entry.spec.query && <p>Query: {entry.spec.query}</p>}
        </div>
      </div>
    </div>
  )
}
