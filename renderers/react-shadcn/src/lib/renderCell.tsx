// ─── Shared Cell Rendering ───
//
// Renders a raw cell value using the same widget/format vocabulary as the
// Table renderer (badge, boolean, currency, date, relative). Shared by the
// parent TableRenderer and the ChildTable widget so child-grid columns
// render identically to parent-table columns.

import { Badge } from "@/widgets/Badge"
import { createFormatter, type Formatter } from "@/lib/format"

export function renderCellValue(
  value: unknown,
  widget?: string,
  format?: string,
  fmt?: Formatter,
) {
  if (value == null) return <span className="text-muted-foreground">-</span>

  if (widget === "badge") {
    return <Badge value={String(value)} />
  }

  if (widget === "boolean") {
    return value ? "Yes" : "No"
  }

  const formatter = fmt ?? createFormatter()

  if (format === "currency" && typeof value === "number") {
    return formatter.money(value)
  }

  if (format === "date" && typeof value === "string") {
    return formatter.date(value)
  }

  if (format === "relative" && typeof value === "string") {
    return formatter.relative(value)
  }

  if (typeof value === "object") return JSON.stringify(value)

  return String(value)
}

/**
 * Derive a display widget/format hint from a field's type, so child-grid
 * columns render like parent-table columns without an explicit config:
 *   - enum        → badge
 *   - boolean     → boolean (Yes/No)
 *   - money       → currency
 *   - date/datetime → date
 */
export function cellHintsForField(field: { type: string; name?: string }): {
  widget?: string
  format?: string
} {
  switch (field.type) {
    case "enum":
      return { widget: "badge" }
    case "boolean":
      return { widget: "boolean" }
    case "money":
      return { format: "currency" }
    case "date":
    case "datetime":
      return { format: "date" }
    default:
      return {}
  }
}
