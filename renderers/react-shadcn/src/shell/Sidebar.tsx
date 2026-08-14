// ─── Sidebar Navigation ───
//
// Renders the navigation tree: bundle.menu is already fully resolved
// server-side (Core §4.4 — App.spec.menu, adopt nodes spliced, `view`
// leaves turned into concrete `route`), merged with derived module menus
// for entities not covered by any authored menu item.
//
// Features:
//   - Permission-filtered items (hides when user lacks backing permission)
//   - FormSpecExpr `when:` condition support
//   - Nested collapsible groups
//   - Icons from lucide-react by name
//   - Active route highlighting

import { useMemo } from "react"
import { NavLink, useParams, useLocation } from "react-router-dom"

import { cn, titleCase } from "@/lib/utils"
import { useMetaStore } from "@/stores/meta"
import { useSessionStore } from "@/stores/session"
import { can as checkPermission } from "@/engine/permissions"
import type { MenuItem } from "@/types/manifest"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Button, buttonVariants } from "@/components/ui/button"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"

// ── Icon resolution ──
// Shared resolveIcon from lib/icon-resolver (same as ActionIcon).

import { resolveIcon } from "@/lib/icon-resolver"

// ── Logo mark (Spec Stack) ──

function LogoMark({ size = 24 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 64 64"
      fill="none"
      aria-hidden
      className="shrink-0"
    >
      <rect width="64" height="64" rx="16" fill="url(#fslogo)" />
      <rect x="13" y="15" width="38" height="8" rx="4" fill="#fff" />
      <rect
        x="13"
        y="28"
        width="28"
        height="8"
        rx="4"
        fill="#fff"
        fillOpacity="0.85"
      />
      <rect
        x="13"
        y="41"
        width="18"
        height="8"
        rx="4"
        fill="#fff"
        fillOpacity="0.70"
      />
      <defs>
        <linearGradient
          id="fslogo"
          x1="0"
          y1="0"
          x2="64"
          y2="64"
          gradientUnits="userSpaceOnUse"
        >
          <stop stopColor="#6366f1" />
          <stop offset="1" stopColor="#10b981" />
        </linearGradient>
      </defs>
    </svg>
  )
}

// ── Sidebar Props ──

interface SidebarProps {
  collapsed: boolean
  onToggle?: () => void
  mobile?: boolean
  mobileOpen?: boolean
  onMobileClose?: () => void
}

