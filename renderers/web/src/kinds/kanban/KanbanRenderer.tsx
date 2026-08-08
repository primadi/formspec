// ─── Kanban Renderer ───
//
// Drag-and-drop status board (kind: Kanban).
// Columns from manifest, cards from entity records.
// Drag = PATCH status_field, with 409 snap-back.
//
// Design doc §5.5 Kanban kind (F4)

import { useEffect, useState, useCallback, useMemo } from "react"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"
import {
  DndContext,
  DragOverlay,
  useDroppable,
  PointerSensor,
  useSensor,
  useSensors,
  closestCorners,
  type DragStartEvent,
  type DragEndEvent,
  type UniqueIdentifier,
} from "@dnd-kit/core"
import {
  SortableContext,
  verticalListSortingStrategy,
  useSortable,
} from "@dnd-kit/sortable"
import { Eye, Edit2, Trash2, GripVertical, MoreHorizontal, AlertTriangle, Check, Play, X } from "lucide-react"

import type {
  Entry,
  KanbanSpec,
  KanbanColumn,
  KanbanCard,
  TableAction,
  FilterSpec,
  EntitySchema,
  MetaBundle,
} from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { resolveEntityRef } from "@/engine/entityRef"
import { can as checkPermission } from "@/engine/permissions"
import { useSurface } from "@/hooks/useSurface"
import { useRealtime } from "@/hooks/useRealtime"
import { useSelectFilterOptions } from "@/hooks/useSelectFilterOptions"
import { apiList, apiPatch, apiDelete } from "@/lib/api"
import {
  buildFixedFilterParams,
  buildUserFilterParams,
  resolveFilterValue,
  shouldShowAll,
  allLabel,
} from "@/lib/filters"
import { titleCase } from "@/lib/utils"
import { Badge } from "@/widgets/Badge"
import { Input } from "@/components/ui/input"
import { Search } from "lucide-react"
import { Select } from "@/components/ui/select"
import ConfirmDialog from "@/components/ui/confirm-dialog"

// ── Constants ──

const DRAGGING_CLASS = "opacity-40"

// Sentinel for null position_field — sorts new records (null) to the END (FIFO).
const POS_NULL_SENTINEL = Number.MAX_SAFE_INTEGER

// ── Interfaces ──

interface KanbanRendererProps {
  entry: Entry<KanbanSpec>
}

// ── Filter control ──
//
// One control per `filters` entry. Select options come from the entity field
// definition (enum_values / related entity master data) — independent of the
// currently loaded (date-scoped) records — plus a configurable "All" (clear)
// option (`show_all` / `all_label`, default "(ALL)").

function KanbanFilterControl({
  filter,
  entity,
  metaBundle,
  getClient,
  value,
  onChange,
}: {
  filter: FilterSpec
  entity: EntitySchema | undefined
  metaBundle: MetaBundle | null
  getClient: () => import("ky").KyInstance
  value: string
  onChange: (value: string) => void
}) {
  const type = filter.type ?? "select"
  const selectOptions = useSelectFilterOptions(filter, entity, metaBundle, getClient)

  if (type === "date") {
    return (
      <Input
        type="date"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={filter.label || filter.field}
        className="h-9 w-40 text-xs"
      />
    )
  }
  if (type === "text") {
    return (
      <Input
        placeholder={filter.label || filter.field}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="h-9 w-40 text-xs"
      />
    )
  }
  // select (default)
  const showAll = shouldShowAll(filter)
  const options = [
    ...(showAll ? [{ value: "", label: allLabel(filter) }] : []),
    ...selectOptions,
  ]
  return (
    <Select
      value={value}
      onChange={onChange}
      options={options}
      placeholder={filter.label || filter.field}
    />
  )
}

// ── Field resolver ──

/**
 * Resolves a dot-path field name from a record object.
 * "patient.name" → record.patient.name
 * "queue_number" → record.queue_number
 * Falls back to empty string if any segment is missing.
 */
function resolveField(record: Record<string, unknown>, path: string): string {
  const parts = path.split(".")
  let value: unknown = record
  for (const part of parts) {
    if (value == null || typeof value !== "object") return ""
    value = (value as Record<string, unknown>)[part]
  }
  return value != null ? String(value) : ""
}

