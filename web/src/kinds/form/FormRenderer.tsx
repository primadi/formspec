// ─── Form Renderer ───
//
// react-hook-form + zod implementation for create/edit/view modes.
// Supports auto-save (debounced) for two_step_autosave lifecycle.
//
// Design doc §5.5 Form kind (F3)

import { useMemo, useState, useEffect, useCallback, useRef } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useNavigate, useParams } from "react-router-dom"
import { z } from "zod"
import { toast } from "sonner"
import { ArrowLeft, Save, Loader2 } from "lucide-react"

import type { EntitySchema, FormSpec } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { resolveForm } from "@/engine/derive"
import { evalReadonlyWhen } from "@/lib/formaexpr"
import { apiGet, apiPost, apiPut } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { TextInput } from "@/widgets/TextInput"
import { NumberInput } from "@/widgets/NumberInput"
import { Select } from "@/widgets/Select"
import { Switch } from "@/widgets/Switch"
import { RelationPicker } from "@/widgets/RelationPicker"

interface FormRendererProps {
  entity: EntitySchema
  mode: "create" | "edit" | "view"
}

export default function FormRenderer({ entity, mode }: FormRendererProps) {
  const navigate = useNavigate()
  const { workspace = "default", id } = useParams<{ workspace: string; id?: string }>()
  const getClient = useSessionStore((s) => s.getClient)
  const bundleForms = useMetaStore((s) => s.bundle?.forms ?? [])

  const authoredForms = useMemo(() => {
    const map = new Map<string, import("@/types/manifest").Entry<FormSpec>>()
    for (const t of bundleForms) {
      map.set(t.name, t as any)
    }
    return map
  }, [bundleForms])

  const formSpec = useMemo(
    () => resolveForm(entity, mode, authoredForms),
    [entity, mode, authoredForms],
  )

  const isView = mode === "view"
  const isEdit = mode === "edit"

  // Build zod schema from fields
  const zodSchema = useMemo(() => {
    const shape: Record<string, z.ZodTypeAny> = {}
    for (const section of formSpec.sections) {
      for (const field of section.fields) {
        const entityField = entity.fields.find((f) => f.name === field.name)
        if (!entityField) continue
        shape[field.name] = buildZodField(entityField, field)
      }
    }
    return z.object(shape)
  }, [formSpec, entity])

  type FormData = z.infer<typeof zodSchema>

  const form = useForm<FormData>({
    resolver: zodResolver(zodSchema),
    defaultValues: {},
  })

  const { handleSubmit, formState: { errors, isSubmitting }, reset, watch } = form
  const formValues = watch()

  // Load existing record in edit/view mode
  const [loading, setLoading] = useState(isEdit || isView)
  const [recordVersion, setRecordVersion] = useState<number | undefined>()
  const autoSaveTimer = useRef<ReturnType<typeof setTimeout>>(undefined)

  useEffect(() => {
    if (!id || (!isEdit && !isView)) return
    const loadRecord = async () => {
      try {
        const client = getClient()
        const record = await apiGet<Record<string, unknown>>(
          client,
          `${entity.module}/${entity.plural}/${id}`,
        )
        reset(record as FormData)
        if (typeof record.version === "number") {
          setRecordVersion(record.version)
        }
      } catch (err) {
        toast.error("Failed to load record")
        navigate(`/${workspace}/_admin/${entity.module}/${entity.plural}`)
      } finally {
        setLoading(false)
      }
    }
    loadRecord()
  }, [id, entity, isEdit, isView, getClient, reset, navigate, workspace])

  // Auto-save (debounced) for two_step_autosave lifecycle
  const autoSave = useCallback(async (data: FormData) => {
    if (!isEdit || !id) return
    try {
      const client = getClient()
      await apiPut(
        client,
        `${entity.module}/${entity.plural}/${id}`,
        data,
        recordVersion,
      )
    } catch {
      // Silent fail for auto-save
    }
  }, [isEdit, id, entity, getClient, recordVersion])

  const debouncedAutoSave = useCallback(
    (data: FormData) => {
      if (autoSaveTimer.current) clearTimeout(autoSaveTimer.current)
      autoSaveTimer.current = setTimeout(() => autoSave(data), 2000)
    },
    [autoSave],
  )

  useEffect(() => {
    if (isEdit && entity.lifecycle === "two_step_autosave") {
      debouncedAutoSave(formValues as FormData)
    }
  }, [formValues, isEdit, entity.lifecycle, debouncedAutoSave])

  // Submit handler
  const onSubmit = async (data: FormData) => {
    try {
      const client = getClient()
      if (isEdit && id) {
        await apiPut(
          client,
          `${entity.module}/${entity.plural}/${id}`,
          data,
          recordVersion,
        )
        toast.success("Updated successfully")
      } else {
        await apiPost(client, `${entity.module}/${entity.plural}`, data)
        toast.success("Created successfully")
      }
      navigate(`/${workspace}/_admin/${entity.module}/${entity.plural}`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Save failed")
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center p-8">
        <Loader2 className="size-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  const title = isView ? "View" : isEdit ? "Edit" : "Create"

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" onClick={() => navigate(-1)}>
          <ArrowLeft className="size-4" />
        </Button>
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            {title} {entity.name.charAt(0).toUpperCase() + entity.name.slice(1)}
          </h1>
        </div>
      </div>

      {/* Form */}
      <form onSubmit={handleSubmit(onSubmit)} autoComplete="off" className="space-y-8">
        {formSpec.sections.map((section, sIdx) => (
          <div key={sIdx} className="space-y-4">
            {section.title && (
              <div>
                <h3 className="text-lg font-medium">{section.title}</h3>
                {section.description && (
                  <p className="text-sm text-muted-foreground">{section.description}</p>
                )}
              </div>
            )}

            <div
              className="grid gap-4"
              style={{
                gridTemplateColumns: `repeat(${section.columns || 1}, 1fr)`,
              }}
            >
              {section.fields.map((field) => {
                const entityField = entity.fields.find((f) => f.name === field.name)
                if (!entityField) return null

                const fieldContext = { fields: formValues as Record<string, unknown> }
                const isReadonly = field.read_only ?? evalReadonlyWhen(field.readonly_when, fieldContext as any)
                const isRequired = entityField.required ?? false

                return (
                  <div key={field.name} className="space-y-2.5">
                    <label className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
                      {field.label ?? field.name}
                      {isRequired && <span className="text-destructive ml-0.5">*</span>}
                    </label>
                    <FormFieldWidget
                      field={field}
                      entityField={entityField}
                      value={formValues[field.name as keyof FormData]}
                      error={errors[field.name]?.message as string | undefined}
                      readonly={isReadonly || isView}
                      currentModule={entity.module}
                      onChange={(value) => form.setValue(field.name as any, value, { shouldValidate: true })}
                    />
                    {field.help && !isView && (
                      <p className="text-xs text-muted-foreground">{field.help}</p>
                    )}
                    {errors[field.name] && (
                      <p className="text-xs text-destructive">{errors[field.name]?.message as string}</p>
                    )}
                  </div>
                )
              })}
            </div>
          </div>
        ))}

        {/* Submit buttons */}
        {!isView && (
          <div className="flex items-center gap-2">
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? (
                <Loader2 className="size-4 mr-1 animate-spin" />
              ) : (
                <Save className="size-4 mr-1" />
              )}
              {isEdit ? "Save Changes" : "Create"}
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={() => navigate(-1)}
            >
              Cancel
            </Button>
          </div>
        )}
      </form>
    </div>
  )
}

// ── Field Widget Router ──

function FormFieldWidget({
  field,
  entityField,
  value,
  error,
  readonly,
  currentModule,
  onChange,
}: {
  field: import("@/types/manifest").FormField
  entityField: import("@/types/manifest").Field
  value: unknown
  error?: string
  readonly: boolean
  currentModule?: string
  onChange: (value: any) => void
}) {
  const widget = field.widget ?? entityField.type

  switch (widget) {
    case "input":
    case "textarea":
      return (
        <TextInput
          value={value as string ?? ""}
          onChange={(v) => onChange(v)}
          placeholder={field.placeholder}
          readonly={readonly}
          maxLength={
            entityField.rules?.find((r) => r.name === "max_length")?.value as number | undefined
          }
          error={error}
        />
      )

    case "number":
      return (
        <NumberInput
          value={value as number | null ?? null}
          onChange={(v) => onChange(v)}
          placeholder={field.placeholder}
          readonly={readonly}
          min={entityField.rules?.find((r) => r.name === "min")?.value as number | undefined}
          max={entityField.rules?.find((r) => r.name === "max")?.value as number | undefined}
          step={entityField.type === "integer" ? 1 : undefined}
          error={error}
        />
      )

    case "enum":
    case "select":
      return (
        <Select
          value={value as string ?? ""}
          onChange={(v) => onChange(v)}
          options={entityField.enum_values ?? []}
          placeholder={field.placeholder}
          readonly={readonly}
          error={error}
        />
      )

    case "switch":
      return (
        <Switch
          value={value as boolean ?? false}
          onChange={(v) => onChange(v)}
          readonly={readonly}
        />
      )

    case "uuid":
      return (
        <div className="py-1 text-sm font-mono text-muted-foreground">
          {readonly ? (value as string) ?? "-" : (value as string) ?? "(auto-generated)"}
        </div>
      )

    case "relation-picker":
    case "relation":
      return (
        <RelationPicker
          value={value as string ?? ""}
          onChange={(v) => onChange(v)}
          entityField={entityField}
          currentModule={currentModule ?? ""}
          placeholder={field.placeholder}
          readonly={readonly}
          error={error}
        />
      )

    default:
      return (
        <TextInput
          value={value as string ?? ""}
          onChange={(v) => onChange(v)}
          placeholder={field.placeholder}
          readonly={readonly}
          error={error}
        />
      )
  }
}

// ── Zod schema builder ──

function buildZodField(
  entityField: import("@/types/manifest").Field,
  _formField: import("@/types/manifest").FormField,
): z.ZodTypeAny {
  let schema: z.ZodTypeAny

  switch (entityField.type) {
    case "string":
      schema = z.string()
      if (entityField.required) schema = (schema as z.ZodString).min(1, "Required")
      else schema = (schema as z.ZodString).optional().or(z.literal(""))
      break
    case "integer":
      schema = z.number({ message: "Must be a number" })
      if (!entityField.required) schema = schema.nullable().optional()
      break
    case "decimal":
      schema = z.number({ message: "Must be a number" })
      if (!entityField.required) schema = schema.nullable().optional()
      break
    case "boolean":
      schema = z.boolean()
      if (!entityField.required) schema = schema.optional()
      break
    case "enum":
      schema = z.string()
      if (entityField.required) schema = (schema as z.ZodString).min(1, "Required")
      else schema = (schema as z.ZodString).optional().or(z.literal(""))
      break
    case "date":
    case "datetime":
      schema = z.string()
      if (!entityField.required) schema = schema.optional().or(z.literal(""))
      break
    case "relation":
      schema = z.string()
      if (entityField.required) schema = (schema as z.ZodString).min(1, "Required")
      else schema = (schema as z.ZodString).optional().or(z.literal(""))
      break
    default:
      schema = z.any()
  }

  // Apply rules
  for (const rule of entityField.rules ?? []) {
    switch (rule.name) {
      case "min_length":
        if (schema instanceof z.ZodString) {
          schema = schema.min(rule.value as number, `Minimum ${rule.value} characters`)
        }
        break
      case "max_length":
        if (schema instanceof z.ZodString) {
          schema = schema.max(rule.value as number, `Maximum ${rule.value} characters`)
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

