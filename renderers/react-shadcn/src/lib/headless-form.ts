// ─── Headless Form Engine (todo 5.9.5) ───
//
// `formspec.form(entity, { mode, id? })` — a headless form instance with
// field state, dirty tracking, client validation from field rules,
// FormSpecExpr evaluation, and submit() wired to the create/update action
// with CAS version. No layout, no widgets — the developer owns 100% of the
// markup (07-component-kinds.md §4).

import { z } from "zod"
import type { EntitySchema } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { apiGet, apiPost, apiPatch } from "@/lib/api"
import { buildZodField } from "@/lib/zod-schema"
import type { RuntimeValue } from "@/lib/formspec-expr"
import {
  evalVisibleWhen,
  evalReadonlyWhen,
  evalRequiredWhen,
  evalCompute,
} from "@/lib/formspec-expr"

export interface HeadlessFormOptions {
  mode: "create" | "edit" | "view"
  id?: string
}

export interface HeadlessForm {
  values: Record<string, unknown>
  setValue: (name: string, value: unknown) => void
  getValue: (name: string) => unknown
  isDirty: () => boolean
  isReadonly: (name: string) => boolean
  isVisible: (name: string) => boolean
  isRequired: (name: string) => boolean
  compute: (name: string) => unknown
  validate: () => Promise<Record<string, string>>
  submit: () => Promise<void>
  reset: (values?: Record<string, unknown>) => void
  load: () => Promise<void>
}

export function createHeadlessForm(
  entity: EntitySchema,
  opts: HeadlessFormOptions,
): HeadlessForm {
  const client = useSessionStore.getState().getClient()
  const path = `${entity.module}/${entity.name}${opts.id ? `/${opts.id}` : ""}`

  let values: Record<string, unknown> = {}
  let original: Record<string, unknown> = {}
  let version: number | undefined

  const ctx = () => ({
    fields: values as Record<string, RuntimeValue>,
    user: useSessionStore.getState().me as unknown as Record<string, RuntimeValue>,
  })

  const schema = z.object(
    Object.fromEntries(entity.fields.map((f) => [f.name, buildZodField(f)])),
  )

  const validate = async (): Promise<Record<string, string>> => {
    const result = schema.safeParse(values)
    if (result.success) return {}
    const errors: Record<string, string> = {}
    for (const issue of result.error.issues) {
      const key = String(issue.path[0])
      if (!errors[key]) errors[key] = issue.message
    }
    return errors
  }

  return {
    values,
    setValue: (name, value) => {
      values = { ...values, [name]: value }
    },
    getValue: (name) => values[name],
    isDirty: () => JSON.stringify(values) !== JSON.stringify(original),
    isReadonly: (name) => {
      const f = entity.fields.find((x) => x.name === name)
      if (!f) return false
      return !!f.read_only || evalReadonlyWhen(f.readonly_when, ctx())
    },
    isVisible: (name) => {
      const f = entity.fields.find((x) => x.name === name)
      if (!f) return true
      return evalVisibleWhen(f.visible_when, ctx())
    },
    isRequired: (name) => {
      const f = entity.fields.find((x) => x.name === name)
      if (!f) return false
      return !!f.required || evalRequiredWhen(f.required_when, ctx())
    },
    compute: (name) => {
      const f = entity.fields.find((x) => x.name === name)
      if (!f?.computed) return null
      return evalCompute(f.computed.formula, ctx())
    },
    validate,
    submit: async () => {
      const errors = await validate()
      if (Object.keys(errors).length > 0) {
        throw new Error(
          "Validation failed: " + Object.values(errors).join("; "),
        )
      }
      if (opts.mode === "edit" && opts.id) {
        await apiPatch(client, path, values, version)
      } else {
        await apiPost(client, path, values)
      }
      original = { ...values }
    },
    reset: (v) => {
      values = v ? { ...v } : {}
      original = { ...values }
    },
    load: async () => {
      if (opts.mode === "edit" && opts.id) {
        const rec = await apiGet<Record<string, unknown>>(client, path)
        values = { ...rec }
        original = { ...rec }
        if (typeof rec.version === "number") version = rec.version
      }
    },
  }
}
