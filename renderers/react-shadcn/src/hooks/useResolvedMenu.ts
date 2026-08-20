// ─── useResolvedMenu ───
//
// Shared hook that resolves the navigation menu for the current surface:
//   - `_admin`: mechanically generated per-module entity groups (no curation).
//   - App surface: the curated, already-resolved bundle.menu, filtered by
//     permission (`permissions:` on menu items) and `when:` conditions.
//
// Shared by the Sidebar (AppShell) and TopNavShell so both chrome variants
// render the exact same menu tree.

import { useMemo } from "react"
import { useLocation, useParams } from "react-router-dom"
import { useMetaStore } from "@/stores/meta"
import { useSessionStore } from "@/stores/session"
import { can as checkPermission } from "@/engine/permissions"
import { titleCase } from "@/lib/utils"
import type { MenuItem } from "@/types/manifest"

/** Permission filter for an authored menu item (Core §4.4). */
export function filterMenuItem(
  item: { route?: string; when?: string; permissions?: string[] },
  permissions: string[],
): boolean {
  if ("permissions" in item && item.permissions?.length) {
    if (!item.permissions.some((p) => checkPermission(p, permissions))) {
      return false
    }
  }
  return true
}

/** Build an absolute href for a menu item relative to the surface base path. */
export function linkHref(item: MenuItem, basePath: string): string {
  return item.route
    ? item.route.startsWith("/")
      ? `${basePath}${item.route}`
      : `${basePath}/${item.route}`
    : "#"
}

export interface ResolvedMenu {
  items: MenuItem[]
  isAdmin: boolean
  /** Absolute prefix of the current surface (/_admin or root_url-based). */
  basePath: string
}

export function useResolvedMenu(): ResolvedMenu {
  const { workspace } = useParams<{ workspace: string }>()
  const bundle = useMetaStore((s) => s.bundle)
  const me = useSessionStore((s) => s.me)
  const location = useLocation()

  const adminPrefix = `/${workspace}/_admin`
  const isAdmin =
    location.pathname === adminPrefix ||
    location.pathname.startsWith(`${adminPrefix}/`)
  const basePath = isAdmin
    ? `/${workspace}/_admin`
    : `/${workspace}${bundle?.app.root_url ?? "/app"}`

  const items = useMemo(() => {
    if (!bundle || !me) return []

    // `_admin` isn't scoped to any App and can't be curated (Core §4.4) — it
    // always shows every module's entities, mechanically generated, with no
    // authored menu and no per-entity permission filtering (the binary
    // `_admin.access` gate already covers "may see this at all").
    if (isAdmin) {
      const entitiesByModule = useMetaStore.getState().getEntitiesByModule()
      const derived: MenuItem[] = []
      for (const [module, entities] of entitiesByModule) {
        if (entities.length === 0) continue
        const children = entities.map((e) => ({
          label: e.label_field ? titleCase(e.name) : e.name,
          icon: "FileText" as string | undefined,
          route: `/${module}/${e.plural}`,
        }))
        derived.push({ label: titleCase(module), icon: "Folder", children })
      }
      return derived
    }

    // App surface: ONLY the authored, curated menu (Core §4.4 — App is a
    // curated cart of entities/views from its modules). Entities not wired
    // into the menu do not appear here at all — no derived fallback.
    // bundle.menu is already fully resolved server-side; only permission/when
    // filtering happens here.
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

  return { items, isAdmin, basePath }
}
