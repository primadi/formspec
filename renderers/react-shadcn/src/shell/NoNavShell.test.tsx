// @vitest-environment jsdom
//
// NoNavShell — `no-nav` means truly no navigation: no nav links, no auth
// controls by default (frontend/05-app-kinds.md §4). Apps opt back in via
// the resolved chrome config (§4.1).

import { describe, it, expect, beforeEach } from "vitest"
import { render, screen, cleanup } from "@testing-library/react"
import { MemoryRouter, Routes, Route } from "react-router-dom"
import "@testing-library/jest-dom/vitest"
import { NoNavShell } from "./NoNavShell"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import type { MetaBundle } from "@/types/manifest"

function makeBundle(
  chrome: Record<string, string>,
  withMenu = true,
): MetaBundle {
  return {
    app: {
      name: "storefront",
      title: "Storefront",
      root_url: "/",
      chrome,
    },
    menu: withMenu
      ? [
          { label: "Katalog", route: "/listing/x" },
          { label: "Login", route: "/login" },
        ]
      : [],
  } as unknown as MetaBundle
}

function renderShell(bundle: MetaBundle) {
  useMetaStore.setState({ bundle })
  return render(
    <MemoryRouter initialEntries={["/default/"]}>
      <Routes>
        <Route path="/:workspace/*" element={<NoNavShell />} />
      </Routes>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  cleanup()
  useSessionStore.setState({ token: "", workspace: "default" })
})

describe("NoNavShell", () => {
  it("renders no nav links and no auth controls by default", () => {
    // Resolved no-nav defaults: nav=none, auth=none.
    renderShell(
      makeBundle({
        brand: "show",
        nav: "none",
        auth: "none",
        footer: "show",
        breadcrumbs: "hide",
        theme_switcher: "hide",
      }),
    )
    expect(screen.getByText("Storefront")).toBeInTheDocument()
    expect(screen.queryByText("Katalog")).not.toBeInTheDocument()
    expect(screen.queryByText("Sign in")).not.toBeInTheDocument()
    expect(screen.queryByText("Sign up")).not.toBeInTheDocument()
  })

  it("opts into nav links + auth controls via chrome config (registry pattern)", () => {
    renderShell(
      makeBundle({
        brand: "show",
        nav: "menu",
        auth: "links",
        footer: "show",
        breadcrumbs: "hide",
        theme_switcher: "hide",
      }),
    )
    expect(screen.getByText("Katalog")).toBeInTheDocument()
    expect(screen.getByText("Sign in")).toBeInTheDocument()
    expect(screen.getByText("Sign up")).toBeInTheDocument()
  })

  it("opts into the theme switcher via chrome.theme_switcher: show", () => {
    // no-nav default is hide — the App must opt in explicitly (§4.1).
    renderShell(
      makeBundle({
        brand: "show",
        nav: "menu",
        auth: "links",
        footer: "show",
        breadcrumbs: "hide",
        theme_switcher: "show",
      }),
    )
    expect(screen.getByTitle("Theme settings")).toBeInTheDocument()
  })

  it("excludes /login and /register menu entries from the nav", () => {
    renderShell(
      makeBundle({
        brand: "show",
        nav: "menu",
        auth: "none",
        footer: "show",
        breadcrumbs: "hide",
        theme_switcher: "hide",
      }),
    )
    // "Katalog" renders; the /login menu entry must not duplicate AuthArea.
    expect(screen.getByText("Katalog")).toBeInTheDocument()
    expect(screen.queryByText("Login")).not.toBeInTheDocument()
  })

  it("hides the brand bar via chrome.brand: hide", () => {
    renderShell(
      makeBundle({
        brand: "hide",
        nav: "none",
        auth: "none",
        footer: "show",
        breadcrumbs: "hide",
        theme_switcher: "hide",
      }),
    )
    expect(screen.queryByText("Storefront")).not.toBeInTheDocument()
  })

  it("hides the footer via chrome.footer: hide", () => {
    renderShell(
      makeBundle({
        brand: "show",
        nav: "none",
        auth: "none",
        footer: "hide",
        breadcrumbs: "hide",
        theme_switcher: "hide",
      }),
    )
    expect(screen.queryByText(/© \d{4}/)).not.toBeInTheDocument()
  })
})
