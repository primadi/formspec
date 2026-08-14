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
import { useSurface } from "@/hooks/useSurface"
import { z } from "zod"
import { toast } from "sonner"
import { ArrowLeft, Save, Loader2 } from "lucide-react"

import type { EntitySchema, FormSpec } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { resolveForm } from "@/engine/derive"
import { getLifecycle } from "@/engine/lifecycle"
import { evalReadonlyWhen, evalVisibleWhen, evalRequiredWhen, evalCompute } from "@/lib/formspec-expr"
import { apiGet, apiPost, apiPatch } from "@/lib/api"
import { titleCase } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { TextInput } from "@/widgets/TextInput"
import { NumberInput } from "@/widgets/NumberInput"
import { Select } from "@/widgets/Select"
import { Switch } from "@/widgets/Switch"
import { RelationPicker } from "@/widgets/RelationPicker"
import { DateInput } from "@/widgets/DateInput"
import { JsonInput } from "@/widgets/JsonInput"
import { ChildTable } from "@/widgets/ChildTable"

interface FormRendererProps {
  entity: EntitySchema
  mode: "create" | "edit" | "view"
  // Fixed record id (Page/Tab block's `form.id`, e.g. a Configuration Page's
  // singleton row) — takes precedence over the :id route param, which is
  // only ever present for the framework's derived per-entity CRUD routes.
  id?: string
  // Explicit Form name (Page/Tab block's `form.ref`) — required whenever
  // more than one authored Form targets the same entity (e.g. a
  // Configuration Page split across tabs), since the naming-convention
  // lookup in resolveForm() can only ever pick one.
  formRef?: string
  // When true, renders inside an overlay (Dialog/Sheet) instead of a
  // standalone page — suppresses its own header/back-button and calls
  // onClose after save instead of navigating away.
  inOverlay?: boolean
  // Called after successful save when inOverlay is true.
  onClose?: () => void
}

