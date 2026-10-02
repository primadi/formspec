// ─── Shell Nav Links ───
//
// Collects leaf routes from the resolved menu for the bare-chrome nav (no-nav
// topbar, region fills). Kept shallow (top-level + one level) so the bare
// chrome stays light. Auth routes are excluded — the chrome renders its own
// Sign in/Sign up controls, so a menu entry pointing there would duplicate them.

import type { MenuItem } from "@/types/manifest"

export function collectPublicLinks(
  items: MenuItem[] | null | undefined,
  depth = 0,
): { label: string; route: string }[] {
  if (!items?.length) return []
  const out: { label: string; route: string }[] = []
  for (const item of items) {
    if (item.route) {
      if (item.route === "/login" || item.route === "/register") continue
      out.push({ label: item.label || item.route, route: item.route })
    } else if (item.children?.length && depth < 1) {
      out.push(...collectPublicLinks(item.children, depth + 1))
    }
  }
  return out
}
