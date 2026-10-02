// @vitest-environment node
//
// The wizard launcher is now reachable from a TABLE row action too, not only
// from the detail page (plan wizard-commit-patch-dan-peluncur.md):
//
//   Kas & Shift → Shift Kasir  →  [Tutup Shift]  →  close-shift-wizard?id=…
//
// The table shares the same rule as DetailPage (`findWizardForTransition`).
// This pins the source-level contract so a refactor cannot silently drop the
// branch and send the click back to `POST /{entity}/{id}/{action}` — a route a
// via-only transition does NOT have (`has_route: false`), so it would 404.

import { readFileSync } from "node:fs"
import { describe, expect, it } from "vitest"

const tableSrc = readFileSync(
  new URL("./TableRenderer.tsx", import.meta.url),
  "utf8",
)
const detailSrc = readFileSync(
  new URL("../page/DetailPage.tsx", import.meta.url),
  "utf8",
)

describe("table row action launches a wizard bound to the action", () => {
  it("TableRenderer consults findWizardForTransition before running the action", () => {
    expect(tableSrc).toMatch(
      /import\s*\{[^}]*findWizardForTransition[^}]*\}\s*from\s*"@\/engine\/wizardCommit"/,
    )
    expect(tableSrc).toMatch(
      /findWizardForTransition\(wizards,\s*entity,\s*action\.action\)/,
    )
  })

  it("both surfaces use the SAME rule (no divergence)", () => {
    // DetailPage and TableRenderer must agree: a transition that opens a wizard
    // from the detail page must open it from the list too.
    for (const src of [tableSrc, detailSrc]) {
      expect(src).toMatch(/findWizardForTransition\(/)
    }
    expect(detailSrc).toMatch(
      /import\s*\{[^}]*findWizardForTransition[^}]*\}\s*from\s*"@\/engine\/wizardCommit"/,
    )
  })

  it("navigates to the wizard route carrying the row's id", () => {
    // `?id=` is what lets the wizard commit a record-scoped transition; without
    // it `resolveWizardCommit` refuses (there is nothing to PATCH).
    expect(tableSrc).toMatch(/surfacePath\("wizard",\s*wizard\.name\)/)
    expect(tableSrc).toMatch(/query\.set\("id",\s*rowId\)/)
  })
})
