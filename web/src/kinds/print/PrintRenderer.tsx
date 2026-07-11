// ─── Print Renderer ───
//
// Printable document (kind: Print).
// Supports `format: html` — uses window.print() + @page CSS.
//
// Design doc §5.5 Print kind (F5)

import { useEffect, useState, useCallback } from "react"
import { useParams } from "react-router-dom"
import { Printer, Loader2 } from "lucide-react"
import { toast } from "sonner"

import type { Entry, PrintSpec } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { apiGet } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Badge } from "@/widgets/Badge"

interface PrintRendererProps {
  entry: Entry<PrintSpec>
}

export default function PrintRenderer({ entry }: PrintRendererProps) {
  const { id } = useParams<{ id?: string }>()
  const getClient = useSessionStore((s) => s.getClient)
  const getEntity = useMetaStore((s) => s.getEntity)

  const entity = getEntity(entry.module, entry.spec.entity)
  const [record, setRecord] = useState<Record<string, unknown> | null>(null)
  const [loading, setLoading] = useState(!!id)
  const [printMode, setPrintMode] = useState(false)

  useEffect(() => {
    if (!id || !entity) return
    const load = async () => {
      try {
        const client = getClient()
        const data = await apiGet<Record<string, unknown>>(
          client,
          `${entity.module}/${entity.plural}/${id}`,
        )
        setRecord(data)
      } catch {
        toast.error("Failed to load record")
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [id, entity, getClient])

  const handlePrint = useCallback(() => {
    setPrintMode(true)
    setTimeout(() => {
      window.print()
      setPrintMode(false)
    }, 100)
  }, [])

  const formatDate = entry.spec.output?.paper?.size ?? "A4"

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
              {entry.spec.entity} Document
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
              <div className="text-2xl font-bold mb-2">Forma</div>
            )}
            {entry.spec.header.title && (
              <h1 className="text-xl font-bold">{entry.spec.header.title}</h1>
            )}
            {entry.spec.header.subtitle && (
              <p className="text-sm text-muted-foreground">
                {entry.spec.header.subtitle}
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
                      <div key={field} className="flex justify-between border-b py-1">
                        <span className="text-muted-foreground">{field}</span>
                        <span className="font-medium">
                          {record ? String(record[field] ?? "-") : `{${field}}`}
                        </span>
                      </div>
                    ))}
                  </div>
                )
              }
              if (item.separator) {
                return (
                  <hr key={idx} className="my-2" />
                )
              }
              if (item.child_table && record) {
                const children = record[item.child_table.field] as Record<string, unknown>[]
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
                              <th key={col} className="border px-2 py-1 text-left text-xs">
                                {col}
                              </th>
                            ))}
                          </tr>
                        </thead>
                        <tbody>
                          {children.map((child, ci) => (
                            <tr key={ci}>
                              {item.child_table!.columns.map((col) => (
                                <td key={col} className="border px-2 py-1 text-xs">
                                  {String(child[col] ?? "")}
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
              if (item.totals && record) {
                return (
                  <div key={idx} className="text-right text-sm font-medium border-t pt-2">
                    {item.totals.field}: {String(record[item.totals.field] ?? "-")}
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
            {entry.spec.footer.text}
          </div>
        )}
      </div>

      {/* Print styles */}
      <style>{`
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
