// ─── Realtime Status Store ───
//
// Liveness of the single shared websocket connection (one per tab = one
// session). Written by the `RealtimeClient` singleton in hooks/useRealtime.ts,
// read by the chrome indicator (shell/RealtimeStatus.tsx).
//
// The heartbeat is owned by the transport — one per connection, never one per
// `useRealtime` subscriber — so this store is the single place the whole app
// reads connection health from.
//
// `active` is the demand signal: it is true only while at least one subscriber
// (a Table/Kanban/… with realtime on the CURRENT page) wants events. A page
// that does not use realtime must be left alone — its socket may be down
// without anything being wrong, so the indicator hides and the watchdog is
// disarmed whenever `active` is false.

import { create } from "zustand"

export type RealtimeStatus =
  | "idle" // no connection wanted (nothing subscribed yet)
  | "connecting" // socket opening, ticket being minted
  | "live" // socket open, heartbeats arriving
  | "stalled" // socket looks open but no frame arrived within the budget
  | "reconnecting" // socket closed; a backoff retry is scheduled

interface RealtimeState {
  status: RealtimeStatus
  /** Whether realtime is wanted right now (≥1 subscriber in this tab). When
   *  false the transport is idle-by-design and no indicator may be shown. */
  active: boolean
  /** epoch ms of the last inbound frame; 0 before the first one. */
  lastMessageAt: number
  /** consecutive reconnect attempts since the last live connection. */
  attempt: number
  setStatus: (status: RealtimeStatus) => void
  setActive: (active: boolean) => void
  markMessage: () => void
  bumpAttempt: () => void
}

export const useRealtimeStore = create<RealtimeState>((set) => ({
  status: "idle",
  active: false,
  lastMessageAt: 0,
  attempt: 0,
  setStatus: (status) =>
    set((s) =>
      s.status === status
        ? s
        : { status, attempt: status === "live" ? 0 : s.attempt },
    ),
  setActive: (active) => set((s) => (s.active === active ? s : { active })),
  markMessage: () => set({ lastMessageAt: Date.now() }),
  bumpAttempt: () => set((s) => ({ attempt: s.attempt + 1 })),
}))
