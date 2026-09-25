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

import { useEffect, useRef, useState } from "react"

import { useSessionStore } from "@/stores/session"
import type { RealtimeMessage } from "@/types/events"

interface RealtimeSub {
  resource: string // "module/entity" or "*"
  event?: string
  onEvent: (msg: RealtimeMessage) => void
  onReconnect?: () => void
}

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
    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
    void this.open()
  }

  subscribe(sub: RealtimeSub): () => void {
    this.subs.add(sub)
    if (!this.ws) void this.open()
    this.syncSubscriptions()
    return () => {
      this.subs.delete(sub)
      this.syncSubscriptions()
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
      // Non-durable: re-register the full subscription set after (re)connect.
      this.syncSubscriptions()
    }

    ws.onmessage = (ev) => {
      let msg: RealtimeMessage
      try {
        msg = JSON.parse(String(ev.data)) as RealtimeMessage
      } catch {
        return
      }
      for (const s of this.subs) {
        if (s.resource !== "*" && s.resource !== msg.resource) continue
        if (s.event && s.event !== msg.event) continue
        try {
          s.onEvent(msg)
        } catch {
          // never let a consumer error kill the message loop
        }
      }
    }

    ws.onerror = () => ws.close()

    ws.onclose = () => {
      if (this.ws !== ws) return
      this.ws = null
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