// ── Main Renderer ──

export default function KanbanRenderer({ entry }: KanbanRendererProps) {
  const navigate = useNavigate()
  const { surfacePath } = useSurface()
  const getClient = useSessionStore((s) => s.getClient)
  const getEntity = useMetaStore((s) => s.getEntity)
  const metaBundle = useMetaStore((s) => s.bundle)
  const me = useSessionStore((s) => s.me)

  const [entityModule, entityName] = resolveEntityRef(entry.spec.entity, entry.module)
  const entity = getEntity(entityModule, entityName)
  const columns = entry.spec.columns
  const statusField = entry.spec.status_field
  const cardTemplate = entry.spec.card_template
  const rowActions = entry.spec.row_actions

  const [records, setRecords] = useState<Record<string, unknown>[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState("")
  const [activeId, setActiveId] = useState<UniqueIdentifier | null>(null)
  // User-adjustable filters, pre-seeded from each filter's `default`
  // (e.g. `{ field: transaction_date, type: date, default: today }` → the
  // board opens scoped to the server's current date, still navigable).
  const [filterValues, setFilterValues] = useState<Record<string, string>>(() =>
    Object.fromEntries(
      (entry.spec.filters ?? [])
        .map((f) => [f.field, resolveFilterValue(f.default)])
        .filter(([, v]) => v !== ""),
    ),
  )
  const [pendingAction, setPendingAction] = useState<{
    action: TableAction
    record: Record<string, unknown>
  } | null>(null)

  // ── Sensors for drag detection ──
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: 8 },
    }),
  )

  // ── Data fetching ──
  const fetchRecords = useCallback(async (silent = false) => {
    if (!entity) {
      toast.error(`entity "${entry.spec.entity}" not found`)
      setLoading(false)
      return
    }
    // Realtime refetch is silent — don't flash the loading spinner on events.
    if (!silent) setLoading(true)
    try {
      const client = getClient()
      // Auto-default sort to position_field when sortable is enabled
      const sortField = entry.spec.sortable && entry.spec.position_field
        ? entry.spec.position_field
        : undefined
      // Merge immutable fixed_filters + the user's active filter values
      // (operator syntax `field[op]=value`) so the DB pre-filters rows
      // before they hit the wire — e.g. a board scoped to one date via a
      // filter with `default: today()`.
      const searchParams: Record<string, string> = {
        ...buildFixedFilterParams(entry.spec.fixed_filters),
        ...buildUserFilterParams(entry.spec.filters ?? [], filterValues),
      }
      if (sortField) searchParams.sort = sortField
      const result = await apiList<Record<string, unknown>>(
        client,
        `${entity.module}/${entity.name}`,
        Object.keys(searchParams).length > 0 ? searchParams : undefined,
      )
      setRecords(result.items)
    } catch {
      toast.error("Failed to load kanban data")
    } finally {
      if (!silent) setLoading(false)
    }
  }, [entity, entry.spec.entity, entry.spec.sortable, entry.spec.position_field, entry.spec.filters, entry.spec.fixed_filters, filterValues, getClient])

  useEffect(() => {
    fetchRecords()
  }, [fetchRecords])

  // ── Realtime (spec §5): matching entity event → silent refetch ──
  // Non-durable: reconnect also bumps the tick, so a dropped connection
  // triggers a refetch too. Event-driven when the Kanban has realtime: true.
  const realtimeTick = useRealtime(
    entry.spec.realtime && entityModule && entityName ? `${entityModule}/${entityName}` : "",
  )
  useEffect(() => {
    if (realtimeTick === 0) return
    fetchRecords(true)
  }, [realtimeTick, fetchRecords])

  // ── Filter/Search helpers ──

  const getColumnRecords = useCallback(
    (status: string) => {
      return records.filter((r) => {
        const statusMatch = (r[statusField] as string) === status
        if (!statusMatch) return false

        // Apply search
        if (search) {
          const searchMatch = Object.values(r).some((v) =>
            String(v ?? "").toLowerCase().includes(search.toLowerCase()),
          )
          if (!searchMatch) return false
        }

        // Apply filters from manifest — match the server's filtering semantics:
        // compare against the raw field value (e.g. the `_id` uuid for relation
        // filters), not a resolved label.
        for (const [field, value] of Object.entries(filterValues)) {
          if (!value) continue
          if (String(r[field] ?? "") !== value) return false
        }

        return true
      })
    },
    [records, statusField, search, filterValues],
  )

  // ── Drag handlers ──

  const handleDragStart = useCallback((event: DragStartEvent) => {
    setActiveId(event.active.id)
  }, [])

  const handleDragEnd = useCallback(
    async (event: DragEndEvent) => {
      const { active, over } = event
      setActiveId(null)

      if (!over || active.id === over.id) return

      const activeRecord = records.find(
        (r) => r.id === active.id,
      ) as Record<string, unknown> | undefined
      if (!activeRecord) return

      const positionField = entry.spec.position_field || null

      // Determine drop target: column status or card ID
      const targetColumn = columns.find((c) => c.status === over.id)
      const overRecord = !targetColumn
        ? (records.find((r) => r.id === over.id) as Record<string, unknown> | undefined)
        : undefined

      let targetStatus: string
      let targetPosition: number | null = null

      if (targetColumn) {
        // ── Drop onto column (append to end) ──
        targetStatus = targetColumn.status
        const currentStatus = activeRecord[statusField] as string
        const isSameColumn = currentStatus === targetStatus

        // WIP limit check — only for cross-column drops
        if (!isSameColumn) {
          const currentCount = getColumnRecords(targetStatus).length
          if (entry.spec.max_cards_per_column && currentCount >= entry.spec.max_cards_per_column) {
            toast.error(`Column "${targetColumn.label}" is full (max ${entry.spec.max_cards_per_column})`)
            return
          }
        }

        // Calculate position = max+1 of target column
        if (positionField) {
          const targetRecords = getColumnRecords(targetStatus)
          const maxPos = targetRecords.reduce(
            (max, r) => Math.max(max, (r[positionField] as number) ?? 0),
            0,
          )
          targetPosition = maxPos + 1000 // gap of 1000 for future insertions
        }

        // For same-column drop on column area → this is "move to end"
        // Don't return early — proceed to PATCH with new position
        if (isSameColumn && !positionField) return
      } else if (overRecord) {
        // ── Drop onto a card ──
        targetStatus = overRecord[statusField] as string
        const activeStatus = activeRecord[statusField] as string
        const activePosition = positionField ? (activeRecord[positionField] as number | null) ?? null : null

        if (activeStatus === targetStatus && positionField) {
          // ── Within-column reorder ──
          // Sort by position to match visual order
          const columnRecords = [...getColumnRecords(targetStatus)].sort((a, b) => {
            const pa = (a[positionField] as number) ?? POS_NULL_SENTINEL
            const pb = (b[positionField] as number) ?? POS_NULL_SENTINEL
            if (pa !== pb) return pa - pb
            // FIFO fallback: null positions sort by created_at ascending
            const ca = a.created_at ? new Date(String(a.created_at)).getTime() : 0
            const cb = b.created_at ? new Date(String(b.created_at)).getTime() : 0
            return ca - cb
          })
          const activeIdx = columnRecords.findIndex((r) => r.id === active.id)
          const overIdx = columnRecords.findIndex((r) => r.id === over.id)
          if (activeIdx === -1 || overIdx === -1) return

          const overPos = (overRecord[positionField] as number) ?? (overIdx + 1) * 1000
          const isDraggingDown = activeIdx < overIdx
          const MIN_GAP = 10 // minimum gap before triggering renumber

          if (isDraggingDown) {
            // Moving DOWN → card goes AFTER overRecord (between over and next)
            const nextRecord = columnRecords[overIdx + 1]
            const nextPos = nextRecord
              ? (nextRecord[positionField] as number) ?? (overIdx + 2) * 1000
              : overPos + 1000

            const rawGap = (nextPos as number) - overPos
            if (rawGap <= MIN_GAP) {
              // Gap too small — renumber column with fresh gaps
              return renumberColumn(columnRecords, activeIdx, overIdx, isDraggingDown, activeRecord, getClient, entity, entityModule, entityName, positionField, statusField, setRecords, toast)
            }
            targetPosition = overPos === nextPos ? overPos + 1000 : Math.floor((overPos + nextPos) / 2)
            if (targetPosition <= overPos) targetPosition = overPos + 1
          } else {
            // Moving UP → card goes BEFORE overRecord (between prev and over)
            const prevRecord = columnRecords[overIdx - 1]
            const prevPos = prevRecord
              ? (prevRecord[positionField] as number) ?? overIdx * 1000
              : overPos - 1000

            const rawGap = overPos - (prevPos as number)
            if (rawGap <= MIN_GAP) {
              // Gap too small — renumber column with fresh gaps
              return renumberColumn(columnRecords, activeIdx, overIdx, isDraggingDown, activeRecord, getClient, entity, entityModule, entityName, positionField, statusField, setRecords, toast)
            }
            targetPosition = overPos === prevPos ? overPos + 1 : Math.floor((prevPos + overPos) / 2)
            if (targetPosition <= (prevPos ?? 0)) targetPosition = overPos + 1
          }

          // Only exit early when active has a non-null position that matches
          if (activePosition != null && activePosition === targetPosition) return
        } else {
          // ── Cross-column drop onto specific card ──
          const currentStatus = activeRecord[statusField] as string
          if (currentStatus === targetStatus) return

          // Insert before overRecord
          if (positionField) {
            const targetRecords = [...getColumnRecords(targetStatus)].sort((a, b) => {
              const pa = (a[positionField] as number) ?? POS_NULL_SENTINEL
              const pb = (b[positionField] as number) ?? POS_NULL_SENTINEL
              if (pa !== pb) return pa - pb
              const ca = a.created_at ? new Date(String(a.created_at)).getTime() : 0
              const cb = b.created_at ? new Date(String(b.created_at)).getTime() : 0
              return ca - cb
            })
            const overIdx = targetRecords.findIndex((r) => r.id === over.id)
            if (overIdx === -1) return

            const overPos = (overRecord[positionField] as number) ?? (overIdx + 1) * 1000
            const prevRecord = targetRecords[overIdx - 1]
            const prevPos = prevRecord
              ? (prevRecord[positionField] as number) ?? overIdx * 1000
              : overPos - 1000
            targetPosition = overPos === prevPos ? overPos + 1 : Math.floor((prevPos + overPos) / 2)
            // Collision safety
            if (targetPosition <= (prevPos ?? 0)) targetPosition = overPos + 1
          }
        }
      } else {
        return // drop target not found
      }

      // ── Permission check ──
      const perm = `${entity?.module ?? entityModule}.${entity?.name ?? entityName}.update`
      if (me && !checkPermission(perm, me.permissions)) {
        toast.error("You don't have permission to move cards")
        return
      }

      // ── Build PATCH body ──
      const patchBody: Record<string, unknown> = { [statusField]: targetStatus }
      if (positionField && targetPosition != null) {
        patchBody[positionField] = targetPosition
      }

      // ── Save snapshot for rollback ──
      const snapshot = [...records]

      // ── Optimistic update ──
      setRecords((prev) =>
        prev.map((r) =>
          r.id === active.id ? { ...r, ...patchBody } : r,
        ),
      )

      // ── Server PATCH ──
      try {
        const client = getClient()
        await apiPatch(
          client,
          `${entity?.module ?? entityModule}/${entity?.name ?? entityName}/${active.id}`,
          patchBody,
          (activeRecord as Record<string, unknown>).version as number | undefined,
        )
        if (targetColumn) {
          toast.success(`Moved to ${targetColumn.label}`)
        } else {
          toast.success("Card reordered")
        }
      } catch (err) {
        // Rollback on error
        setRecords(snapshot)
        const msg =
          err instanceof Error ? err.message : "Failed to move card — server rejected the transition"
        toast.error(msg)
      }
    },
    [
      records,
      columns,
      statusField,
      entity,
      entityModule,
      entityName,
      getClient,
      me,
      getColumnRecords,
      entry.spec.max_cards_per_column,
      entry.spec.position_field,
    ],
  )

  // ── Row action handler ──

  const handleRowAction = useCallback(
    async (action: TableAction, record: Record<string, unknown>, skipConfirm = false) => {
      if (!me || !entity) return

      // Permission check
      const perm = `${entity.module}.${entity.plural}.${action.action}`
      if (!checkPermission(perm, me.permissions)) {
        toast.error("You don't have permission to perform this action")
        return
      }

      // Resolve confirm message
      const entityAction = entity.actions?.find((a) => a.name === action.action)
      const confirmMsg = action.confirm_msg ?? entityAction?.ui?.confirm

      if (confirmMsg && !skipConfirm) {
        setPendingAction({ action, record })
        return
      }

      const id = record.id as string

      switch (action.action) {
        case "view":
          navigate(surfacePath(entity.module, entity.plural, id))
          break
        case "edit":
          navigate(surfacePath(entity.module, entity.plural, id, "edit"))
          break
        case "delete":
          try {
            const client = getClient()
            await apiDelete(client, `${entity.module}/${entity.name}/${id}`)
            toast.success("Deleted successfully")
            setRecords((prev) => prev.filter((r) => r.id !== id))
          } catch (err) {
            toast.error(err instanceof Error ? err.message : "Delete failed")
          }
          break
        default:
          try {
            const client = getClient()
            await client.post(
              `${entity.module}/${entity.name}/${id}/${action.action}`,
            )
            toast.success("Action completed")
            fetchRecords()
          } catch (err) {
            toast.error(err instanceof Error ? err.message : "Action failed")
          }
          break
      }
    },
    [me, entity, navigate, surfacePath, getClient, fetchRecords],
  )

  // ── Refs for DragOverlay ──

  const activeRecord = useMemo(
    () => records.find((r) => r.id === activeId) ?? null,
    [records, activeId],
  )

  // ── Render ──

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            {titleCase(entry.name)}
          </h1>
        </div>

        <div className="flex items-center gap-2">
          {/* Filter controls — one per `filters` entry. `fixed_filters` are
              NOT rendered: they are immutable, server-side scoping. */}
          {entry.spec.filters?.map((f) => (
            <div key={f.field} className="relative min-w-32">
              <KanbanFilterControl
                filter={f}
                entity={entity}
                metaBundle={metaBundle}
                getClient={getClient}
                value={filterValues[f.field] ?? ""}
                onChange={(val) =>
                  setFilterValues((prev) => ({
                    ...prev,
                    [f.field]: val,
                  }))
                }
              />
            </div>
          ))}

          {/* Search */}
          {entry.spec.search && (
            <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
              <Input
                placeholder="Search cards..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="pl-9 max-w-xs"
              />
            </div>
          )}
        </div>
      </div>

      {/* Board */}
      <DndContext
        sensors={sensors}
        collisionDetection={closestCorners}
        onDragStart={handleDragStart}
        onDragEnd={handleDragEnd}
      >
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {columns.map((col) => (
            <KanbanColumn
              key={col.status}
              column={col}
              cards={getColumnRecords(col.status)}
              loading={loading}
              maxCards={entry.spec.max_cards_per_column}
              cardTemplate={cardTemplate}
              rowActions={rowActions}
              activeId={activeId}
              onRowAction={handleRowAction}
              entityModule={entity?.module ?? entityModule}
              entityPlural={entity?.plural ?? entityName}
              positionField={entry.spec.position_field}
            />
          ))}
        </div>

        {/* Drag overlay — rendered when dragging */}
        <DragOverlay>
          {activeRecord ? (
            <div className="rounded-md border bg-card p-3 shadow-xl rotate-3 w-64">
              <KanbanCardContent
                record={activeRecord}
                template={cardTemplate}
                entityModule={entity?.module ?? entityModule}
                entityPlural={entity?.plural ?? entityName}
              />
            </div>
          ) : null}
        </DragOverlay>
      </DndContext>

      {/* Confirm dialog for row actions */}
      <ConfirmDialog
        open={pendingAction !== null}
        onOpenChange={(open) => {
          if (!open) setPendingAction(null)
        }}
        title={pendingAction?.action.label ?? "Confirm Action"}
        message={pendingAction?.action.confirm_msg ?? "Are you sure?"}
        onConfirm={() => {
          if (!pendingAction) return
          handleRowAction(pendingAction.action, pendingAction.record, true)
          setPendingAction(null)
        }}
      />
    </div>
  )
}