export default function FormRenderer({ entity, mode, id: fixedId, formRef, inOverlay, onClose }: FormRendererProps) {
  const navigate = useNavigate()
  const { workspace = "default", id: routeId } = useParams<{ workspace: string; id?: string }>()
  const { surfacePath } = useSurface()
  const id = fixedId ?? routeId
  const getClient = useSessionStore((s) => s.getClient)
  const me = useSessionStore((s) => s.me)
  const bundleForms = useMetaStore((s) => s.bundle?.forms ?? [])

  const authoredForms = useMemo(() => {
    const map = new Map<string, import("@/types/manifest").Entry<FormSpec>>()
    for (const t of bundleForms) {
      map.set(t.name, t as any)
    }
    return map
  }, [bundleForms])

  const formSpec = useMemo(
    () => resolveForm(entity, mode, authoredForms, formRef),
    [entity, mode, authoredForms, formRef],
  )

  const isView = mode === "view"
  const isEdit = mode === "edit"
  const lifecycle = useMemo(() => getLifecycle(entity), [entity])

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

  const { handleSubmit, formState: { errors, isSubmitting, isDirty }, reset, watch } = form
  const formValues = watch()

  // Load existing record in edit/view mode
  const [loading, setLoading] = useState(isEdit || isView)
  const [recordVersion, setRecordVersion] = useState<number | undefined>()
  const autoSaveTimer = useRef<ReturnType<typeof setTimeout>>(undefined)
  // Guards against a stale fetch (superseded by a newer load, e.g. a manual
  // reload while the initial one is still in flight) overwriting fresher data.
  const loadTokenRef = useRef(0)
  // After a failed auto-save, block further auto-saves until the user
  // manually clicks Save or the form is reset — prevents an infinite loop
  // of failed auto-save attempts (e.g. backdate policy violation).
  const autoSaveBlockedRef = useRef(false)

  const loadRecord = useCallback(async () => {
    if (!id || (!isEdit && !isView)) return
    const token = ++loadTokenRef.current
    // Any autosave still pending from before this (re)load is now stale —
    // it would otherwise fire afterwards and silently overwrite the record
    // we're about to load with edits the caller (e.g. Cancel) meant to drop.
    if (autoSaveTimer.current) clearTimeout(autoSaveTimer.current)
    setLoading(true)
    try {
      const client = getClient()
      const record = await apiGet<Record<string, unknown>>(
        client,
        `${entity.module}/${entity.name}/${id}`,
      )
      if (loadTokenRef.current !== token) return
      reset(record as FormData)
      if (typeof record.version === "number") {
        setRecordVersion(record.version)
      }
    } catch (err) {
      if (loadTokenRef.current !== token) return
      toast.error("Failed to load record")
      // A fixed-id embed has no derived list to bounce back to — stay in
      // place (route-driven forms still redirect, since the :id there
      // came from a now-presumably-invalid URL).
      if (inOverlay) {
        onClose?.()
      } else if (!fixedId) {
        navigate(surfacePath(entity.module, entity.plural))
      }
    } finally {
      if (loadTokenRef.current === token) setLoading(false)
    }
  }, [id, entity, isEdit, isView, getClient, reset, navigate, workspace, fixedId])

  useEffect(() => {
    loadRecord()
  }, [loadRecord])

  // Auto-save (debounced) for two_step_autosave lifecycle
  const autoSave = useCallback(async (data: FormData) => {
    if (!isEdit || !id) return
    try {
      const client = getClient()
      await apiPatch(
        client,
        `${entity.module}/${entity.name}/${id}`,
        data,
        recordVersion,
      )
    } catch (err: unknown) {
      autoSaveBlockedRef.current = true
      const msg =
        err instanceof Error ? err.message : "Auto-save gagal"
      toast.error(`Auto-save gagal: ${msg}`, { duration: 5000 })
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
    // `reset(record)` after load also changes formValues but clears isDirty —
    // gating on isDirty keeps a freshly-loaded record from immediately
    // triggering an autosave of the data it was just loaded with. When isDirty
    // drops back to false (that reset, or a Cancel-triggered reload), any
    // save already scheduled from edits made before the reset is now stale
    // and must be cancelled — otherwise it fires later and overwrites the
    // just-(re)loaded record with the abandoned edits.
    if (isEdit && isDirty && entity.lifecycle === "two_step_autosave") {
      // After a failed auto-save, block further auto-saves to prevent an
      // infinite loop (e.g. backdate policy violation). Unblock on reset
      // (isDirty → false) or manual Save.
      if (!autoSaveBlockedRef.current) {
        debouncedAutoSave(formValues as FormData)
      }
    } else if (autoSaveTimer.current) {
      clearTimeout(autoSaveTimer.current)
      // isDirty just became false (reset/undo) — unblock auto-save
      autoSaveBlockedRef.current = false
    }
  }, [formValues, isEdit, isDirty, entity.lifecycle, debouncedAutoSave])

  // Evaluate compute expressions whenever form values change.
  // Computed fields are auto-set by the framework — never user-editable.
  useEffect(() => {
    const needsCompute = formSpec.sections.flatMap((s) =>
      s.fields.filter((f) => f.compute),
    )
    if (needsCompute.length === 0) return

    const ctx = { fields: formValues as Record<string, unknown>, user: me }
    for (const field of needsCompute) {
      const result = evalCompute(field.compute!, ctx as any)
      if (result !== null && result !== undefined) {
        form.setValue(field.name as any, result, { shouldDirty: false })
      }
    }
  }, [formValues, formSpec.sections, form, me])

  // Submit handler
  const onSubmit = async (data: FormData) => {
    autoSaveBlockedRef.current = false // unblock auto-save on manual save
    try {
      const client = getClient()
      if (isEdit && id) {
        await apiPatch(
          client,
          `${entity.module}/${entity.name}/${id}`,
          data,
          recordVersion,
        )
        toast.success("Updated successfully")
      } else if (lifecycle.quickSubmit) {
        // one-step create-submit: POST to create-submit endpoint
        await client.post(`${entity.module}/${entity.name}/create-submit`, { json: data })
        toast.success("Created and submitted successfully")
      } else {
        await apiPost(client, `${entity.module}/${entity.name}`, data)
        toast.success("Created successfully")
      }
      // A fixed-id embed (Page/Tab block's `form.id`, e.g. a Configuration
      // Page singleton) has no derived list to return to — stay in place.
      if (inOverlay) {
        onClose?.()
      } else if (!fixedId) {
        navigate(surfacePath(entity.module, entity.plural))
      }
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
      {/* Header: suppressed in overlay mode (the Dialog/Sheet has its own
          header) and for fixed-id embeds (Page/Tab block's `form.id`). */}
      {!fixedId && !inOverlay && (
        <div className="flex items-center gap-4">
          <Button variant="ghost" size="icon" onClick={() => navigate(-1)}>
            <ArrowLeft className="size-4" />
          </Button>
          <div>
            <h1 className="text-2xl font-bold tracking-tight">
              {title} {titleCase(entity.name)}
            </h1>
          </div>
        </div>
      )}

      {/* Form */}
      <form onSubmit={handleSubmit(onSubmit)} autoComplete="off" className="space-y-8">
        {formSpec.sections
          .filter((section) => {
            // Section-level visible_when: skip invisible sections
            const ctx = { fields: formValues as Record<string, unknown>, user: me }
            return !section.visible_when || evalVisibleWhen(section.visible_when, ctx as any)
          })
          .map((section, sIdx) => (
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

                const fieldContext = { fields: formValues as Record<string, unknown>, user: me }
                const isReadonly = field.read_only ?? evalReadonlyWhen(field.readonly_when, fieldContext as any)
                const isRequired = entityField.required || evalRequiredWhen(field.required_when, fieldContext as any)
                const isVisible = field.visible_when ? evalVisibleWhen(field.visible_when, fieldContext as any) : true

                if (!isVisible) return null

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
                      onChange={(value) =>
                        form.setValue(field.name as any, value, { shouldValidate: true, shouldDirty: true })
                      }
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

        {/* Submit buttons — lifecycle-aware */}
        {!isView && (
          <div className="flex items-center gap-2">
            {/* one_step / quickSubmit: single Create-Submit button */}
            {lifecycle.quickSubmit && mode === "create" ? (
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting ? (
                  <Loader2 className="size-4 mr-1 animate-spin" />
                ) : (
                  <Save className="size-4 mr-1" />
                )}
                {entity.actions.find((a) => a.name === "create-submit")?.ui?.button_label ?? "Create & Submit"}
              </Button>
            ) : lifecycle.hasSave ? (
              /* Save / Save Draft button */
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting ? (
                  <Loader2 className="size-4 mr-1 animate-spin" />
                ) : (
                  <Save className="size-4 mr-1" />
                )}
                {lifecycle.pattern === "two_step_manual"
                  ? "Save Draft"
                  : isEdit
                    ? "Save Changes"
                    : "Create"}
              </Button>
            ) : null}

            {/* Submit button for two_step_manual / two_step_autosave */}
            {lifecycle.hasSubmit && (lifecycle.pattern === "two_step_manual" || lifecycle.pattern === "two_step_autosave") && (
              <Button
                type="button"
                variant="default"
                disabled={isSubmitting}
                onClick={async () => {
                  if (!id) return
                  try {
                    const client = getClient()
                    await client.post(`${entity.module}/${entity.name}/${id}/submit`)
                    toast.success("Submitted successfully")
                    if (inOverlay) {
                      onClose?.()
                    } else if (!fixedId) {
                      navigate(surfacePath(entity.module, entity.plural))
                    }
                  } catch (err) {
                    toast.error(err instanceof Error ? err.message : "Submit failed")
                  }
                }}
              >
                {entity.actions.find((a) => a.name === "submit")?.ui?.button_label ?? "Submit"}
              </Button>
            )}

            <Button
              type="button"
              variant="outline"
              onClick={() => {
                if (inOverlay) {
                  onClose?.()
                } else if (fixedId) {
                  loadRecord()
                } else {
                  navigate(-1)
                }
              }}
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

    case "boolean":
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

    case "datepicker":
    case "date":
    case "datetime":
      return (
        <DateInput
          value={value as string ?? ""}
          onChange={(v) => onChange(v)}
          readonly={readonly}
          withTime={entityField.type === "datetime"}
          error={error}
        />
      )

    case "json":
      return (
        <JsonInput
          value={value}
          onChange={(v) => onChange(v)}
          placeholder={field.placeholder}
          readonly={readonly}
          error={error}
        />
      )

    case "child-grid":
    case "child":
      return entityField.child ? (
        <ChildTable
          value={value as Record<string, unknown>[] | null}
          onChange={(v) => onChange(v)}
          child={entityField.child}
          currentModule={currentModule ?? ""}
          readonly={readonly}
          error={error}
        />
      ) : null

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
      schema = z.any().optional()
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

