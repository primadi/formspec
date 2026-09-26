import type { Field } from "@/types/manifest"

/** Who supplies a natural key — mirrors `spec.NaturalKeyEntry` in `pkg/spec/entity.go`. */
export type NaturalKeyEntry =
  | "auto_generated"
  | "user_entry"
  | "auto_generated_if_empty"

/**
 * The entry mode in effect for a field, applying the same convention as the Go
 * validator (`spec.ResolveNaturalKeyEntry`) so the two never disagree:
 *
 *   - declared explicitly   → that mode;
 *   - no `natural_key_rule` → user_entry (nothing to generate with);
 *   - `strategy: custom`    → user_entry (framework never auto-generates);
 *   - `strategy: sequence`  → auto_generated_if_empty (mint unless supplied).
 *
 * The convention is why every manifest written before the property existed keeps
 * its meaning, so the renderer must apply it too rather than assuming one mode.
 */
export function resolveNaturalKeyEntry(
  field: Field | undefined,
): NaturalKeyEntry | undefined {
  if (!field || field.natural_key !== true) return undefined
  if (field.natural_key_entry) return field.natural_key_entry
  if (!field.natural_key_rule || field.natural_key_rule.strategy === "custom") {
    return "user_entry"
  }
  return "auto_generated_if_empty"
}

/**
 * Whether the engine mints this field's value on insert, so a form must not
 * demand it from the user.
 *
 * A generated key is created by `EntityStore.generateNaturalKeys` BEFORE
 * `validateRequired` runs, so the server's own required-check is satisfied
 * without any user input. "The row must carry a value" is not the same claim as
 * "the user must type it", and a Create form that refused to submit until the
 * user typed a server-generated number would be the second claim.
 */
export function isServerMintedKey(field: Field | undefined): boolean {
  const entry = resolveNaturalKeyEntry(field)
  return entry === "auto_generated" || entry === "auto_generated_if_empty"
}

/**
 * Whether the field is an input at all.
 *
 * `auto_generated` means the engine is the only author — the key is not
 * something a person supplies, so rendering an input for it would offer the user
 * a value the server discards. `user_entry` and `auto_generated_if_empty` are
 * both inputs (the latter is one the engine pre-fills when left blank).
 *
 * Every field that is NOT a natural key is an input; this only ever excludes the
 * one mode that has an author other than the user.
 */
export function isUserEnterableKey(field: Field | undefined): boolean {
  if (!field) return false
  if (field.natural_key !== true) return true
  return resolveNaturalKeyEntry(field) !== "auto_generated"
}

/**
 * The user-facing presence requirement for a field: declared required AND not
 * satisfied by server-side generation. Read by both the zod validation schema
 * and the form's required indicator, so the two cannot disagree about whether
 * the user owes a value.
 */
export function fieldIsUserRequired(field: Field | undefined): boolean {
  if (!field) return false
  return !!field.required && !isServerMintedKey(field)
}
