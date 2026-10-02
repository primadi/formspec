// ─── Landing Entity Selection ───
//
// Which entity the shell may land on, for the surface index when the App has
// no authored home page and no resolved menu (App.tsx `DefaultRedirect`).
//
// History: the redirect used to take the first non-summary entity REGARDLESS
// of permission. `bundle.entities` is sorted module→name server-side
// (internal/entity/registry.go), so an anonymous visitor to the public
// `kafe-qr` App landed on `cafe-master/dining-table` — whose anonymous grant
// is `find` only, so the derived LIST route was never registered (allowsRoute,
// shell/router.tsx) and the visitor got "Page not found" (kafe 10.20).
//
// The fix keeps the "first entity" shortcut but only among entities whose list
// route actually exists for this caller.

import type { EntitySchema } from "@/types/manifest"

/**
 * Whether this caller can actually open the entity's derived list route.
 *
 * Mirrors `allowsRoute(entity, "list")` in shell/router.tsx — the route
 * builder registers the list only when the caller holds `list` (the resource's
 * `view`). Summary entities have no CRUD landing of their own and are skipped.
 *
 * `authorized_actions` absent means "not resolved" (older server): keep the
 * previous behaviour rather than guessing.
 */
export function canLandOnList(entity: EntitySchema): boolean {
  if (entity.characteristic === "summary") return false
  // An entity the App does not expose has no derived list route to land on
  // (plan registered-views.md): landing there would land on a 404.
  if (entity.routable === false) return false
  if (!entity.authorized_actions) return true
  return entity.authorized_actions.includes("list")
}

/** First entity with a usable list landing, or undefined when there is none. */
export function pickLandingEntity(
  entities: EntitySchema[] | null | undefined,
): EntitySchema | undefined {
  return entities?.find(canLandOnList)
}
