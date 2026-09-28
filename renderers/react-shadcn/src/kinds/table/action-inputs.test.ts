// @vitest-environment node
//
// ─── Action input contracts reach every surface ───
//
// Plan: docs_internal/plan/action-input-contract.md (Fase 6).
//
// Before this, no surface sent a body: DetailPage's transition POST called
// `client.post(path)` with nothing, TableRenderer's row/bulk handlers did the
// same, and Kanban matched them. So a transition whose `guard`/`conditions` read
// `params.get('void_reason')` could never pass through the derived UI — the
// declaration looked enforced and was unreachable.
//
// These pin the wiring. The resolution and body-building logic itself is unit
// tested in `lib/actionParams.test.ts`; what cannot be tested there is that each
// surface actually CALLS it, gates on it, and forwards the values — the three
// places the feature is silently lost.
//
// Source-text assertions match the established style for these components
// (see `kinds/table/bulk-actions.test.ts`): each needs a router, stores and an
// API client to mount, so the wiring is what we pin.

import { describe, expect, it } from "vitest"
import { readFileSync } from "node:fs"
import { fileURLToPath } from "node:url"

const read = (rel: string) =>
  readFileSync(fileURLToPath(new URL(rel, import.meta.url)), "utf8")

const detail = read("../page/DetailPage.tsx")
const table = read("./TableRenderer.tsx")
const kanban = read("../kanban/KanbanRenderer.tsx")
const dialog = read("../../shell/ActionInputDialog.tsx")

describe("DetailPage — transition inputs", () => {
  it("resolves the transition's own declaration, not just the action's", () => {
    // The transition's `params` is the authoritative site (the server reads the
    // same union through EffectiveActionSpec); passing `decl` is what makes a
    // contract declared on the transition visible at all.
    expect(detail).toMatch(
      /resolveActionInputs\(entity, action, transition\?\.decl\)/,
    )
  })

  it("opens the dialog only when something is declared", () => {
    // An empty contract must keep the plain-confirm behaviour it had before —
    // otherwise every transition gains a pointless form.
    expect(detail).toMatch(
      /if \(!resolved\.isEmpty\) \{\s*\n\s*setInputTransition\(/,
    )
  })

  it("sends the collected values on the action route", () => {
    // The regression was `client.post(`${path}/${action}`)` — no body.
    expect(detail).toMatch(
      /client\.post\(`\$\{path\}\/\$\{action\}`, \{ json: inputs \?\? \{\} \}\)/,
    )
  })

  it("merges the collected values into the PATCH body alongside the target state", () => {
    // A transition without an `impl` is applied by writing the state field; the
    // inputs must ride in the same request, or they arrive after the guard that
    // needs them has already run.
    expect(detail).toMatch(
      /\{ \[field\]: target, \.\.\.\(inputs \?\? \{\}\) \}/,
    )
  })
})

describe("TableRenderer — row and bulk action inputs", () => {
  it("resolves inputs for a row action", () => {
    expect(table).toMatch(/resolveActionInputs\(entity, action\.action\)/)
  })

  it("sends one body for every row in a bulk run", () => {
    // The whole point of collecting once: the same values apply to the whole
    // selection. Per-row input collection is what made action parameters
    // unusable from the bulk bar (todo 5.12.9).
    expect(table).toMatch(
      /client\.post\(\s*\n?\s*`\$\{entity\.module\}\/\$\{entity\.name\}\/\$\{segments\}\/\$\{action\.action\}`,\s*\n\s*\{ json: inputs \?\? \{\} \}/,
    )
  })

  it("routes a bulk dialog submission into the bulk runner", () => {
    // `row` distinguishes row from bulk; passing the wrong one would run the
    // action against a single row while the UI said "N baris".
    expect(table).toMatch(/await runBulkAction\(pending\.action, values\)/)
    expect(table).toMatch(
      /await handleRowAction\(pending\.action, pending\.row, true, values\)/,
    )
  })

  it("names the selection size so the operator knows the scope", () => {
    expect(table).toMatch(
      /Diterapkan ke \$\{selectedRows\.size\} baris terpilih/,
    )
  })
})

describe("KanbanRenderer — card action inputs", () => {
  it("resolves inputs before running a card action", () => {
    expect(kanban).toMatch(/resolveActionInputs\(entity, action\.action\)/)
  })

  it("sends the collected values with the card action", () => {
    expect(kanban).toMatch(/\{ json: inputs \?\? \{\} \}/)
  })

  it("never collects inputs for delete", () => {
    // Delete carries no contract; gating it behind a form would add a step to a
    // destructive action that already has its own confirm.
    expect(kanban).toMatch(/action\.action !== "delete"/)
  })
})

describe("ActionInputDialog — the one generic mechanism", () => {
  it("reuses the shared field renderer instead of a second widget switch", () => {
    // Reusing `FormFieldWidget` is what keeps the widget catalog parity test
    // (`widgets/catalog.test.tsx`) meaningful: a new widget is added once.
    expect(dialog).toMatch(
      /import \{ FormFieldWidget \} from "@\/kinds\/form\/FormRenderer"/,
    )
  })

  it("validates with the same zod builder a Form uses", () => {
    expect(dialog).toMatch(/buildZodField\(r\.descriptor\)/)
  })

  it("evaluates the shared conditional vocabulary", () => {
    expect(dialog).toMatch(/evalVisibleWhen/)
    expect(dialog).toMatch(/evalReadonlyWhen/)
    expect(dialog).toMatch(/evalRequiredWhen/)
    expect(dialog).toMatch(/evalCompute/)
  })

  it("re-seeds values each time it opens", () => {
    // Otherwise a second attempt inherits the first one's entries, including a
    // value the caller cleared.
    expect(dialog).toMatch(/if \(!open\) return/)
  })
})
