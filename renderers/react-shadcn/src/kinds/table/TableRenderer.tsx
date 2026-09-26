// ─── Table Renderer ───
//
// Full TanStack Table implementation with server-side pagination,
// sorting, filtering, and search. Row actions with confirm dialog.
//
// Design doc §5.5 Table kind (F3)

import React, { useState, useEffect, useMemo, useRef } from "react"
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  flexRender,
  type SortingState,
  type ColumnDef,
} from "@tanstack/react-table"
import { useAppNavigate } from "@/lib/navigation"
import { useSearchParams } from "react-router-dom"
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
  AlertTriangle,
  Check,
} from "lucide-react"
import { ActionIcon } from "@/components/ui/action-icon"
import { toast } from "@/lib/ui"

import type {
  EntitySchema,
  ListParams,
  TableAction,
  FilterSpec,
  FilterOpValue,
  MetaBundle,
  Field,
} from "@/types/manifest"
import { FormaApiError } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { canDoEntityAction } from "@/engine/permissions"
import {
  deriveTable,
  deriveForm,
  DERIVED_TABLE_VISIBLE_COLUMNS,
  withEntityColumnLabels,
} from "@/engine/derive"
import { resolveEntityRef } from "@/engine/entityRef"
import { getLifecycle } from "@/engine/lifecycle"
import { buildListParams, apiList, apiDelete, apiPatch } from "@/lib/api"
import {
  buildFixedFilterParams,
  resolveFilterValue,
  shouldShowAll,
  allLabel,
} from "@/lib/filters"
import { useSelectFilterOptions } from "@/hooks/useSelectFilterOptions"
import { useRealtime } from "@/hooks/useRealtime"
import { renderCellValue, resolveColumnCell } from "@/lib/renderCell"
import {
  fieldOptions,
  parseSingleOptionValue,
} from "@/lib/field-options"
import {
  columnAlignClass,
  columnJustifyClass,
  columnWidthStyle,
} from "@/lib/tableColumn"
import { createFormatter } from "@/lib/format"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { DateInput } from "@/widgets/DateInput"
import { Select } from "@/components/ui/select"
import { cn, interpolateConfirm, titleCase } from "@/lib/utils"
import { resolveIcon } from "@/lib/icon-resolver"
import ConfirmDialog from "@/components/ui/confirm-dialog"
import {
  getEntityRouteIdentifier,
  getEntityRouteSegment,
} from "@/lib/entityIdentity"

interface TableRendererProps {
  entity: EntitySchema
  /** When true, the entity-name <h1> title is suppressed — used when
   *  TableRenderer is embedded inside a Page that already provides its
   *  own title. Standalone (derived CRUD route) keeps the default title. */
  hideTitle?: boolean
  /** Extra filters merged into every list fetch, in addition to whatever the
   *  user picks from the table's own filter UI — used by a Page's `table`
   *  block `param` (e.g. `{ patient_id: ":id" }` scoping the table to the
   *  page's own record). Always applied; not user-clearable. */
  fixedFilters?: Record<string, string>
  /** Master-detail hook (06-page-kinds.md §1.1): fired with the clicked row's
   *  record so a Page can drive a detail block from the selection. When set,
   *  the selected row is highlighted. */
  onSelect?: (record: RowData) => void
}

interface RowData {
  id: string
  [key: string]: unknown
}

