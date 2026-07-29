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
  useDraggable,
  useDroppable,
  PointerSensor,
  useSensor,
  useSensors,
  closestCorners,
  type DragStartEvent,
  type DragEndEvent,
  type UniqueIdentifier,
} from "@dnd-kit/core"
import { Eye, Edit2, Trash2, GripVertical, MoreHorizontal } from "lucide-react"

import type {
  Entry,
  KanbanSpec,
  KanbanColumn,
  KanbanCard,
  TableAction,
} from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { resolveEntityRef } from "@/engine/entityRef"
import { can as checkPermission } from "@/engine/permissions"
import { useSurface } from "@/hooks/useSurface"
import { apiList, apiPatch, apiDelete } from "@/lib/api"
import { Badge } from "@/widgets/Badge"
import { Input } from "@/components/ui/input"
import { Search } from "lucide-react"
import { Select } from "@/components/ui/select"
import ConfirmDialog from "@/components/ui/confirm-dialog"

// ── Constants ──

const DRAGGING_CLASS = "opacity-40"

// ── Interfaces ──

interface KanbanRendererProps {
  entry: Entry<KanbanSpec>
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
  const [filterValues, setFilterValues] = useState<Record<string, string>>({})
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
  const fetchRecords = useCallback(async () => {
    if (!entity) {
      toast.error(`entity "${entry.spec.entity}" not found`)
      setLoading(false)
      return
    }
    setLoading(true)
    try {
      const client = getClient()
      const result = await apiList<Record<string, unknown>>(
        client,
        `${entity.module}/${entity.name}`,
      )
      setRecords(result.items)
    } catch {
      toast.error("Failed to load kanban data")
    } finally {
      setLoading(false)
    }
  }, [entity, entry.spec.entity, getClient])

  useEffect(() => {
    fetchRecords()
  }, [fetchRecords])

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

        // Apply filters from manifest — resolve relation names for _id fields
        for (const [field, value] of Object.entries(filterValues)) {
          if (!value) continue
          const relationKey = field.endsWith("_id") ? field.slice(0, -3) : null
          let recordVal: string | null = null
          if (relationKey) {
            const rel = r[relationKey]
            if (rel && typeof rel === "object" && "name" in (rel as Record<string, unknown>)) {
              recordVal = (rel as Record<string, unknown>).name as string
            }
          }
          if (recordVal == null) {
            recordVal = String(r[field] ?? "")
          }
          if (recordVal !== value) return false
        }

        return true
      })
    },
    [records, statusField, search, filterValues],
  )

  // ── Compute unique filter options per filter field ──
  //
  // For relation fields (convention: ends with `_id`), resolve the display
  // label from the nested relation object — e.g. polyclinic_id →
  // r.polyclinic.name → "Poli Jantung". Falls back to raw value if
  // the relation object is not present.
  const filterOptions = useMemo(() => {
    const options: Record<string, string[]> = {}
    for (const field of entry.spec.filters ?? []) {
      const vals = new Set<string>()
      // Try resolved relation name first (_id → relation.name)
      const relationKey = field.endsWith("_id") ? field.slice(0, -3) : null
      for (const r of records) {
        let v: string | null = null
        if (relationKey) {
          const rel = r[relationKey]
          if (rel && typeof rel === "object" && "name" in (rel as Record<string, unknown>)) {
            v = (rel as Record<string, unknown>).name as string
          }
        }
        if (v == null) {
          const raw = r[field]
          v = raw != null && raw !== "" ? String(raw) : null
        }
        if (v) vals.add(v)
      }
      options[field] = Array.from(vals).sort()
    }
    return options
  }, [records, entry.spec.filters])

  // ── Drag handlers ──

  const handleDragStart = useCallback((event: DragStartEvent) => {
    setActiveId(event.active.id)
  }, [])

  const handleDragEnd = useCallback(
    async (event: DragEndEvent) => {
      const { active, over } = event
      setActiveId(null)

      if (!over || active.id === over.id) return

      const targetColumnStatus = over.id as string
      const targetColumn = columns.find((c) => c.status === targetColumnStatus)
      if (!targetColumn) return

      const activeRecord = records.find(
        (r) => r.id === active.id,
      ) as Record<string, unknown> | undefined
      if (!activeRecord) return

      const currentStatus = activeRecord[statusField] as string
      if (currentStatus === targetColumnStatus) return

      // ── WIP limit check ──
      const currentCount = getColumnRecords(targetColumnStatus).length
      if (entry.spec.max_cards_per_column && currentCount >= entry.spec.max_cards_per_column) {
        toast.error(`Column "${targetColumn.label}" is full (max ${entry.spec.max_cards_per_column})`)
        return
      }

      // ── Permission check ──
      const perm = `${entity?.module ?? entityModule}.${entity?.name ?? entityName}.update`
      if (me && !checkPermission(perm, me.permissions)) {
        toast.error("You don't have permission to move cards")
        return
      }

      // ── Save snapshot for rollback ──
      const snapshot = [...records]

      // ── Optimistic update ──
      setRecords((prev) =>
        prev.map((r) =>
          r.id === active.id ? { ...r, [statusField]: targetColumnStatus } : r,
        ),
      )

      // ── Server PATCH ──
      try {
        const client = getClient()
        await apiPatch(
          client,
          `${entity?.module ?? entityModule}/${entity?.name ?? entityName}/${active.id}`,
          { [statusField]: targetColumnStatus },
          (activeRecord as Record<string, unknown>).version as number | undefined,
        )
        toast.success(`Moved to ${targetColumn.label}`)
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
            {entry.spec.entity} Board
          </h1>
        </div>

        <div className="flex items-center gap-2">
          {/* Filter dropdowns */}
          {entry.spec.filters?.map((field) => (
            <div key={field} className="relative min-w-32">
              <Select
                value={filterValues[field] ?? ""}
                onChange={(val) =>
                  setFilterValues((prev) => ({
                    ...prev,
                    [field]: val,
                  }))
                }
                options={["", ...(filterOptions[field] ?? [])]}
                placeholder={field}
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
}) {
  const { setNodeRef, isOver } = useDroppable({
    id: column.status,
  })

  const isFull = maxCards != null && cards.length >= maxCards
  const isEmpty = cards.length === 0 && !loading

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
        <Badge value={String(cards.length)} />
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
          cards.slice(0, maxCards).map((card) => {
            const id = (card.id as string) ?? ""
            return (
              <DraggableCard
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
          })
        )}

        {maxCards && cards.length > maxCards && (
          <p className="text-center text-xs text-muted-foreground">
            +{cards.length - maxCards} more
          </p>
        )}
      </div>
    </div>
  )
}

// ── Draggable Card Wrapper ──

function DraggableCard({
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

  const { attributes, listeners, setNodeRef, transform } =
    useDraggable({ id })

  const style = transform
    ? {
        transform: `translate3d(${transform.x}px, ${transform.y}px, 0)`,
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
