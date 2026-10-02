// ─── Region Shell ───
//
// The single App shell. It renders the chrome REGIONS resolved on the bundle
// (frontend/05-app-kinds.md §4.2): topbar, sidebar, rightbar, bottombar,
// footer — each `none` (absent), `auto` (the archetype's predefined fill) or a
// component reference (`AssetRenderer`). The page content is the implicit
// `content` region (the Router `Outlet`).
//
// Archetypes are presets over that map (see regions.ts): `sidebar-nav` is
// `no-nav` + a filled sidebar, `topnav` is `no-nav` + a filled topbar. So an
// App that wants a custom topbar AND the built-in sidebar just sets
// `regions: {topbar: <component>, sidebar: auto}` — no new archetype.
//
// The region model is also what gives a `no-nav` App a place to hang a session
// control: `no-nav` has no DEFAULT bar, but it can still fill one (kafe-qr).
//
// OverlayHost is mounted for EVERY composition (it used to be missing from the
// bare no-nav shell, so modal/drawer forms silently died there).

import { useCallback, useEffect, useMemo, useState } from "react"
import { Outlet, useLocation, useParams } from "react-router-dom"
import { AppLink as Link, AppNavLink as NavLink } from "@/lib/navigation"
import { ChevronDown, ChevronRight, Home } from "lucide-react"
import { ErrorBoundary } from "@/components/ErrorBoundary"
import { ThemeSwitcher } from "@/components/ThemeSwitcher"
import { Button } from "@/components/ui/button"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import { TooltipProvider } from "@/components/ui/tooltip"
import { useSurface } from "@/hooks/useSurface"
import { useMetaStore } from "@/stores/meta"
import { usePrefsStore } from "@/stores/prefs"
import { useMediaQuery } from "@/hooks/useMediaQuery"
import { useResolvedMenu, linkHref } from "@/hooks/useResolvedMenu"
import { useRouteIdentityStore } from "@/stores/routeIdentity"
import { resolveIcon } from "@/lib/icon-resolver"
import { cn } from "@/lib/utils"
import type { MenuItem } from "@/types/manifest"
import { Sidebar } from "./Sidebar"
import { AuthArea } from "./AuthArea"
import { AssetRenderer } from "./AssetRenderer"
import { OverlayHost } from "./OverlayHost"
import { buildBreadcrumbs } from "./breadcrumbs"
import { collectPublicLinks } from "./navLinks"
import {
  effectiveRegions,
  isRegionFilled,
  regionComponent,
  regionIsAuto,
} from "./regions"

// ── Top-nav pieces (shared by the topbar fill) ──

function NavIcon({ name }: { name?: string }) {
  const Icon = name ? resolveIcon(name) : null
  return Icon ? <Icon className="size-4 shrink-0" /> : null
}

function TopNavLink({
  item,
  basePath,
  active,
}: {
  item: MenuItem
  basePath: string
  active?: boolean
}) {
  return (
    <NavLink
      to={linkHref(item, basePath)}
      className={({ isActive }) =>
        cn(
          "inline-flex items-center gap-1.5 rounded-md px-3 py-2 text-sm font-medium transition-colors",
          (active ?? isActive)
            ? "bg-muted text-foreground"
            : "text-muted-foreground hover:bg-muted/50 hover:text-foreground",
        )
      }
    >
      <NavIcon name={item.icon} />
      <span>{item.label}</span>
    </NavLink>
  )
}

function TopNavGroup({ item, basePath }: { item: MenuItem; basePath: string }) {
  const [open, setOpen] = useState(false)
  const children = item.children ?? []
  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="inline-flex items-center gap-1.5 rounded-md px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-muted/50 hover:text-foreground"
      >
        <NavIcon name={item.icon} />
        <span>{item.label}</span>
        <ChevronDown className="size-3.5" />
      </button>
      {open && (
        <>
          <div className="fixed inset-0 z-30" onClick={() => setOpen(false)} />
          <div className="absolute left-0 z-40 mt-1 min-w-52 rounded-lg border bg-popover p-1.5 shadow-md">
            {children.map((child, idx) => (
              <NavLink
                key={`${child.label}-${idx}`}
                to={linkHref(child, basePath)}
                onClick={() => setOpen(false)}
                className={({ isActive }) =>
                  cn(
                    "flex items-center gap-2 rounded-md px-2.5 py-2 text-sm transition-colors",
                    isActive
                      ? "bg-muted text-foreground"
                      : "text-muted-foreground hover:bg-muted/50 hover:text-foreground",
                  )
                }
              >
                <NavIcon name={child.icon} />
                <span>{child.label}</span>
              </NavLink>
            ))}
          </div>
        </>
      )}
    </div>
  )
}

// ── Breadcrumb row ──

