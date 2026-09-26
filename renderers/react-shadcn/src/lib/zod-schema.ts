// ─── Zod schema builder for entity fields ───
//
// Shared by FormRenderer and the headless form engine (todo 5.9.5). Builds
// a zod schema from an entity field's type + rules (client-side validation
// for UX; the server remains authoritative).

import { z } from "zod"
import type { Field } from "@/types/manifest"
import { fieldIsUserRequired } from "@/lib/field-presence"

export function buildZodField(entityField: Field): z.ZodTypeAny {
  let schema: z.ZodTypeAny

  // The user-facing presence requirement — a server-minted key (a
  // `natural_key_rule: sequence` natural key) is satisfied by generation, so it
  // must not block an empty Create form. See `lib/field-presence.ts`.
  const required = fieldIsUserRequired(entityField)

  switch (entityField.type) {
    case "string":
      schema = z.string()
      if (required) schema = (schema as z.ZodString).min(1, "Required")
      else schema = (schema as z.ZodString).optional().or(z.literal(""))
      break
    case "integer":
      schema = z.number({ message: "Must be a number" })
      if (!required) schema = schema.nullable().optional()
      break
    case "decimal":
      schema = z.number({ message: "Must be a number" })
      if (!required) schema = schema.nullable().optional()
      break
    case "percent":
      // A percentage is numerically a decimal (S11).
      schema = z.number({ message: "Must be a number" })
      if (!required) schema = schema.nullable().optional()
      break
    case "money":
      // The canonical wire shape is `{amount, currency}` (05-field-types §2),
      // but a legacy bare number/string is still accepted at the API boundary —
      // rejecting it here would block a save the server would happily normalize.
      schema = z.union([
        z.object({
          amount: z.union([z.string(), z.number()]),
          currency: z.string().optional(),
        }),
        z.number(),
        z.string(),
      ])
      if (!required) schema = schema.nullable().optional()
      break
    case "time":
      schema = z
        .string()
        .regex(/^\d{2}:\d{2}(:\d{2})?$/, "Use HH:MM or HH:MM:SS")
      if (!required) schema = schema.optional().or(z.literal(""))
      break
    case "boolean":
      schema = z.boolean()
      if (!required) schema = schema.optional()
      break
    case "enum":
      schema = z.string()
      if (required) schema = (schema as z.ZodString).min(1, "Required")
      else schema = (schema as z.ZodString).optional().or(z.literal(""))
      break
    case "date":
    case "datetime":
      schema = z.string()
      if (!required) schema = schema.optional().or(z.literal(""))
      break
    case "relation":
      schema = z.string()
      if (required) schema = (schema as z.ZodString).min(1, "Required")
      else schema = (schema as z.ZodString).optional().or(z.literal(""))
      break
    default:
      schema = z.any().optional()
  }

  // Apply rules
  for (const rule of entityField.rules ?? []) {
    switch (rule.name) {
      case "min_length":
        if (schema instanceof z.ZodString) {
          schema = schema.min(
            rule.value as number,
            `Minimum ${rule.value} characters`,
          )
        }
        break
      case "max_length":
        if (schema instanceof z.ZodString) {
          schema = schema.max(
            rule.value as number,
            `Maximum ${rule.value} characters`,
          )
        }
        break
      case "min":
        if (schema instanceof z.ZodNumber) {
          schema = (schema as any).min(rule.value as number)
        }
        break
      case "max":
        if (schema instanceof z.ZodNumber) {
          schema = (schema as any).max(rule.value as number)
        }
        break
      case "email":
        if (schema instanceof z.ZodString) {
          schema = schema.email("Invalid email")
        }
        break
      case "pattern":
        if (schema instanceof z.ZodString && typeof rule.value === "string") {
          schema = schema.regex(new RegExp(rule.value), "Invalid format")
        }
        break
    }
  }

  return schema
}
