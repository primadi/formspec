// ─── Entity Reference Resolution ───
//
// A manifest's `entity:` field is either module-local ("setting") or
// cross-module ("clinic.setting"), resolved relative to the referencing
// manifest's own module — mirrors internal/ui/registry.go's
// resolveEntityRef on the Go side.
//
// A bare ref MUST fall back to the referencing manifest's module, not an
// empty string: the meta store's entity map is keyed "module/name", so
// getEntity("", name) can never match anything and silently resolves to no
// entity — the exact bug this helper exists to prevent from recurring.

export function resolveEntityRef(
  ref: string,
  defaultModule: string,
): [string, string] {
  // Split at the LAST dot so dotted module names (e.g. "formspec.core.role"
  // → module "formspec.core", entity "role") resolve correctly. Entity
  // names don't contain dots; module names may (namespaced modules).
  const i = ref.lastIndexOf(".")
  return i > 0 ? [ref.slice(0, i), ref.slice(i + 1)] : [defaultModule, ref]
}
