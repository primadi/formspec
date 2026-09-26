// @vitest-environment jsdom
//
// ─── Bulk actions actually execute (todo 5.12.8) ───
//
// Before this, `BulkActionsBar` rendered one `<Button>` per
// `tableSpec.bulk_actions` entry with NO `onClick`, and `TableRenderer` never
// passed a collective handler. So the bar appeared, looked clickable, and did
// nothing — while the Batch edit bar next to it (5.4.3) worked for real. Two
// features in the same bar behaving differently, with nothing to tell them
// apart.
//
// These pin the contract that makes the fix meaningful:
//
//   1. every rendered button has a handler (no dead button can come back);
//   2. navigation actions (view/edit) are REFUSED, not silently run per row —
//      "edit 12 rows" has no meaning without values;
//   3. permission is checked BEFORE any row is touched, so a partial run can
//      never be blamed on permissions;
//   4. a failure is reported per row, never as a blanket error.

import { describe, expect, it } from "vitest"
import { readFileSync } from "node:fs"
import { fileURLToPath } from "node:url"

const read = (rel: string) =>
  readFileSync(fileURLToPath(new URL(rel, import.meta.url)), "utf8")

const src = read("./TableRenderer.tsx")

describe("BulkActionsBar — no dead buttons", () => {
  it("wires onClick through to the collective runner", () => {
    // The bug was a <Button> with no onClick. Assert the handler is passed in
    // AND called, so removing either half fails here.
    expect(src).toMatch(/onClick=\{\(\) => onRun\(action\)\}/)
    expect(src).toMatch(/onRun=\{requestBulkAction\}/)
  })

  it("disables actions that cannot run collectively", () => {
    // Disabled, not hidden: a declared-but-inapplicable action should be
    // visible so the author knows it was read.
    expect(src).toMatch(/isDisabled=\{\(a\) => canRunBulk\(a\) !== null\}/)
    expect(src).toMatch(/disabled=\{isDisabled\(action\)\}/)
  })
})

describe("bulk action eligibility", () => {
  it("refuses navigation actions instead of running them per row", () => {
    // `view`/`edit` are per-row navigation. Running them across a selection is
    // meaningless (there is one screen), so they are refused with a message
    // pointing at the row actions.
    expect(src).toMatch(
      /action\.action === "view" \|\| action\.action === "edit"/,
    )
  })

  it("checks permission before touching any row", () => {
    // `canRunBulk` is the gate, and `runBulkAction` returns early on it — so a
    // refusal cannot leave half the selection mutated.
    expect(src).toMatch(
      /const reason = canRunBulk\(action\)\s*\n\s*if \(reason\)/,
    )
    expect(src).toMatch(/canDoEntityAction\(me, entity, action\.action\)/)
  })

  it("gate runs BEFORE the confirm dialog, so a refused action never prompts", () => {
    // `requestBulkAction` calls `canRunBulk` first and returns; only then does
    // it consider a confirm message.
    const idx = src.indexOf("const requestBulkAction")
    expect(idx).toBeGreaterThan(-1)
    const body = src.slice(idx, idx + 900)
    expect(body.indexOf("canRunBulk")).toBeLessThan(body.indexOf("confirmMsg"))
  })
})

describe("bulk action reporting", () => {
  it("reports per row, following the batch-edit contract", () => {
    expect(src).toMatch(/setBulkResults\(\{ label: action\.label, results \}\)/)
    expect(src).toMatch(/BulkResultReport/)
    // Partial failure names both counts — never a blanket success.
    expect(src).toMatch(/berhasil, \$\{failCount\} gagal/)
  })

  it("marks 409 rows stale, like the row action path", () => {
    const idx = src.indexOf("const runBulkAction")
    const body = src.slice(idx, idx + 1600)
    expect(body).toMatch(/err\.status === 409/)
    expect(body).toMatch(/setStaleRows/)
  })

  it("asks for confirmation on destructive bulk actions", () => {
    expect(src).toMatch(/pendingBulkAction/)
    expect(src).toMatch(/pendingBulkAction\?\.action\.action === "delete"/)
  })
})
