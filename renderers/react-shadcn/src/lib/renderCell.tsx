// ─── Shared Cell Rendering ───
//
// Renders a raw cell value using the same widget/format vocabulary as the
// Table renderer (badge, boolean, currency, date, relative). Shared by the
// parent TableRenderer and the ChildTable widget so child-grid columns
// render identically to parent-table columns.

import { Badge } from "@/widgets/Badge"
import { QrCode } from "@/widgets/QrCode"
import { createFormatter, moneyAmount, type Formatter } from "@/lib/format"
import { isImageFile, storageAllowsImage } from "@/lib/media"
import { readPath, relationDisplay } from "@/lib/relation"
import {
  fieldOptions,
  optionLabel,
  orderByDeclaration,
  parseOptionValue,
} from "@/lib/field-options"
import type { EntitySchema } from "@/types/manifest"

/** Extra context a cell may need beyond its own value. */
export interface CellRenderOpts {
  /**
   * Download URL for a `file`/`attachment` cell (#4). The route resolves the
   * record's field to the stored object, so it doubles as the `<img src>`.
   * Callers that know the record id + column build it with `fileDownloadUrl()`.
   */
  imageUrl?: string
  /** Alt text for an image cell (defaults to the file name). */
  alt?: string
  /**
   * The field's own decimal `scale` (05-field-types.md §1.2), used by
   * `format: number`. Without it the cell would fall back to the global
   * `settings.decimal_scale` and print a rounded number the database does not
   * agree with.
   */
  scale?: number
}

export function renderCellValue(
  value: unknown,
  widget?: string,
  format?: string,
  fmt?: Formatter,
  opts?: CellRenderOpts,
) {
  if (value == null) return <span className="text-muted-foreground">-</span>

  // Image cell (#4). `file` fields store the object key (or an array of keys
  // when max_count > 1), so one image is a string whose extension is an image.
  // Anything else falls through to the download link — the behaviour a
  // non-image file had before.
  if (widget === "image") {
    const key = Array.isArray(value)
      ? value.find((v) => typeof v === "string" && isImageFile(v))
      : typeof value === "string"
        ? value
        : undefined
    if (typeof key === "string" && isImageFile(key) && opts?.imageUrl) {
      return (
        <img
          src={opts.imageUrl}
          alt={opts.alt ?? key.split("/").pop() ?? "image"}
          className="h-10 w-10 rounded border object-cover"
          loading="lazy"
        />
      )
    }
    if (typeof value === "string") {
      return (
        <a
          href={opts?.imageUrl ?? "#"}
          target="_blank"
          rel="noreferrer"
          className="text-primary underline-offset-2 hover:underline"
        >
          {value.split("/").pop()}
        </a>
      )
    }
    return String(value)
  }

  if (widget === "badge") {
    return <Badge value={String(value)} />
  }

  // A QR cell (gap #3) turns the value into something scannable — the same
  // meaning as the form widget of the same name.
  if (widget === "qrcode") {
    return <QrCode value={String(value)} size={72} />
  }

  if (widget === "boolean") {
    return value ? "Yes" : "No"
  }

  const formatter = fmt ?? createFormatter()

  if (format === "currency") {
    // A money value can be a bare number or the canonical {amount, currency}
    // object — accept both instead of falling through to JSON.stringify.
    const amount = moneyAmount(value)
    if (amount !== undefined) return formatter.money(amount)
  }

  if (format === "date" && typeof value === "string") {
    return formatter.date(value)
  }

  if (format === "relative" && typeof value === "string") {
    return formatter.relative(value)
  }

  // A plain number with locale grouping (05-field-types.md §10: separators
  // follow `settings.locale`). Opt-in rather than derived from the field type:
  // `decimal`/`integer` are just as often an identifier or a line number
  // (`line_number`) as a quantity, and the framework never guesses.
  if (format === "number") {
    const n = typeof value === "number" ? value : Number(value)
    if (!Number.isNaN(n)) return formatter.number(n, opts?.scale)
  }

  // S11: a percentage renders with its unit, so `10` does not read as 10 of
  // something unknown. The stored value is the percentage itself (10 = 10%).
  if (format === "percent") {
    const n = typeof value === "number" ? value : Number(value)
    if (!Number.isNaN(n)) return `${n}%`
  }

  if (typeof value === "object") return JSON.stringify(value)

  return String(value)
}

