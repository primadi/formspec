// ─── Realtime (WebSocket) Hook ───
//
// Connects to `/{workspace}/_ui/_ws` and fans out EventMessages to
// subscribers filtered by resource/event. A single WebSocket is shared across
// all consumers (one connection per tab).
//
// Server-side subscription (Spec Resolution API §5): after connecting the
// client tells the hub which resources (and optional events) it wants via
// `{op:"subscribe",resource}` / `{op:"unsubscribe",resource}` frames, so the
// server only pushes matching events. The RealtimeClient aggregates every
// `useRealtime` subscriber into a union and sends only the deltas when the
// subscriber set changes (e.g. on page navigation), re-registering the full
// set after every reconnect. The local `resource`/`event` filter below stays
// as a safety net (idempotent).
//
// Realtime is non-durable by definition (spec Resolution API §5): there is no
// replay, so consumers MUST refetch after a reconnect. This hook surfaces that
// as a `tick` that increments on every matching event AND on reconnect — a
// consumer just re-runs its load whenever `tick` changes.
//
// Heartbeat: the server emits an application-level `{op:"hb"}` frame every
// ~25s and this singleton answers `{op:"hb_ack"}`. The timer lives HERE (one
// per connection = one per session/tab), never in a `useRealtime` subscriber,
// so N components in a tab still produce one heartbeat. Protocol-level
// ping/pong is invisible to JavaScript, which is why the pulse is a text
// frame: without it a quiet-but-healthy workspace is indistinguishable from a
// dead connection. Status is published to `stores/realtime` for the chrome
// indicator (shell/RealtimeStatus.tsx).

import { useEffect, useRef, useState } from "react"

import { useSessionStore } from "@/stores/session"
import { useRealtimeStore, type RealtimeStatus } from "@/stores/realtime"
import type { RealtimeMessage } from "@/types/events"

interface RealtimeSub {
  resource: string // "module/entity" or "*"
  event?: string
  onEvent: (msg: RealtimeMessage) => void
  onReconnect?: () => void
}

/** Server → client frame. Event frames carry resource/event; control frames
 *  carry `op` (currently only "hb") and MUST NOT be fanned out to subscribers. */
type WireFrame = Partial<RealtimeMessage> & { op?: string; ts?: number }

/** How long the client tolerates silence before declaring the socket stalled.
 *  Must comfortably exceed the server's hb interval (25s) — one missed pulse
 *  is not a failure. */
const HB_STALL_MS = 45000

// ── Singleton connection manager ──

let client: RealtimeClient | null = null

function getClient(): RealtimeClient {
  if (!client) client = new RealtimeClient()
  return client
}

class RealtimeClient {
  private ws: WebSocket | null = null
  private subs = new Set<RealtimeSub>()
  /** WS URL with no credential in it — ticket/token are appended per attempt. */
  private url = ""
  /** POST endpoint that mints a single-use handshake ticket (todo 5.8.4). */
  private ticketUrl = ""
  /** Legacy handshake credential, used only if ticket issuance fails. */
  private token = ""
  /** Bumped by configure(); an in-flight open() whose generation is stale
   *  aborts instead of connecting with the previous URL. */
  private gen = 0
  private retryMs = 1000
  private retryTimer: number | undefined

  /** resource → set of event names the hub is currently told to push, with ""
   *  inside a set meaning "all events on that resource". Mirrors the
   *  connection's subscription state on the server so we can send deltas. */
  private subscribed = new Map<string, Set<string>>()

  /** Liveness watchdog — armed while a socket is open, reset by any inbound
   *  frame, and disarmed while the tab is hidden (browser timers are throttled
   *  in the background, which would otherwise look like a stall). */
  private livenessTimer: number | undefined
  /** Set when the watchdog gave up on a socket. The close it triggers would
   *  otherwise be reported as a plain "reconnecting"; keeping the flag makes
   *  the chrome say "tidak merespons" (the true cause) until a frame arrives. */
  private stalled = false

  constructor() {
    if (typeof document !== "undefined") {
      document.addEventListener("visibilitychange", this.onVisibility)
    }
  }

  private setStatus(status: RealtimeStatus) {
    useRealtimeStore.getState().setStatus(status)
  }

