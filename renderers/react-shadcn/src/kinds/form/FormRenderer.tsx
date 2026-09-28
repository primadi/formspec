// ─── Form Renderer ───
//
// react-hook-form + zod implementation for create/edit/view modes.
// Supports auto-save (debounced) for two_step_autosave lifecycle.
//
// Design doc §5.5 Form kind (F3)

import { useMemo, useState, useEffect, useCallback, useRef, useId } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useAppNavigate } from "@/lib/navigation"
import { useParams, useLocation } from "react-router-dom"
import { useSurface } from "@/hooks/useSurface"
import { z } from "zod"
import { buildZodField } from "@/lib/zod-schema"
import { fieldIsUserRequired } from "@/lib/field-presence"
import { toast } from "@/lib/ui"
import { ArrowLeft, Save, Loader2, AlertTriangle } from "lucide-react"

import type { EntitySchema, FormField, FormSpec } from "@/types/manifest"
import { FormaApiError } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { canDoEntityAction } from "@/engine/permissions"
import { useMetaStore } from "@/stores/meta"
import { resolveForm, deriveFormWidget } from "@/engine/derive"
import { useRenderContext } from "@/hooks/useRenderContext"
import { seedDefaults } from "@/lib/picker"
import { resolveSubmitRedirect } from "@/lib/submitRedirect"
import { serviceCallPath } from "@/lib/serviceCall"
import PickerPanel from "@/kinds/form/PickerPanel"
import { cn } from "@/lib/utils"
import { getLifecycle } from "@/engine/lifecycle"
import {
  evalReadonlyWhen,
  evalVisibleWhen,
  evalRequiredWhen,
  evalCompute,
  strictEvalFormSpecExpr,
} from "@/lib/formspec-expr"
import { apiGet, apiPost, apiPatch } from "@/lib/api"
import { getEntityRouteIdentifier } from "@/lib/entityIdentity"
import { useRouteIdentityStore } from "@/stores/routeIdentity"
import { interpolateConfirm, titleCase } from "@/lib/utils"
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
import { SelectMultiTag } from "@/widgets/SelectMultiTag"
import { QrCode } from "@/widgets/QrCode"
import { MoneyInput } from "@/widgets/MoneyInput"
import { TimeInput } from "@/widgets/TimeInput"
import { isFormWidget, formWidgetNames } from "@/widgets/catalog"
import { fieldOptions } from "@/lib/field-options"

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
  /** Render context resolved by the embedding block (a Page already resolved
   *  it). Merged *under* this form's own `spec.context`, so a standalone form
   *  route works too. Feeds `default_from` and picker `{token}` filters. */
  context?: Record<string, unknown>
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
  context,
}: FormRendererProps) {
  const navigate = useAppNavigate()
  const { workspace = "default", id: routeId } = useParams<{
    workspace: string
    id?: string
  }>()
  const { surfacePath } = useSurface()
  const id = fixedId ?? routeId
  const getClient = useSessionStore((s) => s.getClient)
  const setRouteIdentity = useRouteIdentityStore((s) => s.setIdentity)
  const me = useSessionStore((s) => s.me)
  const bundleForms = useMetaStore((s) => s.bundle?.forms) ?? []
  const appName = useMetaStore((s) => s.bundle?.app.name)
  const appConfirm = useMetaStore((s) => s.bundle?.app.confirm)
  // Unique prefix for field input ids so <label htmlFor> can associate with
  // each form field (a11y — avoids "no label associated with form field").
  const formIdPrefix = useId()

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

  // ── Render context ──
  // The embedding Page already resolved its `context`, so this form only
  // resolves its *own* `spec.context` declarations (the hook short-circuits on
  // empty decls) and merges them over the inherited values. A standalone form
  // route therefore works too, without anything being fetched twice.
  const routeParamsForCtx = useParams()
  const location = useLocation()
  const renderCtx = useMemo(
    () => ({
      ...(context ?? {}),
      route: { params: routeParamsForCtx, path: location.pathname },
      ...(me ? { user: me } : {}),
    }),
    [context, routeParamsForCtx, location.pathname, me],
  )
  const { context: ownCtx } = useRenderContext(formSpec.context, renderCtx, {
    publicSurface: formSpec.public === true,
  })

  // ── Per-submission minted uuid ──
  //
  // `{uuid}` is a block-local token (like `{now}`/`{today}`) that supplies a
  // fresh v4 UUID for THIS form instance. It exists for one job with two uses:
  // a create form that needs a value the *server* cannot mint — a natural key
  // whose value is random rather than a sequence (e.g. the guest token of a
  // table session) — and a `submit.redirect` that must land on that same
  // record (`/menu/{uuid}`).
  //
  // Client-generated on purpose. The server assigns the record's `id` inside
  // `store.Insert` and ignores a caller-supplied one, so a redirect cannot
  // name the id it has not read back. It does not need to: the form writes this
  // value into a `natural_key` field, and `GET /{entity}/{value}` resolves
  // through the natural-key fallback (`WHERE _<key> = ?`) — so the same token
  // works as both the payload and the address.
  const mintId = useMemo(
    () =>
      typeof crypto !== "undefined" && "randomUUID" in crypto
        ? crypto.randomUUID()
        : `${Date.now().toString(16)}-${Math.random().toString(16).slice(2)}`,
    [],
  )

  const ctx = useMemo(
    () => ({ ...renderCtx, ...ownCtx, uuid: mintId }),
    [renderCtx, ownCtx, mintId],
  )

  // ── Child-field pickers ──
  // A child field may declare a `picker` (S1): rows are picked from a source
  // entity instead of typed in one by one. `render.picker_panel: aside` gives
  // the tiles their own column and lets the panel replace the child grid.
  const pickerFields = useMemo(
    () =>
      formSpec.sections
        .flatMap((s) => s.fields)
        .map((field) => ({
          field,
          picker: entity.fields.find((f) => f.name === field.name)?.child
            ?.picker,
        }))
        .filter(
          (
            p,
          ): p is { field: FormField; picker: NonNullable<typeof p.picker> } =>
            Boolean(p.picker),
        ),
    [formSpec, entity],
  )
  const pickerAside =
    formSpec.render?.picker_panel === "aside" && pickerFields.length > 0

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
  // Pending form data awaiting the save-confirmation dialog (plan
  // confirm-dialogs.md). Non-null → the dialog is open.
  const [pendingConfirm, setPendingConfirm] = useState<FormData | null>(null)

  // Identity of the `default_from` declarations — changes when the authored
  // form changes, not on every render.
  const seedKey = useMemo(
    () =>
      JSON.stringify(
        formSpec.sections
          .flatMap((s) => s.fields)
          .filter((f) => f.default_from)
          .map((f) => [f.name, f.default_from] as const),
      ),
    [formSpec],
  )

  // ── Seed fields from the render context (`default_from`) ──
  // Values the user does not type (branch, session, timestamp) arrive here.
  // Only fields that are still unset are seeded: a loaded record always wins,
  // and a value the user has already edited is never overwritten.
  //
  // `ctx` IS a dependency, and that is load-bearing rather than incidental. A
  // form whose own `spec.context` fetches a record (`source: entity`) resolves
  // it ASYNCHRONOUSLY, so the first pass runs with the declaration still
  // pending and `{table.branch_id}` interpolates to nothing. `seedDefaults`
  // leaves an unresolvable token VERBATIM by design (a blank would be a
  // mystery 422), so the placeholder was written into the field as the literal
  // string "{table.branch_id}" and shipped to the server — measured on the
  // kafe QR check-in page: `relation branch_id points to cafe-branch
  // [{table.branch_id}], which does not exist`.
  //
  // The earlier "seed once" shape (a key of the declarations, excluding `ctx`)
  // could not recover: the effect never re-ran when the context resolved. So
  // the rule is now: re-run while a seeded value is still an UNRESOLVED
  // template, and never touch a field the user or a loaded record already
  // filled. A resolved value is never re-written, which is what keeps this from
  // fighting the user's edits.
  //
  // `seedKey` (not `formSpec.sections`) is the dependency that stands in for
  // the declarations: it IS their serialized identity, and depending on the
  // array itself would re-run on every render for no gain.
  useEffect(() => {
    if (seedKey === "[]") return
    const seeds = seedDefaults(
      formSpec.sections.flatMap((s) => s.fields),
      ctx,
    )
    for (const [name, value] of Object.entries(seeds)) {
      const current = form.getValues(name as never) as unknown
      const empty = current === undefined || current === null || current === ""
      // Still holding the unresolved placeholder from an earlier pass?
      const unresolved =
        typeof current === "string" && /^\{[\w.]+\}$/.test(current)
      if (!empty && !unresolved) continue
      form.setValue(name as never, value as never, { shouldDirty: false })
    }
    // The declaration identity stands in for `formSpec.sections` (see above).
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [seedKey, form, ctx])

  // `submit.call` — when set, the form's submit is a SERVICE call rather than
  // an entity write (kafe P3). Hoisted to component scope because both the
  // submit handler (which POSTs to the service) and the button rule (which must
  // NOT gate on the entity permission) need it.
  const submitCall = formSpec.submit?.call

  // Resolve the effective confirm message for this form's mode (plan
  // confirm-dialogs.md): form override > App default > off. Form value
  // semantics: undefined = inherit App, "" = explicitly off, non-empty =
  // custom message.
  const confirmMsg = useMemo(() => {
    const verb = isEdit ? "update" : ("create" as const)
    const formVal = formSpec.confirm?.[verb]
    const effective =
      formVal !== undefined && formVal !== null
        ? formVal
        : (appConfirm?.[verb] ?? "")
    // {name} → entity display name (plan confirm-dialogs.md)
    return interpolateConfirm(effective, entity.name)
  }, [formSpec, appConfirm, isEdit, entity.name])
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
        `${entity.module}/${entity.name}/${encodeURIComponent(id ?? "")}`,
      )
      if (loadTokenRef.current !== token) return
      reset(record as FormData)
      if (routeId && !fixedId && !inOverlay) {
        setRouteIdentity(
          location.pathname,
          getEntityRouteIdentifier(entity, record),
        )
      }
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
    routeId,
    inOverlay,
    location.pathname,
    setRouteIdentity,
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
          `${entity.module}/${entity.name}/${encodeURIComponent(id ?? "")}`,
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
  const doSubmit = async (data: FormData) => {
    autoSaveBlockedRef.current = false // unblock auto-save on manual save
    // `submit.call` — the form's submit is a SERVER OPERATION, not an entity
    // write (kafe P3). The service owns the mutation, so none of the entity
    // branches below run; the response is what the redirect interpolates.
    let serviceResponse: Record<string, unknown> = {}
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
      if (submitCall) {
        // Service call: "module.service.action" → POST /{ws}/_ui/service/...
        //
        // The form payload is sent as the params (same shape an entity create
        // sends), so a hidden field with `default_from` is how a route/session
        // value reaches a service action — no second params vocabulary.
        // `serviceCallPath` owns the `../` prefix arithmetic; it is tested
        // against a real server because getting it wrong 404s silently.
        serviceResponse =
          (await apiPost<Record<string, unknown>>(
            client,
            serviceCallPath(submitCall),
            payload,
          )) ?? {}
        toast.success(formSpec.submit?.message ?? "Submitted successfully")
      } else if (isEdit && id) {
        await apiPatch(
          client,
          `${entity.module}/${entity.name}/${encodeURIComponent(id ?? "")}`,
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
      // An authored `submit.redirect` (kafe 10.37) wins over the derived list:
      // it is how the QR gateway hands the guest to `/menu/{uuid}` after
      // creating their table session. The token is the minted uuid the form
      // wrote into the session's natural key, so the destination resolves it
      // back through the natural-key lookup — no read-back of the response.
      //
      // The value is SURFACE-relative, exactly like an authored Page's `route`
      // ("`/t/:qr_token`" lives under the App root): a manifest must not have to
      // know the workspace slug. `surfacePath` adds the prefix and collapses
      // the join, so `/menu/x` becomes `/kafe/menu/x`.
      const redirect = formSpec.submit?.redirect
      if (redirect) {
        // `{response.*}` resolves against the SERVICE RESPONSE when the submit
        // was a call — the only way to address a server-decided value (after a
        // join, the token is the EXISTING session's, which the client never
        // knew). The helper also REFUSES to navigate on an unresolved token:
        // landing on a literal `/menu/{response.x}` looks like success and is
        // a dead end, so it fails loudly instead.
        const resolved = resolveSubmitRedirect(redirect, ctx, serviceResponse)
        if (!resolved.ok) {
          toast.error(
            `Tidak bisa melanjutkan: nilai "${resolved.unresolved}" tidak ada di respons.`,
          )
          return
        }
        navigate(surfacePath(resolved.target.replace(/^\/+/, "")))
      } else if (inOverlay) {
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

  // Submit entry point — intercepts with the confirmation dialog when a
  // confirm message resolves (form override or App default). Validation has
  // already passed by the time this runs (handleSubmit), so the dialog only
  // appears for valid data.
  const onSubmit = (data: FormData) => {
    if (confirmMsg) {
      setPendingConfirm(data)
      return
    }
    return doSubmit(data)
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
        <div
          className={cn(
            pickerAside && "grid gap-6 lg:grid-cols-[1fr_22rem] lg:items-start",
          )}
        >
          {/* Aside mode: the pickers get their own column, so the catalog is the
              main surface and the picked rows + submit sit beside it. */}
          {pickerAside && (
            <div className="flex flex-col gap-8">
              {pickerFields.map(({ field, picker }) => (
                <PickerPanel
                  key={field.name}
                  decl={picker}
                  module={entity.module}
                  value={
                    formValues[field.name as keyof FormData] as
                      | Record<string, unknown>[]
                      | undefined
                  }
                  context={ctx}
                  showSelection
                  title={field.label ?? field.name}
                  onChange={(rows) =>
                    form.setValue(field.name as never, rows as never, {
                      shouldValidate: true,
                      shouldDirty: true,
                    })
                  }
                />
              ))}
            </div>
          )}
          <div className="space-y-8">
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
                      // Stable per-field id so the <label> below can associate
                      // with the input (a11y — no orphan form fields).
                      const fieldId = `form-${formIdPrefix}-${field.name}`
                      const entityField = entity.fields.find(
                        (f) => f.name === field.name,
                      )
                      if (!entityField) return null

                      // `widget: hidden` carries a value (usually seeded by
                      // `default_from`) without rendering anything — the branch,
                      // session or timestamp a public surface must not show.
                      if (field.widget === "hidden") return null

                      // Aside mode: this field's picker owns the editing UI in its
                      // own column, so the child grid is not rendered twice.
                      if (pickerAside && entityField.child?.picker) return null

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
                        fieldIsUserRequired(entityField) ||
                        (requiredExpr.error
                          ? false
                          : evalRequiredWhen(
                              field.required_when,
                              fieldContext as any,
                            ))
                      const isVisible = field.visible_when
                        ? visibleExpr.error
                          ? true // keep visible so the error state is shown
                          : evalVisibleWhen(
                              field.visible_when,
                              fieldContext as any,
                            )
                        : true

                      if (!isVisible) return null

                      // Only widgets that render a native labelable element
                      // (<input>/<textarea>) may be the target of <label htmlFor>.
                      // Button-based custom controls (select, switch, combobox,
                      // radio-group) and composite widgets (child-grid) get their
                      // accessible name via aria-label / internal labels instead —
                      // pointing <label for> at a <button> or a missing id is
                      // flagged as incorrect by a11y checkers. Readonly fields
                      // render a display <div>, not an input, so they are not
                      // labelable either.
                      const widgetName = field.widget ?? entityField.type
                      const isLabelable =
                        !isReadonly &&
                        !isView &&
                        LABELABLE_WIDGETS.has(widgetName)

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
                          {isLabelable ? (
                            <label
                              htmlFor={fieldId}
                              className="text-sm font-medium leading-snug peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
                            >
                              {field.label ?? field.name}
                              {isRequired && (
                                <span className="text-destructive ml-0.5">
                                  *
                                </span>
                              )}
                            </label>
                          ) : (
                            <span className="text-sm font-medium leading-snug peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
                              {field.label ?? field.name}
                              {isRequired && (
                                <span className="text-destructive ml-0.5">
                                  *
                                </span>
                              )}
                            </span>
                          )}
                          {entityField.child?.picker && (
                            <PickerPanel
                              decl={entityField.child.picker}
                              module={entity.module}
                              value={
                                formValues[field.name as keyof FormData] as
                                  | Record<string, unknown>[]
                                  | undefined
                              }
                              context={ctx}
                              showSelection={false}
                              onChange={(rows) =>
                                form.setValue(
                                  field.name as never,
                                  rows as never,
                                  {
                                    shouldValidate: true,
                                    shouldDirty: true,
                                  },
                                )
                              }
                            />
                          )}
                          <FormFieldWidget
                            field={field}
                            entityField={entityField}
                            id={fieldId}
                            label={field.label ?? field.name}
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
          </div>
        </div>

        {/* Submit buttons — lifecycle-aware. A button the caller cannot use is
            not rendered at all: offering it would only produce a toast/403
            after the click, and the server is the authority either way.

            A `submit.call` form is the deliberate exception. The gate below
            checks the ENTITY's create/update permission, and such a form does
            not write its entity — the service does. Checking the entity
            permission would be checking the wrong thing, and it fails
            outright on the flow this exists for: the guest check-in runs
            anonymously, and once the service owns the write the anonymous
            `create` grant on `table-session` is removed (that grant was the
            original 10.34 complaint — anonymous create with no proof of
            presence). So the client has no service metadata to substitute, and
            the honest position is: render, let the service's own gate decide.

            The cost is real and accepted: on a private form calling a
            restricted service, the button shows and the server answers 403.
            Closing that means shipping service metadata (name + permission +
            public flag) in the meta bundle so the client can pre-check it —
            recorded as a follow-up, not silently ignored. */}
        {!isView && (
          <div className="flex items-center gap-2">
            {/* one_step / quickSubmit: single Create-Submit button */}
            {!submitCall &&
            lifecycle.quickSubmit &&
            mode === "create" &&
            canDoEntityAction(me, entity, "create-submit") ? (
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting ? (
                  <Loader2 className="size-4 mr-1 animate-spin" />
                ) : (
                  <Save className="size-4 mr-1" />
                )}
                {entity.actions.find((a) => a.name === "create-submit")?.ui
                  ?.button_label ?? "Create & Submit"}
              </Button>
            ) : submitCall ||
              (lifecycle.hasSave &&
                canDoEntityAction(me, entity, isEdit ? "update" : "create")) ? (
              /* Save / Save Draft button */
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting ? (
                  <Loader2 className="size-4 mr-1 animate-spin" />
                ) : (
                  <Save className="size-4 mr-1" />
                )}
                {/* An authored `submit.label` wins — it is the only way a
                    manifest can name the action the way its user thinks of it
                    ("Lihat Menu", not "Create"). */}
                {formSpec.submit?.label ??
                  (lifecycle.pattern === "two_step_manual"
                    ? "Save Draft"
                    : isEdit
                      ? "Save Changes"
                      : "Create")}
              </Button>
            ) : null}

            {/* Submit button for two_step_manual / two_step_autosave */}
            {lifecycle.hasSubmit &&
              (lifecycle.pattern === "two_step_manual" ||
                lifecycle.pattern === "two_step_autosave") &&
              canDoEntityAction(me, entity, "submit") && (
                <Button
                  type="button"
                  variant="default"
                  disabled={isSubmitting}
                  onClick={async () => {
                    if (!id) return
                    try {
                      const client = getClient()
                      await client.post(
                        `${entity.module}/${entity.name}/${encodeURIComponent(id ?? "")}/submit`,
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

      <ConfirmDialog
        open={pendingConfirm !== null}
        onOpenChange={(open) => {
          if (!open) setPendingConfirm(null)
        }}
        title={isEdit ? "Simpan Perubahan" : "Simpan Data"}
        message={confirmMsg}
        confirmLabel="Simpan"
        cancelLabel="Batal"
        onConfirm={() => {
          const data = pendingConfirm
          setPendingConfirm(null)
          if (data) void doSubmit(data)
        }}
      />
    </div>
  )
}

// ── Field Widget Router ──

// ── Label association ──
// Widgets that render a native labelable element (<input>/<textarea>) and can
// therefore be targeted by <label htmlFor>. Everything else (select/switch/
// combobox/radio-group trigger buttons, child-grid, uuid, grants-editor) is
// labeled via aria-label or internal labels instead.
const LABELABLE_WIDGETS = new Set([
  "string",
  "text",
  "input",
  "textarea",
  "integer",
  "decimal",
  "number",
  "decimalinput",
  "date",
  "datetime",
  "datepicker",
  "datetimeinput",
  "json",
  "slider",
  "tags",
  "password",
  "relation",
  "relation-picker",
])

// ── Unknown widget ──
//
// `formspec validate` already rejects an unknown `widget:` (closed set, S10), so
// reaching this component means a stale spec cache or a hand-built manifest.
// Render an explicit error instead of a plausible-looking text input: a silently
// ignored widget is what made a typo indistinguishable from an unimplemented
// feature.
function UnknownWidget({
  widget,
  allowed,
}: {
  widget: string
  allowed: string
}) {
  return (
    <div
      className="flex items-start gap-1.5 rounded-md border border-destructive/40 bg-destructive/5 px-2 py-1 text-xs text-destructive"
      title={`Unknown widget: ${widget}. Allowed: ${allowed}`}
      role="alert"
    >
      <AlertTriangle className="size-3.5 mt-0.5 shrink-0" />
      <span>
        Unknown widget <code className="font-mono">{widget}</code> — this field
        is not rendered with a known widget.
      </span>
    </div>
  )
}

// FormFieldWidget routes one field to its widget. Exported for the widget
// catalog parity test (src/widgets/catalog.test.tsx), which asserts every
// catalogued widget name actually renders here.
//
// When a manifest form field declares no `widget:`, the widget comes from
// `deriveFormWidget` — the SAME function the derived Form uses. That is what
// makes the Form follow the Entity: cardinality (`multiple` + `options` on the
// Entity) decides between the set picker and the single-value picker, and an
// authored `kind: Form` no longer has to restate the decision as
// `widget: select-multi-tag` (or silently render a raw JSON editor because it
// did not).
//
// Before this, only `money`/`time` were mapped for manifest forms while every
// other type fell through to the type name — so the two paths disagreed, and
// the disagreement is visible: the same field rendered a tag picker in a
// derived form and a JSON editor in an authored one.
function implicitWidgetForType(
  field: import("@/types/manifest").Field,
): string {
  return deriveFormWidget(field)
}

export function FormFieldWidget({
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
  id,
  label,
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
  /** id of the rendered input — matches the <label htmlFor> above */
  id?: string
  /** Field label, used as accessible name for button-based widgets */
  label?: string
  onChange: (value: any) => void
}) {
  // An explicit manifest `widget:` must be a member of the closed catalog (S10).
  // Before this guard a typo (`widget: relaion-picker`) silently rendered a
  // plain text input, which is indistinguishable from "the widget does not
  // exist yet". The fall-through below (`?? entityField.type`) is different: a
  // field *type* with no dedicated widget (`money`, `time`) legitimately
  // renders the default input, and that stays silent until MoneyInput/TimeInput
  // land.
  if (field.widget && !isFormWidget(field.widget)) {
    return <UnknownWidget widget={field.widget} allowed={formWidgetNames()} />
  }

  const widget = field.widget ?? implicitWidgetForType(entityField)

  switch (widget) {
    // Renders nothing but keeps the value in form state (the field loop skips
    // it entirely, so this is the router's own guarantee).
    case "hidden":
      return null

    // Read-only by nature: the value is the payload (a token or URL), so a QR
    // field shows the code instead of an input the user could type into.
    case "qrcode":
      return <QrCode value={(value as string) ?? ""} size={160} />

    // Money keeps its currency and is edited as an amount (numpad on touch).
    case "moneyinput":
      return (
        <MoneyInput
          value={value}
          onChange={onChange}
          readonly={readonly}
          error={error}
          id={id}
          currency={entityField.currency}
          decimalPlaces={entityField.decimal_places}
        />
      )

    case "timeinput":
      return (
        <TimeInput
          value={value}
          onChange={onChange}
          readonly={readonly}
          error={error}
          id={id}
        />
      )

    case "radio-group":
      return (
        <RadioGroup
          value={value}
          onChange={(v) => onChange(v)}
          // Declared captions when the field has a choice set (`options`), else
          // the bare `enum_values` — the same vocabulary `select` reads, so the
          // three single-value pickers cannot disagree about a field's captions.
          options={fieldOptions(entityField)}
          readonly={readonly}
          error={error}
          label={label}
        />
      )

    case "combobox":
      return (
        <Combobox
          value={value}
          onChange={(v) => onChange(v)}
          options={fieldOptions(entityField)}
          placeholder={field.placeholder}
          readonly={readonly}
          error={error}
          label={label}
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
          id={id}
          name={fieldName}
          // An entity password field always *sets* a password (create or
          // change) — never a login credential. Without this token Chrome
          // treats it as one and offers to save the wrong thing.
          autoComplete="new-password"
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
          id={id}
        />
      )

    case "tags": {
      // Pass the value as-is: string fields keep the comma-separated shape,
      // json fields keep their array shape (TagsInput is value-type-aware).
      // For json fields force array shape even when empty (create mode),
      // so a string never leaks into a json column.
      const isJsonField = entityField.type === "json"
      const tagsValue = isJsonField
        ? ((value as string[] | undefined) ?? [])
        : ((value as string | undefined) ?? "")
      return (
        <TagsInput
          value={tagsValue}
          onChange={(v) => onChange(v)}
          placeholder={field.placeholder}
          readonly={readonly}
          error={error}
          id={id}
        />
      )
    }

    case "select-multi-tag": {
      // Tags chosen from a declared set (`Field.options`, else `enum_values`)
      // rather than typed. The widget owns the value-shape contract, which now
      // comes from the Entity's cardinality (`Field.multiple`) instead of from
      // the field type — so a set on `string` is comma-separated, a set on
      // `json` an array. The value is passed through as-is.
      return (
        <SelectMultiTag
          value={value}
          onChange={(v) => onChange(v)}
          entityField={entityField}
          placeholder={field.placeholder}
          readonly={readonly}
          error={error}
          id={id}
          ariaLabel={label}
        />
      )
    }

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
          id={id}
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
          id={id}
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
          id={id}
        />
      )

    case "enum":
    case "select":
      return (
        <Select
          value={value}
          onChange={(v) => onChange(v)}
          options={fieldOptions(entityField)}
          placeholder={field.placeholder}
          readonly={readonly}
          error={error}
          id={id}
          ariaLabel={label}
        />
      )

    case "boolean":
    case "switch":
      return (
        <Switch
          value={(value as boolean) ?? false}
          onChange={(v) => onChange(v)}
          readonly={readonly}
          ariaLabel={label}
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
          id={id}
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
          id={id}
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
          id={id}
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
          id={id}
        />
      )
  }
}

// ── Zod schema builder ──
// buildZodField lives in @/lib/zod-schema (shared with the headless form
// engine, todo 5.9.5).
