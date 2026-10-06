// ─── Approval Inbox Renderer ───
//
// Pending-approval task queue (kind: ApprovalInbox, 06-page-kinds.md §11).
// Zero-config: sources are pending Workflow steps eligible for the caller.
//
// WHERE THE DATA COMES FROM (todo 5.13.6, plan
// docs_internal/plan/approval-inbox-endpoint.md):
//
// The rows live in the framework table `formspec_workflow_approval`, which is
// NOT an entity. This renderer used to look for a "conventional approval
// entity" in the bundle (`formspec.core.approval` et al.) — an entity that does
// not exist in any workspace — so the lookup always came back empty and the
// inbox showed "No approval source configured" permanently. It now reads the
// kind's real source: `GET /{ws}/_ui/workflow/approvals`, which lists the
// caller's eligible tasks with the workflow step's own `title`/`description`
// and the record values the approver needs.
//
// Eligibility, tenant/App narrowing, and the per-row `can_decide` flag are all
// decided server-side — the client renders what it is given rather than
// re-deriving who may approve what.
//
// The decision call delegates to the same engine as the record's own page, so
// approve/reject here goes through quorum, requester exclusion, the audit
// record and the transition's event emission.
//
// Design doc §5.5 ApprovalInbox kind (F4)

import { useEffect, useMemo, useState } from "react"
import { Link } from "react-router-dom"
import { toast } from "@/lib/ui"
import { Check, Loader2, X } from "lucide-react"

import type { Entry, ApprovalInboxSpec } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { apiList, apiPost } from "@/lib/api"
import { approvalDecisionPath, approvalListPath } from "@/lib/approvalInbox"
import { getEntityRouteSegment } from "@/lib/entityIdentity"
import { useSurface } from "@/hooks/useSurface"
import { createFormatter, moneyAmount, type Formatter } from "@/lib/format"
import { titleCase } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Badge } from "@/widgets/Badge"

/** One value the approver needs in order to decide (step `display_fields`). */
export interface ApprovalField {
  field: string
  label?: string
  /** The entity field's declared type — the server sends it because a money
   *  value arrives as `{amount, currency}` and nothing on the wire would tell
   *  the renderer how to print it. */
  type?: string
  value: unknown
}

/**
 * Format one display value by the field's DECLARED type.
 *
 * `String(value)` printed a money value as "[object Object]" — measured in this
 * inbox on kafe's Total Amount, where the approver is asked to check the total
 * before approving a refund. Formatting every number as money would mislabel a
 * `decimal` or an `integer`, so the type decides. An unknown or absent type
 * falls back to the plain string; an object with no usable amount shows "—"
 * rather than an unreadable blob.
 *
 * Exported (like `PrintRenderer`'s `formatValue`) so the rule is unit-testable
 * without standing up the whole renderer.
 */
export function formatApprovalField(
  f: ApprovalField,
  formatter: Formatter,
): string {
  const { value, type } = f
  if (value === null || value === undefined || value === "") return "—"
  switch (type) {
    case "money": {
      const amount = moneyAmount(value)
      return amount === undefined ? "—" : formatter.money(amount)
    }
    case "datetime":
      return formatter.dateTime(String(value))
    case "date":
      return formatter.date(String(value))
    case "decimal":
    case "integer":
    case "number": {
      const n = moneyAmount(value)
      return n === undefined ? "—" : formatter.number(n)
    }
    default:
      if (typeof value === "object") return "—"
      return String(value)
  }
}

/** One pending approval task, as served by /_ui/workflow/approvals. */
interface ApprovalTask {
  id: string
  workflow: string
  workflow_module: string
  entity: string
  record_id: string
  from: string
  to: string
  status: string
  active_step: number
  total_steps: number
  requester_id?: string
  title?: string
  description?: string
  display_fields?: ApprovalField[]
  can_decide: boolean
  created_at?: string
}

interface ApprovalInboxRendererProps {
  entry: Entry<ApprovalInboxSpec>
}

