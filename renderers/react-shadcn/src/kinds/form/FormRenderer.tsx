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
import { buildZodField } from "@/lib/zod-schema"
import { toast } from "@/lib/ui"
import { ArrowLeft, Save, Loader2, AlertTriangle } from "lucide-react"

import type { EntitySchema, FormSpec } from "@/types/manifest"
import { FormaApiError } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { resolveForm } from "@/engine/derive"
import { getLifecycle } from "@/engine/lifecycle"
import {
  evalReadonlyWhen,
  evalVisibleWhen,
  evalRequiredWhen,
  evalCompute,
  strictEvalFormSpecExpr,
} from "@/lib/formspec-expr"
import { apiGet, apiPost, apiPatch } from "@/lib/api"
import { titleCase } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import ConfirmDialog from "@/components/ui/confirm-dialog"
import { TextInput } from "@/widgets/TextInput"
import { TextareaInput } from "@/widgets/TextareaInput"
import { RichText } from "@/widgets/RichText"
import { FileInput } from "@/widgets/FileInput"
import { NumberInput } from "@/widgets/NumberInput"
import { Select } from "@/widgets/Select"
import { Switch } from "@/widgets/Switch"
import { RelationPicker } from "@/widgets/RelationPicker"
import { DateInput } from "@/widgets/DateInput"
import { JsonInput } from "@/widgets/JsonInput"
import { ChildTable } from "@/widgets/ChildTable"
import { GrantsEditor } from "@/widgets/GrantsEditor"
import { RadioGroup } from "@/widgets/RadioGroup"
import { Combobox } from "@/widgets/Combobox"
import { PasswordInput } from "@/widgets/PasswordInput"
import { SliderInput } from "@/widgets/SliderInput"
import { TagsInput } from "@/widgets/TagsInput"

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

/**
 * Strict field-expression evaluation (5.11.3). Returns the value plus an
 * `error` when the expression failed to parse or produced eval warnings —
 * the renderer surfaces that as a visible error state instead of silently
 * failing safe.
 */
function evalFieldExpr(
  expr: string | undefined,
  context: Record<string, unknown>,
): { value: unknown; error?: string } {
  if (!expr) return { value: undefined }
  const result = strictEvalFormSpecExpr(expr, context as any)
  return { value: result.value, error: result.error }
}

