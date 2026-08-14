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

    // Wildcard match: "billing.invoices.*"
    // Must match prefix AND the remainder must be exactly one segment
    if (perm.endsWith(".*")) {
      const prefix = perm.slice(0, -2) // remove trailing ".*"
      if (required.startsWith(prefix)) {
        const rest = required.slice(prefix.length)
        // Must start with "." AND have no further dots
        if (rest.startsWith(".") && !rest.slice(1).includes(".")) {
          return true
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