function BreadcrumbRow({ withHome }: { withHome?: boolean }) {
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const location = useLocation()
  const { surfacePrefix } = useSurface()
  const entities = useMetaStore((s) => s.bundle?.entities) ?? []
  const routeIdentity = useRouteIdentityStore((s) =>
    s.getIdentity(location.pathname),
  )
  const crumbs = buildBreadcrumbs(
    location.pathname,
    workspace,
    entities,
    routeIdentity,
  )
  return (
    <Breadcrumb>
      <BreadcrumbList>
        <BreadcrumbItem>
          <BreadcrumbLink render={<Link to={surfacePrefix} />}>
            {withHome ? <Home className="size-3.5" /> : "Home"}
          </BreadcrumbLink>
        </BreadcrumbItem>
        {crumbs.map((crumb) => (
          <BreadcrumbItem key={crumb.href}>
            <BreadcrumbSeparator />
            {crumb.isLast ? (
              <BreadcrumbPage>{crumb.label}</BreadcrumbPage>
            ) : (
              <BreadcrumbLink render={<Link to={crumb.href} />}>
                {crumb.label}
              </BreadcrumbLink>
            )}
          </BreadcrumbItem>
        ))}
      </BreadcrumbList>
    </Breadcrumb>
  )
}

// ── Topbar fill (auto) ──
//
// Three shapes, chosen by whether a sidebar is filled and which archetype the
// preset came from:
//   - sidebar filled  → header row: mobile toggle + breadcrumbs + theme + auth
//   - topnav          → menu bar (groups) + breadcrumb row below
//   - otherwise       → brand bar (+ nav links when chrome.nav: menu)

function TopbarAutoFill({
  hasSidebar,
  archetype,
  onMobileToggle,
}: {
  hasSidebar: boolean
  archetype: string
  onMobileToggle: () => void
}) {
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const location = useLocation()
  const bundle = useMetaStore((s) => s.bundle)
  const chrome = bundle?.app.chrome
  const { items, basePath } = useResolvedMenu()
  const rootUrl = bundle?.app.root_url ?? "/"
  const base = `/${workspace}${rootUrl}`.replace(/\/+$/, "")
  const appTitle = bundle?.app.title ?? bundle?.app.name ?? "FormSpec"
  const LogoIcon = bundle?.app.logo ? resolveIcon(bundle.app.logo) : null
  const showBrand = chrome?.brand !== "hide"
  const showBreadcrumbs = chrome?.breadcrumbs !== "hide"
  const showTheme = chrome?.theme_switcher !== "hide"
  const showNavLinks = chrome?.nav === "menu"
  const links = showNavLinks ? collectPublicLinks(bundle?.menu) : []

  if (hasSidebar) {
    return (
      <header className="flex h-14 items-center gap-4 border-b px-4">
        <Button
          variant="ghost"
          size="icon"
          className="md:hidden"
          onClick={onMobileToggle}
        >
          <ChevronRight className="size-4" />
        </Button>
        {showBreadcrumbs && <BreadcrumbRow withHome />}
        <div className="flex-1" />
        {showTheme && <ThemeSwitcher />}
        <AuthArea mode={chrome?.auth} />
      </header>
    )
  }

  if (archetype === "topnav") {
    return (
      <>
        <header className="sticky top-0 z-40 border-b bg-background/95 backdrop-blur supports-backdrop-filter:bg-background/60">
          <div className="flex h-14 items-center gap-2 px-4">
            {showBrand && (
              <Link
                to={base}
                className="mr-2 flex items-center gap-2 font-semibold"
              >
                {LogoIcon && <LogoIcon className="size-5" />}
                <span className="text-lg tracking-tight">{appTitle}</span>
              </Link>
            )}
            <nav className="hidden items-center gap-1 md:flex">
              {items.map((item, idx) =>
                item.children?.length ? (
                  <TopNavGroup
                    key={`${item.label}-${idx}`}
                    item={item}
                    basePath={basePath}
                  />
                ) : (
                  <TopNavLink
                    key={`${item.label}-${idx}`}
                    item={item}
                    basePath={basePath}
                  />
                ),
              )}
            </nav>
            <div className="flex-1" />
            {showTheme && <ThemeSwitcher />}
            <AuthArea mode={chrome?.auth} />
          </div>
        </header>
        {showBreadcrumbs && (
          <div className="border-b px-4 py-2">
            <BreadcrumbRow />
          </div>
        )}
      </>
    )
  }

  // Bare brand bar (no-nav preset with a filled topbar).
  return (
    <header className="sticky top-0 z-40 border-b bg-background/95 backdrop-blur supports-backdrop-filter:bg-background/60">
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4">
        {showBrand && (
          <Link to={base} className="flex items-center gap-2 font-semibold">
            {LogoIcon && <LogoIcon className="size-5" />}
            <span className="text-lg tracking-tight">{appTitle}</span>
          </Link>
        )}
        <div className="flex items-center gap-4">
          {links.length > 0 && (
            <nav className="hidden items-center gap-6 text-sm md:flex">
              {links.map((link) => {
                const href = `${base}${link.route}`.replace(/\/+$/, "")
                const path = location.pathname
                const active =
                  link.route === "/"
                    ? path === base || path === `${base}/`
                    : path === href || path.startsWith(`${href}/`)
                return (
                  <Link
                    key={link.route}
                    to={href}
                    className={
                      active
                        ? "font-medium text-foreground"
                        : "text-muted-foreground transition-colors hover:text-foreground"
                    }
                  >
                    {link.label}
                  </Link>
                )
              })}
            </nav>
          )}
          {showTheme && <ThemeSwitcher />}
          <AuthArea mode={chrome?.auth} />
        </div>
      </div>
    </header>
  )
}

