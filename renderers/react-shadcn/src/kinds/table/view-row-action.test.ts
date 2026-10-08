// @vitest-environment node
//
// The view-target row action (`action.view` — plan docs_internal/plan/print-row-action.md).
//
// A TableAction can carry `view` to NAVIGATE to a view resource instead of
// calling the entity action. This is how renderer builtins with NO backing
// entity action — `print` (and `export`), whitelisted in
// `internal/ui/validate.go` builtinRowActions — get a UI trigger.
//
// The subtle failure this pins: `canDoEntityAction(me, entity, "print")` is
// FALSE (there is no `print` action to authorize), so a view branch placed
// AFTER that guard is dead code — the button never renders and the click never
// navigates. Order is the contract.
//
// It also pins the shared gate: the render filter and the click handler must
// ask the SAME question (`canDoTableAction`), or a button can render and then
// answer "no permission" on click.

import { readFileSync } from "node:fs"
import { describe, expect, it } from "vitest"

const tableSrc = readFileSync(
  new URL("./TableRenderer.tsx", import.meta.url),
  "utf8",
)
const kanbanSrc = readFileSync(
  new URL("../kanban/KanbanRenderer.tsx", import.meta.url),
  "utf8",
)

describe("table: view-target row action", () => {
  it("imports the view-target helpers", () => {
    expect(tableSrc).toMatch(
      /import\s*\{[^}]*resolveViewTarget[^}]*\}\s*from\s*"@\/engine\/viewTarget"/,
    )
  })

  it("dispatches on action.view BEFORE the entity-action guard", () => {
    const viewBranch = tableSrc.indexOf("if (action.view) {")
    const entityGuard = tableSrc.indexOf(
      "if (!canDoEntityAction(me, entity, action.action))",
    )
    expect(viewBranch).toBeGreaterThan(-1)
    expect(entityGuard).toBeGreaterThan(-1)
    // The guard must come AFTER the view branch — otherwise `print` (no entity
    // action) is rejected and the branch is unreachable.
    expect(viewBranch).toBeLessThan(entityGuard)
  })

  it("navigates through surfacePath with the record id for id-routes", () => {
    expect(tableSrc).toMatch(/viewTargetTakesId\(target\.kind\)/)
    expect(tableSrc).toMatch(
      /surfacePath\(\s*target\.kind,\s*target\.name,\s*getEntityRouteSegment\(entity,\s*row\),?\s*\)/,
    )
  })

  it("gates the rendered button with the SAME helper as the click", () => {
    // Both the render filter and the click handler use canDoTableAction.
    const uses = tableSrc.match(/canDoTableAction\(/g) ?? []
    expect(uses.length).toBeGreaterThanOrEqual(2)
  })
})

describe("kanban: view-target row action", () => {
  it("dispatches on action.view BEFORE the entity-action guard", () => {
    const viewBranch = kanbanSrc.indexOf("if (action.view) {")
    const entityGuard = kanbanSrc.indexOf("!canDoEntityAction(")
    expect(viewBranch).toBeGreaterThan(-1)
    expect(entityGuard).toBeGreaterThan(-1)
    expect(viewBranch).toBeLessThan(entityGuard)
  })

  it("filters view actions by their target, not the entity action name", () => {
    expect(kanbanSrc).toMatch(/canDoViewTarget\(me,\s*metaBundle,\s*a\.view\)/)
  })
})