// ── Renumber helper ──

/**
 * Renumbers all cards in a column with fresh 1000-gap positions.
 * Called when the gap between cards becomes too small (< MIN_GAP).
 *
 * 1. Sorts records with active card moved to its target position
 * 2. Assigns positions 1000, 2000, 3000, ...
 * 3. PATCHes each card; partial failure shows toast but keeps local state
 */
async function renumberColumn(
  columnRecords: Record<string, unknown>[],
  activeIdx: number,
  overIdx: number,
  isDraggingDown: boolean,
  _activeRecord: Record<string, unknown>,
  getClient: () => import("ky").KyInstance,
  entity: { module: string; name: string } | null | undefined,
  entityModule: string,
  entityName: string,
  positionField: string,
  statusField: string,
  setRecords: React.Dispatch<React.SetStateAction<Record<string, unknown>[]>>,
  toast: { error: (msg: string) => void; success: (msg: string) => void },
): Promise<void> {
  const client = getClient()
  const path = `${entity?.module ?? entityModule}/${entity?.name ?? entityName}`

  // Build new order: remove active from its position, insert at overIdx
  const newOrder = [...columnRecords]
  const [moved] = newOrder.splice(activeIdx, 1)
  const insertAt = isDraggingDown ? overIdx + 1 : overIdx
  newOrder.splice(insertAt, 0, moved)

  // Assign fresh positions with 1000 gap
  const repositions: { id: string; newPos: number; record: Record<string, unknown> }[] = []
  let versionMap = new Map<string, number>()
  for (const rec of columnRecords) {
    if (rec.id != null) {
      versionMap.set(String(rec.id), (rec as any).version as number)
    }
  }

  for (let i = 0; i < newOrder.length; i++) {
    const rec = newOrder[i]
    const id = String(rec.id ?? "")
    const newPos = (i + 1) * 1000
    repositions.push({ id, newPos, record: rec })
  }

  // Optimistic update: apply all new positions locally
  setRecords((prev) =>
    prev.map((r) => {
      const update = repositions.find((p) => p.id === r.id)
      return update ? { ...r, [positionField]: update.newPos } : r
    }),
  )

  // PATCH all affected cards (Promise.allSettled — best effort)
  const results = await Promise.allSettled(
    repositions.map((p) => {
      const version = (p.record as any).version as number | undefined
      const patchBody: Record<string, unknown> = { [positionField]: p.newPos }
      // Include status field so the PATCH doesn't try to clear it
      patchBody[statusField] = p.record[statusField] as string
      return apiPatch(client, `${path}/${p.id}`, patchBody, version)
    }),
  )

  const failures = results.filter((r) => r.status === "rejected")
  if (failures.length > 0) {
    toast.error(`${failures.length} card(s) failed to renumber — refresh to sync`)
  } else {
    toast.success("Column reordered")
  }
}

