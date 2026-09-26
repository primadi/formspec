// ─── Session Context Preference (per-device) ───
//
// Remembers which session context (role × branch) the caller last signed in
// with, so the picker can be prefilled instead of forcing a choice every time.
//
// Storage is `localStorage`, not `sessionStorage`: the choice is a per-DEVICE
// preference that must survive a tab close (that is exactly what makes OAuth —
// which has no step where the caller could choose — workable). Tokens live in
// sessionStorage precisely so they do NOT survive; a context id is not a
// secret, it is just which of the caller's own assignments they prefer.
//
// Keyed by workspace + App because the same role/branch pair means different
// things in different Apps, and an assignment valid in one may not exist in
// the other (the server answers 409 CONTEXT_REQUIRED for a stale id — fail
// closed, never a silently widened session).
//
// The server never stores this preference and never picks a boundary on the
// caller's behalf (backend §8.7): a remembered id is sent as `assignment`, and
// the server still validates that the principal holds it.

const STORAGE_PREFIX = "formspec-context"

function storageKey(workspace: string, app?: string): string {
  return `${STORAGE_PREFIX}:${workspace}:${app ?? ""}`
}

/**
 * The remembered context id (`<role>@<value>`) for one workspace/App, or
 * `undefined` when the caller has never signed in there on this device.
 */
export function readContextPreference(
  workspace: string,
  app?: string,
): string | undefined {
  if (!workspace) return undefined
  try {
    return localStorage.getItem(storageKey(workspace, app)) ?? undefined
  } catch {
    // Private mode / storage disabled — prefilling is best-effort.
    return undefined
  }
}

/** Remember the context id for this workspace/App on this device. */
export function writeContextPreference(
  workspace: string,
  app: string | undefined,
  id: string,
): void {
  if (!workspace || !id) return
  try {
    localStorage.setItem(storageKey(workspace, app), id)
  } catch {
    // Best-effort — a failed write only costs the prefill next time.
  }
}

/**
 * Pick the id to preselect in the picker: the remembered one when it is still
 * among the offered choices, otherwise the first choice.
 *
 * Falling back to the first choice (rather than to nothing) is deliberate — it
 * keeps the picker usable with one click on a new device, while still
 * requiring the caller to confirm the boundary. The server never infers it.
 */
export function defaultContextChoice(
  choices: { id: string }[],
  remembered: string | undefined,
): string | undefined {
  if (choices.length === 0) return undefined
  if (remembered && choices.some((c) => c.id === remembered)) return remembered
  return choices[0].id
}