/**
 * Render a multi-value cell as comma-separated option **labels**.
 *
 * Without this a `days_of_week` column prints `JSON.stringify([1,2,3])` — the
 * same "value rendered raw in one place, formatted in another" split that the
 * `select-multi-tag` widget exists to close on the form side. Resolved once in
 * `resolveColumnCell` rather than per renderer, so Table, Listing, and
 * ChildTable cannot drift apart. Returns null when no display form applies.
 */
export function renderOptionCell(
  value: unknown,
  field: import("@/types/manifest").Field | undefined,
): string | null {
  if (!field || !Array.isArray(value)) return null
  const options = fieldOptions(field)
  if (options.length === 0) return null
  // Declaration order — the same ordering the form widget shows, so a set reads
  // identically in a table cell and in the form.
  const labels = orderByDeclaration(
    parseOptionValue(value, options),
    options,
  ).map((v) => optionLabel(options, v))
  return labels.length === 0 ? null : labels.join(", ")
}

/**
 * Derive a display widget/format hint from a field's type, so child-grid
 * columns render like parent-table columns without an explicit config:
 *   - enum        → badge
 *   - boolean     → boolean (Yes/No)
 *   - money       → currency
 *   - percent     → percent
 *   - file/attachment → image (#4): an image key renders inline, any other file
 *     falls back to a download link inside the same branch
 *   - date/datetime → date
 */
export function cellHintsForField(field: {
  type: string
  name?: string
  storage?: { allowed_types?: string[] }
}): {
  widget?: string
  format?: string
} {
  switch (field.type) {
    case "enum":
      return { widget: "badge" }
    case "boolean":
      return { widget: "boolean" }
    case "file":
    case "attachment":
      // Only claim the image widget when the field can actually hold an image;
      // a text/PDF field keeps the plain download link.
      return storageAllowsImage(field.storage) ? { widget: "image" } : {}
    case "money":
      return { format: "currency" }
    case "percent":
      return { format: "percent" }
    case "date":
    case "datetime":
      return { format: "date" }
    default:
      return {}
  }
}

/**
 * Resolve the value a column cell should display, handling the two relation
 * spellings and the field's own decimal scale. Both TableRenderer and
 * ListingRenderer use this so a column behaves identically in either kind —
 * the omission that produced 10.26 (align in one renderer only) is exactly
 * what a second copy here would reintroduce.
 *
 * Returns the value plus the scale `format: number` should use; the caller
 * still owns widget/format opts (image URL, alt text).
 */
export function resolveColumnCell(
  row: Record<string, unknown>,
  columnField: string,
  entity: EntitySchema | undefined,
  findEntity: (module: string, name: string) => EntitySchema | undefined,
): { value: unknown; scale?: number } {
  if (columnField.includes(".")) {
    // Dot-path (e.g. `branch.name`) — already nested on the record.
    return { value: readPath(row, columnField) }
  }

  const display = relationDisplay(row, columnField, entity, findEntity)
  if (display !== null) return { value: display }

  const field = entity?.fields.find((f) => f.name === columnField)

  // A multi-value field with a declared choice set prints its labels
  // (`Senin, Selasa`) instead of the raw array. Resolved here, not in each
  // renderer, because Table/Listing/ChildTable all read this one function.
  const optionsText = renderOptionCell(row[columnField], field)
  if (optionsText !== null) return { value: optionsText }

  return {
    value: row[columnField],
    scale: field?.scale,
  }
}
