// ─── Lifecycle Pattern Helpers ───
//
// Implements Frontend §1.7 lifecycle patterns for entity CRUD.
//
// Patterns:
//   - plain_crud:     Save button only (submit action disabled in manifest)
//   - two_step_autosave: Auto-save debounced + Submit button (default)
//   - two_step_manual: Save Draft + Submit buttons
//   - one_step:       Create-Submit button (create-submit action exists)
//
// `characteristic: reference` → no New/Delete buttons (Configuration pattern)

import type {
  EntitySchema,
  Lifecycle as LifecyclePattern,
} from "@/types/manifest"

/** One state-machine transition the caller can trigger from the current state. */
export interface AvailableTransition {
  /**
   * The action name (`via`). EMPTY for a transition declared without `via` —
   * those carry no action, no route and no button (owner decision C,
   * docs_internal/plan/via-sebagai-action-penuh.md), so they are filtered out
   * before reaching the renderer. Kept on the type because the target state is
   * still meaningful to callers that drive the transition directly.
   */
  action: string
  /** Target state (`to`) — what a PATCH must set when there is no action route. */
  to: string
  label: string
  style?: string
  confirm?: string
}

export interface LifecycleActions {
  /** The primary action type */
  pattern: LifecyclePattern
  /** Whether to show a Save / Save Draft button */
  hasSave: boolean
  /** Whether to show a Submit button */
  hasSubmit: boolean
  /** Whether to show a Delete button (reference entities hide it) */
  hasDelete: boolean
  /** Whether to show a New / Create button */
  hasCreate: boolean
  /** Whether auto-save mode (debounced save on field change) */
  autoSave: boolean
  /** Whether to use create-submit (single click create + submit) */
  quickSubmit: boolean
}

/**
 * Determine the lifecycle actions for an entity.
 */
export function getLifecycle(entity: EntitySchema): LifecycleActions {
  const isReference = entity.characteristic === "reference"
  const isSummary = entity.characteristic === "summary"

  switch (entity.lifecycle) {
    case "plain_crud":
    default:
      return {
        pattern: "plain_crud",
        hasSave: true,
        hasSubmit: false,
        hasDelete: !isReference && !isSummary,
        hasCreate: !isReference && !isSummary,
        autoSave: false,
        quickSubmit: false,
      }

    case "two_step_manual":
      return {
        pattern: "two_step_manual",
        hasSave: true,
        hasSubmit: true,
        hasDelete: !isReference && !isSummary,
        hasCreate: !isReference && !isSummary,
        autoSave: false,
        quickSubmit: false,
      }

    case "two_step_autosave":
      return {
        pattern: "two_step_autosave",
        hasSave: true,
        hasSubmit: true,
        hasDelete: !isReference && !isSummary,
        hasCreate: !isReference && !isSummary,
        autoSave: true,
        quickSubmit: entity.has_quick_submit ?? false,
      }
  }
}

/**
 * Get state machine transitions available from the current state.
 *
 * Transitions declared WITHOUT `via` are excluded (owner decision C): they have
 * no action, so there is no route to POST and no button to label. Rely on an
 * explicit filter rather than the accident that `canDoEntityAction(me, entity,
 * "")` happens to be false — those transitions were hidden for the wrong
 * reason, and the same accident left duplicate empty React keys behind.
 *
 * `label` prefers `ui.button_label`, then the transition's own `description`,
 * then the humanised `via`.
 */
export function getAvailableTransitions(
  entity: EntitySchema,
  currentState: string,
): AvailableTransition[] {
  if (!entity.state_machine) return []

  const transitions = entity.state_machine.transitions.filter(
    (t) => t.via && (t.from.includes(currentState) || t.from.includes("*")),
  )

  return transitions.map((t) => {
    const action = entity.actions.find((a) => a.name === t.via)
    return {
      action: t.via,
      to: t.to,
      label:
        action?.ui?.button_label ??
        t.description ??
        t.via.charAt(0).toUpperCase() + t.via.slice(1),
      style: action?.ui?.style,
      confirm: action?.ui?.confirm,
    }
  })
}
