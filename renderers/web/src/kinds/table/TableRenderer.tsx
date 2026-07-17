// ─── Table Renderer ───
//
// Full TanStack Table implementation with server-side pagination,
// sorting, filtering, and search. Row actions with confirm dialog.
//
// Design doc §5.5 Table kind (F3)

import { useState, useEffect, useMemo } from "react"
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  flexRender,
  type SortingState,
  type ColumnDef,
} from "@tanstack/react-table"
import { useNavigate, useParams } from "react-router-dom"
import {
  ChevronUp,
  ChevronDown,
  ChevronsUpDown,
  Search,
  Plus,
  Eye,
  Pencil,
  Trash2,
  ArrowLeft,
  ArrowRight,
} from "lucide-react"
import { toast } from "sonner"

import type { EntitySchema, ListParams, TableAction } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { can as checkPermission } from "@/engine/permissions"
import { deriveTable } from "@/engine/derive"
import { getLifecycle } from "@/engine/lifecycle"
import { buildListParams, apiList, apiDelete } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/widgets/Badge"
import { cn } from "@/lib/utils"

interface TableRendererProps {
  entity: EntitySchema
}

interface RowData {
  id: string
  [key: string]: unknown
}

export default function TableRenderer({ entity }: TableRendererProps) {
  const navigate = useNavigate()
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const me = useSessionStore((s) => s.me)
  const getClient = useSessionStore((s) => s.getClient)

  // Resolve table spec: authored > derived
  const tableSpec = useMemo(
    () => deriveTable(entity),
    [entity],
  )

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
        const result = await apiList<RowData>(
          client,
          `${entity.module}/${entity.plural}`,
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
  }, [entity, page, search, sorting, tableSpec.page_size, getClient, reloadKey])

  // Columns
  const columns = useMemo<ColumnDef<RowData>[]>(
    () =>
      tableSpec.columns.map((col) => ({
        id: col.field,
        accessorKey: col.field,
        header: col.label ?? col.field,
        enableSorting: col.sortable ?? true,
        cell: ({ getValue }) => {
          const value = getValue()
          return renderCellValue(value, col.widget, col.format)
        },
      })),
    [tableSpec.columns],
  )

  // TanStack table
  const table = useReactTable({
    data,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    manualSorting: true,
    manualPagination: true,
    pageCount: totalPages,
  })

  // Row action handler
  const handleRowAction = async (action: TableAction, row: RowData) => {
    if (!me) return

    // Check permission
    const perm = `${entity.module}.${entity.plural}.${action.action}`
    if (!checkPermission(perm, me.permissions)) {
      toast.error("You don't have permission to perform this action")
      return
    }

    // Confirm
    if (action.confirm_msg) {
      const confirmed = window.confirm(action.confirm_msg)
      if (!confirmed) return
    }

    switch (action.action) {
      case "view":
        navigate(`/${workspace}/_admin/${entity.module}/${entity.plural}/${row.id}`)
        break
      case "edit":
        navigate(
          `/${workspace}/_admin/${entity.module}/${entity.plural}/${row.id}/edit`,
        )
        break
      case "delete":
        try {
          const client = getClient()
          await apiDelete(client, `${entity.module}/${entity.plural}/${row.id}`)
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
            `${entity.module}/${entity.plural}/${row.id}/${action.action}`,
          )
          toast.success("Action completed")
          setReloadKey((k) => k + 1)
        } catch (err) {
          toast.error(err instanceof Error ? err.message : "Action failed")
        }
        break
    }
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
            onClick={() =>
              navigate(
                `/${workspace}/_admin/${entity.module}/${entity.plural}/new`,
              )
            }
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
                            .map((action) => (
                              <Button
                                key={action.action}
                                variant="ghost"
                                size="icon"
                                className="size-8"
                                onClick={() => handleRowAction(action, row.original)}
                                title={action.label}
                              >
                                <ActionIcon action={action.action} />
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

function ActionIcon({ action }: { action: string }) {
  switch (action) {
    case "view":
      return <Eye className="size-4" />
    case "edit":
      return <Pencil className="size-4" />
    case "delete":
      return <Trash2 className="size-4" />
    default:
      return <span className="text-xs">{action.charAt(0).toUpperCase()}</span>
  }
}
