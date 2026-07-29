// ─── Timeline Renderer ───
//
// Chronological event journal (kind: Timeline).
// Infinite scroll with cursor-based pagination.
//
// Design doc §5.5 Timeline kind (F4)

import { useEffect, useState, useCallback, useRef } from "react"
import { toast } from "sonner"
import { Clock, Loader2 } from "lucide-react"

import type { Entry, TimelineSpec } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { resolveEntityRef } from "@/engine/entityRef"
import { apiList } from "@/lib/api"
import { Badge } from "@/widgets/Badge"

interface TimelineRendererProps {
  entry: Entry<TimelineSpec>
}

export default function TimelineRenderer({ entry }: TimelineRendererProps) {
  const getClient = useSessionStore((s) => s.getClient)
  const getEntity = useMetaStore((s) => s.getEntity)

  const [entityModule, entityName] = resolveEntityRef(entry.spec.entity, entry.module)
  const entity = getEntity(entityModule, entityName)
  const dateField = entry.spec.date_field ?? "created_at"
  const groupBy = entry.spec.group_by ?? "date"
  const sort = entry.spec.sort ?? "desc"

  const [items, setItems] = useState<Record<string, unknown>[]>([])
  const [loading, setLoading] = useState(true)
  const [hasMore, setHasMore] = useState(true)
  const cursorRef = useRef<string | undefined>(undefined)
  const observerRef = useRef<IntersectionObserver>(undefined)

  const fetchItems = useCallback(async () => {
    if (!hasMore) return
    if (!entity) {
      toast.error(`entity "${entry.spec.entity}" not found`)
      setLoading(false)
      setHasMore(false)
      return
    }
    setLoading(true)
    try {
      const client = getClient()
      const result = await apiList<Record<string, unknown>>(
        client,
        `${entity.module}/${entity.name}`,
        {
          sort: `${sort === "desc" ? "-" : ""}${dateField}`,
          ...(cursorRef.current ? { page: cursorRef.current } : {}),
        },
      )
      setItems((prev) => [...prev, ...result.items])
      setHasMore(result.meta.page < result.meta.total_pages)
      cursorRef.current = String(result.meta.page + 1)
    } catch {
      toast.error("Failed to load timeline")
    } finally {
      setLoading(false)
    }
  }, [entry.spec.entity, entity, dateField, sort, hasMore, getClient])

  useEffect(() => {
    fetchItems()
  }, [])

  // Infinite scroll observer
  const lastItemRef = useCallback(
    (node: HTMLDivElement | null) => {
      if (loading) return
      if (observerRef.current) observerRef.current.disconnect()
      observerRef.current = new IntersectionObserver((entries) => {
        if (entries[0].isIntersecting && hasMore) {
          fetchItems()
        }
      })
      if (node) observerRef.current.observe(node)
    },
    [loading, hasMore, fetchItems],
  )

  // Group items by date
  const grouped = groupItems(items, dateField, groupBy)

  return (
    <div className="max-w-2xl mx-auto space-y-4">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">
          {entry.spec.entity} Timeline
        </h1>
        {entry.spec.empty_state && items.length === 0 && !loading && (
          <p className="text-sm text-muted-foreground">{entry.spec.empty_state}</p>
        )}
      </div>

      <div className="relative space-y-6">
        {grouped.map(([date, groupItems], groupIdx) => (
          <div key={date}>
            {/* Date header */}
            <div className="flex items-center gap-2 mb-3 sticky top-0 bg-background py-1 z-10">
              <Clock className="size-4 text-muted-foreground" />
              <h3 className="text-sm font-medium">{date}</h3>
              <Badge value={String(groupItems.length)} />
            </div>

            {/* Timeline items */}
            <div className="space-y-3 ml-6 border-l pl-4">
              {groupItems.map((item, idx) => {
                const isLastItem = groupIdx === grouped.length - 1 && idx === groupItems.length - 1
                return (
                  <div
                    key={item.id as string ?? idx}
                    ref={isLastItem ? lastItemRef : undefined}
                    className="relative"
                  >
                    {/* Timeline dot */}
                    <div className="absolute -left-4.75 top-1.5 size-2.5 rounded-full bg-primary ring-2 ring-background" />

                    {/* Card */}
                    <div className="rounded-md border p-3">
                      <p className="text-sm font-medium">
                        {(item[entry.spec.display?.title_field ?? "name"] as string) ??
                          item.id as string}
                      </p>
                      {entry.spec.display?.subtitle_field && (
                        <p className="text-xs text-muted-foreground mt-0.5">
                          {item[entry.spec.display.subtitle_field] as string}
                        </p>
                      )}
                      {entry.spec.display?.content_field && (
                        <p className="text-sm mt-2">
                          {item[entry.spec.display.content_field] as string}
                        </p>
                      )}
                      <p className="text-xs text-muted-foreground mt-1">
                        {new Date(
                          (item[dateField] as string) ?? "",
                        ).toLocaleString()}
                      </p>
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        ))}

        {loading && (
          <div className="flex justify-center py-4">
            <Loader2 className="size-5 animate-spin text-muted-foreground" />
          </div>
        )}

        {!hasMore && items.length > 0 && (
          <p className="text-center text-xs text-muted-foreground py-4">
            All items loaded
          </p>
        )}
      </div>
    </div>
  )
}

function groupItems(
  items: Record<string, unknown>[],
  dateField: string,
  groupBy: string,
): [string, Record<string, unknown>[]][] {
  const groups = new Map<string, Record<string, unknown>[]>()

  for (const item of items) {
    const date = item[dateField] as string
    if (!date) continue

    let key: string
    const d = new Date(date)
    switch (groupBy) {
      case "year":
        key = d.getFullYear().toString()
        break
      case "month":
        key = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`
        break
      case "none":
        key = "All"
        break
      case "date":
      default:
        key = d.toLocaleDateString(undefined, {
          weekday: "long",
          year: "numeric",
          month: "long",
          day: "numeric",
        })
        break
    }

    const list = groups.get(key) ?? []
    list.push(item)
    groups.set(key, list)
  }

  return Array.from(groups.entries())
}
