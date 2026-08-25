// ─── Calendar Renderer ───
//
// Calendar view over an entity with a date/datetime field (kind: Calendar,
// 06-page-kinds.md §5). Zero-config: entity + date_field renders a month
// view; events titled from label_field. Views: month | week | day | resource.
//
// Interactions:
//   - Click event → entity detail page
//   - Click empty slot → Form create with date_field pre-filled
//   - Drag reschedule → PATCH date_field (end_field shifts proportionally);
//     server-side validation is authority; submitted rows are not draggable
//   - RRULE (RFC 5545) recurrence expanded to instances for the visible range
//     (render-time, not materialized) via the `rrule` library
//
// Design doc §5.5 Calendar kind (F4)

import { useEffect, useMemo, useState, useCallback } from "react"
import { useNavigate, useSearchParams } from "react-router-dom"
import { toast } from "@/lib/ui"
import { ChevronLeft, ChevronRight, Loader2, Plus } from "lucide-react"
import { RRule } from "rrule"

import type { Entry, CalendarSpec, EntitySchema } from "@/types/manifest"
import { FormaApiError } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { resolveEntityRef } from "@/engine/entityRef"
import { can as checkPermission } from "@/engine/permissions"
import { useSurface } from "@/hooks/useSurface"
import { useRealtime } from "@/hooks/useRealtime"
import { apiList, apiPatch } from "@/lib/api"
import { createFormatter } from "@/lib/format"
import { titleCase } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

interface CalendarRendererProps {
  entry: Entry<CalendarSpec>
}

interface CalEvent {
  id: string
  title: string
  start: Date
  end: Date
  record: Record<string, unknown>
  color?: string
  resource?: string
  /** True when this event is an RRULE-expanded occurrence (not the raw row). */
  recurring?: boolean
}

const VIEWS = ["month", "week", "day", "resource"] as const
type View = (typeof VIEWS)[number]

const DAY_MS = 24 * 60 * 60 * 1000

// ── Date helpers (local-time, business dates) ──

function startOfDay(d: Date): Date {
  const out = new Date(d)
  out.setHours(0, 0, 0, 0)
  return out
}

function addDays(d: Date, n: number): Date {
  const out = new Date(d)
  out.setDate(out.getDate() + n)
  return out
}

function addMonths(d: Date, n: number): Date {
  const out = new Date(d)
  out.setMonth(out.getMonth() + n)
  return out
}

function toDate(value: unknown): Date | null {
  if (value == null) return null
  const d = new Date(String(value))
  return isNaN(d.getTime()) ? null : d
}

function sameDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  )
}

function fmtTime(d: Date): string {
  return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
}

// ── RRULE expansion (5.6.5) ──
//
// Expands a record's `recurrence` field (RFC 5545 RRULE string) into concrete
// instances within [rangeStart, rangeEnd]. Pure render-time computation — no
// materialized rows. Returns [] when the record has no recurrence.
function expandRecurrence(
  record: Record<string, unknown>,
  dateField: string,
  endField: string | undefined,
  rangeStart: Date,
  rangeEnd: Date,
): CalEvent[] {
  const rruleStr = record.recurrence
  if (typeof rruleStr !== "string" || !rruleStr.trim()) return []

  const start = toDate(record[dateField])
  if (!start) return []

  let rule: RRule
  try {
    rule = RRule.fromString(rruleStr)
  } catch {
    return [] // malformed RRULE — render the base event only
  }

  // Expand within the visible range (bounded — never unbounded).
  const instances = rule.between(
    new Date(rangeStart.getTime() - DAY_MS),
    new Date(rangeEnd.getTime() + DAY_MS),
    true,
  )

  const duration = endField
    ? (toDate(record[endField])?.getTime() ?? 0) - start.getTime()
    : 0

  return instances.map((inst) => ({
    id: `${record.id}:${inst.getTime()}`,
    title: String(record.title ?? record.id ?? ""),
    start: inst,
    end: new Date(inst.getTime() + Math.max(duration, 0)),
    record,
    recurring: true,
  }))
}