export default function FormRenderer({
  entity,
  mode,
  id: fixedId,
  formRef,
  inOverlay,
  onClose,
}: FormRendererProps) {
  const navigate = useNavigate()
  const { workspace = "default", id: routeId } = useParams<{
    workspace: string
    id?: string
  }>()
  const { surfacePath } = useSurface()
  const id = fixedId ?? routeId
  const getClient = useSessionStore((s) => s.getClient)
  const me = useSessionStore((s) => s.me)
  const bundleForms = useMetaStore((s) => s.bundle?.forms ?? [])
  const appName = useMetaStore((s) => s.bundle?.app.name)

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
        shape[field.name] = buildZodField(entityField)
      }
    }
    return z.object(shape)
  }, [formSpec, entity])

  type FormData = z.infer<typeof zodSchema>

  const form = useForm<FormData>({
    resolver: zodResolver(zodSchema),
    defaultValues: {},
  })

  const {
    handleSubmit,
    formState: { errors, isSubmitting, isDirty },
    reset,
    watch,
  } = form
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
  // 409 CAS conflict (someone else updated the record first): the pending
  // save the user just attempted, stashed so it can be re-applied on top of
  // the freshly-reloaded record once they confirm the reload prompt.
  const [conflictOpen, setConflictOpen] = useState(false)
  const pendingSaveRef = useRef<FormData | null>(null)

  const loadRecord = useCallback(async () => {
    if (!id || (!isEdit && !isView)) return undefined
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
      return record
    } catch (err) {
      if (loadTokenRef.current !== token) return undefined
      toast.error("Failed to load record")
      // A fixed-id embed has no derived list to bounce back to — stay in
      // place (route-driven forms still redirect, since the :id there
      // came from a now-presumably-invalid URL).
      if (inOverlay) {
        onClose?.()
      } else if (!fixedId) {
        navigate(surfacePath(entity.module, entity.plural))
      }
      return undefined
    } finally {
      if (loadTokenRef.current === token) setLoading(false)
    }
  }, [
    id,
    entity,
    isEdit,
    isView,
    getClient,
    reset,
    navigate,
    workspace,
    fixedId,
  ])

  useEffect(() => {
    loadRecord()
  }, [loadRecord])

  // 409 CAS conflict handler shared by auto-save and manual submit: the
  // record was updated by someone else since it was loaded. Stash the data
  // the user was trying to save and prompt them to reload + re-apply it.
  const handleConflict = useCallback((data: FormData) => {
    pendingSaveRef.current = data
    autoSaveBlockedRef.current = true
    setConflictOpen(true)
  }, [])

  const resolveConflict = useCallback(async () => {
    setConflictOpen(false)
    const pending = pendingSaveRef.current
    pendingSaveRef.current = null
    const record = await loadRecord()
    if (pending && record) {
      // loadRecord() already reset() the form to the fresh server record
      // and updated recordVersion — now layer the user's pending edits back
      // on top so they can review and re-save against the new version.
      reset({ ...(record as FormData), ...pending })
    }
  }, [loadRecord, reset])

  // Auto-save (debounced) for two_step_autosave lifecycle
  const autoSave = useCallback(
    async (data: FormData) => {
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
        if (err instanceof FormaApiError && err.status === 409) {
          handleConflict(data)
          return
        }
        autoSaveBlockedRef.current = true
        const msg = err instanceof Error ? err.message : "Auto-save gagal"
        toast.error(`Auto-save gagal: ${msg}`, { duration: 5000 })
      }
    },
    [isEdit, id, entity, getClient, recordVersion, handleConflict],
  )

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
    // Roles are scoped per-App (security per-App) — the form no longer asks
    // for `app`; auto-fill it from the current App context when empty.
    const payload: Record<string, unknown> = {
      ...(data as Record<string, unknown>),
    }
    if (
      entity.module === "formspec.core" &&
      entity.name === "role" &&
      appName
    ) {
      payload.app = (payload.app as string) || appName
    }
    try {
      const client = getClient()
      if (isEdit && id) {
        await apiPatch(
          client,
          `${entity.module}/${entity.name}/${id}`,
          payload,
          recordVersion,
        )
        toast.success("Updated successfully")
      } else if (lifecycle.quickSubmit) {
        // one-step create-submit: POST to create-submit endpoint
        await client.post(`${entity.module}/${entity.name}/create-submit`, {
          json: payload,
        })
        toast.success("Created and submitted successfully")
      } else {
        await apiPost(client, `${entity.module}/${entity.name}`, payload)
        toast.success("Created successfully")
      }
      // A fixed-id embed (Page/Tab block's `form.id`, e.g. a Configuration
      // Page singleton) has no derived list to return to — stay in place.
      if (inOverlay) {
        onClose?.()
      } else if (!fixedId) {
        navigate(surfacePath(entity.module, entity.plural))
      }
      // Global settings changed — refresh the meta bundle so `bundle.settings`
      // reflects the new running value (auto-apply across the whole UI).
      if (entity.module === "formspec.core" && entity.name === "app-setting") {
        useMetaStore.getState().refresh(workspace, "app")
      }
    } catch (err) {
      if (isEdit && err instanceof FormaApiError && err.status === 409) {
        handleConflict(data)
        return
      }
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
      <form
        onSubmit={handleSubmit(onSubmit)}
        autoComplete="off"
        className="space-y-8"
      >
        {formSpec.sections
          .filter((section) => {
            // Section-level visible_when: skip invisible sections
            const ctx = {
              fields: formValues as Record<string, unknown>,
              user: me,
            }
            return (
              !section.visible_when ||
              evalVisibleWhen(section.visible_when, ctx as any)
            )
          })
          .map((section, sIdx) => (
            <div key={sIdx} className="space-y-4">
              {section.title && (
                <div>
                  <h3 className="text-lg font-medium">{section.title}</h3>
                  {section.description && (
                    <p className="text-sm text-muted-foreground">
                      {section.description}
                    </p>
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
                  const entityField = entity.fields.find(
                    (f) => f.name === field.name,
                  )
                  if (!entityField) return null

                  const fieldContext = {
                    fields: formValues as Record<string, unknown>,
                    user: me,
                  }
                  // Strict eval (5.11.3): surface expression failures as a
                  // visible error state instead of silently failing safe.
                  const readonlyExpr = evalFieldExpr(
                    field.readonly_when,
                    fieldContext,
                  )
                  const requiredExpr = evalFieldExpr(
                    field.required_when,
                    fieldContext,
                  )
                  const visibleExpr = evalFieldExpr(
                    field.visible_when,
                    fieldContext,
                  )
                  const exprError =
                    readonlyExpr.error ??
                    requiredExpr.error ??
                    visibleExpr.error
                  const isReadonly =
                    field.read_only ??
                    (readonlyExpr.error
                      ? false
                      : evalReadonlyWhen(
                          field.readonly_when,
                          fieldContext as any,
                        ))
                  const isRequired =
                    entityField.required ||
                    (requiredExpr.error
                      ? false
                      : evalRequiredWhen(
                          field.required_when,
                          fieldContext as any,
                        ))
                  const isVisible = field.visible_when
                    ? visibleExpr.error
                      ? true // keep visible so the error state is shown
                      : evalVisibleWhen(field.visible_when, fieldContext as any)
                    : true

                  if (!isVisible) return null

                  return (
                    <div key={field.name} className="flex flex-col gap-2">
                      {exprError && (
                        <div
                          className="flex items-start gap-1.5 rounded-md border border-destructive/40 bg-destructive/5 px-2 py-1 text-xs text-destructive"
                          title={`FormSpecExpr error: ${exprError}`}
                        >
                          <AlertTriangle className="size-3.5 mt-0.5 shrink-0" />
                          <span>Expression error: {exprError}</span>
                        </div>
                      )}
                      <label className="text-sm font-medium leading-snug peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
                        {field.label ?? field.name}
                        {isRequired && (
                          <span className="text-destructive ml-0.5">*</span>
                        )}
                      </label>
                      <FormFieldWidget
                        field={field}
                        entityField={entityField}
                        value={formValues[field.name as keyof FormData]}
                        error={
                          errors[field.name]?.message as string | undefined
                        }
                        readonly={isReadonly || isView}
                        currentModule={entity.module}
                        entityModule={entity.module}
                        entityName={entity.name}
                        recordId={id}
                        fieldName={field.name}
                        onChange={(value) =>
                          form.setValue(field.name as any, value, {
                            shouldValidate: true,
                            shouldDirty: true,
                          })
                        }
                      />
                      {field.help && !isView && (
                        <p className="text-xs text-muted-foreground">
                          {field.help}
                        </p>
                      )}
                      {errors[field.name] && (
                        <p className="text-xs text-destructive">
                          {errors[field.name]?.message as string}
                        </p>
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
                {entity.actions.find((a) => a.name === "create-submit")?.ui
                  ?.button_label ?? "Create & Submit"}
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
            {lifecycle.hasSubmit &&
              (lifecycle.pattern === "two_step_manual" ||
                lifecycle.pattern === "two_step_autosave") && (
                <Button
                  type="button"
                  variant="default"
                  disabled={isSubmitting}
                  onClick={async () => {
                    if (!id) return
                    try {
                      const client = getClient()
                      await client.post(
                        `${entity.module}/${entity.name}/${id}/submit`,
                      )
                      toast.success("Submitted successfully")
                      if (inOverlay) {
                        onClose?.()
                      } else if (!fixedId) {
                        navigate(surfacePath(entity.module, entity.plural))
                      }
                    } catch (err) {
                      toast.error(
                        err instanceof Error ? err.message : "Submit failed",
                      )
                    }
                  }}
                >
                  {entity.actions.find((a) => a.name === "submit")?.ui
                    ?.button_label ?? "Submit"}
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

      <ConfirmDialog
        open={conflictOpen}
        onOpenChange={setConflictOpen}
        title="Conflict Detected"
        message="This record was changed by someone else since you loaded it. Reload the latest version and re-apply your changes?"
        variant="warning"
        confirmLabel="Reload & Reapply"
        cancelLabel="Keep Editing"
        onConfirm={resolveConflict}
      />
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
  entityModule,
  entityName,
  recordId,
  fieldName,
  onChange,
}: {
  field: import("@/types/manifest").FormField
  entityField: import("@/types/manifest").Field
  value: unknown
  error?: string
  readonly: boolean
  currentModule?: string
  entityModule?: string
  entityName?: string
  recordId?: string
  fieldName?: string
  onChange: (value: any) => void
}) {
  const widget = field.widget ?? entityField.type

  switch (widget) {
    case "radio-group":
      return (
        <RadioGroup
          value={(value as string) ?? ""}
          onChange={(v) => onChange(v)}
          options={entityField.enum_values ?? []}
          readonly={readonly}
          error={error}
        />
      )

    case "combobox":
      return (
        <Combobox
          value={(value as string) ?? ""}
          onChange={(v) => onChange(v)}
          options={entityField.enum_values ?? []}
          placeholder={field.placeholder}
          readonly={readonly}
          error={error}
        />
      )

    case "password":
      return (
        <PasswordInput
          value={(value as string) ?? ""}
          onChange={(v) => onChange(v)}
          placeholder={field.placeholder}
          readonly={readonly}
          error={error}
        />
      )

    case "slider":
      return (
        <SliderInput
          value={(value as number | null) ?? null}
          onChange={(v) => onChange(v)}
          min={
            entityField.rules?.find((r) => r.name === "min")?.value as
              | number
              | undefined
          }
          max={
            entityField.rules?.find((r) => r.name === "max")?.value as
              | number
              | undefined
          }
          step={
            entityField.type === "decimal" && entityField.scale
              ? 1 / Math.pow(10, entityField.scale)
              : 1
          }
          readonly={readonly}
          error={error}
        />
      )

    case "tags":
      return (
        <TagsInput
          value={(value as string) ?? ""}
          onChange={(v) => onChange(v)}
          placeholder={field.placeholder}
          readonly={readonly}
          error={error}
        />
      )
    case "input":
      return (
        <TextInput
          value={(value as string) ?? ""}
          onChange={(v) => onChange(v)}
          placeholder={field.placeholder}
          readonly={readonly}
          maxLength={
            entityField.rules?.find((r) => r.name === "max_length")?.value as
              | number
              | undefined
          }
          error={error}
        />
      )

    case "textarea":
      return (
        <TextareaInput
          value={(value as string) ?? ""}
          onChange={(v) => onChange(v)}
          placeholder={field.placeholder}
          readonly={readonly}
          maxLength={
            entityField.rules?.find((r) => r.name === "max_length")?.value as
              | number
              | undefined
          }
          error={error}
        />
      )

    case "richtext":
      return (
        <RichText
          value={(value as string) ?? ""}
          onChange={(v) => onChange(v)}
          readonly={readonly}
          error={error}
        />
      )

    case "number":
    case "integer":
    case "decimal":
    case "decimalinput":
      return (
        <NumberInput
          value={(value as number | null) ?? null}
          onChange={(v) => onChange(v)}
          placeholder={field.placeholder}
          readonly={readonly}
          integer={entityField.type === "integer"}
          scale={entityField.scale}
          min={
            entityField.rules?.find((r) => r.name === "min")?.value as
              | number
              | undefined
          }
          max={
            entityField.rules?.find((r) => r.name === "max")?.value as
              | number
              | undefined
          }
          error={error}
        />
      )

    case "enum":
    case "select":
      return (
        <Select
          value={(value as string) ?? ""}
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
          value={(value as boolean) ?? false}
          onChange={(v) => onChange(v)}
          readonly={readonly}
        />
      )

    case "uuid":
      return (
        <div className="py-1 text-sm font-mono text-muted-foreground">
          {readonly
            ? ((value as string) ?? "-")
            : ((value as string) ?? "(auto-generated)")}
        </div>
      )

    case "relation-picker":
    case "relation":
      return (
        <RelationPicker
          value={(value as string) ?? ""}
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
    case "datetimeinput":
      return (
        <DateInput
          value={(value as string) ?? ""}
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

    case "fileinput":
    case "file":
      return (
        <FileInput
          value={(value as string) ?? ""}
          onChange={(v) => onChange(v)}
          readonly={readonly}
          error={error}
          entityModule={entityModule}
          entityName={entityName}
          recordId={recordId}
          fieldName={fieldName}
          maxSizeMB={entityField.storage?.max_size_mb}
          allowedTypes={entityField.storage?.allowed_types}
        />
      )

    case "grants-editor":
      return (
        <GrantsEditor
          value={value}
          onChange={(v) => onChange(v)}
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
          value={(value as string) ?? ""}
          onChange={(v) => onChange(v)}
          placeholder={field.placeholder}
          readonly={readonly}
          error={error}
        />
      )
  }
}

// ── Zod schema builder ──
// buildZodField lives in @/lib/zod-schema (shared with the headless form
// engine, todo 5.9.5).
