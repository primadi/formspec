// @vitest-environment jsdom
//
// ─── SearchSelect quick-create: help text follows the shared precedence ───
//
// Todo 5.23.3. `SearchSelect` was the ONE Form-reading site that did not apply
// the help precedence every other surface uses (`FormRenderer`, `OverlayHost`,
// `WizardFormStep`, `WizardRenderer` all go through `withEntityFieldDefaults`
// or `entityFieldHelp`). It read `entityFieldLabel` only, so a wizard's
// quick-create dialog was the only place in the app where an Entity's
// `description` never reached the user — labels were inherited, help was not.
//
// The rule under test is the same one stated in `06-page-kinds.md` §2 and
// shared with `withEntityFieldDefaults`:
//
//   authored `help:`  →  Entity field `description`  →  nothing

import { afterEach, describe, expect, it } from "vitest"
import { cleanup } from "@testing-library/react"
import { readFileSync } from "node:fs"
import { fileURLToPath } from "node:url"
import { entityFieldHelp } from "@/engine/derive"
import type { EntitySchema, Field } from "@/types/manifest"

afterEach(cleanup)

/** A minimal entity schema whose fields carry `description` (= help). */
function entityWith(fields: Field[]): EntitySchema {
  return {
    module: "clinic",
    name: "patient",
    plural: "patients",
    label_field: "name",
    fields,
  } as EntitySchema
}

describe("entityFieldHelp — inheritance from Entity `description`", () => {
  it("returns the Entity field description when no help is authored", () => {
    const entity = entityWith([
      {
        name: "national_id",
        type: "string",
        description: "Nomor KTP 16 digit",
      },
    ])
    expect(entityFieldHelp(entity, "national_id")).toBe("Nomor KTP 16 digit")
  })

  it("resolves a relation field through its `_id` suffix", () => {
    // Field names on a form are often "<relation>_id" while the Entity declares
    // the relation without the suffix; both must find the same description.
    const entity = entityWith([
      {
        name: "branch_id",
        type: "string",
        description: "Cabang tempat mendaftar",
      },
    ])
    expect(entityFieldHelp(entity, "branch_id")).toBe("Cabang tempat mendaftar")
  })

  it("returns undefined — not empty string — when nothing is declared", () => {
    // The distinction matters: only `undefined` means "inherit"; "" would be a
    // deliberately emptied help and must not fall through to the entity.
    const entity = entityWith([{ name: "name", type: "string" }])
    expect(entityFieldHelp(entity, "name")).toBeUndefined()
  })

  it("survives a missing entity", () => {
    expect(entityFieldHelp(undefined, "name")).toBeUndefined()
  })
})

describe("SearchSelect help precedence", () => {
  // The precedence itself is a one-liner at the call site, so pin it here as
  // the contract rather than through a full dialog render: `field.help ?? …`.
  const authored = (help?: string) => help

  it("authored help wins over the Entity description", () => {
    const entity = entityWith([
      { name: "phone", type: "string", description: "Entity description" },
    ])
    expect(authored("Authored help") ?? entityFieldHelp(entity, "phone")).toBe(
      "Authored help",
    )
  })

  it("falls back to the Entity description when help is absent", () => {
    const entity = entityWith([
      { name: "phone", type: "string", description: "Entity description" },
    ])
    expect(authored(undefined) ?? entityFieldHelp(entity, "phone")).toBe(
      "Entity description",
    )
  })

  it("renders nothing when neither is declared", () => {
    const entity = entityWith([{ name: "phone", type: "string" }])
    expect(
      authored(undefined) ?? entityFieldHelp(entity, "phone"),
    ).toBeUndefined()
  })
})

// A source-level guard on the CALL SITE. Mounting the component needs the meta
// store populated, which this repo's widget tests deliberately avoid (they
// assert contracts via source scans — see `widgets/catalog.test.tsx`). What can
// regress here without typecheck noticing is subtler than a compile error: the
// five hand-written JSX branches were collapsed into `wrap(control)` calls, and
// a branch that forgets `wrap` would silently render a field with no help
// again — the exact bug. So assert the shape of the source.
describe("SearchSelect callsite — every field branch goes through `wrap`", () => {
  // The path MUST go through a variable: Vite rewrites
  // `new URL("literal", import.meta.url)` into an asset URL, and `fileURLToPath`
  // then throws "The URL must be of scheme file".
  const rel = "./SearchSelect.tsx"
  const src = readFileSync(fileURLToPath(new URL(rel, import.meta.url)), "utf8")

  it("resolves help with the shared precedence", () => {
    expect(src).toMatch(/entityFieldHelp/)
    // `field.help ??` — authored help wins; the Entity description is the
    // fallback. Asserting the operator keeps a future edit from swapping the
    // order (which reads identically but inverts the precedence).
    expect(src).toMatch(/field\.help\s*\?\?/)
  })

  it("routes EVERY field branch through `wrap`", () => {
    // The old shape was a hand-written `<label>` + control per branch, so a
    // branch could omit help (all five did). Now `wrap` owns the label and the
    // help, so the branch count and the `wrap` call count must match — a
    // branch that reverts to hand-written JSX drops the help again.
    //
    // Counting `return wrap(` is what makes a regression visible; counting
    // labels is not, because the boolean branch builds its label across
    // multiple lines (so a naive label regex misses it either way).
    const returns = src.match(/return wrap\(/g) ?? []
    expect(returns).toHaveLength(5) // date, enum, boolean, numeric, default
  })

  it("keeps exactly one help render, inside `wrap`", () => {
    const helpRenders =
      src.match(/text-xs text-muted-foreground">\{help\}/g) ?? []
    expect(helpRenders).toHaveLength(1)
  })
})