// ── Column Component ──

function KanbanColumn({
  column,
  cards,
  loading,
  maxCards,
  cardTemplate,
  rowActions,
  activeId,
  onRowAction,
  entityModule,
  entityPlural,
  positionField,
}: {
  column: KanbanColumn
  cards: Record<string, unknown>[]
  loading: boolean
  maxCards?: number
  cardTemplate?: KanbanCard
  rowActions?: TableAction[]
  activeId: UniqueIdentifier | null
  onRowAction: (action: TableAction, record: Record<string, unknown>) => void
  entityModule: string
  entityPlural: string
  positionField?: string
}) {
  const { setNodeRef, isOver } = useDroppable({
    id: column.status,
  })

  const isFull = maxCards != null && cards.length >= maxCards
  const isEmpty = cards.length === 0 && !loading

  // Sort cards by position_field when available
  const sortedCards = useMemo(() => {
    if (!positionField) return cards
    return [...cards].sort((a, b) => {
      const pa = (a[positionField] as number) ?? POS_NULL_SENTINEL
      const pb = (b[positionField] as number) ?? POS_NULL_SENTINEL
      if (pa !== pb) return pa - pb
      const ca = a.created_at ? new Date(String(a.created_at)).getTime() : 0
      const cb = b.created_at ? new Date(String(b.created_at)).getTime() : 0
      return ca - cb
    })
  }, [cards, positionField])

  const visibleCards = sortedCards.slice(0, maxCards)
  const cardIds = useMemo(() => visibleCards.map((c) => (c.id as string) ?? ""), [visibleCards])

  return (
    <div
      ref={setNodeRef}
      className={`rounded-md border bg-muted/30 flex flex-col h-full min-h-50 transition-colors ${
        isOver ? "border-primary/50 bg-primary/5 ring-1 ring-primary/20" : ""
      } ${isFull ? "border-dashed border-red-300" : ""}`}
    >
      {/* Column header */}
      <div className="border-b px-3 py-2 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <div
            className="size-2 rounded-full"
            style={{ backgroundColor: column.color ?? "var(--muted-foreground)" }}
          />
          <span className="text-sm font-medium">{column.label}</span>
          {isFull && (
            <span className="text-[10px] text-red-500 font-medium">FULL</span>
          )}
        </div>
        <Badge value={String(sortedCards.length)} />
      </div>

      {/* Cards */}
      <div className="flex-1 p-2 space-y-2 overflow-y-auto">
        {loading ? (
          <div className="text-center py-8 text-sm text-muted-foreground">
            Loading...
          </div>
        ) : isEmpty ? (
          <div className="text-center py-8 text-sm text-muted-foreground">
            No items
          </div>
        ) : (
          <SortableContext items={cardIds} strategy={verticalListSortingStrategy}>
            {visibleCards.map((card) => {
              const id = (card.id as string) ?? ""
              return (
                <SortableCard
                  key={id}
                  id={id}
                  record={card}
                  template={cardTemplate}
                  isDragging={activeId === id}
                  rowActions={rowActions}
                  onRowAction={onRowAction}
                  entityModule={entityModule}
                  entityPlural={entityPlural}
                />
              )
            })}
          </SortableContext>
        )}

        {maxCards && sortedCards.length > maxCards && (
          <p className="text-center text-xs text-muted-foreground">
            +{sortedCards.length - maxCards} more
          </p>
        )}
      </div>
    </div>
  )
}

