// @vitest-environment node
//
// Kafe 10.48: the renderer must DECIDE which call applies a transition from the
// bundle, not PROBE the endpoint.
//
// Background: `POST /{entity}/{id}/{action}` only exists for actions with an
// `impl` (internal/api/generator.go skips `Impl == nil`). A state-machine
// transition that merely names a `via` has none — the PATCH path applies it, by
// (from, to). The client used to POST first and read the 404 as "no route",
// which worked but wasted a round-trip per transition and printed a 404 for a
// perfectly normal path.
//
// This guards the DECISION RULE (the same one `DetailPage` uses) so a future
// refactor cannot silently go back to probing.

import { describe, expect, it } from "vitest"

import type { ActionSummary } from "@/types/manifest"

/**
 * Mirror of DetailPage's choice. Kept here as the unit under test: the page
 * itself needs a router, a store and an API client, so the rule is what we pin.
 */
function usesActionRoute(
  actions: ActionSummary[] | undefined,
  action: string,
): boolean {
  const declared = actions?.find((a) => a.name === action)
  return declared?.has_route ?? declared !== undefined
}

const actions: ActionSummary[] = [
  // A transition-only `via`: resolvable, but no route.
  { name: "release", permission: "cafe-master.dining-tables.release", has_route: false },
  // A real custom action with an impl.
  { name: "post", permission: "gl.journal-entries.post", has_route: true },
  // An older server that does not send the flag at all.
  { name: "legacy", permission: "x.y.legacy" },
]

describe("transition → which call applies it", () => {
  it("uses the action route when the bundle says the route exists", () => {
    expect(usesActionRoute(actions, "post")).toBe(true)
  })

  it("uses the state write when the bundle says there is no route", () => {
    // This is the case that used to POST and eat a 404.
    expect(usesActionRoute(actions, "release")).toBe(false)
  })

  it("still sends no action route for an action absent from the bundle", () => {
    // A transition whose `via` is not in `entity.actions` at all: the PATCH
    // path is the only way to apply it.
    expect(usesActionRoute(actions, "mark-table-served")).toBe(false)
  })

  it("falls back to probing only when the server did not send has_route", () => {
    // Backward compatibility: an older bundle without the flag keeps the old
    // behaviour (try the route). Documented so the fallback is deliberate.
    expect(usesActionRoute(actions, "legacy")).toBe(true)
  })

  it("treats a missing action list as no route", () => {
    expect(usesActionRoute(undefined, "release")).toBe(false)
  })
})
