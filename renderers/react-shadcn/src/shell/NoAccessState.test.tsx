// @vitest-environment jsdom
//
// NoAccessState — the honest in-shell state when the App has nothing the
// caller may open (kafe 10.20 / 10.22). Replaces "No entities found. Load a
// manifest to get started.", which blamed the manifest for a permission gap.

import { describe, it, expect, beforeEach, vi } from "vitest"
import { render, screen, cleanup, fireEvent } from "@testing-library/react"
import { MemoryRouter, Routes, Route } from "react-router-dom"
import "@testing-library/jest-dom/vitest"
import { NoAccessState } from "./NoAccessState"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import type { MetaBundle } from "@/types/manifest"

function renderState(appName?: string) {
  return render(
    <MemoryRouter initialEntries={["/kafe/"]}>
      <Routes>
        <Route
          path="/:workspace/*"
          element={<NoAccessState appName={appName} />}
        />
      </Routes>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  cleanup()
  useSessionStore.setState({ token: "", workspace: "kafe" })
  useMetaStore.setState({
    bundle: {
      app: { name: "kafe-qr", root_url: "/" },
    } as unknown as MetaBundle,
  })
})

describe("NoAccessState", () => {
  it("states the truth and offers Sign in to an anonymous visitor", () => {
    renderState("Kafe QR")
    expect(
      screen.getByText("Nothing here for your account"),
    ).toBeInTheDocument()
    expect(screen.queryByText(/No entities found/)).not.toBeInTheDocument()
    expect(screen.getByRole("button", { name: /Sign in/i })).toBeInTheDocument()
    expect(
      screen.queryByRole("button", { name: /Sign out/i }),
    ).not.toBeInTheDocument()
  })

  it("offers Sign out (an exit) to a signed-in session", () => {
    useSessionStore.setState({ token: "tok-123", workspace: "kafe" })
    renderState("Kafe QR")
    expect(
      screen.getByRole("button", { name: /Sign out/i }),
    ).toBeInTheDocument()
    expect(
      screen.queryByRole("button", { name: /^Sign in$/i }),
    ).not.toBeInTheDocument()
  })

  it("clears the session on Sign out", () => {
    useSessionStore.setState({ token: "tok-123", workspace: "kafe" })
    const clearSession = vi.fn()
    useSessionStore.setState({ clearSession })
    renderState()
    fireEvent.click(screen.getByRole("button", { name: /Sign out/i }))
    expect(clearSession).toHaveBeenCalled()
  })
})