// ── Region slot ──

function RegionSlot({
  regions,
  region,
  auto,
  enclosed,
}: {
  regions: Record<string, string>
  region: "topbar" | "sidebar" | "rightbar" | "bottombar" | "footer"
  auto?: React.ReactNode
  enclosed?: boolean
}) {
  const component = regionComponent(regions as never, region)
  if (component) {
    return (
      <div data-region={region} className="contents">
        <AssetRenderer asset={component} />
      </div>
    )
  }
  if (!isRegionFilled(regions as never, region)) return null
  if (regionIsAuto(regions as never, region) && auto) {
    void enclosed
    return <>{auto}</>
  }
  return null
}

// ── Shell ──

export function RegionShell({ archetype }: { archetype?: string }) {
  const bundle = useMetaStore((s) => s.bundle)
  const chrome = bundle?.app.chrome
  const arch = archetype ?? bundle?.app.app_renderer ?? "sidebar-nav"
  const regions = useMemo(() => effectiveRegions(chrome, arch), [chrome, arch])

  const hasSidebar = isRegionFilled(regions, "sidebar")
  const hasRightbar = isRegionFilled(regions, "rightbar")
  const enclosed = hasSidebar || hasRightbar

  const isMobile = useMediaQuery("(max-width: 767px)")
  const location = useLocation()
  const sidebarCollapsed = usePrefsStore((s) => s.sidebarCollapsed)
  const toggleSidebar = usePrefsStore((s) => s.toggleSidebar)
  const [mobileSidebarOpen, setMobileSidebarOpen] = useState(false)

  useEffect(() => {
    setMobileSidebarOpen(false)
  }, [location.pathname])

  const handleMobileToggle = useCallback(
    () => setMobileSidebarOpen((p) => !p),
    [],
  )
  const handleMobileClose = useCallback(() => setMobileSidebarOpen(false), [])

  const sidebarNode = hasSidebar ? (
    <>
      {isMobile && mobileSidebarOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50"
          onClick={handleMobileClose}
        />
      )}
      <Sidebar
        collapsed={isMobile ? false : sidebarCollapsed}
        onToggle={isMobile ? handleMobileToggle : toggleSidebar}
        mobile={isMobile}
        mobileOpen={mobileSidebarOpen}
        onMobileClose={handleMobileClose}
      />
    </>
  ) : null

  const topbarNode = (
    <RegionSlot
      regions={regions}
      region="topbar"
      auto={
        <TopbarAutoFill
          hasSidebar={hasSidebar}
          archetype={arch}
          onMobileToggle={handleMobileToggle}
        />
      }
    />
  )

  const footerNode = (
    <RegionSlot
      regions={regions}
      region="footer"
      auto={
        <footer className="border-t py-6">
          <div className="mx-auto max-w-6xl px-4 text-center text-sm text-muted-foreground">
            © {new Date().getFullYear()} {bundle?.app.name ?? "FormSpec"}
          </div>
        </footer>
      }
    />
  )

  const bottombarNode = <RegionSlot regions={regions} region="bottombar" />
  const rightbarNode = <RegionSlot regions={regions} region="rightbar" />

  const content = (
    <ErrorBoundary>
      <Outlet />
    </ErrorBoundary>
  )

  // Layout follows which bar regions are filled, matching the three historic
  // chrome shapes: sidebar+header (enclosed), top navigation (full-width,
  // h-screen), and bare content (centered, min-h-screen — the no-nav shape).
  if (enclosed) {
    return (
      <TooltipProvider>
        <div className="flex h-screen overflow-hidden">
          {sidebarNode}
          <div className="flex flex-1 flex-col overflow-hidden">
            {topbarNode}
            <main className="flex-1 overflow-auto p-6 [view-transition-name:page-content]">
              {content}
            </main>
            {bottombarNode}
            {footerNode}
          </div>
          {rightbarNode}
        </div>
        <OverlayHost />
      </TooltipProvider>
    )
  }

  if (arch === "topnav") {
    return (
      <TooltipProvider>
        <div className="flex h-screen flex-col overflow-hidden">
          {topbarNode}
          <main className="flex-1 overflow-auto p-6 [view-transition-name:page-content]">
            {content}
          </main>
          {bottombarNode}
          {footerNode}
        </div>
        <OverlayHost />
      </TooltipProvider>
    )
  }

  // Bare / no-nav shape: centered content column, topbar (brand bar) above.
  return (
    <TooltipProvider>
      <div className="flex min-h-screen flex-col">
        {topbarNode}
        <main className="flex-1 [view-transition-name:page-content]">
          <div className="mx-auto max-w-6xl px-4 py-6">{content}</div>
        </main>
        {bottombarNode}
        {footerNode}
      </div>
      <OverlayHost />
    </TooltipProvider>
  )
}
