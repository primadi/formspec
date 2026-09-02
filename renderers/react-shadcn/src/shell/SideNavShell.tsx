// ─── Side-nav Shell ───
//
// Main application layout: sidebar + header/breadcrumb + content area.
//
// Renders inside a BrowserRouter context via Outlet.
// Wraps children with TooltipProvider for sidebar tooltips.

import { Outlet, useParams, useLocation, Link } from "react-router-dom"
import { useSurface } from "@/hooks/useSurface"
import { useMetaStore } from "@/stores/meta"
import { ChevronLeft, ChevronRight, Menu, Home } from "lucide-react"
import { usePrefsStore } from "@/stores/prefs"
import { useMediaQuery } from "@/hooks/useMediaQuery"
import { ErrorBoundary } from "@/components/ErrorBoundary"
import { ThemeSwitcher } from "@/components/ThemeSwitcher"
import { Sidebar } from "./Sidebar"
import { AuthArea } from "./AuthArea"
import { useState, useCallback, useEffect } from "react"
import { OverlayHost } from "./OverlayHost"
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

export function SideNavShell() {
  const { workspace } = useParams<{ workspace: string }>()
  const location = useLocation()
  const { surfacePrefix } = useSurface()
  // Resolved chrome composition (frontend/05-app-kinds.md §4.1) — final
  // values from the meta API; undefined only before the bundle loads.
  const chrome = useMetaStore((s) => s.bundle?.app.chrome)
  const sidebarCollapsed = usePrefsStore((s) => s.sidebarCollapsed)
  const toggleSidebar = usePrefsStore((s) => s.toggleSidebar)

  const isMobile = useMediaQuery("(max-width: 767px)")
  const [mobileSidebarOpen, setMobileSidebarOpen] = useState(false)

  // Close mobile sidebar on route change
  useEffect(() => {
    setMobileSidebarOpen(false)
  }, [location.pathname])

  const handleMobileToggle = useCallback(() => {
    setMobileSidebarOpen((prev) => !prev)
  }, [])

  const handleMobileClose = useCallback(() => {
    setMobileSidebarOpen(false)
  }, [])

  // Build breadcrumbs from current path
  const pathParts = location.pathname.split("/").filter(Boolean).slice(1) // remove workspace

  const breadcrumbs = pathParts.map((part, idx) => {
    const href = `/${workspace}/${pathParts.slice(0, idx + 1).join("/")}`
    const label = part
      .replace(/[-_]/g, " ")
      .replace(/\b\w/g, (c) => c.toUpperCase())
    return { label, href, isLast: idx === pathParts.length - 1 }
  })

  return (
    <TooltipProvider>
      <div className="flex h-screen overflow-hidden">
        {/* Sidebar — overlay on mobile, static on desktop */}
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

        {/* Main content */}
        <div className="flex flex-1 flex-col overflow-hidden">
          {/* Header */}
          <header className="flex h-14 items-center gap-4 border-b px-4">
            {/* Mobile menu toggle */}
            <Button
              variant="ghost"
              size="icon"
              className="md:hidden"
              onClick={handleMobileToggle}
            >
              <Menu className="size-4" />
            </Button>

            {/* Sidebar toggle (desktop) */}
            <Button
              variant="ghost"
              size="icon"
              className="hidden md:inline-flex"
              onClick={toggleSidebar}
            >
              {sidebarCollapsed ? (
                <ChevronRight className="size-4" />
              ) : (
                <ChevronLeft className="size-4" />
              )}
            </Button>

            {/* Breadcrumb — hidden via explicit chrome.breadcrumbs: hide */}
            {chrome?.breadcrumbs !== "hide" && (
              <Breadcrumb>
                <BreadcrumbList>
                  <BreadcrumbItem>
                    <BreadcrumbLink render={<Link to={surfacePrefix} />}>
                      <Home className="size-3.5" />
                    </BreadcrumbLink>
                  </BreadcrumbItem>
                  {breadcrumbs.map((crumb, _idx) => (
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
            )}

            {/* Spacer */}
            <div className="flex-1" />

            {/* Theme switcher — hidden via explicit chrome.theme_switcher: hide */}
            {chrome?.theme_switcher !== "hide" && <ThemeSwitcher />}

            {/* Auth controls (resolved chrome.auth) — single user identity */}
            <AuthArea mode={chrome?.auth} />
          </header>

          {/* Page content */}
          <main className="flex-1 overflow-auto p-6">
            <ErrorBoundary>
              <Outlet />
            </ErrorBoundary>
          </main>
        </div>

        {/* Overlay host for modal/drawer forms */}
        <OverlayHost />
      </div>
    </TooltipProvider>
  )
}
