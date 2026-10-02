// @vitest-environment jsdom
//
// RegionShell — the single App shell composed from chrome REGIONS
// (frontend/05-app-kinds.md §4.2). Archetypes are presets over one region map;
// `no-nav` is not "no chrome", it is "no DEFAULT fill", so an App can attach a
// topbar/sidebar/rightbar/bottombar of its own.
//
// jsdom has no layout engine, so these assert the contract (which regions are
// present, which content they carry), not pixel sizes.

import { describe, it, expect, beforeEach, beforeAll, vi } from "vitest"
import { render, screen, cleanup } from "@testing-library/react"
import { MemoryRouter, Routes, Route } from "react-router-dom"
import "@testing-library/jest-dom/vitest"

// Component regions render through AssetRenderer (dynamic import of an asset
// URL) — stub it so we can assert the region *was* filled by a component.
vi.mock("./AssetRenderer", () => ({
  AssetRenderer: ({ asset }: { asset: string }) => (
    <div data-testid="asset" data-asset={asset} />
  ),
}))

import { RegionShell } from "./RegionShell"
import { useSessionStore } from "@/stores/session"
import { usePrefsStore } from "@/stores/prefs"
import { useMetaStore } from "@/stores/meta"
import type { ChromeConfig, MetaBundle } from "@/types/manifest"

// jsdom has no matchMedia; the shell uses useMediaQuery to pick the mobile
// sidebar layout. Report "not mobile" so the desktop tree renders.
beforeAll(() => {
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia
})

function makeBundle(
  app_renderer: string,
  chrome: Partial<ChromeConfig>,
  withMenu = true,
): MetaBundle {
  return {
    app: {
      name: "storefront",
      title: "Storefront",
      root_url: "/",
      app_renderer,
      chrome: {
        brand: "show",
        nav: "none",
        auth: "none",
        footer: "show",
        breadcrumbs: "hide",
        theme_switcher: "hide",
        ...chrome,
      } as ChromeConfig,
    },
    entities: [],
    menu: withMenu
      ? [
          { label: "Katalog", route: "/listing/x" },
          { label: "Login", route: "/login" },
        ]
      : [],
  } as unknown as MetaBundle
}

function renderShell(bundle: MetaBundle, archetype?: string) {
  useMetaStore.setState({ bundle, error: null })
  return render(
    <MemoryRouter initialEntries={["/default/"]}>
      <Routes>
        <Route
          path="/:workspace/*"
          element={<RegionShell archetype={archetype} />}
        />
      </Routes>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  cleanup()
  // useResolvedMenu returns [] without a session identity — set a minimal one
  // so the menu-driven fills (sidebar/topnav) render. token stays empty, so
  // the auth area shows the anonymous Sign in/Sign up affordances.
  useSessionStore.setState({
    token: "",
    workspace: "default",
    me: { user_id: "test-user", username: "test-user", roles: [] } as never,
  })
  usePrefsStore.setState({ sidebarCollapsed: false })
})

describe("RegionShell — archetype presets", () => {
  it("no-nav: a minimal brand bar, no nav links, no auth controls", () => {
    // no-nav preset: chrome exists (just a brand bar), no navigation. This is
    // the whole correction — no-nav is not "no chrome".
    renderShell(
      makeBundle("no-nav", {
        regions: {
          topbar: "auto",
          sidebar: "none",
          rightbar: "none",
          bottombar: "none",
          footer: "auto",
        },
      }),
    )
    expect(screen.getByText("Storefront")).toBeInTheDocument()
    expect(screen.queryByText("Katalog")).not.toBeInTheDocument()
    expect(screen.queryByText("Sign in")).not.toBeInTheDocument()
  })

  it("sidebar-nav: fills the sidebar region (chrome exists by default)", () => {
    renderShell(
      makeBundle("sidebar-nav", {
        nav: "menu",
        auth: "links",
        footer: "hide",
        breadcrumbs: "show",
        theme_switcher: "show",
        regions: {
          topbar: "auto",
          sidebar: "auto",
          rightbar: "none",
          bottombar: "none",
          footer: "none",
        },
      }),
      "sidebar-nav",
    )
    // The sidebar nav is present (menu item) — proof the sidebar region filled.
    expect(screen.getByText("Katalog")).toBeInTheDocument()
  })

  it("topnav: fills the topbar region with the menu", () => {
    renderShell(
      makeBundle("topnav", {
        nav: "menu",
        auth: "links",
        footer: "hide",
        breadcrumbs: "show",
        theme_switcher: "show",
        regions: {
          topbar: "auto",
          sidebar: "none",
          rightbar: "none",
          bottombar: "none",
          footer: "none",
        },
      }),
      "topnav",
    )
    expect(screen.getByText("Katalog")).toBeInTheDocument()
    expect(screen.getByText("Sign in")).toBeInTheDocument()
  })
})

describe("RegionShell — explicit regions (§4.2)", () => {
  it("no-nav + regions.topbar: auto gives the bare App a topbar (and its session control)", () => {
    // The kafe-qr shape once the fix lands: no-nav, but a filled topbar so the
    // chrome has somewhere to host AuthArea.
    renderShell(
      makeBundle("no-nav", {
        auth: "links",
        regions: {
          topbar: "auto",
          sidebar: "none",
          rightbar: "none",
          bottombar: "none",
          footer: "auto",
        },
      }),
      "no-nav",
    )
    expect(screen.getByText("Storefront")).toBeInTheDocument()
    expect(screen.getByText("Sign in")).toBeInTheDocument()
  })

  it("renders a component ref region through AssetRenderer", () => {
    renderShell(
      makeBundle("no-nav", {
        regions: {
          topbar: "none",
          sidebar: "none",
          rightbar: "cafe/components/help",
          bottombar: "none",
          footer: "none",
        },
      }),
      "no-nav",
    )
    expect(screen.getByTestId("asset")).toHaveAttribute(
      "data-asset",
      "cafe/components/help",
    )
  })

  it("regions.footer: none removes the footer even when the legacy flag says show", () => {
    renderShell(
      makeBundle("no-nav", {
        footer: "hide",
        regions: {
          topbar: "none",
          sidebar: "none",
          rightbar: "none",
          bottombar: "none",
          footer: "none",
        },
      }),
      "no-nav",
    )
    expect(screen.queryByText(/© \d{4}/)).not.toBeInTheDocument()
  })

  it("legacy bundle (no regions field) reconstructs the preset", () => {
    // An older server ships only the booleans; the shell must not regress.
    renderShell(makeBundle("no-nav", { footer: "hide" }, true), "no-nav")
    expect(screen.getByText("Storefront")).toBeInTheDocument()
    expect(screen.queryByText(/© \d{4}/)).not.toBeInTheDocument()
  })
})
