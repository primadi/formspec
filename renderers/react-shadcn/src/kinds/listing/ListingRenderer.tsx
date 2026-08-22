// ─── Listing Renderer ───
//
// Public catalog (06-page-kinds.md §10) — the natural Page counterpart to an
// `access: public` App. Structurally a read-only table: search + filters from
// the manifest, but NO row_actions / bulk_actions / create (those imply
// authenticated writes). Clicking a row navigates to the entity's public
// detail route.
//
// Rendering reuses the same field-formatting conventions as the Table renderer
// (currency/date/relative/badge) so listings look consistent across surfaces.

import { useCallback, useEffect, useMemo, useState } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { Search } from "lucide-react"
import { resolveEntityRef } from "@/engine/entityRef"
import type {
  Entry,
  ListingSpec,
  FilterSpec,
  EntitySchema,
} from "@/types/manifest"
import { useMetaStore } from "@/stores/meta"
import { useSessionStore } from "@/stores/session"
import { apiList } from "@/lib/api"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Select } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"

interface RowData {
  id: string
  [key: string]: unknown
}

// ── Field formatting (mirrors Table renderer conventions) ──

function renderCellValue(value: unknown, widget?: string, format?: string) {
  if (value == null) return <span className="text-muted-foreground">-</span>
  if (widget === "boolean") return value ? "Yes" : "No"
  if (format === "currency" && typeof value === "number") {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: "USD",
    }).format(value)
  }
  if (format === "date" && typeof value === "string") {
    return new Date(value).toLocaleDateString()
  }
  if (typeof value === "object") return JSON.stringify(value)
  return String(value)
}

// ── Resolve filter options from a field's enum values ──

function filterOptions(filter: FilterSpec, entity?: EntitySchema): string[] {
  const field = entity?.fields.find((f) => f.name === filter.field)
  if (Array.isArray(field?.enum_values)) {
    return field.enum_values.map((e) => String(e))
  }
  return []
}

export default function ListingRenderer({
  entry,
}: {
  entry: Entry<ListingSpec>
}) {
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const navigate = useNavigate()
  const spec = entry.spec
  const getEntity = useMetaStore((s) => s.getEntity)
  const getClient = useSessionStore((s) => s.getClient)

  const entity = useMemo(() => {
    const ref = spec.entity
    const [mod, name] = resolveEntityRef(ref, entry.module)
    return getEntity(mod, name)
  }, [spec.entity, entry.module, getEntity])

  const [items, setItems] = useState<RowData[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [search, setSearch] = useState("")
  const [searchInput, setSearchInput] = useState("")
  const [filterVals, setFilterVals] = useState<Record<string, string>>({})

  // ── Fetch list (server-side search + filters) ──
  const load = useCallback(async () => {
    if (!entity) return
    setLoading(true)
    setError(null)
    try {
      const client = getClient()
      const params: Record<string, string> = { per_page: "100" }
      if (search) params.search = search
      for (const [field, val] of Object.entries(filterVals)) {
        if (val) params[field] = val
      }
      const { items } = await apiList<RowData>(
        client,
        `_ui/entity/${entity.module}/${entity.name}`,
        params,
      )
      setItems(items)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load listing")
    } finally {
      setLoading(false)
    }
  }, [entity, search, filterVals, getClient])

  useEffect(() => {
    load()
  }, [load])

  const columns = spec.columns ?? []
  const filters = spec.filters ?? []

  const submitSearch = () => setSearch(searchInput.trim())

  if (error) {
    return <div className="p-6 text-sm text-destructive">{error}</div>
  }

  return (
    <div className="mx-auto max-w-6xl px-4 py-8">
      {/* Title */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold tracking-tight">
          {entry.description || entry.name}
        </h1>
      </div>

      {/* Toolbar: search + filters */}
      {(spec.search || filters.length > 0) && (
        <div className="mb-4 flex flex-wrap items-center gap-3">
          {spec.search && (
            <div className="relative w-full max-w-xs">
              <Search className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                className="pl-8"
                placeholder="Cari..."
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && submitSearch()}
              />
            </div>
          )}
          {filters.map((filter) => (
            <FilterControl
              key={filter.field}
              filter={filter}
              entity={entity}
              value={filterVals[filter.field] ?? ""}
              onChange={(v) =>
                setFilterVals((prev) => ({ ...prev, [filter.field]: v }))
              }
            />
          ))}
          {(spec.search || filters.length > 0) && (
            <Button variant="outline" size="sm" onClick={load}>
              Terapkan
            </Button>
          )}
        </div>
      )}

      {/* Loading / Empty / Table */}
      {loading ? (
        <div className="space-y-3">
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
        </div>
      ) : items.length === 0 ? (
        <div className="py-16 text-center text-sm text-muted-foreground">
          Tidak ada data.
        </div>
      ) : (
        <div className="overflow-hidden rounded-lg border">
          <table className="w-full text-sm">
            <thead className="border-b bg-muted/50 text-left">
              <tr>
                {columns.map((col) => (
                  <th key={col.field} className="px-4 py-3 font-medium">
                    {col.label ?? col.field}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {items.map((row) => (
                <tr
                  key={row.id}
                  className="cursor-pointer transition-colors hover:bg-muted/40"
                  onClick={() =>
                    navigate(`/${workspace}${detailPath(entity, row)}`)
                  }
                >
                  {columns.map((col) => (
                    <td key={col.field} className="border-t px-4 py-3">
                      {renderCellValue(row[col.field], col.widget, col.format)}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

// ── Filter control ──

function FilterControl({
  filter,
  entity,
  value,
  onChange,
}: {
  filter: FilterSpec
  entity?: EntitySchema
  value: string
  onChange: (v: string) => void
}) {
  const options = filterOptions(filter, entity)
  const type = filter.type ?? "text"

  if (type === "select" || options.length > 0) {
    return (
      <Select
        className="w-40"
        value={value}
        onChange={onChange}
        placeholder={filter.label ?? filter.field}
        options={["", ...options]}
      />
    )
  }

  return (
    <Input
      className="w-40"
      placeholder={filter.label ?? filter.field}
      value={value}
      onChange={(e) => onChange(e.target.value)}
    />
  )
}

// ── Detail path ──
// The listing navigates to the entity's derived detail route:
// /{ws}/{surface}/{module}/{plural}/{id}. Reuses the entity plural.

function detailPath(entity: EntitySchema | undefined, row: RowData): string {
  if (!entity) return `/${row.id}`
  return `/${entity.module}/${entity.plural}/${row.id}`
}
