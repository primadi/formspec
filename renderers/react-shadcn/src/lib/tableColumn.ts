// ─── Table column presentation helpers ───
//
// `TableColumn.align` and `.width` are part of the contract
// (docs/spec/frontend/06-page-kinds.md §3 lists both as supported, next to
// `sortable`/`link`). They were declared by authors — `order-table-pos.yaml`
// puts `align: right` on Total, `stock-level-table.yaml` on three numeric
// columns, `visit/tables/list.yaml` carries six `width` values — and read by
// NO renderer, so every declaration was silently discarded.
//
// Both renderers (`kinds/table/TableRenderer.tsx` and
// `kinds/listing/ListingRenderer.tsx`) go through these helpers rather than
// re-deriving a class each, because that is exactly how the `money → currency`
// mapping drifted apart between `derive.tableFormat()` and
// `cellHintsForField()` (kafe 10.25): one vocabulary, two spellings, one hole.
//
// An unrecognised `align` yields no class instead of guessing. The value is a
// closed set in the schema (`pkg/spec/frontend.go`), so this only guards
// hand-written or stale data.

import type { CSSProperties } from "react"

/** Text alignment for a column's header and body cells. */
export function columnAlignClass(align?: string): string {
  if (align === "center") return "text-center"
  if (align === "right") return "text-right"
  return ""
}

/**
 * Alignment for a cell whose content is a flex row (the sortable header wraps
 * its label in one). `text-align` cannot move a flex child — the box is only
 * as wide as its content — so the header needs its own justify value or a
 * right-aligned column would still render its label on the left.
 */
export function columnJustifyClass(align?: string): string {
  if (align === "center") return "justify-center"
  if (align === "right") return "justify-end"
  return ""
}

/**
 * CSS for a column box. `<th>` is what the browser uses to size a table
 * column, so the width belongs on the header cell; `minWidth` keeps the
 * author's intent when the table is narrower than the sum of its columns.
 */
export function columnWidthStyle(width?: string): CSSProperties | undefined {
  if (!width) return undefined
  return { width, minWidth: width }
}