  /** Whether realtime is actually wanted right now. With no subscribers the
   *  socket is idle-by-design: a dead connection on a non-realtime page is not
   *  a problem, so the watchdog stays disarmed and the indicator stays hidden. */
  private hasDemand(): boolean {
    return this.subs.size > 0
  }

  /** Republish the demand signal and reconcile the watchdog with it. */
  private updateDemand() {
    const active = this.hasDemand()
    useRealtimeStore.getState().setActive(active)
    if (!active) {
      this.clearLiveness()
      this.stalled = false
      this.setStatus("idle")
      return
    }
    if (this.ws?.readyState === WebSocket.OPEN) this.armLiveness()
  }

  /** Back in the foreground: re-evaluate staleness immediately instead of
   *  making the user wait another full budget. */
  private onVisibility = () => {
    if (typeof document === "undefined") return
    if (document.visibilityState === "hidden") {
      this.clearLiveness()
      return
    }
    if (!this.hasDemand()) return
    const ws = this.ws
    if (!ws || ws.readyState !== WebSocket.OPEN) return
    const last = useRealtimeStore.getState().lastMessageAt
    if (last > 0 && Date.now() - last > HB_STALL_MS) {
      this.stalled = true
      this.setStatus("stalled")
      ws.close()
      return
    }
    this.armLiveness()
  }

  private armLiveness() {
    this.clearLiveness()
    // No demand → nothing to watch; the socket may be down harmlessly.
    if (!this.hasDemand()) return
    if (
      typeof document !== "undefined" &&
      document.visibilityState === "hidden"
    )
      return
    this.livenessTimer = window.setTimeout(() => {
      this.livenessTimer = undefined
      // The socket still looks OPEN but nothing has arrived: declare the
      // heartbeat dead and force the reconnect path (onclose reschedules).
      this.stalled = true
      this.setStatus("stalled")
      this.ws?.close()
    }, HB_STALL_MS)
  }

  private clearLiveness() {
    if (this.livenessTimer !== undefined) {
      window.clearTimeout(this.livenessTimer)
      this.livenessTimer = undefined
    }
  }

  /** Any inbound frame — event or control — proves the connection is alive. */
  private markAlive() {
    useRealtimeStore.getState().markMessage()
    this.stalled = false
    // Only a page that wants realtime can be "live"; otherwise stay idle so a
    // later, unrelated outage is not misreported as a problem here.
    if (!this.hasDemand()) return
    this.setStatus("live")
    this.armLiveness()
  }

  /** (Re)configure the connection URL; closes & reopens on change. */
  configure(url: string, opts?: { ticketUrl?: string; token?: string }) {
    const ticketUrl = opts?.ticketUrl ?? ""
    const token = opts?.token ?? ""
    if (
      this.url === url &&
      this.ticketUrl === ticketUrl &&
      this.token === token
    )
      return
    this.url = url
    this.ticketUrl = ticketUrl
    this.token = token
    this.gen += 1
    this.retryMs = 1000
    this.clearRetry()
    this.clearLiveness()
    this.stalled = false
    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
    this.setStatus("connecting")
    void this.open()
  }

  subscribe(sub: RealtimeSub): () => void {
    this.subs.add(sub)
    if (!this.ws) {
      this.setStatus("connecting")
      void this.open()
    }
    this.syncSubscriptions()
    this.updateDemand()
    return () => {
      this.subs.delete(sub)
      this.syncSubscriptions()
      // Last subscriber gone → this page does not need realtime any more.
      this.updateDemand()
    }
  }

  /**
   * Exchanges the access token for a single-use ticket (todo 5.8.4). The
   * ticket is opaque and dies in ~30s, so an access log that records the WS
   * URL no longer captures a reusable credential. Returns "" when issuance
   * fails — the caller then falls back to the legacy ?token= handshake.
   */
  private async fetchTicket(): Promise<string> {
    try {
      const res = await fetch(this.ticketUrl, {
        method: "POST",
        headers: { Authorization: `Bearer ${this.token}` },
      })
      if (!res.ok) return ""
      const body = (await res.json()) as { ticket?: string }
      return typeof body.ticket === "string" ? body.ticket : ""
    } catch {
      return ""
    }
  }

