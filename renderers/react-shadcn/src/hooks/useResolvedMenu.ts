// ─── useResolvedMenu ───
//
// Shared hook that resolves the navigation menu for the current surface:
//   - `_admin`: mechanically generated per-module entity groups (no curation).
//   - App surface: the curated, already-resolved bundle.menu, filtered by its
//     `when:` condition.
//
// Who filters what — one axis each, deliberately NOT both in both places:
//   - `permissions:` is RBAC and is enforced SERVER-side in filterMenu, so an
//     item the caller may not have never arrives. The client has nothing to
//     check (and should not: a client-side RBAC check is bypassable and, worse,
//     silently disagrees with the server the moment the two drift).
//   - `when:` is a business condition and is evaluated HERE, because it may
//     depend on the clock (`today()`) — putting it in the bundle would poison
//     the ETag cache (`internal/api/meta.go` hashes the bundle body).
//
// Shared by the Sidebar (SideNavShell) and TopNavShell so both chrome variants
// render the exact same menu tree.

import { useMemo } from "react"
import { useLocation, useParams } from "react-router-dom"
import { useMetaStore } from "@/stores/meta"
import { useSessionStore } from "@/stores/session"
import { deriveMenuItems } from "@/engine/derive"
import { evalFormSpecExpr } from "@/lib/formspec-expr"
import type { MenuItem } from "@/types/manifest"
import type { MeResponse } from "@/types/manifest"

/**
 * Evaluate an authored menu item's condition (Core §4.4).
 *
 * `permissions:` is NOT checked here — the server already withheld the item.
 *
 * `when:` is a FormSpecExpr evaluated against `{ user: me }`. It is presentation
 * only, never authorization: hiding a link grants nothing, because the route and
 * the data behind it stay protected by the entity visibility filter and the
 * resource's own `required_permission`.
 *
 * FAIL-OPEN, deliberately. An expression that cannot be evaluated shows the item
 * and reports; it does not hide it. Hiding navigation because of a renderer bug
 * is indistinguishable from "this item legitimately does not exist", and since
 * `when` is not a security boundary, showing it costs nothing. `formspec check`
 * rejects the malformed shapes at deploy time, so this path should not be
 * reachable from a validated spec.
 */
export function filterMenuItem(
  item: { route?: string; when?: string },
  me: MeResponse | null,
): boolean {
  if (!item.when) return true

  const result = evalFormSpecExpr(item.when, me ? { user: me as never } : {})
  if (!result.valid) {
    console.error(
      `menu item ${item.route ?? "?"}: \`when\` could not be evaluated (${result.warnings.join("; ")}) — showing the item. Fix the expression; \`formspec check\` rejects it at deploy time.`,
    )
    return true
  }
  return Boolean(result.value)
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
      // Access Management shortcut — authored page in formspec.core
      // (kelola user, role, dan role assignment dalam satu halaman) —
      // followed by the mechanically-derived per-module entity groups.
      return [
        {
          label: "Access Management",
          icon: "Shield",
          route: "/access-management",
        },
        ...deriveMenuItems(bundle.entities),
      ]
    }

    // App surface: ONLY the authored, curated menu (Core §4.4 — App is a
    // curated cart of entities/views from its modules). Entities not wired
    // into the menu do not appear here at all — no derived fallback.
    // bundle.menu is already fully resolved server-side (adopt nodes spliced,
    // `view` leaves turned into routes, permission-unreachable items dropped);
    // the only thing left to decide here is each item's `when` condition.
    const filterTree = (list: MenuItem[]): MenuItem[] =>
      list
        .filter((item) => filterMenuItem(item, me))
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
