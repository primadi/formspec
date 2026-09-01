// @vitest-environment jsdom
//
// AuthArea — shared auth controls driven by the resolved chrome.auth value
// (frontend/05-app-kinds.md §4.1).

import { describe, it, expect, beforeEach } from "vitest"
import { render, screen, cleanup } from "@testing-library/react"
import { MemoryRouter, Routes, Route } from "react-router-dom"
import "@testing-library/jest-dom/vitest"
import { AuthArea } from "./AuthArea"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import type { MetaBundle } from "@/types/manifest"

function makeBundle(chrome?: Record<string, string>): MetaBundle {
  return {
    app: {
      name: "testapp",
      root_url: "/",
      chrome,
    },
  } as unknown as MetaBundle
}

function renderAuthArea(mode: string | undefined) {
  return render(
    <MemoryRouter initialEntries={["/default/"]}>
      <Routes>
        <Route path="/:workspace/*" element={<AuthArea mode={mode} />} />
      </Routes>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  cleanup()
  useSessionStore.setState({ token: "", workspace: "default" })
  useMetaStore.setState({ bundle: makeBundle() })
})

describe("AuthArea", () => {
  it('renders nothing for mode "none"', () => {
    renderAuthArea("none")
    expect(screen.queryByText("Sign in")).not.toBeInTheDocument()
    expect(screen.queryByText("Sign up")).not.toBeInTheDocument()
  })

  it("renders nothing before the bundle loads (mode undefined)", () => {
    renderAuthArea(undefined)
    expect(screen.queryByText("Sign in")).not.toBeInTheDocument()
  })

  it('renders Sign in + Sign up links for mode "links" (anon)', () => {
    renderAuthArea("links")
    const signIn = screen.getByText("Sign in")
    expect(signIn).toHaveAttribute("href", "/default/login")
    expect(screen.getByText("Sign up")).toHaveAttribute(
      "href",
      "/default/register",
    )
  })

  it('renders a single Sign in button for mode "button" (anon)', () => {
    renderAuthArea("button")
    expect(screen.getByText("Sign in")).toBeInTheDocument()
    expect(screen.queryByText("Sign up")).not.toBeInTheDocument()
  })

  it("renders user menu for a signed-in session", () => {
    useSessionStore.setState({ token: "tok-123", workspace: "default" })
    renderAuthArea("links")
    expect(screen.getByLabelText("User menu")).toBeInTheDocument()
    expect(screen.queryByText("Sign in")).not.toBeInTheDocument()
  })
})