export default function ApprovalInboxRenderer({
  entry,
}: ApprovalInboxRendererProps) {
  const getClient = useSessionStore((s) => s.getClient)
  const getEntity = useMetaStore((s) => s.getEntity)
  const appName = useMetaStore((s) => s.bundle?.app.name)
  const settings = useMetaStore((s) => s.bundle?.settings)
  const formatter = useMemo(() => createFormatter(settings), [settings])
  const { surfacePath } = useSurface()

  const [tasks, setTasks] = useState<ApprovalTask[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busyId, setBusyId] = useState<string | null>(null)

  const fetchTasks = async (silent = false) => {
    if (!silent) setLoading(true)
    try {
      const client = getClient()
      const result = await apiList<ApprovalTask>(
        client,
        approvalListPath(appName),
      )
      setTasks(result.items)
      setError(null)
    } catch (err) {
      // A failed LIST is reported, never rendered as an empty queue: "no work
      // to do" and "we could not ask" are different answers, and this is the
      // one place where confusing them costs an unapproved void.
      setTasks([])
      setError(err instanceof Error ? err.message : "Failed to load approvals")
    } finally {
      if (!silent) setLoading(false)
    }
  }

  useEffect(() => {
    fetchTasks()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [appName])

  // TODO(5.13.7 ⏸️): `realtime: true` is NOT honoured yet. The WS hub pushes
  // per `{module}/{entity}`, and an approval is not an entity, so there is no
  // topic to subscribe to; the server would have to broadcast on a workflow
  // topic first. Until then a supervisor must reload to see a new task.

  const decide = async (id: string, decision: "approve" | "reject") => {
    setBusyId(id)
    try {
      const client = getClient()
      await apiPost(client, approvalDecisionPath(id), { decision })
      toast.success(decision === "approve" ? "Approved" : "Rejected")
      // Refetch rather than dropping the row locally: a multi-step workflow
      // only advances, so the task may still be pending on the next step.
      fetchTasks(true)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Action failed")
      fetchTasks(true)
    } finally {
      setBusyId(null)
    }
  }

  const resolveEntity = (ref: string) => {
    const i = ref.lastIndexOf(".")
    if (i <= 0) return undefined
    return getEntity(ref.slice(0, i), ref.slice(i + 1))
  }

  const formatFieldValue = (f: ApprovalField) =>
    formatApprovalField(f, formatter)

  const pendingCount = tasks.filter(
    (t) => String(t.status ?? "").toLowerCase() === "pending",
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

      {loading ? (
        <div className="flex items-center justify-center py-16">
          <Loader2 className="size-6 animate-spin text-muted-foreground" />
        </div>
      ) : error ? (
        <div className="rounded-md border border-dashed p-10 text-center text-sm text-muted-foreground">
          <p className="font-medium">Could not load approvals</p>
          <p className="mt-1">{error}</p>
        </div>
      ) : tasks.length === 0 ? (
        <div className="rounded-md border border-dashed p-10 text-center text-sm text-muted-foreground">
          No pending approvals.
        </div>
      ) : (
        <ul className="space-y-2">
          {tasks.map((task) => {
            const entity = resolveEntity(task.entity)
            const isPending = String(task.status).toLowerCase() === "pending"
            const busy = busyId === task.id
            return (
              <li
                key={task.id}
                className="rounded-md border p-4 transition-colors hover:bg-muted/40"
              >
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="min-w-0 space-y-1">
                    <div className="flex items-center gap-2">
                      <p className="font-medium">
                        {task.title || task.workflow}
                      </p>
                      <Badge value={titleCase(task.status)} />
                      {task.total_steps > 1 && (
                        <span className="text-xs text-muted-foreground">
                          step {task.active_step + 1} of {task.total_steps}
                        </span>
                      )}
                    </div>
                    {task.description && (
                      <p className="text-sm text-muted-foreground">
                        {task.description}
                      </p>
                    )}
                    {task.display_fields && task.display_fields.length > 0 && (
                      <dl className="flex flex-wrap gap-x-6 gap-y-1 pt-1 text-sm">
                        {task.display_fields.map((f) => (
                          <div key={f.field} className="flex gap-1.5">
                            <dt className="text-muted-foreground">
                              {f.label || titleCase(f.field)}:
                            </dt>
                            <dd className="font-medium">
                              {formatFieldValue(f)}
                            </dd>
                          </div>
                        ))}
                      </dl>
                    )}
                    <div className="flex flex-wrap items-center gap-x-3 pt-1 text-xs text-muted-foreground">
                      {entity ? (
                        <Link
                          className="underline underline-offset-2 hover:text-foreground"
                          to={surfacePath(
                            entity.module,
                            entity.plural,
                            getEntityRouteSegment(entity, {
                              id: task.record_id,
                            }),
                          )}
                        >
                          {titleCase(entity.name)}
                        </Link>
                      ) : (
                        <span>{task.entity}</span>
                      )}
                      <span>
                        {task.from} → {task.to}
                      </span>
                      {task.created_at && (
                        <span>{formatter.dateTime(task.created_at)}</span>
                      )}
                    </div>
                  </div>

                  <div className="flex shrink-0 items-center gap-1">
                    {isPending && task.can_decide && (
                      <>
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={busy}
                          onClick={() => decide(task.id, "approve")}
                        >
                          <Check className="size-3.5 mr-1" />
                          Approve
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={busy}
                          onClick={() => decide(task.id, "reject")}
                        >
                          <X className="size-3.5 mr-1" />
                          Reject
                        </Button>
                      </>
                    )}
                    {/* Eligible but not permitted: the buttons are withheld and
                        the REASON is stated. A task silently missing its
                        actions reads as a broken page. */}
                    {isPending && !task.can_decide && (
                      <span className="text-xs text-muted-foreground">
                        Not permitted to decide
                      </span>
                    )}
                  </div>
                </div>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
