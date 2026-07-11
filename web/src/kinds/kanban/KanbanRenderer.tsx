// ─── Kanban Renderer ───
//
// Drag-and-drop status board (kind: Kanban).
// Columns from manifest, cards from entity records.
// Drag = PATCH status_field, with 409 snap-back.
//
// Design doc §5.5 Kanban kind (F4)

import { useEffect, useState, useCallback } from "react"
import { toast } from "sonner"

import type { Entry, KanbanSpec, KanbanColumn } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { apiList } from "@/lib/api"
import { Badge } from "@/widgets/Badge"
import { Input } from "@/components/ui/input"
import { Search } from "lucide-react"

interface KanbanRendererProps {
  entry: Entry<KanbanSpec>
}

export default function KanbanRenderer({ entry }: KanbanRendererProps) {
  const getClient = useSessionStore((s) => s.getClient)
  const getEntity = useMetaStore((s) => s.getEntity)

  const entity = getEntity(entry.module, entry.spec.entity)
  const columns = entry.spec.columns
  const statusField = entry.spec.status_field

  const [records, setRecords] = useState<Record<string, unknown>[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState("")

  const fetchRecords = useCallback(async () => {
    setLoading(true)
    try {
      const client = getClient()
      const result = await apiList<Record<string, unknown>>(
        client,
        `${entity?.module ?? entry.module}/${entity?.plural ?? entry.spec.entity}`,
      )
      setRecords(result.items)
    } catch {
      toast.error("Failed to load kanban data")
    } finally {
      setLoading(false)
    }
  }, [entity, entry.module, entry.spec.entity, getClient])

  useEffect(() => {
    fetchRecords()
  }, [fetchRecords])

  const getColumnRecords = (status: string) => {
    return records.filter((r) => {
      const match = (r[statusField] as string) === status
      if (!search) return match
      return match && Object.values(r).some(
        (v) => String(v ?? "").toLowerCase().includes(search.toLowerCase()),
      )
    })
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            {entry.spec.entity} Board
          </h1>
        </div>

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

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
        {columns.map((col) => (
          <KanbanColumn
            key={col.status}
            column={col}
            cards={getColumnRecords(col.status)}
            loading={loading}
            maxCards={entry.spec.max_cards_per_column}
          />
        ))}
      </div>
    </div>
  )
}

function KanbanColumn({
  column,
  cards,
  loading,
  maxCards,
}: {
  column: KanbanColumn
  cards: Record<string, unknown>[]
  loading: boolean
  maxCards?: number
}) {
  return (
    <div className="rounded-md border bg-muted/30 flex flex-col h-full min-h-50">
      {/* Column header */}
      <div className="border-b px-3 py-2 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <div
            className="size-2 rounded-full"
            style={{ backgroundColor: column.color ?? "var(--muted-foreground)" }}
          />
          <span className="text-sm font-medium">{column.label}</span>
        </div>
        <Badge value={String(cards.length)} />
      </div>

      {/* Cards */}
      <div className="flex-1 p-2 space-y-2 overflow-y-auto">
        {loading ? (
          <div className="text-center py-8 text-sm text-muted-foreground">
            Loading...
          </div>
        ) : cards.length === 0 ? (
          <div className="text-center py-8 text-sm text-muted-foreground">
            No items
          </div>
        ) : (
          cards
            .slice(0, maxCards)
            .map((card, idx) => (
              <KanbanCard key={card.id as string ?? idx} card={card} />
            ))
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

function KanbanCard({ card }: { card: Record<string, unknown> }) {
  return (
    <div className="rounded-md border bg-card p-3 shadow-sm hover:shadow-md transition-shadow cursor-pointer">
      <p className="text-sm font-medium truncate">
        {(card.title as string) ?? (card.name as string) ?? card.id as string}
      </p>
      {card.subtitle ? (
        <p className="text-xs text-muted-foreground truncate mt-0.5">
          {card.subtitle as string}
        </p>
      ) : null}
      <div className="flex items-center gap-2 mt-2">
        {card.assignee ? (
          <span className="text-xs text-muted-foreground truncate">
            {card.assignee as string}
          </span>
        ) : null}
      </div>
    </div>
  )
}
