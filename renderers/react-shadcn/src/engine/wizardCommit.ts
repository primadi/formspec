// ─── Wizard commit & launcher rules ───
//
// One place that answers two questions about a `kind: Wizard`:
//
//   1. HOW it commits (used by WizardRenderer). A wizard bound to an `action`
//      must not assume that action has its own route: `POST /{entity}/{id}/{action}`
//      exists ONLY for actions with an `impl` (internal/api/generator.go skips
//      `Impl == nil`). A transition that merely names a `via` has no route — the
//      PATCH path applies it, matching the transition by (from, to). This is the
//      same rule DetailPage already uses (kafe 10.48): read `has_route` off the
//      bundle instead of probing the endpoint.
//
//      The old code POSTed `spec.action` RAW to a client whose prefix is
//      `/{ws}/_ui/entity`, producing `POST …/_ui/entity/close-shift` — a path
//      that cannot exist (`{module}/{entity}/{id}/{action}` is required), and it
//      never carried the record id. So a wizard committing a via-only transition
//      could never work.
//
//   2. WHICH wizard launches a transition (used by DetailPage). A transition
//      whose `via` matches a wizard's `action` on the same entity should open
//      that wizard rather than PATCH the state directly — otherwise the wizard's
//      inputs (e.g. counted_cash/note/supervisor_id) are never collected, and
//      computed fields are evaluated from empty data.

import type {
  ActionSummary,
  EntitySchema,
  Entry,
  WizardSpec,
} from "@/types/manifest"
import { resolveEntityRef } from "@/engine/entityRef"

/** The call a wizard's final submit must make. */
export type WizardCommit =
  | { kind: "action"; path: string; body: Record<string, unknown> }
  | { kind: "patch"; path: string; body: Record<string, unknown> }
  | { kind: "error"; message: string }

/**
 * Whether `action` is applied by its own route. Mirrors DetailPage's rule
 * (kafe 10.48): `has_route` from the bundle decides; when the flag is absent
 * (older server) fall back to "declared means it has a route".
 */
export function usesActionRoute(
  actions: ActionSummary[] | undefined,
  action: string,
): boolean {
  const declared = actions?.find((a) => a.name === action)
  return declared?.has_route ?? declared !== undefined
}

/** Keep only the collected values that are real fields of the entity. */
function entityFieldValues(
  entity: EntitySchema,
  collected: Record<string, unknown>,
): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(collected)) {
    if (entity.fields?.some((f) => f.name === key)) out[key] = value
  }
  return out
}

/**
 * Decide the commit call for a wizard step submit.
 *
 * `collected` is the accumulated step data. Only entity fields are forwarded —
 * the same filter the entity-create branch uses, so a step's UI-only keys do not
 * become a rejected payload.
 */
export function resolveWizardCommit(opts: {
  entity: EntitySchema
  action: string
  id?: string
  collected: Record<string, unknown>
}): WizardCommit {
  const { entity, action, id, collected } = opts
  const base = `${entity.module}/${entity.name}`

  if (usesActionRoute(entity.actions, action)) {
    if (!id) {
      return {
        kind: "error",
        message: `Cannot run "${action}": this wizard commits an existing record, so it needs one to open on (missing ?id=)`,
      }
    }
    return {
      kind: "action",
      path: `${base}/${encodeURIComponent(id)}/${action}`,
      body: collected,
    }
  }

  // No route: the only path that applies a via-only transition is PATCH, setting
  // the state field. The target state is authoritative in the manifest, so the
  // wizard never guesses it.
  const transition = entity.state_machine?.transitions.find(
    (t) => t.via === action,
  )
  const stateField = entity.state_machine?.field
  if (!transition || !stateField) {
    return {
      kind: "error",
      message: `Cannot commit "${action}" on ${base}: it has no action route and matches no state-machine transition`,
    }
  }
  if (!id) {
    return {
      kind: "error",
      message: `Cannot close the record: "${action}" applies to an existing record, so the wizard needs one to open on (missing ?id=)`,
    }
  }
  return {
    kind: "patch",
    path: `${base}/${encodeURIComponent(id)}`,
    body: {
      [stateField]: transition.to,
      ...entityFieldValues(entity, collected),
    },
  }
}

/**
 * The wizard that launches `action` on `entity`, or undefined.
 *
 * Matching is by the same edge the wizard declares to bind itself to its host:
 * `spec.action === transition.via` AND `spec.entity` resolving to this entity.
 * A wizard without `action` (plain entity-create) never matches.
 */
export function findWizardForTransition(
  wizards: Entry<WizardSpec>[] | undefined,
  entity: EntitySchema,
  action: string,
): Entry<WizardSpec> | undefined {
  if (!action) return undefined
  return wizards?.find((w) => {
    if (!w.spec?.action || w.spec.action !== action) return false
    if (!w.spec.entity) return false
    const [mod, name] = resolveEntityRef(w.spec.entity, w.module)
    return mod === entity.module && name === entity.name
  })
}
