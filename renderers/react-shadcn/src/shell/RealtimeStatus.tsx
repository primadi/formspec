// ─── RealtimeStatus — non-intrusive connection indicator (chrome) ───
//
// Reads the transport-owned liveness status (stores/realtime.ts, written by the
// RealtimeClient singleton in hooks/useRealtime.ts). Renders NOTHING while the
// connection is healthy: a permanent "online" dot would be chrome noise. Only a
// degraded connection earns a small pill — no modal, no toast — so the user
// learns that data may be stale without being interrupted.

import { useRealtimeStore } from "@/stores/realtime"
import { cn } from "@/lib/utils"

export function RealtimeStatus() {
  const active = useRealtimeStore((s) => s.active)
  const status = useRealtimeStore((s) => s.status)
  const attempt = useRealtimeStore((s) => s.attempt)

  // Not every page uses realtime (a Table/Kanban without `realtime: true`
  // subscribes to nothing). When nothing on the current page wants events, a
  // dropped connection is not a problem the user should be told about — stay
  // silent even if the transport is down.
  if (!active) return null
  if (status !== "stalled" && status !== "reconnecting") return null

  const stalled = status === "stalled"
  const label = stalled
    ? "Realtime tidak merespons"
    : attempt > 1
      ? `Menyambung ulang… (${attempt})`
      : "Menyambung ulang…"

  return (
    <span
      role="status"
      aria-live="polite"
      title={
        stalled
          ? "Koneksi realtime tidak merespons — data mungkin tidak terbaru"
          : "Koneksi realtime terputus — menyambung ulang"
      }
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border border-border/60",
        "bg-muted/60 px-2 py-0.5 text-xs font-medium text-muted-foreground",
      )}
    >
      <span
        aria-hidden="true"
        className={cn(
          "size-1.5 shrink-0 rounded-full bg-amber-500",
          !stalled && "animate-pulse",
        )}
      />
      <span className="hidden sm:inline">{label}</span>
    </span>
  )
}