export default function CalendarRenderer({ entry }: CalendarRendererProps) {
  const navigate = useNavigate()
  const { surfacePath } = useSurface()
  const [searchParams, setSearchParams] = useSearchParams()
  const getClient = useSessionStore((s) => s.getClient)
  const getEntity = useMetaStore((s) => s.getEntity)
  const me = useSessionStore((s) => s.me)
  const settings = useMetaStore((s) => s.bundle?.settings)
  const formatter = useMemo(() => createFormatter(settings), [settings])

  const [entityModule, entityName] = resolveEntityRef(
    entry.spec.entity,
    entry.module,
  )
  const entity = getEntity(entityModule, entityName)
  const dateField = entry.spec.date_field
  const endField = entry.spec.end_field
  const titleField = entry.spec.title_field
  const resourceField = entry.spec.resource_field
  const colorField = entry.spec.color_field
  const views = entry.spec.views?.length ? entry.spec.views : ["month"]

  const [records, setRecords] = useState<Record<string, unknown>[]>([])
  const [loading, setLoading] = useState(true)
  const [cursor, setCursor] = useState<Date>(startOfDay(new Date()))
  const [view, setView] = useState<View>(
    (views.includes("month") ? "month" : views[0]) as View,
  )

  // ── Data fetching ──
  const fetchRecords = useCallback(
    async (silent = false) => {
      if (!entity) {
        toast.error(`entity "${entry.spec.entity}" not found`)
        setLoading(false)
        return
      }
      if (!silent) setLoading(true)
      try {
        const client = getClient()
        const result = await apiList<Record<string, unknown>>(
          client,
          `${entity.module}/${entity.name}`,
          { per_page: "1000", sort: dateField },
        )
        setRecords(result.items)
      } catch {
        toast.error("Failed to load calendar data")
      } finally {
        if (!silent) setLoading(false)
      }
    },
    [entity, entry.spec.entity, dateField, getClient],
  )

  useEffect(() => {
    fetchRecords()
  }, [fetchRecords])

  // Realtime (spec §5): matching entity event → silent refetch.
  const realtimeTick = useRealtime(
    entry.spec.realtime && entityModule && entityName
      ? `${entityModule}/${entityName}`
      : "",
  )
  useEffect(() => {
    if (realtimeTick === 0) return
    fetchRecords(true)
  }, [realtimeTick, fetchRecords])

  // ── Visible range per view ──
  const range = useMemo(() => {
    const start = startOfDay(cursor)
    if (view === "month") {
      const first = new Date(start.getFullYear(), start.getMonth(), 1)
      const last = new Date(start.getFullYear(), start.getMonth() + 1, 0)
      return { start: first, end: last }
    }
    if (view === "week") {
      const dow = start.getDay()
      return { start: addDays(start, -dow), end: addDays(start, 6 - dow) }
    }
    if (view === "day") {
      return { start, end: start }
    }
    // resource view — show the month by default
    const first = new Date(start.getFullYear(), start.getMonth(), 1)
    const last = new Date(start.getFullYear(), start.getMonth() + 1, 0)
    return { start: first, end: last }
  }, [cursor, view])

  // ── Build events (base + RRULE-expanded) ──
  const events = useMemo<CalEvent[]>(() => {
    if (!entity) return []
    const out: CalEvent[] = []
    for (const rec of records) {
      const start = toDate(rec[dateField])
      if (!start) continue
      const end = endField ? (toDate(rec[endField]) ?? start) : start
      const title = titleField
        ? String(rec[titleField] ?? "")
        : String(rec[entity.label_field] ?? rec.id ?? "")
      const color = colorField ? String(rec[colorField] ?? "") : undefined
      const resource = resourceField
        ? String(rec[resourceField] ?? "")
        : undefined
      out.push({
        id: String(rec.id),
        title,
        start,
        end,
        record: rec,
        color,
        resource,
      })
      // RRULE expansion (5.6.5) — only for the base row, within range.
      out.push(
        ...expandRecurrence(rec, dateField, endField, range.start, range.end),
      )
    }
    return out
  }, [
    entity,
    records,
    dateField,
    endField,
    titleField,
    colorField,
    resourceField,
    range,
  ])

  // ── Resource lanes (5.6.6) ──
  const resources = useMemo(() => {
    if (!resourceField) return []
    const set = new Set<string>()
    for (const e of events) if (e.resource) set.add(e.resource)
    return Array.from(set).sort()
  }, [events, resourceField])

  // ── Navigation ──
  const prev = () => {
    if (view === "month") setCursor(addMonths(cursor, -1))
    else if (view === "week") setCursor(addDays(cursor, -7))
    else setCursor(addDays(cursor, -1))
  }
  const next = () => {
    if (view === "month") setCursor(addMonths(cursor, 1))
    else if (view === "week") setCursor(addDays(cursor, 7))
    else setCursor(addDays(cursor, 1))
  }

  // ── Click event → detail (5.6.3) ──
  const openEvent = (e: CalEvent) => {
    if (!entity) return
    navigate(surfacePath(entity.module, entity.plural, e.record.id as string))
  }

  // ── Click empty slot → Form create with date pre-filled (5.6.3) ──
  const createAt = (day: Date) => {
    if (!entity) return
    const iso = day.toISOString()
    setSearchParams({
      action: "create",
      entity: `${entity.module}.${entity.name}`,
      [`prefill.${dateField}`]: iso,
    })
  }

  // ── Drag reschedule (5.6.4) ──
  const [dragging, setDragging] = useState<CalEvent | null>(null)
  const [dropDay, setDropDay] = useState<Date | null>(null)

  const onDragStart = (e: CalEvent) => {
    if (!entity || !me) return
    const perm = `${entity.module}.${entity.plural}.update`
    if (!checkPermission(perm, me.permissions)) {
      toast.error("You don't have permission to reschedule")
      return
    }
    // Submitted rows are immutable — not draggable (server also enforces).
    if (
      e.record.doc_status === "submitted" ||
      e.record.status === "submitted"
    ) {
      toast.error("Submitted records cannot be rescheduled")
      return
    }
    setDragging(e)
  }

  const onDrop = async (day: Date) => {
    const e = dragging
    setDragging(null)
    setDropDay(null)
    if (!e || !entity) return
    const delta = day.getTime() - startOfDay(e.start).getTime()
    if (delta === 0) return

    const newStart = new Date(e.start.getTime() + delta)
    const patch: Record<string, unknown> = {
      [dateField]: newStart.toISOString(),
    }
    // Shift end_field proportionally for ranges (5.6.4).
    if (endField && e.end.getTime() > e.start.getTime()) {
      const duration = e.end.getTime() - e.start.getTime()
      patch[endField] = new Date(newStart.getTime() + duration).toISOString()
    }

    try {
      const client = getClient()
      await apiPatch(
        client,
        `${entity.module}/${entity.name}/${e.record.id}`,
        patch,
        typeof e.record.version === "number" ? e.record.version : undefined,
      )
      toast.success("Rescheduled")
      fetchRecords(true)
    } catch (err) {
      if (err instanceof FormaApiError && err.status === 409) {
        toast.error("Data telah diubah oleh pengguna lain — reload")
      } else {
        toast.error(err instanceof Error ? err.message : "Reschedule failed")
      }
    }
  }

  const rangeLabel = useMemo(() => {
    if (view === "month") {
      return cursor.toLocaleDateString([], { month: "long", year: "numeric" })
    }
    if (view === "week") {
      return `${range.start.toLocaleDateString()} – ${range.end.toLocaleDateString()}`
    }
    return cursor.toLocaleDateString()
  }, [view, cursor, range])

  if (!entity) {
    return (
      <div className="rounded-md border border-destructive/50 p-4 text-sm text-destructive">
        Entity "{entry.spec.entity}" not found.
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            {titleCase(entry.name)}
          </h1>
          <p className="text-sm text-muted-foreground">
            {formatter.date(range.start.toISOString())} —{" "}
            {formatter.date(range.end.toISOString())}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <div className="flex rounded-md border overflow-hidden">
            {VIEWS.filter((v) => views.includes(v)).map((v) => (
              <button
                key={v}
                type="button"
                onClick={() => setView(v)}
                className={cn(
                  "px-3 py-1.5 text-xs font-medium",
                  view === v
                    ? "bg-primary text-primary-foreground"
                    : "hover:bg-muted",
                )}
              >
                {titleCase(v)}
              </button>
            ))}
          </div>
          <div className="flex items-center gap-1">
            <Button variant="outline" size="sm" onClick={prev}>
              <ChevronLeft className="size-4" />
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setCursor(startOfDay(new Date()))}
            >
              Today
            </Button>
            <Button variant="outline" size="sm" onClick={next}>
              <ChevronRight className="size-4" />
            </Button>
          </div>
        </div>
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-16">
          <Loader2 className="size-6 animate-spin text-muted-foreground" />
        </div>
      ) : view === "resource" ? (
        <ResourceView
          events={events}
          resources={resources}
          range={range}
          onOpen={openEvent}
          onDragStart={onDragStart}
          onDrop={onDrop}
          dragging={dragging}
          dropDay={dropDay}
          setDropDay={setDropDay}
          onCreate={createAt}
        />
      ) : view === "month" ? (
        <MonthView
          cursor={cursor}
          events={events}
          onOpen={openEvent}
          onDragStart={onDragStart}
          onDrop={onDrop}
          dragging={dragging}
          dropDay={dropDay}
          setDropDay={setDropDay}
          onCreate={createAt}
        />
      ) : view === "week" ? (
        <WeekView
          range={range}
          events={events}
          onOpen={openEvent}
          onDragStart={onDragStart}
          onDrop={onDrop}
          dragging={dragging}
          dropDay={dropDay}
          setDropDay={setDropDay}
          onCreate={createAt}
        />
      ) : (
        <DayView
          day={range.start}
          events={events}
          onOpen={openEvent}
          onDragStart={onDragStart}
          onDrop={onDrop}
          dragging={dragging}
          dropDay={dropDay}
          setDropDay={setDropDay}
          onCreate={createAt}
        />
      )}
    </div>
  )
}