  private clearRetry() {
    if (this.retryTimer !== undefined) {
      window.clearTimeout(this.retryTimer)
      this.retryTimer = undefined
    }
  }

  private sendFrame(frame: Record<string, unknown>) {
    const ws = this.ws
    if (!ws || ws.readyState !== WebSocket.OPEN) return
    ws.send(JSON.stringify(frame))
  }

  /**
   * Reconciles the hub's view of this connection with the union of every
   * subscriber's interests, sending only the deltas. Called whenever the
   * subscriber set changes and after every (re)connect — realtime is
   * non-durable, so a reconnected connection must re-register everything.
   */
  private syncSubscriptions() {
    const ws = this.ws
    if (!ws || ws.readyState !== WebSocket.OPEN) return

    // Normalize the union: resource → set of events; "" = all events.
    const desired = new Map<string, Set<string>>()
    for (const s of this.subs) {
      if (!s.resource) continue
      let evs = desired.get(s.resource)
      if (!evs) {
        evs = new Set<string>()
        desired.set(s.resource, evs)
      }
      if (s.event) {
        if (!evs.has("")) evs.add(s.event)
      } else {
        evs.clear()
        evs.add("") // any subscriber wanting all events ⇒ all events
      }
    }

    // Unsubscribe resources no longer wanted by anyone.
    for (const res of [...this.subscribed.keys()]) {
      if (!desired.has(res)) {
        this.sendFrame({ op: "unsubscribe", resource: res })
        this.subscribed.delete(res)
      }
    }

    // Subscribe / adjust the rest.
    for (const [res, want] of desired) {
      const prev = this.subscribed.get(res)
      if (!prev) {
        this.subscribeResource(res, want)
        this.subscribed.set(res, new Set(want))
        continue
      }
      const prevAll = prev.has("")
      const wantAll = want.has("")
      if (prevAll === wantAll) {
        if (!wantAll) {
          // Both event-scoped: diff the specific event sets.
          for (const e of [...prev]) {
            if (!want.has(e)) {
              this.sendFrame({ op: "unsubscribe", resource: res, event: e })
              prev.delete(e)
            }
          }
          for (const e of want) {
            if (!prev.has(e))
              this.sendFrame({ op: "subscribe", resource: res, event: e })
          }
          for (const e of want) prev.add(e)
        }
        // Both "all events" → no change.
      } else if (wantAll) {
        // Was event-scoped, now all-events: a single resource subscribe covers it.
        this.sendFrame({ op: "subscribe", resource: res })
        this.subscribed.set(res, new Set([""]))
      } else {
        // Was all-events, now event-scoped: resubscribe at event granularity.
        this.sendFrame({ op: "unsubscribe", resource: res })
        for (const e of want)
          this.sendFrame({ op: "subscribe", resource: res, event: e })
        this.subscribed.set(res, new Set(want))
      }
    }
  }

  private subscribeResource(res: string, evs: Set<string>) {
    if (evs.has("")) {
      this.sendFrame({ op: "subscribe", resource: res })
    } else {
      for (const e of evs)
        this.sendFrame({ op: "subscribe", resource: res, event: e })
    }
  }

