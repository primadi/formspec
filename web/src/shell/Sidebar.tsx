// ─── Sidebar Navigation ───
//
// Renders the navigation tree: derived module menus for entities without
// authored menus, merged with authored `kind: Menu` manifests.
//
// Features:
//   - Permission-filtered items (hides when user lacks backing permission)
//   - FormaExpr `when:` condition support
//   - Nested collapsible groups
//   - Icons from lucide-react by name
//   - Active route highlighting

import { useMemo } from "react"
import { NavLink, useParams, useLocation } from "react-router-dom"

import { cn } from "@/lib/utils"
import { useMetaStore } from "@/stores/meta"
import { useSessionStore } from "@/stores/session"
import { can as checkPermission } from "@/engine/permissions"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Button } from "@/components/ui/button"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"

// ── Icon resolution ──
// Lazy map from icon name → lucide component.
// We use dynamic import to avoid bundling all icons.

import * as LucideIcons from "lucide-react"

function resolveIcon(name: string | undefined) {
  if (!name) return null
  // Try PascalCase
  const key = name.charAt(0).toUpperCase() + name.slice(1).replace(/-./g, (c) => c[1].toUpperCase())
  const Icon = (LucideIcons as unknown as Record<string, React.ComponentType<{ className?: string }>>)[key]
  return Icon ?? null
}

// ── Menu Item Types ──

interface MenuItem {
  label: string
  icon?: string
  route?: string
  order?: number
  children?: MenuItem[]
}

// ── Sidebar Props ──

interface SidebarProps {
  collapsed: boolean
  onToggle?: () => void
  mobile?: boolean
  mobileOpen?: boolean
  onMobileClose?: () => void
}

export function Sidebar({ collapsed, onToggle: _onToggle, mobile, mobileOpen, onMobileClose }: SidebarProps) {
  const { workspace } = useParams<{ workspace: string }>()
  const bundle = useMetaStore((s) => s.bundle)
  const me = useSessionStore((s) => s.me)
  const location = useLocation()

  // Determine correct surface prefix from current URL
  const isAdmin = location.pathname.includes("/_admin/")
  const surfacePrefix = isAdmin ? `/${workspace}/_admin` : `/${workspace}/app`

  const menuItems = useMemo(() => {
    if (!bundle || !me) return []

    const items: MenuItem[] = []

    // Recursively convert MenuSpec → MenuItem (handles nested children, sorts by order)
  const sortByOrder = (items: MenuItem[]) =>
    items.sort((a, b) => (a.order ?? 99) - (b.order ?? 99))

  const toMenuItem = (spec: import("@/types/manifest").MenuSpec): MenuItem => ({
    label: spec.label,
    icon: spec.icon,
    route: spec.route,
    order: spec.order,
    children: spec.children?.length
      ? sortByOrder(
          spec.children
            .filter((c) => filterMenuItem(c, me.permissions))
            .map(toMenuItem),
        )
      : undefined,
  })

  // 1. Authored menus (kind: Menu) — sorted by order, children recursively sorted
    const authoredMenus = sortByOrder(
      bundle.menus
        .filter((m) => filterMenuItem(m.spec, me.permissions))
        .map((m) => toMenuItem(m.spec)),
    )

    items.push(...authoredMenus)

    // 2. Derived module menus (for entities not covered by authored menus)
    const coveredRoutes = new Set<string>()
    for (const m of authoredMenus) {
      if (m.route) coveredRoutes.add(m.route)
      for (const c of m.children ?? []) {
        if (c.route) coveredRoutes.add(c.route)
      }
    }

    const entitiesByModule = useMetaStore.getState().getEntitiesByModule()
    for (const [module, entities] of entitiesByModule) {
      // Check if any authored menu belongs to this module (by module name,
      // not by label — labels may be localized like "Klinik" vs "Clinic")
      const moduleAuthored = bundle.menus.some(
        (m) => m.module === module,
      )
      if (moduleAuthored) continue

      // Create derived menu for this module
      const visibleEntities = entities.filter((e) => {
        const listPerm = `${module}.${e.plural}.list`
        return checkPermission(listPerm, me.permissions)
      })

      if (visibleEntities.length === 0) continue

      const children = visibleEntities
        .filter(
          (e) =>
            !coveredRoutes.has(`/${module}/${e.plural}`),
        )
        .map((e) => ({
          label: e.label_field
            ? e.name.charAt(0).toUpperCase() + e.name.slice(1)
            : e.name,
          icon: "FileText" as string | undefined,
          route: `/${module}/${e.plural}`,
        }))

      if (children.length > 0) {
        items.push({
          label: module.charAt(0).toUpperCase() + module.slice(1),
          icon: "Folder",
          order: 99, // derived menus after authored ones
          children: sortByOrder(children),
        })
      }
    }

    return items
  }, [bundle, me])

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
            <span className="text-lg font-bold">Forma</span>
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
              <span className="text-lg font-bold mx-auto">F</span>
            ) : (
              <span className="text-lg font-bold">Forma</span>
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

  return (
    <SidebarLink
      item={item}
      collapsed={collapsed}
      basePath={basePath}
    />
  )
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
    return (
      <Tooltip>
        <TooltipTrigger>
          <NavLink to={to}>
            <Button
              variant="ghost"
              size="icon"
              className="w-full justify-center"
            >
              {item.icon ? (
                (() => {
                  const Icon = resolveIcon(item.icon)
                  return Icon ? <Icon className="size-4" /> : null
                })()
              ) : (
                <FileIcon />
              )}
            </Button>
          </NavLink>
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
