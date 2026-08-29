// ─── Notification Center Renderer ───
//
// In-app notification feed (kind: NotificationCenter, 06-page-kinds.md §12).
// Zero-config: lists the caller's notifications from formspec/notify.
//
// The backing data source is the caller's notifications. When a conventional
// notification entity is present in the bundle (e.g. `formspec.core.
// notification`), this renderer lists them with unread badge + mark-read;
// otherwise it shows a clear empty state — the kind plumbing (spec, registry,
// bundle, route) is complete regardless.
//
// Design doc §5.5 NotificationCenter kind (F4)

import { useEffect, useMemo, useState } from "react"
import { toast } from "@/lib/ui"
import { Bell, Check, Loader2 } from "lucide-react"

import type { Entry, NotificationCenterSpec } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { apiList, apiPatch } from "@/lib/api"
import { useRealtime } from "@/hooks/useRealtime"
import { createFormatter } from "@/lib/format"
import { titleCase } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Badge } from "@/widgets/Badge"
import { cn } from "@/lib/utils"

interface NotificationCenterRendererProps {
  entry: Entry<NotificationCenterSpec>
}

/** Conventional notification entity refs this renderer looks for. */
const NOTIFICATION_ENTITY_REFS = [
  "formspec.core.notification",
  "formspec.core.notify",
]

export default function NotificationCenterRenderer({
  entry,
}: NotificationCenterRendererProps) {
  const getClient = useSessionStore((s) => s.getClient)
  const getEntity = useMetaStore((s) => s.getEntity)
  const settings = useMetaStore((s) => s.bundle?.settings)
  const formatter = useMemo(() => createFormatter(settings), [settings])

  const notificationEntity = useMemo(() => {
    for (const ref of NOTIFICATION_ENTITY_REFS) {
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
    if (!notificationEntity) {
      setLoading(false)
      return
    }
    if (!silent) setLoading(true)
    try {
      const client = getClient()
      const result = await apiList<Record<string, unknown>>(
        client,
        `${notificationEntity.module}/${notificationEntity.name}`,
        { per_page: "100", sort: "-created_at" },
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
  }, [notificationEntity])

  // Realtime (spec §5): matching entity event → silent refetch.
  const realtimeTick = useRealtime(
    entry.spec.realtime && notificationEntity
      ? `${notificationEntity.module}/${notificationEntity.name}`
      : "",
  )
  useEffect(() => {
    if (realtimeTick === 0) return
    fetchItems(true)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [realtimeTick])

  const markRead = async (id: string) => {
    if (!notificationEntity) return
    setBusyId(id)
    try {
      const client = getClient()
      await apiPatch(
        client,
        `${notificationEntity.module}/${notificationEntity.name}/${id}`,
        { read: true },
      )
      fetchItems(true)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to mark read")
    } finally {
      setBusyId(null)
    }
  }

  const unreadCount = items.filter(
    (i) => !i.read && String(i.read) !== "true",
  ).length

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            {titleCase(entry.name)}
          </h1>
          <p className="text-sm text-muted-foreground">{unreadCount} unread</p>
        </div>
        {unreadCount > 0 && <Badge value={String(unreadCount)} />}
      </div>

      {!notificationEntity ? (
        <div className="rounded-md border border-dashed p-10 text-center text-sm text-muted-foreground">
          <Bell className="size-8 mx-auto mb-2 opacity-40" />
          <p className="font-medium">No notification source configured</p>
          <p className="mt-1">
            NotificationCenter is zero-config — it lists the caller's
            notifications from formspec/notify. Add a notification entity (e.g.{" "}
            <code className="mx-1">formspec.core.notification</code>) to
            populate this feed.
          </p>
        </div>
      ) : loading ? (
        <div className="flex items-center justify-center py-16">
          <Loader2 className="size-6 animate-spin text-muted-foreground" />
        </div>
      ) : items.length === 0 ? (
        <div className="rounded-md border border-dashed p-10 text-center text-sm text-muted-foreground">
          No notifications.
        </div>
      ) : (
        <div className="space-y-2">
          {items.map((item) => {
            const isUnread = !item.read && String(item.read) !== "true"
            return (
              <div
                key={String(item.id)}
                className={cn(
                  "rounded-md border p-3 flex items-start justify-between gap-3",
                  isUnread && "bg-primary/5 border-primary/20",
                )}
              >
                <div className="min-w-0">
                  <p className="text-sm font-medium truncate">
                    {String(item.title ?? item.message ?? item.id)}
                  </p>
                  {Boolean(item.message) && item.title != null && (
                    <p className="text-sm text-muted-foreground truncate">
                      {String(item.message)}
                    </p>
                  )}
                  <p className="text-xs text-muted-foreground mt-1">
                    {item.created_at
                      ? formatter.dateTime(String(item.created_at))
                      : "—"}
                  </p>
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  {isUnread && (
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={busyId === item.id}
                      onClick={() => markRead(String(item.id))}
                    >
                      <Check className="size-3.5 mr-1" />
                      Mark read
                    </Button>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