// ── Sortable Card Wrapper ──

function SortableCard({
  id,
  record,
  template,
  isDragging,
  rowActions,
  onRowAction,
  entityModule,
  entityPlural,
}: {
  id: string
  record: Record<string, unknown>
  template?: KanbanCard
  isDragging: boolean
  rowActions?: TableAction[]
  onRowAction: (action: TableAction, record: Record<string, unknown>) => void
  entityModule: string
  entityPlural: string
}) {
  const [menuOpen, setMenuOpen] = useState(false)

  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
  } = useSortable({ id })

  const style = transform
    ? {
        transform: `translate3d(${transform.x}px, ${transform.y}px, 0)`,
        transition,
      }
    : undefined

  return (
    <div
      ref={setNodeRef}
      className={`rounded-md border bg-card p-3 shadow-sm hover:shadow-md transition-shadow space-y-1 relative group ${
        isDragging ? DRAGGING_CLASS : ""
      }`}
      style={style}
    >
      {/* Drag handle + action menu row */}
      <div className="flex items-start justify-between gap-1">
        {/* Drag handle */}
        <button
          className="cursor-grab active:cursor-grabbing text-muted-foreground hover:text-foreground -ml-1 -mt-1 p-0.5 rounded opacity-0 group-hover:opacity-100 transition-opacity shrink-0"
          {...listeners}
          {...attributes}
          title="Drag to move"
        >
          <GripVertical className="size-3.5" />
        </button>

        {/* Card content (takes remaining space) */}
        <div className="flex-1 min-w-0">
          <KanbanCardContent
            record={record}
            template={template}
            entityModule={entityModule}
            entityPlural={entityPlural}
          />
        </div>

        {/* Row actions dropdown */}
        {rowActions && rowActions.length > 0 && (
          <div className="relative shrink-0">
            <button
              className="text-muted-foreground hover:text-foreground p-0.5 rounded opacity-0 group-hover:opacity-100 transition-opacity"
              onClick={(e) => {
                e.stopPropagation()
                setMenuOpen((prev) => !prev)
              }}
            >
              <MoreHorizontal className="size-3.5" />
            </button>
            {menuOpen && (
              <>
                <div
                  className="fixed inset-0 z-10"
                  onClick={() => setMenuOpen(false)}
                />
                <div className="absolute right-0 top-6 z-20 w-36 rounded-md border bg-popover p-1 shadow-md">
                  {rowActions.map((action) => (
                    <button
                      key={action.action}
                      className="w-full text-left px-2 py-1.5 text-xs rounded hover:bg-accent flex items-center gap-2"
                      onClick={(e) => {
                        e.stopPropagation()
                        setMenuOpen(false)
                        onRowAction(action, record)
                      }}
                    >
                      {action.icon === "eye" || action.action === "view" ? (
                        <Eye className="size-3" />
                      ) : action.icon === "edit" || action.action === "edit" ? (
                        <Edit2 className="size-3" />
                      ) : action.icon === "trash" || action.action === "delete" ? (
                        <Trash2 className="size-3" />
                      ) : action.icon === "check" ? (
                        <Check className="size-3" />
                      ) : action.icon === "play" ? (
                        <Play className="size-3" />
                      ) : action.icon === "x" ? (
                        <X className="size-3" />
                      ) : action.icon === "alert-triangle" ? (
                        <AlertTriangle className="size-3" />
                      ) : null}
                      {action.label}
                    </button>
                  ))}
                </div>
              </>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

// ── Card Content (shared between static card and drag overlay) ──

function KanbanCardContent({
  record,
  template,
  entityModule,
  entityPlural,
}: {
  record: Record<string, unknown>
  template?: KanbanCard
  entityModule: string
  entityPlural: string
}) {
  const navigate = useNavigate()
  const { surfacePath } = useSurface()

  const title = useMemo(() => {
    if (template?.title) return resolveField(record, template.title)
    return (record.name as string) ?? (record.id as string) ?? ""
  }, [record, template?.title])

  const subtitle = useMemo(() => {
    if (template?.subtitle) return resolveField(record, template.subtitle)
    return null
  }, [record, template?.subtitle])

  const badge = useMemo(() => {
    if (template?.badge) return resolveField(record, template.badge)
    return null
  }, [record, template?.badge])

  const assignee = useMemo(() => {
    if (template?.assignee) return resolveField(record, template.assignee)
    return null
  }, [record, template?.assignee])

  // Click handler: navigate to detail page
  const handleClick = useCallback(() => {
    const id = record.id as string
    if (!id) return
    navigate(surfacePath(entityModule, entityPlural, id))
  }, [record, navigate, surfacePath, entityModule, entityPlural])

  return (
    <div
      className="cursor-pointer space-y-1"
      onClick={handleClick}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") handleClick()
      }}
    >
      {/* Title */}
      <p className="text-sm font-medium truncate">{title}</p>

      {/* Subtitle */}
      {subtitle && (
        <p className="text-xs text-muted-foreground truncate">{subtitle}</p>
      )}

      {/* Badge */}
      {badge && (
        <div>
          <span className="inline-flex items-center rounded-full bg-primary/10 px-2 py-0.5 text-[10px] font-medium text-primary">
            {badge}
          </span>
        </div>
      )}

      {/* Extra fields from card_template.fields */}
      {template?.fields && template.fields.length > 0 && (
        <div className="space-y-0.5 pt-1">
          {template.fields.map((fieldPath) => {
            const val = resolveField(record, fieldPath)
            if (!val) return null
            return (
              <p
                key={fieldPath}
                className="text-[11px] text-muted-foreground truncate"
              >
                {val}
              </p>
            )
          })}
        </div>
      )}

      {/* Assignee */}
      {assignee && (
        <div className="flex items-center gap-2 pt-1">
          <span className="text-[11px] text-muted-foreground truncate">
            {assignee}
          </span>
        </div>
      )}
    </div>
  )
}