export function Sidebar({
  collapsed,
  onToggle: _onToggle,
  mobile,
  mobileOpen,
  onMobileClose,
}: SidebarProps) {
  const { workspace } = useParams<{ workspace: string }>()
  const bundle = useMetaStore((s) => s.bundle)
  const me = useSessionStore((s) => s.me)
  const location = useLocation()

  // Determine correct surface prefix from current URL. The App surface uses
  // this App's own root_url (Core §4.4) — not a hardcoded "/app" — since a
  // workspace can resolve to more than one App, each mounted at its own
  // root_url under the shared /{ws}/app/* renderer SPA.
  // Matches the route table in App.tsx ("/:workspace/_admin/*") exactly —
  // a plain `.includes("/_admin/")` misses the bare root path
  // (`/{workspace}/_admin`, no trailing segment), which is exactly what's
  // hit on first load before any redirect appends a subpath.
  const adminPrefix = `/${workspace}/_admin`
  const isAdmin =
    location.pathname === adminPrefix ||
    location.pathname.startsWith(`${adminPrefix}/`)
  const surfacePrefix = isAdmin
    ? `/${workspace}/_admin`
    : `/${workspace}${bundle?.app.root_url ?? "/app"}`

  const menuItems = useMemo(() => {
    if (!bundle || !me) return []

    // `_admin` isn't scoped to any App and can't be curated (Core §4.4) — it
    // always shows every module's entities, mechanically generated, with no
    // authored menu and no per-entity permission filtering (the binary
    // `_admin.access` gate already covers "may see this at all").
    if (isAdmin) {
      const entitiesByModule = useMetaStore.getState().getEntitiesByModule()
      const items: MenuItem[] = []
      for (const [module, entities] of entitiesByModule) {
        if (entities.length === 0) continue
        const children = entities.map((e) => ({
          label: e.label_field ? titleCase(e.name) : e.name,
          icon: "FileText" as string | undefined,
          route: `/${module}/${e.plural}`,
        }))
        items.push({
          label: titleCase(module),
          icon: "Folder",
          children,
        })
      }
      return items
    }

    // App surface: ONLY the authored, curated menu (Core §4.4 — App is a
    // curated cart of entities/views from its modules). Entities not wired
    // into the menu do not appear here at all — no derived fallback.
    // bundle.menu is already fully resolved server-side (adopt nodes
    // spliced, `view` leaves turned into `route`); only permission/when
    // filtering and ordering happen here.
    const filterTree = (list: MenuItem[]): MenuItem[] =>
      list
        .filter((item) => filterMenuItem(item, me.permissions))
        .map((item) => ({
          ...item,
          children: item.children?.length
            ? filterTree(item.children)
            : undefined,
        }))

    return filterTree(bundle.menu ?? [])
  }, [bundle, me, isAdmin])

  if (!bundle || !me) return null

  return (
    <>
      {/* Mobile overlay sidebar */}
      {mobile && (
        <aside
          className={cn(
            "fixed inset-y-0 left-0 z-50 flex flex-col border-r bg-sidebar shadow-2xl transition-transform duration-300",
            "w-60",
            mobileOpen ? "translate-x-0" : "-translate-x-full",
          )}
        >
          {/* Logo + close button */}
          <div className="flex h-14 items-center justify-between border-b px-4">
            <div className="flex items-center gap-2">
              <LogoMark size={22} />
              <span className="text-lg font-bold">FormSpec</span>
            </div>
            <Button
              variant="ghost"
              size="icon"
              onClick={onMobileClose}
              aria-label="Close sidebar"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                className="size-4"
              >
                <path d="M18 6 6 18" />
                <path d="m6 6 12 12" />
              </svg>
            </Button>
          </div>

          {/* Navigation */}
          <ScrollArea className="flex-1 py-2">
            <nav className="space-y-1 px-2">
              {menuItems.map((item, idx) => (
                <SidebarGroup
                  key={`${item.label}-${idx}`}
                  item={item}
                  collapsed={false}
                  basePath={surfacePrefix}
                />
              ))}
            </nav>
          </ScrollArea>
        </aside>
      )}

      {/* Desktop static sidebar */}
      {!mobile && (
        <aside
          className={cn(
            "flex flex-col border-r bg-sidebar transition-all duration-200",
            collapsed ? "w-14" : "w-60",
          )}
        >
          {/* Logo */}
          <div className="flex h-14 items-center border-b px-4">
            {collapsed ? (
              <LogoMark size={24} />
            ) : (
              <div className="flex items-center gap-2">
                <LogoMark size={24} />
                <span className="text-lg font-bold">FormSpec</span>
              </div>
            )}
          </div>

          {/* Navigation */}
          <ScrollArea className="flex-1 py-2">
            <nav className="space-y-1 px-2">
              {menuItems.map((item, idx) => (
                <SidebarGroup
                  key={`${item.label}-${idx}`}
                  item={item}
                  collapsed={collapsed}
                  basePath={surfacePrefix}
                />
              ))}
            </nav>
          </ScrollArea>
        </aside>
      )}
    </>
  )
}

// ── Helpers ──

function linkHref(item: MenuItem, basePath: string): string {
  return item.route
    ? item.route.startsWith("/")
      ? `${basePath}${item.route}`
      : `${basePath}/${item.route}`
    : "#"
}

function renderIcon(name?: string) {
  if (!name) return <FileIcon />
  const Icon = resolveIcon(name)
  return Icon ? <Icon className="size-4 shrink-0" /> : <FileIcon />
}

// ── Sidebar Group (with children) ──

function SidebarSubGroup({
  item,
  collapsed,
  basePath,
}: {
  item: MenuItem
  collapsed: boolean
  basePath: string
}) {
  if (collapsed) {
    return (
      <div className="space-y-1">
        {item.children?.length ? (
          item.children.map((child, idx) => (
            <SidebarLink
              key={`${child.label}-${idx}`}
              item={child}
              collapsed={true}
              basePath={basePath}
            />
          ))
        ) : (
          <SidebarLink item={item} collapsed basePath={basePath} />
        )}
      </div>
    )
  }

  return (
    <div className="ml-1">
      <SidebarLink item={item} collapsed={false} basePath={basePath} />
      {item.children?.length ? (
        <div className="ml-3 border-l pl-2 space-y-0.5">
          {item.children.map((child, idx) => (
            <SidebarLink
              key={`${child.label}-${idx}`}
              item={child}
              collapsed={false}
              basePath={basePath}
            />
          ))}
        </div>
      ) : null}
    </div>
  )
}

