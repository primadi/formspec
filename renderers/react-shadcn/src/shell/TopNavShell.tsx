// ─── Top-Nav Shell ───
//
// Full chrome with top navigation (frontend/05-app-kinds.md §2): brand +
// horizontal nav (top-level items; groups open a dropdown), breadcrumb row,
// theme switcher + avatar, then the page content via Outlet. No left sidebar.
//
// Shares the resolved menu with the Sidebar via useResolvedMenu, so both
// chrome variants render the same permission-filtered tree.

import { useState } from "react"
import { Link, NavLink, Outlet, useParams } from "react-router-dom"
import { ChevronDown, Menu } from "lucide-react"
import { useSurface } from "@/hooks/useSurface"
import { useMediaQuery } from "@/hooks/useMediaQuery"
import { useResolvedMenu, linkHref } from "@/hooks/useResolvedMenu"
import { resolveIcon } from "@/lib/icon-resolver"
import { cn } from "@/lib/utils"
import type { MenuItem } from "@/types/manifest"
import { ErrorBoundary } from "@/components/ErrorBoundary"
import { ThemeSwitcher } from "@/components/ThemeSwitcher"
import { OverlayHost } from "./OverlayHost"
import { LogoutButton } from "./LogoutButton"
import { Button } from "@/components/ui/button"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { TooltipProvider } from "@/components/ui/tooltip"

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

// ── Group item → dropdown ──

function TopNavGroup({ item, basePath }: { item: MenuItem; basePath: string }) {
  const [open, setOpen] = useState(false)
  const children = item.children ?? []
  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className={cn(
          "inline-flex items-center gap-1.5 rounded-md px-3 py-2 text-sm font-medium transition-colors",
          "text-muted-foreground hover:bg-muted/50 hover:text-foreground",
        )}
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

// ── Shell ──

export function TopNavShell() {
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const { surfacePrefix } = useSurface()
  const locationPath = window.location.pathname
  const isMobile = useMediaQuery("(max-width: 767px)")
  const [mobileOpen, setMobileOpen] = useState(false)
  const { items, basePath } = useResolvedMenu()

  // Breadcrumbs from the current path (same pattern as AppShell).
  const pathParts = locationPath.split("/").filter(Boolean).slice(1)
  const breadcrumbs = pathParts.map((part, idx) => {
    const href = `/${workspace}/${pathParts.slice(0, idx + 1).join("/")}`
    const label = part
      .replace(/[-_]/g, " ")
      .replace(/\b\w/g, (c) => c.toUpperCase())
    return { label, href, isLast: idx === pathParts.length - 1 }
  })

  return (
    <TooltipProvider>
      <div className="flex h-screen flex-col overflow-hidden">
        {/* Header */}
        <header className="sticky top-0 z-40 border-b bg-background/95 backdrop-blur supports-backdrop-filter:bg-background/60">
          <div className="flex h-14 items-center gap-2 px-4">
            {/* Mobile hamburger */}
            {isMobile && (
              <Button
                variant="ghost"
                size="icon"
                className="md:hidden"
                onClick={() => setMobileOpen((o) => !o)}
              >
                <Menu className="size-4" />
              </Button>
            )}
            {/* Brand */}
            <Link
              to={surfacePrefix}
              className="mr-4 flex items-center gap-2 font-semibold"
            >
              <span className="text-lg tracking-tight">FormSpec</span>
            </Link>

            {/* Horizontal nav (desktop) */}
            {!isMobile && (
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
            )}

            <div className="flex-1" />
            <ThemeSwitcher />
            <LogoutButton />
            <Avatar className="size-8">
              <AvatarFallback className="text-xs">
                {workspace?.charAt(0).toUpperCase() ?? "F"}
              </AvatarFallback>
            </Avatar>
          </div>
        </header>

        {/* Mobile nav drawer */}
        {isMobile && mobileOpen && (
          <div className="border-b bg-background px-4 py-2">
            <nav className="flex flex-col gap-1">
              {items.map((item, idx) => (
                <div key={`${item.label}-${idx}`} className="flex flex-col">
                  <TopNavLink item={item} basePath={basePath} />
                  {item.children?.map((child, ci) => (
                    <NavLink
                      key={`${child.label}-${ci}`}
                      to={linkHref(child, basePath)}
                      onClick={() => setMobileOpen(false)}
                      className="ml-4 flex items-center gap-2 rounded-md px-3 py-1.5 text-sm text-muted-foreground hover:bg-muted/50 hover:text-foreground"
                    >
                      <NavIcon name={child.icon} />
                      <span>{child.label}</span>
                    </NavLink>
                  ))}
                </div>
              ))}
            </nav>
          </div>
        )}

        {/* Breadcrumb row */}
        <div className="border-b px-4 py-2">
          <Breadcrumb>
            <BreadcrumbList>
              <BreadcrumbItem>
                <BreadcrumbLink render={<Link to={surfacePrefix} />}>
                  Home
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
        </div>

        {/* Page content */}
        <main className="flex-1 overflow-auto p-6">
          <ErrorBoundary>
            <Outlet />
          </ErrorBoundary>
        </main>

        {/* Overlay host for modal/drawer forms */}
        <OverlayHost />
      </div>
    </TooltipProvider>
  )
}
