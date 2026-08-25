// ─── Approval Inbox Renderer ───
//
// Pending-approval task queue (kind: ApprovalInbox, 06-page-kinds.md §11).
// Zero-config: sources are pending Workflow steps eligible for the caller.
//
// The backing data source is the caller's pending approvals. When a
// conventional approval entity is present in the bundle (e.g.
// `formspec.core.approval`), this renderer lists its pending rows with
// inline approve/reject actions; otherwise it shows a clear empty state —
// the kind plumbing (spec, registry, bundle, route) is complete regardless.
//
// Design doc §5.5 ApprovalInbox kind (F4)

import { useEffect, useMemo, useState } from "react"
import { toast } from "@/lib/ui"
import { Check, Loader2, X } from "lucide-react"

import type { Entry, ApprovalInboxSpec } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { apiList, apiPatch } from "@/lib/api"
import { useRealtime } from "@/hooks/useRealtime"
import { createFormatter } from "@/lib/format"
import { titleCase } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Badge } from "@/widgets/Badge"

interface ApprovalInboxRendererProps {
  entry: Entry<ApprovalInboxSpec>
}

/** Conventional approval entity refs this renderer looks for in the bundle. */
const APPROVAL_ENTITY_REFS = [
  "formspec.core.approval",
  "formspec.core.approval-task",
  "formspec.core.workflow-task",
]

export default function ApprovalInboxRenderer({
  entry,
}: ApprovalInboxRendererProps) {
  const getClient = useSessionStore((s) => s.getClient)
  const getEntity = useMetaStore((s) => s.getEntity)
  const settings = useMetaStore((s) => s.bundle?.settings)
  const formatter = useMemo(() => createFormatter(settings), [settings])

  // Resolve the backing approval entity, if any is present in the bundle.
  const approvalEntity = useMemo(() => {
    for (const ref of APPROVAL_ENTITY_REFS) {
      const [m, n] = ref.split(".")
      const e = getEntity(m, n)
      if (e) return e
    }
    return undefined
  }, [getEntity])

  const [items, setItems] = useState<Record<string, unknown>[]>([])
  const [loading, setLoading] = useState(true)
  const [busyId, setBusyId] = useState<string | null>(null)

  const fetchItems = async (silent = false) => {
    if (!approvalEntity) {
      setLoading(false)
      return
    }
    if (!silent) setLoading(true)
    try {
      const client = getClient()
      const result = await apiList<Record<string, unknown>>(
        client,
        `${approvalEntity.module}/${approvalEntity.name}`,
        { per_page: "100" },
      )
      setItems(result.items)
    } catch {
      setItems([])
    } finally {
      if (!silent) setLoading(false)
    }
  }

  useEffect(() => {
    fetchItems()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [approvalEntity])

  // Realtime (spec §5): matching entity event → silent refetch.
  const realtimeTick = useRealtime(
    entry.spec.realtime && approvalEntity
      ? `${approvalEntity.module}/${approvalEntity.name}`
      : "",
  )
  useEffect(() => {
    if (realtimeTick === 0) return
    fetchItems(true)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [realtimeTick])

  const decide = async (id: string, decision: "approve" | "reject") => {
    if (!approvalEntity) return
    setBusyId(id)
    try {
      const client = getClient()
      await apiPatch(
        client,
        `${approvalEntity.module}/${approvalEntity.name}/${id}`,
        { status: decision === "approve" ? "approved" : "rejected" },
      )
      toast.success(decision === "approve" ? "Approved" : "Rejected")
      fetchItems(true)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Action failed")
    } finally {
      setBusyId(null)
    }
  }

  const pendingCount = items.filter(
    (i) => String(i.status ?? "").toLowerCase() === "pending",
  ).length

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            {titleCase(entry.name)}
          </h1>
          <p className="text-sm text-muted-foreground">
            {pendingCount} pending approval{pendingCount !== 1 ? "s" : ""}
          </p>
        </div>
        {pendingCount > 0 && <Badge value={String(pendingCount)} />}
      </div>

      {!approvalEntity ? (
        <div className="rounded-md border border-dashed p-10 text-center text-sm text-muted-foreground">
          <p className="font-medium">No approval source configured</p>
          <p className="mt-1">
            ApprovalInbox is zero-config — it reads pending Workflow steps
            eligible for the caller. Add an approval entity (e.g.
            <code className="mx-1">formspec.core.approval</code>) to populate
            this inbox.
          </p>
        </div>
      ) : loading ? (
        <div className="flex items-center justify-center py-16">
          <Loader2 className="size-6 animate-spin text-muted-foreground" />
        </div>
      ) : items.length === 0 ? (
        <div className="rounded-md border border-dashed p-10 text-center text-sm text-muted-foreground">
          No pending approvals.
        </div>
      ) : (
        <div className="rounded-md border overflow-hidden">
          <table className="w-full caption-bottom text-sm">
            <thead className="border-b bg-muted/50">
              <tr>
                <th className="h-10 px-3 text-left font-medium text-muted-foreground">
                  Subject
                </th>
                <th className="h-10 px-3 text-left font-medium text-muted-foreground">
                  Status
                </th>
                <th className="h-10 px-3 text-left font-medium text-muted-foreground">
                  Requested
                </th>
                <th className="h-10 px-3 text-right font-medium text-muted-foreground">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody>
              {items.map((item) => {
                const status = String(item.status ?? "pending")
                const isPending = status.toLowerCase() === "pending"
                return (
                  <tr
                    key={String(item.id)}
                    className="border-b transition-colors hover:bg-muted/50"
                  >
                    <td className="p-3 align-middle font-medium">
                      {String(item.title ?? item.subject ?? item.id)}
                    </td>
                    <td className="p-3 align-middle">
                      <Badge value={titleCase(status)} />
                    </td>
                    <td className="p-3 align-middle text-muted-foreground">
                      {item.created_at
                        ? formatter.dateTime(String(item.created_at))
                        : "—"}
                    </td>
                    <td className="p-3 align-middle">
                      <div className="flex items-center justify-end gap-1">
                        {isPending && (
                          <>
                            <Button
                              size="sm"
                              variant="outline"
                              disabled={busyId === item.id}
                              onClick={() => decide(String(item.id), "approve")}
                            >
                              <Check className="size-3.5 mr-1" />
                              Approve
                            </Button>
                            <Button
                              size="sm"
                              variant="outline"
                              disabled={busyId === item.id}
                              onClick={() => decide(String(item.id), "reject")}
                            >
                              <X className="size-3.5 mr-1" />
                              Reject
                            </Button>
                          </>
                        )}
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
