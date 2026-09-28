// Tests for the action-input resolver — the ONE place that decides what a
// transition or action asks its caller for.
//
// Plan: docs_internal/plan/action-input-contract.md (Fase 5).

import { describe, expect, it } from "vitest"

import {
  buildActionInputBody,
  declaredInputs,
  declaredRenderMode,
  deriveInputRenderMode,
  resolveActionInputs,
  toFieldDescriptor,
} from "@/lib/actionParams"
import type { EntitySchema } from "@/types/manifest"

function entity(overrides: Partial<EntitySchema> = {}): EntitySchema {
  return {
    module: "cafe-order",
    name: "order",
    plural: "orders",
    label_field: "number",
    fields: [
      { name: "name", type: "string" } as never,
      { name: "status", type: "string" } as never,
      { name: "void_reason", type: "string", title: "Alasan Void" } as never,
      {
        name: "reason_code",
        type: "string",
        enum_values: ["spoil", "mistake"],
      } as never,
    ],
    actions: [],
    lifecycle: "plain_crud",
    ...overrides,
  } as EntitySchema
}

describe("resolveActionInputs", () => {
  it("returns empty when nothing is declared", () => {
    const got = resolveActionInputs(entity(), "void-order")
    expect(got.isEmpty).toBe(true)
    expect(got.inputs).toEqual([])
  })

  it("reads inputs from the transition and inherits the entity field", () => {
    const got = resolveActionInputs(entity(), "void-order", {
      from: ["posted"],
      to: "voided",
      via: "void-order",
      params: { inputs: [{ name: "void_reason", widget: "textarea" }] },
    })

    expect(got.isEmpty).toBe(false)
    expect(got.inputs).toHaveLength(1)
    const [input] = got.inputs
    // The caption prefers the declaration, then the FIELD's title (not a
    // humanised name: "Alasan Void" is what the entity says it is called).
    expect(input.label).toBe("Alasan Void")
    // The descriptor IS the entity field, so the widget router and zod builder
    // see exactly what a Form would see.
    expect(input.descriptor.title).toBe("Alasan Void")
    expect(input.persists).toBe(true)
  })

  it("falls back to the declared action when the transition has no params", () => {
    // The server reads declared-∪-transition through EffectiveActionSpec; the
    // renderer must see the same contract, or a manifest keeping the declaration
    // on the action would render no form while the server enforced one.
    const withAction = entity({
      actions: [
        {
          name: "void-order",
          permission: "cafe-order.orders.void-order",
          params: { inputs: [{ name: "void_reason" }] },
        },
      ] as never,
    })
    const got = resolveActionInputs(withAction, "void-order", {
      from: ["posted"],
      to: "voided",
      via: "void-order",
    })
    expect(got.inputs).toHaveLength(1)
    expect(got.inputs[0].input.name).toBe("void_reason")
  })

  it("resolves inputs_from against the entity input sets", () => {
    const withSet = entity({
      input_sets: [{ name: "reason", inputs: [{ name: "void_reason" }] }],
    } as never)
    const got = resolveActionInputs(withSet, "void-order", {
      from: ["posted"],
      to: "voided",
      via: "void-order",
      params: { inputs_from: ["reason"] },
    })
    expect(got.inputs.map((i) => i.input.name)).toEqual(["void_reason"])
  })

  it("prefers an inline declaration over the same name from a set", () => {
    const withSet = entity({
      input_sets: [{ name: "reason", inputs: [{ name: "void_reason" }] }],
    } as never)
    const got = resolveActionInputs(withSet, "void-order", {
      from: ["posted"],
      to: "voided",
      via: "void-order",
      params: {
        inputs: [{ name: "void_reason", label: "Alasan (penting)" }],
        inputs_from: ["reason"],
      },
    })
    expect(got.inputs).toHaveLength(1)
    expect(got.inputs[0].label).toBe("Alasan (penting)")
  })

  it("treats an ad-hoc input as not persisting", () => {
    const got = resolveActionInputs(entity(), "note", {
      from: ["a"],
      to: "b",
      via: "note",
      params: { inputs: [{ name: "approver_note", type: "text" }] },
    })
    expect(got.inputs[0].persists).toBe(false)
  })

  it("honours an explicit persist override", () => {
    const got = resolveActionInputs(entity(), "void-order", {
      from: ["posted"],
      to: "voided",
      via: "void-order",
      params: { inputs: [{ name: "void_reason", persist: false }] },
    })
    expect(got.inputs[0].persists).toBe(false)
  })
})

