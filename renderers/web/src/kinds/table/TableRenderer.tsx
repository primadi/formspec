// ─── Table Renderer ───
//
// Full TanStack Table implementation with server-side pagination,
// sorting, filtering, and search. Row actions with confirm dialog.
//
// Design doc §5.5 Table kind (F3)

import React, { useState, useEffect, useMemo } from "react"
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  flexRender,
  type SortingState,
  type ColumnDef,
} from "@tanstack/react-table"
import { useNavigate, useSearchParams } from "react-router-dom"
import { useSurface } from "@/hooks/useSurface"
import {
  ChevronUp,
  ChevronDown,
  ChevronsUpDown,
  Search,
  Plus,
  ArrowLeft,
  ArrowRight,
  X,
  ListFilter,
  RotateCcw,
} from "lucide-react"
import { ActionIcon } from "@/components/ui/action-icon"
import { toast } from "sonner"

import type {
  EntitySchema,
  ListParams,
  TableAction,
  TableFilter,
  MetaBundle,
} from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { can as checkPermission } from "@/engine/permissions"
import { deriveTable } from "@/engine/derive"
import { getLifecycle } from "@/engine/lifecycle"
import { buildListParams, apiList, apiDelete } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select } from "@/components/ui/select"
import { Badge } from "@/widgets/Badge"
import { cn } from "@/lib/utils"
import { resolveIcon } from "@/lib/icon-resolver"
import ConfirmDialog from "@/components/ui/confirm-dialog"

interface TableRendererProps {
  entity: EntitySchema
}

interface RowData {
  id: string
  [key: string]: unknown
}