// ── Shared event chip ──

function EventChip({
  event,
  onOpen,
  onDragStart,
}: {
  event: CalEvent
  onOpen: (e: CalEvent) => void
  onDragStart: (e: CalEvent) => void
}) {
  return (
    <div
      draggable
      onDragStart={(ev) => {
        ev.dataTransfer.setData("text/plain", event.id)
        onDragStart(event)
      }}
      onClick={(ev) => {
        ev.stopPropagation()
        onOpen(event)
      }}
      className="cursor-pointer rounded px-1.5 py-0.5 text-xs truncate hover:opacity-80"
      style={{
        background: event.color ? `${event.color}22` : "var(--primary)",
        color: event.color ? event.color : "var(--primary-foreground)",
        borderLeft: `3px solid ${event.color ?? "var(--primary)"}`,
      }}
      title={event.recurring ? `${event.title} (recurring)` : event.title}
    >
      {event.recurring && <span className="mr-0.5">↻</span>}
      {event.title}
    </div>
  )
}

// ── Month view ──

function MonthView({
  cursor,
  events,
  onOpen,
  onDragStart,
  onDrop,
  dragging,
  dropDay,
  setDropDay,
  onCreate,
}: {
  cursor: Date
  events: CalEvent[]
  onOpen: (e: CalEvent) => void
  onDragStart: (e: CalEvent) => void
  onDrop: (day: Date) => void
  dragging: CalEvent | null
  dropDay: Date | null
  setDropDay: (d: Date | null) => void
  onCreate: (day: Date) => void
}) {
  const year = cursor.getFullYear()
  const month = cursor.getMonth()
  const first = new Date(year, month, 1)
  const startDow = first.getDay()
  const daysInMonth = new Date(year, month + 1, 0).getDate()
  const cells: (Date | null)[] = []
  for (let i = 0; i < startDow; i++) cells.push(null)
  for (let d = 1; d <= daysInMonth; d++) cells.push(new Date(year, month, d))

  return (
    <div className="rounded-md border overflow-hidden">
      <div className="grid grid-cols-7 border-b bg-muted/50 text-center text-xs font-medium text-muted-foreground">
        {["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"].map((d) => (
          <div key={d} className="py-2">
            {d}
          </div>
        ))}
      </div>
      <div className="grid grid-cols-7">
        {cells.map((day, idx) => {
          if (!day) return <div key={idx} className="min-h-24 bg-muted/20" />
          const dayEvents = events.filter((e) => sameDay(e.start, day))
          const isDrop = dropDay && sameDay(dropDay, day)
          return (
            <div
              key={idx}
              className={cn(
                "min-h-24 border-t border-l p-1 space-y-0.5",
                day.getMonth() !== month && "bg-muted/20",
                isDrop && "bg-primary/10 ring-2 ring-primary ring-inset",
              )}
              onClick={() => onCreate(day)}
              onDragOver={(e) => {
                e.preventDefault()
                setDropDay(day)
              }}
              onDragLeave={() =>
                setDropDay((d) => (d && sameDay(d, day) ? null : d))
              }
              onDrop={(e) => {
                e.preventDefault()
                onDrop(day)
              }}
            >
              <div className="text-xs text-muted-foreground">
                {day.getDate()}
              </div>
              {dayEvents.slice(0, 3).map((e) => (
                <EventChip
                  key={e.id}
                  event={e}
                  onOpen={onOpen}
                  onDragStart={onDragStart}
                />
              ))}
              {dayEvents.length > 3 && (
                <div className="text-[10px] text-muted-foreground">
                  +{dayEvents.length - 3} more
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}

// ── Week view ──

function WeekView({
  range,
  events,
  onOpen,
  onDragStart,
  onDrop,
  dragging,
  dropDay,
  setDropDay,
  onCreate,
}: {
  range: { start: Date; end: Date }
  events: CalEvent[]
  onOpen: (e: CalEvent) => void
  onDragStart: (e: CalEvent) => void
  onDrop: (day: Date) => void
  dragging: CalEvent | null
  dropDay: Date | null
  setDropDay: (d: Date | null) => void
  onCreate: (day: Date) => void
}) {
  const days: Date[] = []
  for (let i = 0; i < 7; i++) days.push(addDays(range.start, i))

  return (
    <div className="rounded-md border overflow-hidden">
      <div className="grid grid-cols-7 border-b bg-muted/50 text-center text-xs font-medium text-muted-foreground">
        {days.map((d) => (
          <div key={d.toISOString()} className="py-2">
            {d.toLocaleDateString([], { weekday: "short", day: "numeric" })}
          </div>
        ))}
      </div>
      <div className="grid grid-cols-7">
        {days.map((day) => {
          const dayEvents = events.filter((e) => sameDay(e.start, day))
          const isDrop = dropDay && sameDay(dropDay, day)
          return (
            <div
              key={day.toISOString()}
              className={cn(
                "min-h-40 border-t border-l p-1 space-y-0.5",
                isDrop && "bg-primary/10 ring-2 ring-primary ring-inset",
              )}
              onClick={() => onCreate(day)}
              onDragOver={(e) => {
                e.preventDefault()
                setDropDay(day)
              }}
              onDragLeave={() =>
                setDropDay((d) => (d && sameDay(d, day) ? null : d))
              }
              onDrop={(e) => {
                e.preventDefault()
                onDrop(day)
              }}
            >
              {dayEvents.map((e) => (
                <EventChip
                  key={e.id}
                  event={e}
                  onOpen={onOpen}
                  onDragStart={onDragStart}
                />
              ))}
            </div>
          )
        })}
      </div>
    </div>
  )
}

// ── Day view ──

function DayView({
  day,
  events,
  onOpen,
  onDragStart,
  onDrop,
  dragging,
  dropDay,
  setDropDay,
  onCreate,
}: {
  day: Date
  events: CalEvent[]
  onOpen: (e: CalEvent) => void
  onDragStart: (e: CalEvent) => void
  onDrop: (day: Date) => void
  dragging: CalEvent | null
  dropDay: Date | null
  setDropDay: (d: Date | null) => void
  onCreate: (day: Date) => void
}) {
  const dayEvents = events
    .filter((e) => sameDay(e.start, day))
    .sort((a, b) => a.start.getTime() - b.start.getTime())

  return (
    <div className="rounded-md border overflow-hidden">
      <div className="border-b bg-muted/50 px-3 py-2 text-sm font-medium">
        {day.toLocaleDateString([], {
          weekday: "long",
          day: "numeric",
          month: "long",
        })}
      </div>
      <div
        className="min-h-64 p-2 space-y-1"
        onClick={() => onCreate(day)}
        onDragOver={(e) => e.preventDefault()}
        onDrop={(e) => {
          e.preventDefault()
          onDrop(day)
        }}
      >
        {dayEvents.length === 0 && (
          <p className="text-sm text-muted-foreground text-center py-8">
            No events — click to create
          </p>
        )}
        {dayEvents.map((e) => (
          <div
            key={e.id}
            className="flex items-center gap-2"
            onClick={(ev) => ev.stopPropagation()}
          >
            <span className="text-xs text-muted-foreground w-12 shrink-0">
              {fmtTime(e.start)}
            </span>
            <div className="flex-1">
              <EventChip event={e} onOpen={onOpen} onDragStart={onDragStart} />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

// ── Resource view (5.6.6) ──

function ResourceView({
  events,
  resources,
  range,
  onOpen,
  onDragStart,
  onDrop,
  dragging,
  dropDay,
  setDropDay,
  onCreate,
}: {
  events: CalEvent[]
  resources: string[]
  range: { start: Date; end: Date }
  onOpen: (e: CalEvent) => void
  onDragStart: (e: CalEvent) => void
  onDrop: (day: Date) => void
  dragging: CalEvent | null
  dropDay: Date | null
  setDropDay: (d: Date | null) => void
  onCreate: (day: Date) => void
}) {
  // Build the day columns for the visible month.
  const year = range.start.getFullYear()
  const month = range.start.getMonth()
  const daysInMonth = new Date(year, month + 1, 0).getDate()
  const days: Date[] = []
  for (let d = 1; d <= daysInMonth; d++) days.push(new Date(year, month, d))

  const lanes = resources.length > 0 ? resources : ["—"]

  return (
    <div className="rounded-md border overflow-hidden">
      <div
        className="grid border-b bg-muted/50 text-center text-xs font-medium text-muted-foreground"
        style={{
          gridTemplateColumns: `120px repeat(${days.length}, minmax(0,1fr))`,
        }}
      >
        <div className="py-2 px-2 text-left">Resource</div>
        {days.map((d) => (
          <div key={d.toISOString()} className="py-2">
            {d.getDate()}
          </div>
        ))}
      </div>
      {lanes.map((lane) => (
        <div
          key={lane}
          className="grid border-b last:border-b-0"
          style={{
            gridTemplateColumns: `120px repeat(${days.length}, minmax(0,1fr))`,
          }}
        >
          <div className="px-2 py-1 text-xs font-medium truncate border-r">
            {lane}
          </div>
          {days.map((day) => {
            const dayEvents = events.filter(
              (e) => sameDay(e.start, day) && (e.resource ?? "—") === lane,
            )
            const isDrop = dropDay && sameDay(dropDay, day)
            return (
              <div
                key={day.toISOString()}
                className={cn(
                  "min-h-12 border-r p-0.5 space-y-0.5",
                  isDrop && "bg-primary/10 ring-2 ring-primary ring-inset",
                )}
                onClick={() => onCreate(day)}
                onDragOver={(e) => {
                  e.preventDefault()
                  setDropDay(day)
                }}
                onDragLeave={() =>
                  setDropDay((d) => (d && sameDay(d, day) ? null : d))
                }
                onDrop={(e) => {
                  e.preventDefault()
                  onDrop(day)
                }}
              >
                {dayEvents.map((e) => (
                  <EventChip
                    key={e.id}
                    event={e}
                    onOpen={onOpen}
                    onDragStart={onDragStart}
                  />
                ))}
              </div>
            )
          })}
        </div>
      ))}
    </div>
  )
}