describe("container decision", () => {
  it("derives from the input count like a Form does", () => {
    expect(deriveInputRenderMode(1)).toBe("modal")
    expect(deriveInputRenderMode(5)).toBe("modal")
    expect(deriveInputRenderMode(6)).toBe("drawer")
    expect(deriveInputRenderMode(12)).toBe("drawer")
    expect(deriveInputRenderMode(13)).toBe("separate_page")
  })

  it("prefers the declared mode — a design-time decision", () => {
    const got = resolveActionInputs(entity(), "void-order", {
      from: ["posted"],
      to: "voided",
      via: "void-order",
      params: {
        inputs: [{ name: "void_reason" }],
        render: { mode: "separate_page" },
      },
    })
    expect(got.renderMode).toBe("separate_page")
    expect(declaredRenderMode({ render: { mode: "drawer" } })).toBe("drawer")
  })
})

describe("toFieldDescriptor", () => {
  it("returns the entity field unchanged for a referring input", () => {
    const field = {
      name: "void_reason",
      type: "string",
      title: "Alasan Void",
    } as never
    const got = toFieldDescriptor(
      { name: "void_reason", widget: "textarea" },
      field,
    )
    expect(got).toBe(field)
  })

  it("builds a descriptor from the declaration for an ad-hoc input", () => {
    const got = toFieldDescriptor(
      {
        name: "approver_note",
        type: "text",
        label: "Catatan",
        help: "Opsional",
      },
      null,
    )
    expect(got.name).toBe("approver_note")
    expect(got.type).toBe("text")
    expect(got.title).toBe("Catatan")
    expect(got.description).toBe("Opsional")
    expect(got.required).toBe(false)
  })

  it("recognises `rules: [required]` as requiredness", () => {
    // Two spellings of one statement, both in real manifests.
    const viaFlag = toFieldDescriptor(
      { name: "a", type: "string", required: true },
      null,
    )
    const viaRule = toFieldDescriptor(
      {
        name: "b",
        type: "string",
        rules: [{ name: "required", value: undefined }],
      },
      null,
    )
    expect(viaFlag.required).toBe(true)
    expect(viaRule.required).toBe(true)
  })
})

describe("buildActionInputBody", () => {
  const resolved = resolveActionInputs(entity(), "void-order", {
    from: ["posted"],
    to: "voided",
    via: "void-order",
    params: {
      inputs: [
        { name: "void_reason" },
        { name: "approver_note", type: "text" },
      ],
    },
  })

  it("emits only declared names", () => {
    // The contract decides what travels — a stray key in the dialog's state must
    // not be forwarded to the server as if it were declared.
    const body = buildActionInputBody(resolved, {
      void_reason: "because",
      not_declared: "smuggled",
    })
    expect(body).toEqual({ void_reason: "because" })
  })

  it("drops empty values so an untouched optional input does not blank a stored one", () => {
    const body = buildActionInputBody(resolved, {
      void_reason: "because",
      approver_note: "",
    })
    expect(body).toEqual({ void_reason: "because" })
  })

  it("keeps falsy-but-meaningful values", () => {
    const zero = resolveActionInputs(entity(), "a", {
      from: ["x"],
      to: "y",
      via: "a",
      params: { inputs: [{ name: "qty", type: "integer" }] },
    })
    expect(buildActionInputBody(zero, { qty: 0 })).toEqual({ qty: 0 })
    expect(buildActionInputBody(zero, { qty: false })).toEqual({ qty: false })
  })
})

describe("declaredInputs", () => {
  it("is empty for undefined params", () => {
    expect(declaredInputs(undefined, undefined)).toEqual([])
  })

  it("ignores an inputs_from naming a set that does not exist", () => {
    // The server refuses this outright; the renderer must not crash on an older
    // bundle that still carries it.
    expect(declaredInputs({ inputs_from: ["nope"] }, [])).toEqual([])
  })
})