export default function TableRenderer({ entity }: TableRendererProps) {
  const navigate = useNavigate()
  const { surfacePath } = useSurface()
  const [, setSearchParams] = useSearchParams()
  const me = useSessionStore((s) => s.me)
  const getClient = useSessionStore((s) => s.getClient)
  const metaBundle = useMetaStore((s) => s.bundle)

  // Find an authored form for this entity + mode to check render mode.
  const authoredForm = useMemo(() => {
    if (!metaBundle) return undefined
    const entityRef = `${entity.module}.${entity.name}`
    return metaBundle.forms.find((f) => f.spec.entity === entityRef)
  }, [metaBundle, entity])

  // Resolve table spec: authored > derived, with fallback for missing fields
  const tableSpec = useMemo(() => {
    if (!metaBundle) return deriveTable(entity)
    // Look for an authored table whose spec.entity matches this entity
    const entityRef = `${entity.module}.${entity.name}`
    const authored = metaBundle.tables.find(
      (t) => t.spec.entity === entityRef || t.spec.entity === entity.name,
    )
    if (!authored) return deriveTable(entity)
    // Merge: use authored where present, fill gaps from deriveTable
    const derived = deriveTable(entity)
    return {
      ...derived,
      ...authored.spec,
      // columns, row_actions: authored if non-empty, else derived
      columns: authored.spec.columns?.length ? authored.spec.columns : derived.columns,
      row_actions: authored.spec.row_actions?.length ? authored.spec.row_actions : derived.row_actions,
    }
  }, [entity, metaBundle])

  const lifecycle = useMemo(() => getLifecycle(entity), [entity])

  // State
  const [data, setData] = useState<RowData[]>([])
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(0)
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState("")
  const [sorting, setSorting] = useState<SortingState>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [reloadKey, setReloadKey] = useState(0)

  // Filter state
  const [filterValues, setFilterValues] = useState<Record<string, string>>({})
  const [selectedRows, setSelectedRows] = useState<Set<string>>(new Set())

  // Pending confirm action
  const [pendingAction, setPendingAction] = useState<{
    action: TableAction
    row: RowData
  } | null>(null)

  // Reset page when filters change
  useEffect(() => {
    setPage(1)
  }, [filterValues])

  // Fetch data when params change
  useEffect(() => {
    let cancelled = false
    const load = async () => {
      setLoading(true)
      setError(null)
      try {
        const client = getClient()
        const params: ListParams = {
          page,
          per_page: tableSpec.page_size ?? 25,
          search: search || undefined,
        }
        if (sorting.length > 0) {
          params.sort = sorting.map((s) => `${s.desc ? "-" : ""}${s.id}`).join(",")
        }
        // Add filter values to params
        const activeFilters: Record<string, string> = {}
        for (const [field, value] of Object.entries(filterValues)) {
          if (value) {
            activeFilters[field] = value
          }
        }
        if (Object.keys(activeFilters).length > 0) {
          params.filters = activeFilters
        }
        const result = await apiList<RowData>(
          client,
          `${entity.module}/${entity.name}`,
          buildListParams(params),
        )
        if (!cancelled) {
          setData(result.items)
          setTotal(result.meta.total)
          setTotalPages(result.meta.total_pages)
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Failed to load data")
          setData([])
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    load()
    return () => { cancelled = true }
  }, [entity, page, search, sorting, filterValues, tableSpec.page_size, getClient, reloadKey])

  // Row selection state for bulk actions
  const [rowSelection, setRowSelection] = useState({})

  // Sync rowSelection to our Set state
  useEffect(() => {
    const newSet = new Set<string>()
    for (const [id, selected] of Object.entries(rowSelection)) {
      if (selected) newSet.add(id)
    }
    setSelectedRows(newSet)
  }, [rowSelection])

  // Columns
  const hasBulkActions =
    tableSpec.bulk_actions && tableSpec.bulk_actions.length > 0
  const columns = useMemo<ColumnDef<RowData>[]>(
    () => {
      const cols: ColumnDef<RowData>[] = []

      // Selection checkbox column for bulk actions
      if (hasBulkActions) {
        cols.push({
          id: "__select",
          header: ({ table }) => (
            <input
              type="checkbox"
              className="size-4 cursor-pointer"
              checked={table.getIsAllRowsSelected()}
              onChange={table.getToggleAllRowsSelectedHandler()}
            />
          ),
          cell: ({ row }) => (
            <input
              type="checkbox"
              className="size-4 cursor-pointer"
              checked={row.getIsSelected()}
              onChange={row.getToggleSelectedHandler()}
            />
          ),
          enableSorting: false,
          size: 40,
        })
      }

      for (const col of tableSpec.columns) {
        cols.push({
          id: col.field,
          accessorKey: col.field,
          header: col.label ?? col.field,
          enableSorting: col.sortable ?? true,
          cell: ({ getValue }) => {
            const value = getValue()
            return renderCellValue(value, col.widget, col.format)
          },
        })
      }

      return cols
    },
    [tableSpec.columns, hasBulkActions],
  )

  // TanStack table
  const table = useReactTable({
    data,
    columns,
    state: {
      sorting,
      rowSelection: hasBulkActions ? rowSelection : undefined,
    },
    onSortingChange: setSorting,
    onRowSelectionChange: hasBulkActions ? setRowSelection : undefined,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    manualSorting: true,
    manualPagination: true,
    pageCount: totalPages,
    enableRowSelection: hasBulkActions,
  })

  // Row action handler
  const handleRowAction = async (
    action: TableAction,
    row: RowData,
    skipConfirm = false,
  ) => {
    if (!me) return

    // Check permission
    const perm = `${entity.module}.${entity.plural}.${action.action}`
    if (!checkPermission(perm, me.permissions)) {
      toast.error("You don't have permission to perform this action")
      return
    }

    // Resolve confirm message: table action first, then entity action's ui.confirm
    const entityAction = entity.actions?.find((a) => a.name === action.action)
    const confirmMsg = action.confirm_msg ?? entityAction?.ui?.confirm

    // Confirm — intercept if confirm_msg exists and not skipped
    if (confirmMsg && !skipConfirm) {
      setPendingAction({ action, row })
      return
    }

    switch (action.action) {
      case "view":
        navigate(surfacePath(entity.module, entity.plural, row.id))
        break
      case "edit":
        // If there's an authored form with overlay mode, trigger overlay
        if (authoredForm?.spec.render?.mode && authoredForm.spec.render.mode !== "separate_page") {
          setSearchParams({
            action: "edit",
            form: authoredForm.name,
            id: row.id,
            mode: authoredForm.spec.render.mode,
          })
        } else {
          navigate(surfacePath(entity.module, entity.plural, row.id, "edit"))
        }
        break
      case "delete":
        try {
          const client = getClient()
          await apiDelete(client, `${entity.module}/${entity.name}/${row.id}`)
          toast.success("Deleted successfully")
          setReloadKey((k) => k + 1)
        } catch (err) {
          toast.error(err instanceof Error ? err.message : "Delete failed")
        }
        break
      default:
        if (action.action.startsWith("_")) break
        try {
          const client = getClient()
          await client.post(
            `${entity.module}/${entity.name}/${row.id}/${action.action}`,
          )
          toast.success("Action completed")
          setReloadKey((k) => k + 1)
        } catch (err) {
          toast.error(err instanceof Error ? err.message : "Action failed")
        }
        break
    }
  }

  // Handle confirm from ConfirmDialog
  const handleConfirm = () => {
    if (!pendingAction) return
    const { action, row } = pendingAction
    setPendingAction(null)
    handleRowAction(action, row, true)
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            {entity.name.charAt(0).toUpperCase() + entity.name.slice(1)}
          </h1>
          <p className="text-sm text-muted-foreground">
            {total} record{total !== 1 ? "s" : ""}
          </p>
        </div>

        {lifecycle.hasCreate && (
          <Button
            onClick={() => {
              // If there's an authored form with overlay mode, trigger overlay
              if (authoredForm?.spec.render?.mode && authoredForm.spec.render.mode !== "separate_page") {
                setSearchParams({
                  action: "create",
                  form: authoredForm.name,
                  mode: authoredForm.spec.render.mode,
                })
              } else {
                navigate(surfacePath(entity.module, entity.plural, "new"))
              }
            }}
          >
            <Plus className="size-4 mr-1" />
            New
          </Button>
        )}
      </div>

      {/* Search */}
      {tableSpec.search && (
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
          <Input
            placeholder="Search..."
            value={search}
            onChange={(e) => {
              setSearch(e.target.value)
              setPage(1)
            }}
            className="pl-9 max-w-sm"
          />
        </div>
      )}

      {/* Filters */}
      {tableSpec.filters && tableSpec.filters.length > 0 && (
        <FilterBar
          filters={tableSpec.filters}
          entity={entity}
          metaBundle={metaBundle}
          getClient={getClient}
          filterValues={filterValues}
          onFilterChange={(field, value) =>
            setFilterValues((prev) => ({ ...prev, [field]: value }))
          }
          onClear={() => setFilterValues({})}
        />
      )}

      {/* Bulk Actions */}
      {tableSpec.bulk_actions && tableSpec.bulk_actions.length > 0 && selectedRows.size > 0 && (
        <BulkActionsBar
          bulkActions={tableSpec.bulk_actions}
          selectedCount={selectedRows.size}
          onClear={() => setSelectedRows(new Set())}
        />
      )}

      {/* Error */}
      {error && (
        <div className="rounded-md border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {/* Table */}
      <div className="rounded-md border overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full caption-bottom text-sm">
            <thead className="border-b bg-muted/50">
              {table.getHeaderGroups().map((headerGroup) => (
                <tr key={headerGroup.id}>
                  {headerGroup.headers.map((header) => (
                    <th
                      key={header.id}
                      className={cn(
                        "h-10 px-3 text-left align-middle font-medium text-muted-foreground",
                        header.column.getCanSort() && "cursor-pointer select-none hover:bg-muted",
                      )}
                      onClick={header.column.getToggleSortingHandler()}
                    >
                      <div className="flex items-center gap-1">
                        {flexRender(
                          header.column.columnDef.header,
                          header.getContext(),
                        )}
                        {{
                          asc: <ChevronUp className="size-3" />,
                          desc: <ChevronDown className="size-3" />,
                        }[header.column.getIsSorted() as string] ??
                          (header.column.getCanSort() && (
                            <ChevronsUpDown className="size-3 opacity-50" />
                          ))}
                      </div>
                    </th>
                  ))}
                  {tableSpec.row_actions?.length ? (
                    <th className="h-10 px-3 text-right align-middle font-medium text-muted-foreground w-20">
                      Actions
                    </th>
                  ) : null}
                </tr>
              ))}
            </thead>
            <tbody className="[&_tr:last-child]:border-0">
              {loading ? (
                <tr>
                  <td
                    colSpan={columns.length + (tableSpec.row_actions?.length ? 1 : 0)}
                    className="h-24 text-center text-muted-foreground"
                  >
                    Loading...
                  </td>
                </tr>
              ) : data.length === 0 ? (
                <tr>
                  <td
                    colSpan={columns.length + (tableSpec.row_actions?.length ? 1 : 0)}
                    className="h-24 text-center text-muted-foreground"
                  >
                    No records found.
                  </td>
                </tr>
              ) : (
                table.getRowModel().rows.map((row) => (
                  <tr
                    key={row.id}
                    className="border-b transition-colors hover:bg-muted/50"
                  >
                    {row.getVisibleCells().map((cell) => (
                      <td key={cell.id} className="p-3 align-middle">
                        {flexRender(
                          cell.column.columnDef.cell,
                          cell.getContext(),
                        )}
                      </td>
                    ))}
                    {tableSpec.row_actions?.length ? (
                      <td className="p-3 align-middle text-right">
                        <div className="flex items-center justify-end gap-1">
                          {tableSpec.row_actions
                            .filter((a) => {
                              if (!me) return false
                              const perm = `${entity.module}.${entity.plural}.${a.action}`
                              return checkPermission(perm, me.permissions)
                            })
                            .filter((action) =>
                              isActionAllowedForRow(action, row.original, entity),
                            )
                            .map((action) => (
                              <Button
                                key={action.action}
                                variant="ghost"
                                size="icon"
                                className="size-8"
                                onClick={() => handleRowAction(action, row.original)}
                                title={action.label}
                              >
                                <ActionIcon iconName={action.icon ?? action.action} />
                              </Button>
                            ))}
                        </div>
                      </td>
                    ) : null}
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Pagination */}
      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <div>
          Page {page} of {totalPages || 1}
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={page <= 1}
            onClick={() => setPage((p) => Math.max(1, p - 1))}
          >
            <ArrowLeft className="size-3 mr-1" />
            Previous
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={page >= totalPages}
            onClick={() => setPage((p) => p + 1)}
          >
            Next
            <ArrowRight className="size-3 ml-1" />
          </Button>
        </div>
      </div>

      {/* Confirm Dialog */}
      {(() => {
        const actionName = pendingAction?.action.action
        const actionIconName = pendingAction?.action.icon
        const ActionIconComponent = actionIconName
          ? resolveIcon(actionIconName)
          : null
        const customIcon = ActionIconComponent
          ? React.createElement(ActionIconComponent, { className: "size-5" })
          : undefined

        // Resolve confirm message: table action first, then entity action
        const entityActionForConfirm = actionName
          ? entity.actions?.find((a) => a.name === actionName)
          : undefined
        const confirmMsg =
          pendingAction?.action.confirm_msg ??
          entityActionForConfirm?.ui?.confirm ??
          ""

        return (
          <ConfirmDialog
            open={!!pendingAction}
            onOpenChange={(open) => {
              if (!open) setPendingAction(null)
            }}
            title={pendingAction?.action.label ?? "Konfirmasi"}
            message={confirmMsg}
            icon={customIcon}
            variant={
              actionName === "delete" || actionName === "cancel"
                ? "destructive"
                : "default"
            }
            confirmLabel={actionName === "delete" ? "Hapus" : "Konfirmasi"}
            onConfirm={handleConfirm}
            onCancel={() => setPendingAction(null)}
          />
        )
      })()}
    </div>
  )
}

// ── Helpers ──

function renderCellValue(value: unknown, widget?: string, format?: string) {
  if (value == null) return <span className="text-muted-foreground">-</span>

  if (widget === "badge") {
    return <Badge value={String(value)} />
  }

  if (widget === "boolean") {
    return value ? "Yes" : "No"
  }

  if (format === "currency" && typeof value === "number") {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: "USD",
    }).format(value)
  }

  if (format === "date" && typeof value === "string") {
    return new Date(value).toLocaleDateString()
  }

  if (format === "relative" && typeof value === "string") {
    const d = new Date(value)
    const now = new Date()
    const diff = now.getTime() - d.getTime()
    const mins = Math.floor(diff / 60000)
    if (mins < 1) return "Just now"
    if (mins < 60) return `${mins}m ago`
    const hours = Math.floor(mins / 60)
    if (hours < 24) return `${hours}h ago`
    const days = Math.floor(hours / 24)
    if (days < 7) return `${days}d ago`
    return d.toLocaleDateString()
  }

  if (typeof value === "object") return JSON.stringify(value)

  return String(value)
}

/**
 * Check if a table action can be performed on a given row based on the
 * entity's state machine. Actions not declared in any transition (e.g.
 * "view", "edit", "delete") are always allowed. Actions whose current
 * state doesn't match any transition's `from` list are hidden.
 *
 * Guard expressions are evaluated server-side and cannot be checked here;
 * this only checks the `from`-state gate for basic UX filtering.
 */
function isActionAllowedForRow(
  action: TableAction,
  row: RowData,
  entity: EntitySchema,
): boolean {
  const sm = entity.state_machine
  if (!sm) return true

  // Find transitions keyed by this action name (via `via`)
  const matchingTransitions = sm.transitions.filter(
    (t) => t.via === action.action,
  )
  if (matchingTransitions.length === 0) {
    // Not a state-machine action — always show (view, edit, delete, export, …)
    return true
  }

  const currentState = String(row[sm.field] ?? sm.initial)
  return matchingTransitions.some((t) => t.from.includes(currentState))
}

// ── FilterBar ──

function FilterBar({
  filters,
  entity,
  metaBundle,
  getClient,
  filterValues,
  onFilterChange,
  onClear,
}: {
  filters: TableFilter[]
  entity: EntitySchema
  metaBundle: MetaBundle | null
  getClient: () => import("ky").KyInstance
  filterValues: Record<string, string>
  onFilterChange: (field: string, value: string) => void
  onClear: () => void
}) {
  const hasActiveFilters = Object.values(filterValues).some(Boolean)

  return (
    <div className="flex flex-wrap items-center gap-2">
      <ListFilter className="size-4 text-muted-foreground shrink-0" />
      {filters.map((f) => (
        <FilterControl
          key={f.field}
          filter={f}
          entity={entity}
          metaBundle={metaBundle}
          getClient={getClient}
          value={filterValues[f.field] ?? ""}
          onChange={(v) => onFilterChange(f.field, v)}
        />
      ))}
      {hasActiveFilters && (
        <Button variant="ghost" size="sm" onClick={onClear} className="h-8 px-2">
          <RotateCcw className="size-3 mr-1" />
          Reset
        </Button>
      )}
    </div>
  )
}

function FilterControl({
  filter,
  entity,
  metaBundle,
  getClient,
  value,
  onChange,
}: {
  filter: TableFilter
  entity: EntitySchema
  metaBundle: MetaBundle | null
  getClient: () => import("ky").KyInstance
  value: string
  onChange: (value: string) => void
}) {
  switch (filter.type) {
    case "select": {
      // Derive options from entity field enum_values or relation entity
      const fieldDef = entity.fields.find((f) => f.name === filter.field)
      const isRelation = fieldDef?.type === "relation" && fieldDef?.relation
      const [relationOptions, setRelationOptions] = useState<
        { value: string; label: string }[]
      >([])

      // Fetch relation options if this is a relation field
      useEffect(() => {
        if (!isRelation || !metaBundle || !fieldDef?.relation) return
        const resource = fieldDef.relation.resource
        // Find the related entity in metaBundle
        const relatedEntity = metaBundle.entities.find(
          (e) => e.name === resource,
        )
        if (!relatedEntity) return

        const client = getClient()
        const labelField = relatedEntity.label_field || "name"
        client
          .get(`${relatedEntity.module}/${resource}`, {
            searchParams: { per_page: "500" },
          })
          .json<{ data: Record<string, unknown>[] }>()
          .then((body) => {
            const items = body.data ?? []
            setRelationOptions(
              items.map((item) => ({
                value: String(item.id ?? ""),
                label: String(item[labelField] ?? item.id ?? ""),
              })),
            )
          })
          .catch(() => {
            // Silently fail — filter just shows "All" only
          })
      }, [isRelation, metaBundle, fieldDef, getClient])

      const options = isRelation
        ? relationOptions
        : (fieldDef?.enum_values ?? []).map((o) => ({ value: o, label: o }))

      return (
        <Select
          value={value}
          onChange={(v) => onChange(v === "__all__" ? "" : v)}
          options={[
            { value: "__all__", label: "All" },
            ...options,
          ]}
          placeholder={filter.label}
          className="h-8 w-35 text-xs"
        />
      )
    }
    case "date_range": {
      return (
        <Input
          type="date"
          placeholder={filter.label}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="h-8 w-40 text-xs"
        />
      )
    }
    case "text":
    default: {
      return (
        <Input
          placeholder={filter.label}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="h-8 w-40 text-xs"
        />
      )
    }
  }
}

// ── BulkActionsBar ──

function BulkActionsBar({
  bulkActions,
  selectedCount,
  onClear,
}: {
  bulkActions: TableAction[]
  selectedCount: number
  onClear: () => void
}) {
  return (
    <div className="flex items-center gap-2 px-3 py-2 rounded-md border bg-muted/30">
      <span className="text-sm text-muted-foreground mr-2">
        {selectedCount} selected
      </span>
      {bulkActions.map((action) => (
        <Button key={action.action} variant="secondary" size="sm">
          <ActionIcon iconName={action.icon ?? action.action} />
          <span className="ml-1">{action.label}</span>
        </Button>
      ))}
      <Button variant="ghost" size="sm" onClick={onClear} className="ml-auto">
        <X className="size-3 mr-1" />
        Clear
      </Button>
    </div>
  )
}
