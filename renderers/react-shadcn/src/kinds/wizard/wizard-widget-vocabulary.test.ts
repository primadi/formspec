// @vitest-environment jsdom
//
// ─── Wizard step speaks the same widget vocabulary as a Form (todo 5.10.17) ───
//
// Before this, `WizardFormStep` rendered every input by hand (`import { Input }`)
// and never called `FormFieldWidget`, so the entire catalog was ignored inside a
// wizard:
//
//   examples/kafe/.../close-shift-wizard.yaml
//     - field: counted_cash      # entity type: money
//     - field: supervisor_id
//       widget: relation-picker
//
// `counted_cash` is `type: money`, so outside a wizard it renders `MoneyInput`
// (currency + amount, numpad on touch); inside the wizard it rendered a bare
// `<Input type="number">` — the currency was invisible and a typo could not be
// caught. Nothing failed: the widget name was legal, validated green, and was
// simply never read.
//
// These tests pin the routing contract, via source scan for the same reason
// `widgets/catalog.test.tsx` does — mounting needs the meta store populated,
// and the property being protected is structural: which router each field type
// reaches.

import { describe, expect, it } from "vitest"
import { readFileSync } from "node:fs"
import { fileURLToPath } from "node:url"

const read = (rel: string) =>
  readFileSync(fileURLToPath(new URL(rel, import.meta.url)), "utf8")

const wizardStepSrc = read("./WizardFormStep.tsx")

describe("WizardFormStep — widget routing", () => {
  it("delegates to the shared FormFieldWidget router", () => {
    // The one line that makes the catalog available inside a wizard.
    expect(wizardStepSrc).toMatch(
      /import \{ FormFieldWidget \} from "@\/kinds\/form\/FormRenderer"/,
    )
    expect(wizardStepSrc).toMatch(/<FormFieldWidget/)
  })

  it("honours `widget:` outside the native set instead of always using Input", () => {
    // Every branch that does NOT have wizard-specific behaviour must check
    // `field.widget` first. Counting the guard makes a regression visible: if a
    // branch drops it, that field type silently falls back to a plain input
    // again — the exact bug, with no error anywhere.
    const guards =
      wizardStepSrc.match(/WIZARD_NATIVE_WIDGETS\.has\(field\.widget\)/g) ?? []
    expect(guards.length).toBeGreaterThanOrEqual(2) // date/datetime + numeric
  })

  it("keeps the wizard-specific branches hand-written", () => {
    // Relation options are fetched with `depends_on` filtering, and enum
    // renders a plain select inside a step — behaviour `FormFieldWidget` does
    // not have. These must NOT be replaced by the generic router.
    expect(wizardStepSrc).toMatch(/entityField\.relation\?\.resource/)
    expect(wizardStepSrc).toMatch(/depends_on|dependsOnField/)
    expect(wizardStepSrc).toMatch(/entityField\.enum_values/)
  })

  it("routes every branch through `wrap` so no branch drops label/help", () => {
    // `return wrap(`... is the shared label+help carrier. The boolean branch
    // keeps its own inline label (bound to the checkbox id) and renders help
    // directly — so exactly ONE bare `{help}` remains, and every other branch
    // gets help via `wrap`'s `helpNode` parameter.
    const wraps = wizardStepSrc.match(/return wrap\(/g) ?? []
    // relation, enum, date-native, date-widget, numeric-native, numeric-widget,
    // default = 7 hand-written returns; boolean is the one that does not wrap.
    expect(wraps).toHaveLength(7)
    expect(wizardStepSrc.match(/\{help\}/g) ?? []).toHaveLength(1)
    // ...and `wrap` itself must forward it, or the 7 branches lose help.
    expect(wizardStepSrc).toMatch(/\{helpNode\}/)
  })
})
