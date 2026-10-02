// ─── useResolvedMenu ───
//
// Shared hook that resolves the navigation menu for the current App surface:
// the curated, already-resolved bundle.menu, filtered by its `when:` condition.
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
// Shared by the Sidebar and the topbar/RegionShell fills so every chrome
// variant renders the exact same menu tree.

import { useMemo } from "react"
import { useParams } from "react-router-dom"
import { useMetaStore } from "@/stores/meta"
import { useSessionStore } from "@/stores/session"
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
  /** Absolute prefix of the current App surface (its own root_url). */
  basePath: string
}

export function useResolvedMenu(): ResolvedMenu {
  const { workspace } = useParams<{ workspace: string }>()
  const bundle = useMetaStore((s) => s.bundle)
  const me = useSessionStore((s) => s.me)
  const basePath = `/${workspace}${bundle?.app.root_url ?? "/app"}`.replace(
    /\/+$/,
    "",
  )

  const items = useMemo(() => {
    if (!bundle || !me) return []

    // ONLY the authored, curated menu (Core §4.4 — App is a curated cart of
    // entities/views from its modules). Entities not wired into the menu do
    // not appear here at all — no derived fallback. The old `_admin` branch
    // (every module's entities, mechanically generated) is gone with the
    // surface (plan app-scoped-login.md D4).
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
  }, [bundle, me])

  return { items, basePath }
}
