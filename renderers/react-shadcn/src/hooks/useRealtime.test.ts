// @vitest-environment jsdom

// ─── RealtimeClient heartbeat & liveness ───
//
// The transport owns ONE heartbeat per connection (= per session/tab). These
// tests pin the two contract points the chrome indicator depends on:
//   1. server `{op:"hb"}` frames are acknowledged and never fanned out to
//      subscribers (a `"*"` subscriber would otherwise match a resource-less
//      frame and refetch spuriously);
//   2. a socket that goes silent is reported as `stalled` (not silently
//      "still open") and the status recovers to `live` when frames resume.

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { subscribeRealtime, __resetRealtimeClientForTests } from "./useRealtime"
import { useRealtimeStore } from "@/stores/realtime"
import { useSessionStore } from "@/stores/session"

class FakeWebSocket {
  static instances: FakeWebSocket[] = []
  static OPEN = 1

  readyState = 0
  sent: string[] = []
  onopen: (() => void) | null = null
  onmessage: ((ev: { data: string }) => void) | null = null
  onerror: (() => void) | null = null
  onclose: (() => void) | null = null
  url: string

  constructor(url: string) {
    this.url = url
    FakeWebSocket.instances.push(this)
  }

  send(data: string) {
    this.sent.push(data)
  }

  close() {
    this.readyState = 3
    this.onclose?.()
  }

  // ── test helpers ──
  accept() {
    this.readyState = FakeWebSocket.OPEN
    this.onopen?.()
  }
  deliver(frame: unknown) {
    this.onmessage?.({ data: JSON.stringify(frame) })
  }
}

function lastSocket(): FakeWebSocket {
  const ws = FakeWebSocket.instances.at(-1)
  if (!ws) throw new Error("no websocket was opened")
  return ws
}

beforeEach(() => {
  vi.useFakeTimers()
  FakeWebSocket.instances = []
  vi.stubGlobal("WebSocket", FakeWebSocket)
  __resetRealtimeClientForTests()
  useRealtimeStore.setState({
    status: "idle",
    active: false,
    lastMessageAt: 0,
    attempt: 0,
  })
  // Empty token: `open()` connects synchronously (no ticket round-trip), which
  // keeps these tests about liveness, not about handshake auth (5.8.4).
  useSessionStore.setState({ workspace: "acme", token: "" })
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  __resetRealtimeClientForTests()
})

function connect(resource = "clinic/visit") {
  const received: unknown[] = []
  const unsub = subscribeRealtime(resource, (msg) => received.push(msg))
  const ws = lastSocket()
  ws.accept()
  return { ws, received, unsub }
}

describe("realtime heartbeat", () => {
  it("acknowledges an hb frame without fanning it out to subscribers", () => {
    const { ws, received } = connect("*")

    ws.deliver({ op: "hb", ts: 1759718400123 })

    expect(ws.sent).toContain(JSON.stringify({ op: "hb_ack" }))
    expect(received).toHaveLength(0)
    expect(useRealtimeStore.getState().status).toBe("live")
    expect(useRealtimeStore.getState().active).toBe(true)
  })

  it("still fans out real events", () => {
    const { ws, received } = connect("clinic/visit")

    ws.deliver({
      event: "created",
      resource: "clinic/visit",
      payload: { id: "v1" },
      emitted_at: "2026-10-06T00:00:00Z",
    })

    expect(received).toHaveLength(1)
    expect(received[0]).toMatchObject({
      event: "created",
      resource: "clinic/visit",
    })
  })

  it("reports stalled when the heartbeat goes silent, and recovers on the next frame", () => {
    const { ws } = connect("clinic/visit")
    expect(useRealtimeStore.getState().status).toBe("live")

    // No frames at all — past the client's stall budget.
    vi.advanceTimersByTime(46000)
    expect(useRealtimeStore.getState().status).toBe("stalled")

    // The watchdog closed the dead socket; a frame on the replacement proves
    // recovery, and the pill disappears (status back to live).
    const next = lastSocket()
    next.accept()
    next.deliver({ op: "hb", ts: 1 })
    expect(useRealtimeStore.getState().status).toBe("live")
    void ws
  })

  it("keeps a live socket alive across many heartbeats", () => {
    const { ws } = connect("clinic/visit")
    for (let i = 0; i < 4; i++) {
      vi.advanceTimersByTime(25000)
      ws.deliver({ op: "hb", ts: i })
    }
    expect(useRealtimeStore.getState().status).toBe("live")
    expect(FakeWebSocket.instances).toHaveLength(1)
  })

  it("ignores a broken connection once the page stops wanting realtime", () => {
    const { ws, unsub } = connect("clinic/visit")
    expect(useRealtimeStore.getState().active).toBe(true)

    // Navigate away: the page's last subscriber unmounts.
    unsub()
    expect(useRealtimeStore.getState().active).toBe(false)
    expect(useRealtimeStore.getState().status).toBe("idle")

    // The socket dies later — nothing on this page wants events, so no status
    // change is reported and no watchdog timer is running.
    ws.close()
    expect(useRealtimeStore.getState().status).toBe("idle")
    expect(useRealtimeStore.getState().active).toBe(false)
  })

  it("does not arm the watchdog while no subscriber wants realtime", () => {
    const { ws, unsub } = connect("clinic/visit")
    unsub()

    // Even though the socket is still open and silent, silence is not a fault
    // here: without demand the client must not report `stalled`.
    vi.advanceTimersByTime(120000)
    expect(useRealtimeStore.getState().status).toBe("idle")
    void ws
  })
})