  private async open() {
    if (!this.url || this.ws) return
    const gen = this.gen
    let handshakeURL = this.url
    if (this.token) {
      const ticket = await this.fetchTicket()
      handshakeURL += ticket
        ? `?ticket=${encodeURIComponent(ticket)}`
        : `?token=${encodeURIComponent(this.token)}`
    }
    // A configure() landed while the ticket request was in flight, or another
    // attempt already opened a socket — this attempt is stale.
    if (this.ws || gen !== this.gen) return

    const ws = new WebSocket(handshakeURL)
    this.ws = ws
    // Fresh connection: the hub knows nothing about our interests yet.
    this.subscribed.clear()

    ws.onopen = () => {
      this.retryMs = 1000
      // The connection is live from the handshake; heartbeats keep it honest.
      this.markAlive()
      // Non-durable: re-register the full subscription set after (re)connect.
      this.syncSubscriptions()
    }

    ws.onmessage = (ev) => {
      this.markAlive()
      let msg: WireFrame
      try {
        msg = JSON.parse(String(ev.data)) as WireFrame
      } catch {
        return
      }
      // Transport control frames are not events: acknowledge the heartbeat and
      // stop — never fan them out (a "*" subscriber would otherwise match a
      // frame that has no resource and fire a spurious refetch).
      if (typeof msg.op === "string") {
        if (msg.op === "hb") this.sendFrame({ op: "hb_ack" })
        return
      }
      const event = msg as RealtimeMessage
      for (const s of this.subs) {
        if (s.resource !== "*" && s.resource !== event.resource) continue
        if (s.event && s.event !== event.event) continue
        try {
          s.onEvent(event)
        } catch {
          // never let a consumer error kill the message loop
        }
      }
    }

    ws.onerror = () => ws.close()

    ws.onclose = () => {
      if (this.ws !== ws) return
      this.ws = null
      this.clearLiveness()
      const hasSubs = this.subs.size > 0
      // A close caused by the watchdog keeps its own, more accurate label;
      // the retry below still runs, so the connection recovers on its own.
      // With no subscribers the socket is idle-by-design: report idle, not a
      // problem the current page should be told about.
      this.setStatus(
        this.stalled ? "stalled" : hasSubs ? "reconnecting" : "idle",
      )
      if (hasSubs) useRealtimeStore.getState().bumpAttempt()
      // Non-durable: tell every subscriber a reconnect is needed (refetch).
      for (const s of this.subs) {
        try {
          s.onReconnect?.()
        } catch {
          /* ignore */
        }
      }
      this.clearRetry()
      this.retryTimer = window.setTimeout(() => {
        this.retryTimer = undefined
        this.retryMs = Math.min(this.retryMs * 2, 15000)
        void this.open()
      }, this.retryMs)
    }
  }
}

// ── React hook ──

/**
 * Subscribes to realtime events for a resource ("module/name" or "*").
 * Returns a `tick` that increments on every matching event and on reconnect,
 * so consumers can treat it as a refetch trigger (keep polling as a backstop).
 *
 * @param resource "module/entity" (e.g. "clinic/visit") or "*" for all.
 * @param opts.event optional exact event-name filter (e.g. "completed").
 */
export function useRealtime(
  resource: string,
  opts?: { event?: string },
): number {
  const workspace = useSessionStore((s) => s.workspace)
  const token = useSessionStore((s) => s.token)
  const [tick, setTick] = useState(0)
  const tickRef = useRef(0)
  const optsRef = useRef(opts)
  optsRef.current = opts

  useEffect(() => {
    if (!resource || !workspace) return

    const proto = window.location.protocol === "https:" ? "wss:" : "ws:"
    const base = `${proto}//${window.location.host}/${workspace}/_ui`
    getClient().configure(`${base}/_ws`, {
      ticketUrl: `${base}/_ws/ticket`,
      token: token ?? "",
    })

    const bump = () => {
      tickRef.current += 1
      setTick(tickRef.current)
    }
    const unsub = getClient().subscribe({
      resource,
      event: optsRef.current?.event,
      onEvent: () => bump(),
      onReconnect: () => bump(),
    })
    return unsub
  }, [resource, workspace, token])

  return tick
}

/**
 * Imperative realtime subscription (for asset components' formspec.subscribe).
 * Configures the shared WebSocket and subscribes to a resource.
 */
export function subscribeRealtime(
  resource: string,
  onEvent: (msg: RealtimeMessage) => void,
): () => void {
  const workspace = useSessionStore.getState().workspace
  const token = useSessionStore.getState().token
  if (!resource || !workspace) return () => {}
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:"
  const base = `${proto}//${window.location.host}/${workspace}/_ui`
  getClient().configure(`${base}/_ws`, {
    ticketUrl: `${base}/_ws/ticket`,
    token: token ?? "",
  })
  return getClient().subscribe({ resource, onEvent })
}

/**
 * Test-only: drop the module-level singleton so each test starts from a clean
 * connection. Not part of the public API — app code never needs it.
 */
export function __resetRealtimeClientForTests() {
  client = null
}
