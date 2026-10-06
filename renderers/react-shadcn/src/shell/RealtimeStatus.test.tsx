// @vitest-environment jsdom

// ─── RealtimeStatus chrome indicator ───
//
// Non-intrusive by design: nothing renders while the connection is healthy,
// and the degraded text is a small pill (role="status"), not a modal/toast.

import { beforeEach, describe, expect, it } from "vitest"
import { render, screen, cleanup } from "@testing-library/react"
import "@testing-library/jest-dom/vitest"

import { RealtimeStatus } from "./RealtimeStatus"
import { useRealtimeStore } from "@/stores/realtime"

beforeEach(() => {
  cleanup()
  useRealtimeStore.setState({
    status: "idle",
    active: false,
    lastMessageAt: 0,
    attempt: 0,
  })
})

describe("RealtimeStatus", () => {
  it.each(["idle", "connecting", "live"] as const)(
    "renders nothing when status is %s",
    (status) => {
      useRealtimeStore.setState({ status, active: true })
      const { container } = render(<RealtimeStatus />)
      expect(container).toBeEmptyDOMElement()
    },
  )

  it.each(["stalled", "reconnecting"] as const)(
    "ignores a broken connection when the page does not use realtime (%s)",
    (status) => {
      useRealtimeStore.setState({ status, active: false, attempt: 2 })
      const { container } = render(<RealtimeStatus />)
      expect(container).toBeEmptyDOMElement()
    },
  )

  it("shows a stalled heartbeat as a non-blocking status pill", () => {
    useRealtimeStore.setState({ status: "stalled", active: true })
    render(<RealtimeStatus />)

    const pill = screen.getByRole("status")
    expect(pill).toHaveTextContent("Realtime tidak merespons")
    expect(pill).toHaveAttribute("aria-live", "polite")
    // No dialog/alert — it must not interrupt.
    expect(screen.queryByRole("alert")).toBeNull()
    expect(screen.queryByRole("dialog")).toBeNull()
  })

  it("shows reconnecting with the attempt count", () => {
    useRealtimeStore.setState({
      status: "reconnecting",
      active: true,
      attempt: 3,
    })
    render(<RealtimeStatus />)
    expect(screen.getByRole("status")).toHaveTextContent(
      "Menyambung ulang… (3)",
    )
  })
})
