// ─── Print Renderer ───
//
// Printable document (kind: Print).
// Supports `format: html` — uses window.print() + @page CSS.
//
// Design doc §5.5 Print kind (F5)

import { useEffect, useMemo, useState, useCallback } from "react"
import { useParams } from "react-router-dom"
import { Printer, Loader2 } from "lucide-react"
import { toast } from "@/lib/ui"

import type { Entry, PrintQrcode, PrintSpec } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { resolveEntityRef } from "@/engine/entityRef"
import { apiGet } from "@/lib/api"
import { titleCase } from "@/lib/utils"
import { interpolate, resolvePath } from "@/lib/interpolate"
import { createFormatter, moneyAmount, type Formatter } from "@/lib/format"
import { Button } from "@/components/ui/button"
import { Badge } from "@/widgets/Badge"
import { QrCode } from "@/widgets/QrCode"

interface PrintRendererProps {
  entry: Entry<PrintSpec>
}

export default function PrintRenderer({ entry }: PrintRendererProps) {
  const { id, workspace } = useParams<{ id?: string; workspace?: string }>()
  const getClient = useSessionStore((s) => s.getClient)
  const getEntity = useMetaStore((s) => s.getEntity)
  // Money values arrive as `{amount, currency}` objects; the formatter carries
  // the resolved currency symbol + locale, so print output matches every other
  // surface instead of stringifying the object (item 2.6 / gap #1 residual).
  const settings = useMetaStore((s) => s.bundle?.settings)
  const formatter = useMemo(() => createFormatter(settings), [settings])

  const [entityModule, entityName] = resolveEntityRef(
    entry.spec.entity,
    entry.module,
  )
  const entity = getEntity(entityModule, entityName)
  const [record, setRecord] = useState<Record<string, unknown> | null>(null)
  const [loading, setLoading] = useState(!!id)
  const [printMode, setPrintMode] = useState(false)

  useEffect(() => {
    if (!id) return
    if (!entity) {
      toast.error(`entity "${entry.spec.entity}" not found`)
      setLoading(false)
      return
    }
    const load = async () => {
      try {
        const client = getClient()
        const data = await apiGet<Record<string, unknown>>(
          client,
          `${entity.module}/${entity.name}/${id}`,
        )
        setRecord(data)
      } catch {
        toast.error("Failed to load record")
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [id, entity, entry.spec.entity, getClient])

  const handlePrint = useCallback(() => {
    setPrintMode(true)
    setTimeout(() => {
      window.print()
      setPrintMode(false)
    }, 100)
  }, [])

  const formatDate = entry.spec.output?.paper?.size ?? "A4"

  // Interpolation context: flat record fields (already relation-expanded by
  // the backend, e.g. record.polyclinic.name), the record again keyed by its
  // own entity name (so `{visit.queue_number}` resolves the same as
  // `{queue_number}`), and the current workspace.
  const ctx: Record<string, unknown> | null = record
    ? { ...record, [entry.spec.entity]: record, workspace: { name: workspace } }
    : null

  if (loading) {
    return (
      <div className="flex items-center justify-center p-8">
        <Loader2 className="size-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* Toolbar (hidden when printing) */}
      {!printMode && (
        <div className="flex items-center justify-between no-print">
          <div>
            <h1 className="text-2xl font-bold tracking-tight">
              {titleCase(entry.name)}
            </h1>
            <Badge value={`Format: ${formatDate}`} />
          </div>
          <Button onClick={handlePrint}>
            <Printer className="size-4 mr-1" />
            Print
          </Button>
        </div>
      )}

      {/* Document */}
      <div
        className={`print-doc bg-white rounded-md border p-8 ${printMode ? "shadow-none" : "shadow-sm"}`}
        style={printMode ? { maxWidth: "210mm", margin: "0 auto" } : undefined}
      >
        {/* Header */}
        {entry.spec.header && (
          <div className="border-b pb-4 mb-6">
            {entry.spec.header.logo && (
              <div className="text-2xl font-bold mb-2">FormSpec</div>
            )}
            {entry.spec.header.title && (
              <h1 className="text-xl font-bold">
                {interpolate(entry.spec.header.title, ctx)}
              </h1>
            )}
            {entry.spec.header.subtitle && (
              <p className="text-sm text-muted-foreground">
                {interpolate(entry.spec.header.subtitle, ctx)}
              </p>
            )}
          </div>
        )}

        {/* Body */}
        {entry.spec.body?.length ? (
          <div className="space-y-4">
            {entry.spec.body.map((item, idx) => {
              if (item.fields) {
                return (
                  <div key={idx} className="grid grid-cols-2 gap-2 text-sm">
                    {item.fields.map((field) => (
                      <div
                        key={field}
                        className="flex justify-between border-b py-1"
                      >
                        <span className="text-muted-foreground">{field}</span>
                        <span className="font-medium">
                          {ctx
                            ? resolveCellValue(ctx, field, formatter)
                            : `{${field}}`}
                        </span>
                      </div>
                    ))}
                  </div>
                )
              }
              if (item.separator) {
                return <hr key={idx} className="my-2" />
              }
              if (item.child_table && record) {
                const children = record[item.child_table.field] as Record<
                  string,
                  unknown
                >[]
                if (children?.length) {
                  return (
                    <div key={idx} className="space-y-1">
                      <h3 className="text-sm font-medium">
                        {item.child_table.field}
                      </h3>
                      <table className="w-full text-sm border">
                        <thead>
                          <tr className="bg-muted/50">
                            {item.child_table.columns.map((col) => (
                              <th
                                key={col}
                                className="border px-2 py-1 text-left text-xs"
                              >
                                {col}
                              </th>
                            ))}
                          </tr>
                        </thead>
                        <tbody>
                          {children.map((child, ci) => (
                            <tr key={ci}>
                              {item.child_table!.columns.map((col) => (
                                <td
                                  key={col}
                                  className="border px-2 py-1 text-xs"
                                >
                                  {formatValue(child[col], formatter)}
                                </td>
                              ))}
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )
                }
              }
              if (item.qrcode) {
                // QR code (gap #3 / S4). The payload is interpolated from the
                // record and, when declared `absolute`, prefixed with the
                // origin of the page it is printed from — the browser knows the
                // origin, the record never does. A record with nothing to
                // encode omits the element rather than showing a code that
                // leads nowhere (the server pipelines do the same).
                const payload = resolveQrPayload(
                  item.qrcode,
                  ctx,
                  window.location.origin,
                )
                if (!payload) return null
                return (
                  <div
                    key={idx}
                    className="flex flex-col items-center gap-1 py-2"
                  >
                    {item.qrcode.label && (
                      <span className="text-xs text-muted-foreground">
                        {ctx ? interpolate(item.qrcode.label, ctx) : ""}
                      </span>
                    )}
                    <QrCode
                      value={payload}
                      size={qrPixelSize(item.qrcode.size_mm)}
                      emptyText="QR belum bisa dibuat"
                    />
                  </div>
                )
              }
              if (item.totals && ctx) {
                return (
                  <div
                    key={idx}
                    className="text-right text-sm font-medium border-t pt-2"
                  >
                    {item.totals.field}:{" "}
                    {resolveCellValue(ctx, item.totals.field, formatter)}
                  </div>
                )
              }
              return null
            })}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">
            {record
              ? JSON.stringify(record, null, 2)
              : "No data loaded. Select a record to print."}
          </p>
        )}

        {/* Footer */}
        {entry.spec.footer && (
          <div className="border-t pt-4 mt-6 text-xs text-muted-foreground text-center">
            {interpolate(entry.spec.footer.text, ctx)}
          </div>
        )}
      </div>

      {/* Print styles */}
      <style>{`
        /* Force light-theme colors inside the document so print output is
           never affected by the app's dark mode. */
        .print-doc {
          --color-background: oklch(1 0 0);
          --color-foreground: oklch(0.145 0 0);
          --color-muted: oklch(0.97 0 0);
          --color-muted-foreground: oklch(0.556 0 0);
          color: var(--color-foreground);
          background: white;
        }
        @media print {
          .no-print { display: none !important; }
          body { background: white; }
          .print-doc { box-shadow: none !important; border: none !important; padding: 0 !important; }
          @page { margin: 20mm; size: ${formatDate}; }
        }
      `}</style>
    </div>
  )
}

// ── Cell value display (distinct from `interpolate` — this formats a single
// resolved value for a `body.fields` cell, defaulting to "-" when missing,
// rather than substituting tokens inside a larger string) ──
function resolveCellValue(
  ctx: Record<string, unknown>,
  path: string,
  formatter?: Formatter,
): string {
  const value = resolvePath(ctx, path)
  return value == null || value === "" ? "-" : formatValue(value, formatter)
}

// A money value is recognized by its shape (`{amount, currency}` — the canonical
// wire form written at the API boundary, 2.3), never by guessing from a plain
// number: formatting a quantity as money would be worse than printing it raw.
export function isMoneyObject(value: unknown): boolean {
  if (typeof value !== "object" || value === null) return false
  const v = value as Record<string, unknown>
  return "amount" in v && "currency" in v
}

/** Render one print value: money via the shared formatter, everything else as text. */
export function formatValue(value: unknown, formatter?: Formatter): string {
  if (value == null || value === "") return ""
  if (isMoneyObject(value) && formatter) {
    const amount = moneyAmount(value)
    if (amount !== undefined) return formatter.money(amount)
  }
  return String(value)
}

/**
 * Resolve a `qrcode` body item's payload. `absolute` prepends the origin the
 * document is printed from; a payload that is already a full URL is left as
 * written. An empty result renders the widget's explicit empty state rather
 * than a blank, unscannable code.
 *
 * `interpolate()` leaves unresolvable `{tokens}` verbatim (so they are visible
 * in the document instead of vanishing), which for a QR would encode a URL that
 * leads nowhere — so a leftover token is treated as an empty payload here. The
 * server pipelines fail loudly on the same condition.
 */
export function resolveQrPayload(
  qr: PrintQrcode,
  ctx: Record<string, unknown> | null,
  origin: string,
): string {
  const raw = (ctx ? interpolate(qr.payload, ctx) : qr.payload).trim()
  if (!raw || /\{[^}]*\}/.test(raw)) return ""
  if (!qr.absolute || /^[a-z][a-z0-9+.-]*:\/\//i.test(raw)) return raw
  return origin.replace(/\/$/, "") + (raw.startsWith("/") ? raw : `/${raw}`)
}

/** Millimetres → CSS pixels at 96dpi (1mm ≈ 3.7795px), default 30mm. */
export function qrPixelSize(sizeMm?: number): number {
  const mm = sizeMm && sizeMm > 0 ? sizeMm : 30
  return Math.round(mm * 3.7795)
}