// ── Sidebar Group (with children) — root-level module group ──

function SidebarGroup({
  item,
  collapsed,
  basePath,
}: {
  item: MenuItem
  collapsed: boolean
  basePath: string
}) {
  if (item.children?.length) {
    const hasRoute = !!item.route
    const groupLabel = hasRoute ? (
      <NavLink
        to={linkHref(item, basePath)}
        className={({ isActive }) =>
          cn(
            "flex items-center gap-2 px-3 py-1 text-xs font-medium uppercase tracking-wider transition-colors",
            isActive
              ? "text-sidebar-primary"
              : "text-sidebar-foreground/60 hover:text-sidebar-foreground",
          )
        }
      >
        {item.icon && renderIcon(item.icon)}
        {item.label}
      </NavLink>
    ) : (
      <p className="px-3 text-xs font-medium text-sidebar-foreground/60 uppercase tracking-wider">
        {item.label}
      </p>
    )

    return (
      <div>
        {collapsed ? (
          <div className="space-y-1">
            {item.children.map((child, idx) =>
              child.children?.length ? (
                <SidebarSubGroup
                  key={`${child.label}-${idx}`}
                  item={child}
                  collapsed={true}
                  basePath={basePath}
                />
              ) : (
                <SidebarLink
                  key={`${child.label}-${idx}`}
                  item={child}
                  collapsed={true}
                  basePath={basePath}
                />
              ),
            )}
          </div>
        ) : (
          <div className="mb-1">
            {groupLabel}
            <div className="mt-1 space-y-0.5">
              {item.children.map((child, idx) =>
                child.children?.length ? (
                  <SidebarSubGroup
                    key={`${child.label}-${idx}`}
                    item={child}
                    collapsed={false}
                    basePath={basePath}
                  />
                ) : (
                  <SidebarLink
                    key={`${child.label}-${idx}`}
                    item={child}
                    collapsed={false}
                    basePath={basePath}
                  />
                ),
              )}
            </div>
          </div>
        )}
      </div>
    )
  }

  return <SidebarLink item={item} collapsed={collapsed} basePath={basePath} />
}

// ── Sidebar Link (single item) ──

function SidebarLink({
  item,
  collapsed,
  basePath,
}: {
  item: MenuItem
  collapsed: boolean
  basePath: string
}) {
  const to = item.route ? linkHref(item, basePath) : "#"

  if (collapsed) {
    // Single real anchor element doing triple duty (nav link, tooltip
    // trigger, button styling) via base-ui's `render` composition — nesting
    // separate <a>/<button> elements here (as before) produces invalid HTML
    // (`<button><a><button>`) that browsers resolve with an extra
    // hover/activate step, requiring two clicks to actually navigate.
    return (
      <Tooltip>
        <TooltipTrigger
          render={
            <NavLink
              to={to}
              className={cn(
                buttonVariants({ variant: "ghost", size: "icon" }),
                "w-full justify-center",
              )}
            />
          }
        >
          {item.icon ? (
            (() => {
              const Icon = resolveIcon(item.icon)
              return Icon ? <Icon className="size-4" /> : null
            })()
          ) : (
            <FileIcon />
          )}
        </TooltipTrigger>
        <TooltipContent side="right">
          <p>{item.label}</p>
        </TooltipContent>
      </Tooltip>
    )
  }

  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        cn(
          "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
          isActive
            ? "bg-sidebar-accent text-sidebar-accent-foreground"
            : "text-sidebar-foreground/70 hover:bg-sidebar-accent/50 hover:text-sidebar-foreground",
        )
      }
    >
      {item.icon ? (
        (() => {
          const Icon = resolveIcon(item.icon)
          return Icon ? <Icon className="size-4 shrink-0" /> : <FileIcon />
        })()
      ) : (
        <FileIcon />
      )}
      <span className="truncate">{item.label}</span>
    </NavLink>
  )
}

// ── Helpers ──

function filterMenuItem(
  item: { route?: string; when?: string; permissions?: string[] },
  permissions: string[],
): boolean {
  // Permission check for authored menu items
  if ("permissions" in item && item.permissions?.length) {
    if (!item.permissions.some((p) => checkPermission(p, permissions))) {
      return false
    }
  }
  return true
}

function FileIcon() {
  const Icon = resolveIcon("FileText")
  return Icon ? <Icon className="size-4" /> : <div className="size-4" />
}