export default function TableRenderer({
  entity,
  hideTitle,
  fixedFilters,
  onSelect,
}: TableRendererProps) {
  const navigate = useAppNavigate()
  const { surfacePath } = useSurface()
  const [searchParams, setSearchParams] = useSearchParams()
  const me = useSessionStore((s) => s.me)
  const getClient = useSessionStore((s) => s.getClient)
  const metaBundle = useMetaStore((s) => s.bundle)

  // Resolved global settings → centralized formatter (spec §10).
  const formatter = useMemo(
    () => createFormatter(metaBundle?.settings),
    [metaBundle?.settings],
  )

  // Find an authored form for this entity + mode to check render mode.
  const authoredForm = useMemo(() => {
    if (!metaBundle) return undefined
    return metaBundle.forms.find((f) => {
      const [m, n] = resolveEntityRef(f.spec.entity ?? "", f.module ?? "")
      return m === entity.module && n === entity.name
    })
  }, [metaBundle, entity])

  // Render mode (§1.6 heuristic: ≤5 fields → modal, 6–12 → drawer, else
  // separate_page) — authored form wins if it sets one explicitly, otherwise
  // fall back to the same heuristic deriveForm() applies to fully-derived
  // entities (most entities in this showcase have no authored Form at all).
  const formRenderMode =
    authoredForm?.spec.render?.mode ??
    deriveForm(entity, "create").render?.mode ??
    "separate_page"

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
      // columns, row_actions: authored if non-empty, else derived.
      // Authored columns missing a `label` inherit the entity field's `title`
      // (`withEntityColumnLabels`) — authoring a Table to change two columns
      // must not demote every other caption to a raw field name.
      columns: authored.spec.columns?.length
        ? withEntityColumnLabels(authored.spec.columns, entity)
        : derived.columns,
      row_actions: authored.spec.row_actions?.length
        ? authored.spec.row_actions
        : derived.row_actions,
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
  // Set true by the realtime effect so the next list load stays silent
  // (no full-table "Loading..." flash on every pushed event).
  const silentRefetch = useRef(false)

  // Filter state — pre-seeded from each filter's `default` (e.g.
  // `{ field: transaction_date, type: date, default: today }` → the table
  // opens scoped to the server's current date, still user-adjustable).
  const [filterValues, setFilterValues] = useState<Record<string, string>>(() =>
    Object.fromEntries(
      (tableSpec.filters ?? [])
        .map((f) => [f.field, resolveFilterValue(f.default)])
        .filter(([, v]) => v !== ""),
    ),
  )
  const [selectedRows, setSelectedRows] = useState<Set<string>>(new Set())

  // Master-detail selection (06-page-kinds.md §1.1) — the row id currently
  // highlighted as the master selection driving a detail block.
  const [selectedId, setSelectedId] = useState<string | null>(null)

  // Row-expand (5.4.4 / 5.14.1): rows whose overflow columns (beyond the
  // first DERIVED_TABLE_VISIBLE_COLUMNS) are revealed inline. Overflow is
  // never silently dropped — it is always reachable via this expand toggle.
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set())

  // Inline editing (5.4.2) — the cell currently being edited + its draft
  // value. Commit = per-row update with CAS version; a 409 marks the row
  // stale (never silently overwritten).
  const [editingCell, setEditingCell] = useState<{
    rowId: string
    field: string
  } | null>(null)
  const [editValue, setEditValue] = useState<string>("")
  // Rows marked stale after a 409 CAS conflict — shown with a stale badge.
  const [staleRows, setStaleRows] = useState<Set<string>>(new Set())

  // Batch editing (5.4.3) — draft values for the batch_edit fields, applied
  // per row (loop PATCH) with partial failure reported per row.
  const [batchDraft, setBatchDraft] = useState<Record<string, string>>({})
  const [batchResults, setBatchResults] = useState<
    { id: string; ok: boolean; message?: string }[] | null
  >(null)

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
    const silent = silentRefetch.current
    silentRefetch.current = false // consume the realtime-silent flag
    const load = async () => {
      if (!silent) setLoading(true)
      setError(null)
      try {
        const client = getClient()
        const params: ListParams = {
          page,
          per_page: tableSpec.page_size ?? 25,
          search: search || undefined,
        }
        if (sorting.length > 0) {
          params.sort = sorting
            .map((s) => `${s.desc ? "-" : ""}${s.id}`)
            .join(",")
        }
        // Add filter values to params — user picks first, then the page's
        // runtime fixedFilters, then the manifest's fixed_filters (operator
        // syntax) so an immutable server-side scope can never be removed by
        // the UI (e.g. a table pinned to today()).
        const activeFilters: Record<string, string | FilterOpValue> = {
          ...filterValues,
          ...fixedFilters,
          ...buildFixedFilterParams(tableSpec.fixed_filters),
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
        if (!cancelled && !silent) setLoading(false)
      }
    }
    load()
    return () => {
      cancelled = true
    }
  }, [
    entity,
    page,
    search,
    sorting,
    filterValues,
    tableSpec.page_size,
    // Read inside the effect (buildFixedFilterParams). The manifest's
    // `fixed_filters` is an immutable server-side scope, so a change must
    // refetch — leaving it out meant a reload with a different scope silently
    // kept the old rows.
    tableSpec.fixed_filters,
    getClient,
    reloadKey,
    fixedFilters,
  ])

  // ── Realtime (spec §5): matching entity event → silent refetch ──
  const realtimeTick = useRealtime(
    tableSpec.realtime && entity ? `${entity.module}/${entity.name}` : "",
  )
  useEffect(() => {
    if (realtimeTick === 0) return
    silentRefetch.current = true
    setReloadKey((k) => k + 1)
  }, [realtimeTick])

  // ── Overlay close → refresh ──
  // The create/edit modal or drawer is URL-driven (?action=create&...). The
  // table itself never unmounts while the overlay is open, so when the
  // overlay closes (URL params removed after save/cancel) the list would
  // otherwise keep showing stale data. Detect the open→closed transition
  // and silently refetch — this also covers derived tables that have no
  // realtime subscription.
  const overlayWasOpen = useRef(false)
  useEffect(() => {
    const isOpen = !!searchParams.get("action")
    if (overlayWasOpen.current && !isOpen) {
      silentRefetch.current = true
      setReloadKey((k) => k + 1)
    }
    overlayWasOpen.current = isOpen
  }, [searchParams])

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

  // 5.4.4 / 5.14.1 — the derived table lists every eligible field in priority
  // order; the renderer shows the first N by default and exposes the rest via
  // row expand. Authored tables are shown in full (author is in control).
  const hasOverflow = tableSpec.columns.length > DERIVED_TABLE_VISIBLE_COLUMNS
  const visibleColumns = hasOverflow
    ? tableSpec.columns.slice(0, DERIVED_TABLE_VISIBLE_COLUMNS)
    : tableSpec.columns
  const overflowColumns = hasOverflow
    ? tableSpec.columns.slice(DERIVED_TABLE_VISIBLE_COLUMNS)
    : []

  // Author-declared column presentation. `align`/`width` are part of the
  // contract (06-page-kinds.md §3) but were read by no renderer, so
  // `align: right` on a numeric column was silently discarded. Keyed by column
  // id, which is the column's `field` (dot-paths included, e.g.
  // "patient.name" — restore relations use those).
  const columnByField = useMemo(
    () => new Map(tableSpec.columns.map((c) => [c.field, c])),
    [tableSpec.columns],
  )

  /**
   * Render one cell: relation columns show the related record's label, other
   * columns go through the shared cell renderer. Without this an authored
   * table naming a foreign key (`field: branch_id`) printed the raw UUID even
   * though the API had already put the resolved object on the same row.
   */
  const renderCell = (row: RowData, columnField: string) => {
    const col = columnByField.get(columnField)
    const { value, scale } = resolveColumnCell(
      row,
      columnField,
      entity,
      (m, n) =>
        metaBundle?.entities.find(
          (e) => e.module === m && e.name === n,
        ) as never,
    )
    return renderCellValue(value, col?.widget, col?.format, formatter, {
      scale,
    })
  }

  const toggleExpand = (id: string) => {
    setExpandedRows((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  // ── Inline editing (5.4.2) ──
  // Commit a single cell = per-row update with CAS version. A 409 marks the
  // row stale (never silently overwritten); the user can reload to get the
  // fresh record. Submitted rows reject inline-edit (server also enforces).
  const commitInlineEdit = async (
    row: RowData,
    field: string,
    value: string,
  ) => {
    const version = typeof row.version === "number" ? row.version : undefined
    try {
      await apiPatch(
        getClient(),
        `${entity.module}/${entity.name}/${row.id}`,
        { [field]: value },
        version,
      )
      toast.success("Saved")
      setEditingCell(null)
      setReloadKey((k) => k + 1)
    } catch (err) {
      if (err instanceof FormaApiError && err.status === 409) {
        setStaleRows((prev) => new Set(prev).add(row.id))
        toast.error(
          "Data telah diubah oleh pengguna lain — reload untuk versi terbaru",
        )
      } else {
        toast.error(err instanceof Error ? err.message : "Save failed")
      }
      setEditingCell(null)
    }
  }

  // ── Batch editing (5.4.3) ──
  // Apply the batch_edit draft values to every selected row via a per-row
  // PATCH loop. Partial failure is reported per row — never all-or-nothing.
  const applyBatchEdit = async () => {
    const fields = Object.keys(batchDraft).filter(
      (f) => batchDraft[f] !== "" && batchDraft[f] != null,
    )
    if (fields.length === 0) {
      toast.error("Set at least one field value")
      return
    }
    const rows = data.filter((r) => selectedRows.has(r.id as string))
    if (rows.length === 0) return

    const results: { id: string; ok: boolean; message?: string }[] = []
    for (const row of rows) {
      const version = typeof row.version === "number" ? row.version : undefined
      const patch: Record<string, unknown> = {}
      for (const f of fields) {
        // A draft value chosen from a declared option set is stored in its
        // declared scalar form (`1`, not `"1"`) — the same rule the form widgets
        // follow, so a batch PATCH cannot silently turn a numeric set into
        // strings. A plain text draft passes through unchanged.
        const decl = entity.fields.find((ef) => ef.name === f)
        patch[f] = decl
          ? (parseSingleOptionValue(batchDraft[f], fieldOptions(decl)) ??
            batchDraft[f])
          : batchDraft[f]
      }
      try {
        await apiPatch(
          getClient(),
          `${entity.module}/${entity.name}/${row.id}`,
          patch,
          version,
        )
        results.push({ id: row.id as string, ok: true })
      } catch (err) {
        if (err instanceof FormaApiError && err.status === 409) {
          setStaleRows((prev) => new Set(prev).add(row.id as string))
        }
        results.push({
          id: row.id as string,
          ok: false,
          message: err instanceof Error ? err.message : "Failed",
        })
      }
    }
    setBatchResults(results)
    const okCount = results.filter((r) => r.ok).length
    const failCount = results.length - okCount
    if (failCount === 0) {
      toast.success(`Updated ${okCount} row${okCount !== 1 ? "s" : ""}`)
    } else {
      toast.error(
        `${okCount} updated, ${failCount} failed — see per-row report`,
      )
    }
    setBatchDraft({})
    setSelectedRows(new Set())
    setReloadKey((k) => k + 1)
  }

  const columns = useMemo<ColumnDef<RowData>[]>(() => {
    const cols: ColumnDef<RowData>[] = []

    // Row-expand toggle column (5.4.4 / 5.14.1) — reveals overflow columns.
    if (hasOverflow) {
      cols.push({
        id: "__expand",
        header: () => null,
        enableSorting: false,
        size: 36,
        cell: ({ row }) => (
          <button
            type="button"
            className="text-muted-foreground hover:text-foreground p-0.5 rounded"
            onClick={(e) => {
              e.stopPropagation()
              toggleExpand(row.original.id as string)
            }}
            title={
              expandedRows.has(row.original.id as string)
                ? "Collapse"
                : "Show more fields"
            }
          >
            {expandedRows.has(row.original.id as string) ? (
              <ChevronUp className="size-4" />
            ) : (
              <ChevronDown className="size-4" />
            )}
          </button>
        ),
      })
    }

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

    for (const col of visibleColumns) {
      cols.push({
        id: col.field,
        accessorKey: col.field,
        header: col.label ?? col.field,
        enableSorting: col.sortable ?? true,
        cell: ({ getValue, row }) => {
          const value = getValue()
          // Inline editing (5.4.2): editable cells render an in-place input.
          const editable =
            !!tableSpec.inline_edit &&
            isFieldInlineEditable(entity, col.field, row.original, me)
          const isEditing =
            editingCell?.rowId === row.original.id &&
            editingCell?.field === col.field
          if (editable && isEditing) {
            return (
              <input
                autoFocus
                value={editValue}
                onChange={(e) => setEditValue(e.target.value)}
                onBlur={() =>
                  commitInlineEdit(row.original, col.field, editValue)
                }
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    commitInlineEdit(row.original, col.field, editValue)
                  } else if (e.key === "Escape") {
                    setEditingCell(null)
                  }
                }}
                className="w-full rounded border border-primary px-2 py-1 text-sm"
              />
            )
          }
          if (editable) {
            return (
              <button
                type="button"
                className="w-full text-left rounded px-1 py-0.5 hover:bg-muted/60"
                onClick={(e) => {
                  e.stopPropagation()
                  setEditingCell({
                    rowId: row.original.id as string,
                    field: col.field,
                  })
                  setEditValue(value == null ? "" : String(value))
                }}
                title="Click to edit"
              >
                {renderCell(row.original, col.field)}
              </button>
            )
          }
          return renderCell(row.original, col.field)
        },
      })
    }

    return cols
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    tableSpec.columns,
    visibleColumns,
    hasBulkActions,
    formatter,
    expandedRows,
    tableSpec.inline_edit,
    editingCell,
    editValue,
    me,
    entity,
  ])

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
    if (!canDoEntityAction(me, entity, action.action)) {
      toast.error("You don't have permission to perform this action")
      return
    }

    // Resolve confirm message: table action first, then entity action's ui.confirm,
    // then the App-level delete default (plan confirm-dialogs.md).
    const entityAction = entity.actions?.find((a) => a.name === action.action)
    const confirmMsg =
      action.confirm_msg ??
      entityAction?.ui?.confirm ??
      (action.action === "delete"
        ? interpolateConfirm(metaBundle?.app.confirm?.delete ?? "", entity.name)
        : "")

    // Confirm — intercept if confirm_msg exists and not skipped
    if (confirmMsg && !skipConfirm) {
      setPendingAction({ action, row })
      return
    }

    switch (action.action) {
      case "view":
        navigate(
          surfacePath(
            entity.module,
            entity.plural,
            getEntityRouteSegment(entity, row),
          ),
        )
        break
      case "edit":
        const routeIdentifier = getEntityRouteIdentifier(entity, row)
        // Modal/drawer render mode → overlay (authored form name if there is
        // one, otherwise the entity itself — OverlayHost derives the form).
        if (formRenderMode !== "separate_page") {
          setSearchParams({
            action: "edit",
            ...(authoredForm
              ? { form: authoredForm.name }
              : { entity: `${entity.module}.${entity.name}` }),
            id: routeIdentifier,
            mode: formRenderMode,
          })
        } else {
          navigate(
            surfacePath(
              entity.module,
              entity.plural,
              getEntityRouteSegment(entity, row),
              "edit",
            ),
          )
        }
        break
      case "delete":
        try {
          const client = getClient()
          await apiDelete(
            client,
            `${entity.module}/${entity.name}/${getEntityRouteSegment(entity, row)}`,
          )
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
            `${entity.module}/${entity.name}/${getEntityRouteSegment(entity, row)}/${action.action}`,
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
          {!hideTitle && (
            <h1 className="text-2xl font-bold tracking-tight">
              {titleCase(entity.name)}
            </h1>
          )}
          <p className="text-sm text-muted-foreground">
            {total} record{total !== 1 ? "s" : ""}
          </p>
        </div>

        {/* A button the caller cannot use is not rendered at all: `hasCreate`
            reflects the entity's lifecycle, NOT the caller's authorization, so
            without this check a cashier lacking `create` was offered "New" and
            then got a form with no Save button (kafe 10.19). */}
        {lifecycle.hasCreate && canDoEntityAction(me, entity, "create") && (
          <Button
            onClick={() => {
              // Modal/drawer render mode → overlay (authored form name if
              // there is one, otherwise the entity itself — OverlayHost
              // derives the form).
              if (formRenderMode !== "separate_page") {
                setSearchParams({
                  action: "create",
                  ...(authoredForm
                    ? { form: authoredForm.name }
                    : { entity: `${entity.module}.${entity.name}` }),
                  mode: formRenderMode,
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
      {tableSpec.bulk_actions &&
        tableSpec.bulk_actions.length > 0 &&
        selectedRows.size > 0 && (
          <BulkActionsBar
            bulkActions={tableSpec.bulk_actions}
            selectedCount={selectedRows.size}
            onClear={() => setSelectedRows(new Set())}
          />
        )}

      {/* Batch editing (5.4.3) — set values for batch_edit fields across the
          selected rows; applied per row with partial failure reported. */}
      {tableSpec.batch_edit && tableSpec.batch_edit.length > 0 && (
        <BatchEditBar
          fields={tableSpec.batch_edit}
          entity={entity}
          selectedCount={selectedRows.size}
          draft={batchDraft}
          results={batchResults}
          onDraftChange={(field, value) =>
            setBatchDraft((prev) => ({ ...prev, [field]: value }))
          }
          onApply={applyBatchEdit}
          onClear={() => {
            setBatchDraft({})
            setBatchResults(null)
            setSelectedRows(new Set())
          }}
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
                  {headerGroup.headers.map((header) => {
                    // The expand/select columns have no author config.
                    const col = columnByField.get(header.column.id)
                    return (
                      <th
                        key={header.id}
                        style={columnWidthStyle(col?.width)}
                        className={cn(
                          "h-10 px-3 text-left align-middle font-medium text-muted-foreground",
                          columnAlignClass(col?.align),
                          header.column.getCanSort() &&
                            "cursor-pointer select-none hover:bg-muted",
                        )}
                        onClick={header.column.getToggleSortingHandler()}
                      >
                        <div
                          className={cn(
                            "flex items-center gap-1",
                            columnJustifyClass(col?.align),
                          )}
                        >
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
                    )
                  })}
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
                    colSpan={
                      columns.length + (tableSpec.row_actions?.length ? 1 : 0)
                    }
                    className="h-24 text-center text-muted-foreground"
                  >
                    Loading...
                  </td>
                </tr>
              ) : data.length === 0 ? (
                <tr>
                  <td
                    colSpan={
                      columns.length + (tableSpec.row_actions?.length ? 1 : 0)
                    }
                    className="h-24 text-center text-muted-foreground"
                  >
                    No records found.
                  </td>
                </tr>
              ) : (
                table.getRowModel().rows.map((row) => (
                  <React.Fragment key={row.id}>
                    <tr
                      onClick={
                        onSelect
                          ? () => {
                              setSelectedId(row.id)
                              onSelect(row.original)
                            }
                          : undefined
                      }
                      className={cn(
                        "border-b transition-colors hover:bg-muted/50",
                        onSelect && "cursor-pointer",
                        onSelect && selectedId === row.id && "bg-primary/5",
                        staleRows.has(row.original.id as string) &&
                          "bg-destructive/5",
                      )}
                    >
                      {staleRows.has(row.original.id as string) && (
                        <td className="p-3 align-middle">
                          <span className="inline-flex items-center gap-1 rounded bg-destructive/10 px-2 py-0.5 text-xs text-destructive">
                            <AlertTriangle className="size-3" />
                            Stale — reload
                          </span>
                        </td>
                      )}
                      {row.getVisibleCells().map((cell) => (
                        <td
                          key={cell.id}
                          className={cn(
                            "p-3 align-middle",
                            columnAlignClass(
                              columnByField.get(cell.column.id)?.align,
                            ),
                          )}
                        >
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
                              .filter((a) =>
                                canDoEntityAction(me, entity, a.action),
                              )
                              .filter((action) =>
                                isActionAllowedForRow(
                                  action,
                                  row.original,
                                  entity,
                                ),
                              )
                              .map((action) => (
                                <Button
                                  key={action.action}
                                  variant="ghost"
                                  size="icon"
                                  className="size-8"
                                  onClick={() =>
                                    handleRowAction(action, row.original)
                                  }
                                  title={action.label}
                                >
                                  <ActionIcon
                                    iconName={action.icon ?? action.action}
                                  />
                                </Button>
                              ))}
                          </div>
                        </td>
                      ) : null}
                    </tr>
                    {/* Row-expand: overflow columns (5.4.4 / 5.14.1) — never
                        silently dropped, always reachable via expand. */}
                    {expandedRows.has(row.original.id as string) &&
                      overflowColumns.length > 0 && (
                        <tr className="border-b bg-muted/20">
                          <td
                            colSpan={
                              columns.length +
                              (tableSpec.row_actions?.length ? 1 : 0)
                            }
                            className="px-6 py-3"
                          >
                            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-x-6 gap-y-2">
                              {overflowColumns.map((col) => (
                                <div
                                  key={col.field}
                                  className="flex flex-col gap-0.5"
                                >
                                  <span className="text-[11px] uppercase tracking-wide text-muted-foreground">
                                    {col.label ?? col.field}
                                  </span>
                                  <span className="text-sm">
                                    {renderCell(row.original, col.field)}
                                  </span>
                                </div>
                              ))}
                            </div>
                          </td>
                        </tr>
                      )}
                  </React.Fragment>
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

        // Resolve confirm message: table action first, then entity action,
        // then the App-level delete default (plan confirm-dialogs.md).
        const entityActionForConfirm = actionName
          ? entity.actions?.find((a) => a.name === actionName)
          : undefined
        const confirmMsg =
          pendingAction?.action.confirm_msg ??
          entityActionForConfirm?.ui?.confirm ??
          (pendingAction?.action.action === "delete"
            ? interpolateConfirm(
                metaBundle?.app.confirm?.delete ?? "",
                entity.name,
              )
            : "")

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

/**
 * Inline-edit eligibility (5.4.2): a cell is editable only when the field's
 * rules allow it — not readonly/computed/immutable, not a relation dot-path,
 * within the caller's `update` permission, and the row is not submitted
 * (lifecycle guard; the server enforces this authoritatively).
 */
function isFieldInlineEditable(
  entity: EntitySchema,
  fieldName: string,
  row: RowData,
  me: { permissions: string[] } | null,
): boolean {
  if (!me) return false
  // Relation dot-paths (e.g. patient.name) are read-only display — never
  // inline-editable.
  if (fieldName.includes(".")) return false
  const field: Field | undefined = entity.fields.find(
    (f) => f.name === fieldName,
  )
  if (!field) return false
  if (field.computed || field.read_only || field.immutable) return false
  if (field.type === "child") return false

  // update permission on the entity
  if (!canDoEntityAction(me, entity, "update")) return false

  // Submitted rows reject inline-edit (doc_status lifecycle).
  if (
    entity.fields.some((f) => f.name === "doc_status") &&
    row.doc_status === "submitted"
  ) {
    return false
  }
  return true
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
  filters: FilterSpec[]
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
        <Button
          variant="ghost"
          size="sm"
          onClick={onClear}
          className="h-8 px-2"
        >
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
  filter: FilterSpec
  entity: EntitySchema
  metaBundle: MetaBundle | null
  getClient: () => import("ky").KyInstance
  value: string
  onChange: (value: string) => void
}) {
  // Called unconditionally, ABOVE the switch. A hook inside a `case` makes the
  // hook count depend on `filter.type`, which breaks the Rules of Hooks the
  // moment a caller re-renders this control with a different type (oxlint:
  // `react-hooks/rules-of-hooks`). Only the select branch consumes the result;
  // for other types the hook's own `filter.type`-independent guards apply.
  const selectOptions = useSelectFilterOptions(
    filter,
    entity,
    metaBundle,
    getClient,
  )

  switch (filter.type) {
    case "select": {
      // Options come from the entity field definition (enum_values / related
      // entity master data), independent of the current rows — so a table or
      // board scoped to an empty date range still shows valid filter options.
      const showAll = shouldShowAll(filter)
      const optionsWithAll = [
        ...(showAll ? [{ value: "__all__", label: allLabel(filter) }] : []),
        ...selectOptions,
      ]
      return (
        <Select
          value={value}
          onChange={(v) => onChange(v === "__all__" ? "" : v)}
          options={optionsWithAll}
          placeholder={filter.label}
          className="h-8 w-35 text-xs"
        />
      )
    }
    case "date":
    case "date_range": {
      return (
        <DateInput
          value={value}
          onChange={onChange}
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

// ── BatchEditBar (5.4.3) ──
// Set values for the batch_edit fields across the selected rows. Applied per
// row (loop PATCH) with partial failure reported per row — never
// all-or-nothing. Only fields the caller may update are offered.

function BatchEditBar({
  fields,
  entity,
  selectedCount,
  draft,
  results,
  onDraftChange,
  onApply,
  onClear,
}: {
  fields: string[]
  entity: EntitySchema
  selectedCount: number
  draft: Record<string, string>
  results: { id: string; ok: boolean; message?: string }[] | null
  onDraftChange: (field: string, value: string) => void
  onApply: () => void
  onClear: () => void
}) {
  const me = useSessionStore((s) => s.me)
  const editableFields = fields.filter((f) => {
    if (!canDoEntityAction(me, entity, "update")) return false
    const field = entity.fields.find((ef) => ef.name === f)
    if (!field) return false
    return !field.computed && !field.read_only && !field.immutable
  })

  const hasDraft = Object.values(draft).some((v) => v !== "" && v != null)

  return (
    <div className="rounded-md border bg-muted/30 p-3 space-y-2">
      <div className="flex items-center gap-2">
        <span className="text-sm font-medium">
          Batch edit — {selectedCount} selected
        </span>
        <Button
          variant="ghost"
          size="sm"
          onClick={onClear}
          className="ml-auto h-7 px-2"
        >
          <X className="size-3 mr-1" />
          Clear
        </Button>
      </div>
      {editableFields.length === 0 ? (
        <p className="text-xs text-muted-foreground">
          No editable fields for your permissions.
        </p>
      ) : (
        <div className="flex flex-wrap items-end gap-2">
          {editableFields.map((f) => {
            const field = entity.fields.find((ef) => ef.name === f)
            // A field with a declared choice set gets a picker that offers the
            // declared values — with their captions when `options` provides
            // them (`1` → "Senin"), not just the raw enum values.
            const choices = field ? fieldOptions(field) : []
            return (
              <div key={f} className="flex flex-col gap-1">
                <label className="text-[11px] uppercase tracking-wide text-muted-foreground">
                  {field?.title ?? f}
                </label>
                {choices.length > 0 ? (
                  <Select
                    value={draft[f] ?? ""}
                    // The draft holds the option's canonical key; it is resolved
                    // back to the declared scalar (`1`, not `"1"`) when the PATCH
                    // body is built, so one place owns that mapping.
                    onChange={(v) => onDraftChange(f, v === null ? "" : String(v))}
                    options={[
                      { value: "", label: "(unchanged)" },
                      ...choices.map((c) => ({
                        value: c.key,
                        label: c.label,
                      })),
                    ]}
                    className="h-8 w-44 text-xs"
                  />
                ) : (
                  <Input
                    value={draft[f] ?? ""}
                    onChange={(e) => onDraftChange(f, e.target.value)}
                    placeholder="(unchanged)"
                    className="h-8 w-44 text-xs"
                  />
                )}
              </div>
            )
          })}
          <Button
            variant="secondary"
            size="sm"
            disabled={!hasDraft || selectedCount === 0}
            onClick={onApply}
          >
            <Check className="size-3 mr-1" />
            Apply to {selectedCount} row{selectedCount !== 1 ? "s" : ""}
          </Button>
        </div>
      )}
      {results && results.length > 0 && (
        <div className="max-h-40 overflow-y-auto rounded border bg-background p-2 text-xs">
          {results.map((r) => (
            <div
              key={r.id}
              className={cn(
                "flex items-center gap-2 py-0.5",
                r.ok ? "text-emerald-600" : "text-destructive",
              )}
            >
              {r.ok ? (
                <Check className="size-3 shrink-0" />
              ) : (
                <AlertTriangle className="size-3 shrink-0" />
              )}
              <span className="truncate">{r.id}</span>
              {!r.ok && r.message && (
                <span className="truncate text-muted-foreground">
                  — {r.message}
                </span>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
