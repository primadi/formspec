import type { ActionSummary } from "@/types/manifest"

// ─── Permission Gate ───
//
// Port of internal/auth/auth.go Identity.HasPermission to TypeScript.
// Used by the renderer's Permission Gate (design doc §5.8) for client-side
// UI filtering. This is UX-only — the server always enforces permissions.
//
// Matching rules (parity with Go):
//   - Exact match: "billing.invoices.list" == "billing.invoices.list"
//   - Wildcard: "billing.invoices.*" matches "billing.invoices.list"
//   - Super-wildcard: "*" matches everything (dev mode)
//   - "public" always returns true (anonymous access)
//   - Empty string always returns true

/**
 * Check if the given permissions list contains the required permission.
 *
 * @param required - The permission string to check (e.g. "billing.invoices.list")
 * @param permissions - The list of granted permissions
 * @returns true if the permission is granted
 *
 * @example
 * ```ts
 * can("billing.invoices.list", ["billing.invoices.*"])      // → true
 * can("billing.invoices.list", ["*"])                       // → true
 * can("billing.invoices.list", ["billing.customers.list"])  // → false
 * can("public", [])                                          // → true
 * ```
 */
export function can(required: string, permissions: string[]): boolean {
  // Public/anonymous access
  if (required === "" || required === "public") {
    return true
  }

  for (const perm of permissions) {
    // Super-wildcard (dev mode)
    if (perm === "*") {
      return true
    }

    // Exact match
    if (perm === required) {
      return true
    }

    // Wildcard match — two shapes, matching Go's Identity.HasPermission:
    //   "billing.invoices.*" → the remainder must be exactly ONE segment
    //                          (one action under this entity)
    //   "billing.*"          → any depth under the module (entity.action)
    if (perm.endsWith(".*")) {
      const prefix = perm.slice(0, -2) // remove trailing ".*"
      if (required.startsWith(prefix)) {
        const rest = required.slice(prefix.length)
        if (rest.startsWith(".")) {
          // Module-level wildcard ("billing.*", no dot in the prefix) matches
          // any entity.action under that module. Omitting this branch made the
          // renderer hide buttons for callers the SERVER would allow — a
          // parity break the module-level grant exposed.
          if (!prefix.includes(".")) {
            return true
          }
          // Entity-level wildcard ("billing.invoices.*") → exactly one more
          // segment (the action).
          if (!rest.slice(1).includes(".")) {
            return true
          }
        }
      }
    }
  }

  return false
}

/**
 * Check if a permission string is well-formed.
 * Valid format: "{module}.{entity}.{action}" (3 segments)
 * Wildcard: "{module}.{entity}.*" or "*"
 */
export function isValidPermissionFormat(perm: string): boolean {
  if (perm === "*" || perm === "" || perm === "public") return true

  const parts = perm.split(".")

  // Wildcard: module.entity.*
  if (perm.endsWith(".*") && parts.length === 3) return true

  // Exact: module.entity.action
  if (parts.length === 3 && !parts.some((p) => p === "")) return true

  return false
}

/**
 * Qualify a module-relative permission string with the module prefix.
 * Same logic as internal/ui/meta.go qualifyPerm().
 *
 * - "visits.list" → "clinic.visits.list" (if module is "clinic")
 * - Already qualified "clinic.visits.list" → unchanged
 * - "*" → unchanged
 */
export function qualifyPerm(module: string, perm: string): string {
  if (perm === "*" || module === "") return perm
  if ((perm.match(/\./g) || []).length >= 2) return perm
  return `${module}.${perm}`
}

/**
 * The resource action a UI action actually performs. The two vocabularies
 * differ on purpose (`internal/api/descriptor.go` StandardRESTActions): the UI
 * shows "View"/"Edit", the resource demands `.view`/`.update`.
 *
 * Spelling the permission out from the UI word produced `${...}.edit`, which
 * NEVER exists — so `manajer`, who holds `.update`, lost the Edit button that
 * the server would have accepted. Same bug class as the derived route issue
 * (kafe 10.23/10.19), opposite direction.
 */
export function resourceAction(action: string): string {
  switch (action) {
    case "view":
      return "find"
    case "edit":
      return "update"
    default:
      return action
  }
}

/**
 * Resolve the permission an entity action requires, following the same rule
 * the server uses to fill `ActionSummary.permission` (internal/ui/meta.go
 * buildEntitySchema): an explicit `required_permission` (module-qualified when
 * relative) wins, otherwise it is derived from the RESOURCE action name as
 * `{module}.{plural}.{action}`.
 *
 * Kept in one place because several render sites were each spelling the
 * derived form out by hand — and that form is WRONG for actions that declare
 * an explicit permission, which is exactly the case the bundle's `permission`
 * field exists to convey.
 */
export function entityActionPermission(
  entity: { module: string; plural: string; actions?: ActionSummary[] },
  action: string,
): string {
  const resource = resourceAction(action)
  // Look the permission up under both spellings: a custom action is declared
  // in the bundle under its own name, and an authored manifest may name either.
  const declared =
    entity.actions?.find((a) => a.name === action)?.permission ??
    entity.actions?.find((a) => a.name === resource)?.permission
  if (declared) return declared
  return `${entity.module}.${entity.plural}.${resource}`
}

/**
 * Whether the caller may perform an entity action. UX-only — the server
 * enforces authoritatively; this exists so a button the caller cannot use is
 * not shown at all (rather than shown and rejected with a toast).
 *
 * `entity.authorized_actions` (resolved server-side for THIS caller) is the
 * authority when present: it covers the public-App grant, which the caller's
 * own permission list cannot express — a guest holds no permissions at all yet
 * may still `list` and `create`. Falling back to `me.permissions` keeps older
 * servers working.
 *
 * A null/undefined identity fails closed.
 */
export function canDoEntityAction(
  me: { permissions: string[] } | null | undefined,
  entity: {
    module: string
    plural: string
    actions?: ActionSummary[]
    authorized_actions?: string[]
  },
  action: string,
): boolean {
  if (!me) return false
  const resource = resourceAction(action)
  if (entity.authorized_actions) {
    return entity.authorized_actions.includes(resource)
  }
  return can(entityActionPermission(entity, action), me.permissions)
}
