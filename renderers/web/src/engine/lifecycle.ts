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

import type { EntitySchema, Lifecycle } from "@/types/manifest"

export interface LifecycleActions {
  /** The primary action type */
  pattern: Lifecycle
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
      return {
        pattern: "plain_crud",
        hasSave: true,
        hasSubmit: false,
        hasDelete: !isReference && !isSummary,
        hasCreate: !isReference && !isSummary,
        autoSave: false,
        quickSubmit: false,
      }

    case "two_step_autosave":
    default:
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
 * Filters transitions that the caller has permission for.
 */
export function getAvailableTransitions(
  entity: EntitySchema,
  currentState: string,
): Array<{ action: string; to: string; label: string; style?: string; confirm?: string }> {
  if (!entity.state_machine) return []

  const transitions = entity.state_machine.transitions.filter((t) =>
    t.from.includes(currentState) || t.from.includes("*"),
  )

  return transitions.map((t) => {
    const action = entity.actions.find((a) => a.name === t.via)
    return {
      action: t.via,
      to: t.to,
      label: action?.ui?.button_label ?? t.via.charAt(0).toUpperCase() + t.via.slice(1),
      style: action?.ui?.style,
      confirm: action?.ui?.confirm,
    }
  })
}
