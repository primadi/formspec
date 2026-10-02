// @vitest-environment node
//
// The two wizard rules (plan wizard-commit-patch-dan-peluncur.md):
//
//   B — how a wizard commits. `POST /{entity}/{id}/{action}` exists ONLY for
//       actions with an `impl`; a via-only transition is applied by PATCH, by
//       (from, to). The wizard must therefore read the same `has_route` the
//       bundle carries (kafe 10.48) instead of assuming a route — the old code
//       POSTed `spec.action` raw and produced `…/_ui/entity/close-shift`, a path
//       that cannot exist, with no record id.
//
//   C — which wizard launches a transition. Matching is the edge the wizard
//       declares to bind itself to its host: `spec.action === transition.via`
//       AND `spec.entity` resolving to the same entity.

import { describe, expect, it } from "vitest"

import {
  findWizardForTransition,
  resolveWizardCommit,
  usesActionRoute,
} from "./wizardCommit"
import type {
  ActionSummary,
  EntitySchema,
  Entry,
  WizardSpec,
} from "@/types/manifest"

function entityWith(overrides: Partial<EntitySchema> = {}): EntitySchema {
  return {
    module: "cafe-order",
    name: "shift",
    plural: "shifts",
    label_field: "name",
    fields: [
      { name: "counted_cash", type: "money" },
      { name: "note", type: "text" },
      { name: "supervisor_id", type: "relation" },
      { name: "status", type: "enum" },
    ],
    actions: [
      // A via-only transition: resolvable, but NO route.
      {
        name: "close-shift",
        permission: "cafe-order.shifts.close-shift",
        has_route: false,
      },
      // A real custom action with an impl.
      { name: "post", permission: "gl.journal-entries.post", has_route: true },
    ],
    state_machine: {
      field: "status",
      initial: "open",
      states: [
        { name: "open", label: "Terbuka" },
        { name: "closed", label: "Tutup" },
      ],
      transitions: [
        {
          from: ["open"],
          to: "closed",
          via: "close-shift",
          description: "Tutup shift",
        },
      ],
    },
    lifecycle: "plain_crud",
    ...overrides,
  } as EntitySchema
}

const wizard = (
  name: string,
  module: string,
  entityRef: string | undefined,
  action: string | undefined,
): Entry<WizardSpec> =>
  ({
    name,
    module,
    spec: { title: name, entity: entityRef, action, steps: [] },
  }) as Entry<WizardSpec>

describe("usesActionRoute", () => {
  it("follows has_route when the bundle carries it", () => {
    const actions: ActionSummary[] = [
      { name: "post", permission: "x.y.post", has_route: true },
      { name: "close-shift", permission: "x.y.close-shift", has_route: false },
    ]
    expect(usesActionRoute(actions, "post")).toBe(true)
    expect(usesActionRoute(actions, "close-shift")).toBe(false)
  })

  it("treats a declared action without the flag as having a route (older server)", () => {
    const actions: ActionSummary[] = [
      { name: "legacy", permission: "x.y.legacy" },
    ]
    expect(usesActionRoute(actions, "legacy")).toBe(true)
  })

  it("treats an action absent from the bundle as having no route", () => {
    expect(
      usesActionRoute(
        [{ name: "post", permission: "x.y.post", has_route: true }],
        "ghost",
      ),
    ).toBe(false)
    expect(usesActionRoute(undefined, "ghost")).toBe(false)
  })
})

describe("resolveWizardCommit — B: how the wizard commits", () => {
  it("PATCHes the state field for a via-only transition, and carries the collected fields", () => {
    const commit = resolveWizardCommit({
      entity: entityWith(),
      action: "close-shift",
      id: "shift-1",
      collected: { counted_cash: 500000, note: "kurang", ui_only: "x" },
    })
    expect(commit.kind).toBe("patch")
    if (commit.kind !== "patch") return
    expect(commit.path).toBe("cafe-order/shift/shift-1")
    // The target state comes from the manifest, never from the caller.
    expect(commit.body.status).toBe("closed")
    expect(commit.body.counted_cash).toBe(500000)
    expect(commit.body.note).toBe("kurang")
    // A step's UI-only key must not leak into the payload.
    expect(commit.body).not.toHaveProperty("ui_only")
  })

  it("POSTs the action route when the action has one", () => {
    const commit = resolveWizardCommit({
      entity: entityWith(),
      action: "post",
      id: "j-9",
      collected: { note: "ok" },
    })
    expect(commit.kind).toBe("action")
    if (commit.kind !== "action") return
    // The old code sent `entry.spec.action` RAW, which resolved to
    // `…/_ui/entity/post` — module/entity/id were missing entirely.
    expect(commit.path).toBe("cafe-order/shift/j-9/post")
    expect(commit.body).toEqual({ note: "ok" })
  })

  it("refuses a record-scoped commit with no record id", () => {
    const patch = resolveWizardCommit({
      entity: entityWith(),
      action: "close-shift",
      collected: {},
    })
    expect(patch.kind).toBe("error")

    const post = resolveWizardCommit({
      entity: entityWith(),
      action: "post",
      collected: {},
    })
    expect(post.kind).toBe("error")
  })

  it("refuses an action that has neither a route nor a matching transition", () => {
    const commit = resolveWizardCommit({
      entity: entityWith(),
      action: "banish",
      id: "shift-1",
      collected: {},
    })
    expect(commit.kind).toBe("error")
  })
})

describe("findWizardForTransition — C: which wizard launches a transition", () => {
  const shift = entityWith()

  it("matches a wizard bound by action + entity (dotted ref)", () => {
    const wizards = [
      wizard(
        "close-shift-wizard",
        "cafe-order",
        "cafe-order.shift",
        "close-shift",
      ),
    ]
    expect(findWizardForTransition(wizards, shift, "close-shift")?.name).toBe(
      "close-shift-wizard",
    )
  })

  it("matches a module-local (bare) entity ref", () => {
    const wizards = [wizard("w", "cafe-order", "shift", "close-shift")]
    expect(findWizardForTransition(wizards, shift, "close-shift")?.name).toBe(
      "w",
    )
  })

  it("ignores a wizard for another entity, another action, or without an action", () => {
    expect(
      findWizardForTransition(
        [wizard("w", "cafe-order", "order", "close-shift")],
        shift,
        "close-shift",
      ),
    ).toBeUndefined()
    expect(
      findWizardForTransition(
        [wizard("w", "cafe-order", "shift", "reopen")],
        shift,
        "close-shift",
      ),
    ).toBeUndefined()
    // A plain entity-create wizard binds to no transition.
    expect(
      findWizardForTransition(
        [wizard("w", "cafe-order", "shift", undefined)],
        shift,
        "close-shift",
      ),
    ).toBeUndefined()
  })

  it("returns undefined for a missing wizard list or empty action", () => {
    expect(
      findWizardForTransition(undefined, shift, "close-shift"),
    ).toBeUndefined()
    expect(findWizardForTransition([], shift, "")).toBeUndefined()
  })
})
