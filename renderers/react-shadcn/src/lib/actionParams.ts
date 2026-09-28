// ─── Action input resolution ───
//
// One place that answers "what must the caller supply before this transition or
// action runs, and how should it be drawn".
//
// Before this, nothing answered it: a transition button POSTed an empty body and
// a table action did the same, so an action whose guard read `params.get(...)`
// could never pass through the derived UI — and `ActionSummary.has_params`
// shipped in the bundle with no consumer, because a boolean cannot describe a
// field.
//
// Design decisions this file implements (plan
// docs_internal/plan/action-input-contract.md):
//
//   - ONE mechanism, N declarations. The dialog is generic; the inputs are
//     declared per transition/action. Nothing here is per-kind.
//   - A referring input inherits the Entity field's shape (type, options,
//     cardinality). Only what is genuinely surface-specific — caption, widget,
//     predicates — is read from the declaration.
//   - The container is a design-time decision: declared via `params.render.mode`
//     when present, otherwise derived from the input count by the same
//     thresholds `deriveFormRenderMode` uses.

import type {
  ActionSummary,
  EntitySchema,
  Field,
  InputSet,
  ParamInput,
  ParamsDecl,
  ParamsRenderHint,
  TransitionDecl,
} from "@/types/manifest"

/** How the input form should be presented. */
export type ActionInputRenderMode = "modal" | "drawer" | "separate_page"

/** A resolved input: the declaration plus the Entity field it refers to, if any. */
export interface ResolvedActionInput {
  input: ParamInput
  /** The Entity field this input refers to, or null for an ad-hoc parameter. */
  field: Field | null
  /** Caption, following the shared precedence: label → field title → humanised. */
  label: string
  /** The field shape `FormFieldWidget`/`buildZodField` consume. */
  descriptor: Field
  /** Whether the value is written to the record when it names a field. */
  persists: boolean
}

export interface ResolvedActionInputs {
  inputs: ResolvedActionInput[]
  renderMode: ActionInputRenderMode
  /** True when there is nothing to collect — the caller may run the action directly. */
  isEmpty: boolean
}

export const EMPTY_ACTION_INPUTS: ResolvedActionInputs = {
  inputs: [],
  renderMode: "modal",
  isEmpty: true,
}

/** Humanise a snake_case identifier: "void_reason" → "Void Reason". */
function humanize(name: string): string {
  return name
    .replace(/_/g, " ")
    .replace(/\bid\b/i, "ID")
    .replace(/\b\w/g, (c) => c.toUpperCase())
}

/**
 * Thresholds mirroring `deriveFormRenderMode` (engine/derive.ts): a two-field
 * approval prompt should not open a full page, and a twelve-field one should not
 * be squeezed into a modal.
 */
export function deriveInputRenderMode(count: number): ActionInputRenderMode {
  if (count <= 5) return "modal"
  if (count <= 12) return "drawer"
  return "separate_page"
}

/** Resolve which inputs a `params:` contract declares, in declaration order. */
export function declaredInputs(
  params: ParamsDecl | undefined,
  inputSets: InputSet[] | undefined,
): ParamInput[] {
  if (!params) return []
  const out: ParamInput[] = [...(params.inputs ?? [])]
  for (const name of params.inputs_from ?? []) {
    const set = (inputSets ?? []).find((s) => s.name === name)
    if (!set) continue
    for (const input of set.inputs ?? []) {
      // A name declared both inline and through a set is one input; the inline
      // declaration wins so the more specific one is the one honoured (the server
      // rejects the duplicate outright, so this only keeps the renderer sane
      // against an older bundle).
      if (out.some((i) => i.name === input.name)) continue
      out.push(input)
    }
  }
  return out
}

/**
 * Resolve an action's or transition's input contract into renderable inputs.
 *
 * `transition` is preferred when present because the transition carries its own
 * `params`; the matching `ActionSummary` is consulted as a fallback so a
 * contract declared on a shared `actions:` entry is found too (the server reads
 * the same union through `EffectiveActionSpec`).
 */
export function resolveActionInputs(
  entity: EntitySchema,
  actionName: string,
  transition?: TransitionDecl,
): ResolvedActionInputs {
  const action = entity.actions?.find((a) => a.name === actionName)
  const params = transition?.params ?? action?.params
  const inputs = declaredInputs(params, entity.input_sets)

  const resolved: ResolvedActionInput[] = []
  for (const input of inputs) {
    const field = entity.fields.find((f) => f.name === input.name) ?? null
    resolved.push({
      input,
      field,
      label: input.label ?? field?.title ?? humanize(input.name),
      descriptor: toFieldDescriptor(input, field),
      persists: input.persist ?? field !== null,
    })
  }

  const renderMode =
    (params?.render?.mode as ActionInputRenderMode | undefined) ??
    deriveInputRenderMode(resolved.length)

  return { inputs: resolved, renderMode, isEmpty: resolved.length === 0 }
}

/**
 * Build the `Field` shape the shared field renderer understands.
 *
 * `FormFieldWidget` and `buildZodField` both take a `Field`, so expressing an
 * ad-hoc parameter as one lets the existing widget router and zod builder serve
 * the input dialog unchanged — no second widget switch to keep in sync with the
 * catalog, which is the whole reason this is an adapter rather than a new
 * renderer.
 */
export function toFieldDescriptor(
  input: ParamInput,
  field: Field | null,
): Field {
  // A referring input has nothing to add: the field already states the type,
  // options and cardinality, and the server refuses a second declaration of them.
  if (field) return field

  const required = input.required === true || hasRequiredRule(input)
  return {
    name: input.name,
    type: input.type ?? "string",
    title: input.label,
    description: input.help,
    required,
    enum_values: input.enum_values,
    options: input.options,
    multiple: input.multiple,
    rules: input.rules,
  } as Field
}

function hasRequiredRule(input: ParamInput): boolean {
  return (input.rules ?? []).some((r) => r.name === "required")
}

/**
 * The body to send for a set of resolved inputs.
 *
 * Only declared names are emitted — the contract, not the form's incidental
 * state, decides what travels — and empty values are dropped so an untouched
 * optional input does not overwrite a stored value with `""`.
 */
export function buildActionInputBody(
  resolved: ResolvedActionInputs,
  values: Record<string, unknown>,
): Record<string, unknown> {
  const body: Record<string, unknown> = {}
  for (const { input } of resolved.inputs) {
    if (!(input.name in values)) continue
    const value = values[input.name]
    if (value === undefined || value === null || value === "") continue
    body[input.name] = value
  }
  return body
}

/** Whether an action declares anything to collect. */
export function actionNeedsInput(action: ActionSummary | undefined): boolean {
  if (!action?.params) return false
  const p = action.params
  return (p.inputs?.length ?? 0) > 0 || (p.inputs_from?.length ?? 0) > 0
}

/** The declared container hint, if any. */
export function declaredRenderMode(
  params: ParamsDecl | undefined,
): ParamsRenderHint["mode"] | undefined {
  return params?.render?.mode
}
