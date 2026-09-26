// ─── Detail Page ───
//
// Readonly field grid + child tables + state machine transition buttons
// + lifecycle patterns (Frontend §1.7).
//
// Design doc §5.5 Detail page (F3)

import { useEffect, useState, useMemo } from "react"
import { useAppNavigate } from "@/lib/navigation"
import { useLocation, useParams } from "react-router-dom"
import { useSurface } from "@/hooks/useSurface"
import { toast } from "@/lib/ui"
import { ArrowLeft, Edit, FileText, Loader2 } from "lucide-react"

import type { EntitySchema } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { canDoEntityAction } from "@/engine/permissions"
import { resolveEntityRef } from "@/engine/entityRef"
import { deriveDetailFields, entityFieldLabel } from "@/engine/derive"
import { getLifecycle, getAvailableTransitions } from "@/engine/lifecycle"
import { apiGet } from "@/lib/api"
import { titleCase } from "@/lib/utils"
import { createFormatter, moneyAmount, type Formatter } from "@/lib/format"
import { isImageFile } from "@/lib/media"
import { sanitizeHTML } from "@/lib/sanitize"
import { Badge } from "@/widgets/Badge"
import { OptionChips } from "@/widgets/SelectMultiTag"
import { hasFieldOptions } from "@/lib/field-options"
import { renderOptionCell } from "@/lib/renderCell"
import { Button } from "@/components/ui/button"
import ConfirmDialog from "@/components/ui/confirm-dialog"
import ImageLightbox from "@/components/ui/image-lightbox"
import { getEntityRouteIdentifier } from "@/lib/entityIdentity"
import { useRouteIdentityStore } from "@/stores/routeIdentity"

interface DetailPageProps {
  entity: EntitySchema
}

export default function DetailPage({ entity }: DetailPageProps) {
  const navigate = useAppNavigate()
  const location = useLocation()
  const { workspace = "default", id } = useParams<{
    workspace: string
    id: string
  }>()
  const { surfacePath } = useSurface()
  const me = useSessionStore((s) => s.me)
  const getClient = useSessionStore((s) => s.getClient)
  const setRouteIdentity = useRouteIdentityStore((s) => s.setIdentity)

  const { mainFields, childFields } = useMemo(
    () => deriveDetailFields(entity),
    [entity],
  )
  const lifecycle = useMemo(() => getLifecycle(entity), [entity])

  const [record, setRecord] = useState<Record<string, unknown> | null>(null)
  const [loading, setLoading] = useState(true)
  const [transitioning, setTransitioning] = useState<string | null>(null)
  const [pendingTransition, setPendingTransition] = useState<{
    action: string
    label: string
    confirm: string
  } | null>(null)

  useEffect(() => {
    const loadRecord = async () => {
      try {
        const client = getClient()
        const data = await apiGet<Record<string, unknown>>(
          client,
          `${entity.module}/${entity.name}/${encodeURIComponent(id ?? "")}`,
        )
        setRecord(data)
        setRouteIdentity(location.pathname, getEntityRouteIdentifier(entity, data))
      } catch (err) {
        toast.error("Failed to load record")
        navigate(surfacePath(entity.module, entity.plural))
      } finally {
        setLoading(false)
      }
    }
    loadRecord()
  }, [id, entity, getClient, navigate, workspace, location.pathname, setRouteIdentity])

  // All entity schemas from the meta bundle — used to resolve relation display fields
  // Select the raw value (stable reference) and fall back outside the selector
  // — a `?? []` inside would return a fresh array every render and loop forever.
  const entities = useMetaStore((s) => s.bundle?.entities) ?? []
  const settings = useMetaStore((s) => s.bundle?.settings)
  const formatter = useMemo(() => createFormatter(settings), [settings])

  const currentState = record?.[entity.state_machine?.field ?? ""] as
    | string
    | undefined
  const transitions = useMemo(
    () => (currentState ? getAvailableTransitions(entity, currentState) : []),
    [entity, currentState],
  )

  // Transitions the caller may not perform are hidden entirely — the click-time
  // check below stays as a backstop, but offering a button that can only fail
  // is worse UX than not showing it.
  const visibleTransitions = useMemo(
    () => transitions.filter((t) => canDoEntityAction(me, entity, t.action)),
    [transitions, me, entity],
  )

  const handleTransition = async (action: string, skipConfirm = false) => {
    if (!me) return
    if (!canDoEntityAction(me, entity, action)) {
      toast.error("You don't have permission")
      return
    }

    // Find the transition so its confirm message can be shown first.
    const transition = transitions.find((t) => t.action === action)
    if (transition?.confirm && !skipConfirm) {
      setPendingTransition({
        action,
        label: transition.label,
        confirm: transition.confirm,
      })
      return
    }

    setTransitioning(action)
    try {
      const client = getClient()
      await client.post(
        `${entity.module}/${entity.name}/${encodeURIComponent(id ?? "")}/${action}`,
      )
      toast.success("State updated")
      // Reload
      const data = await apiGet<Record<string, unknown>>(
        client,
        `${entity.module}/${entity.name}/${encodeURIComponent(id ?? "")}`,
      )
      setRecord(data)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Transition failed")
    } finally {
      setTransitioning(null)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center p-8">
        <Loader2 className="size-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (!record) return null

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" onClick={() => navigate(-1)}>
          <ArrowLeft className="size-4" />
        </Button>
        <div className="flex-1">
          <h1 className="text-2xl font-bold tracking-tight">
            {titleCase(entity.name)}
          </h1>
          {entity.state_machine && currentState && (
            <Badge value={currentState} />
          )}
        </div>

        {/* The button navigates to the guarded edit route, so it must use the
            same authorization as that route — otherwise the detail page offers
            an edit affordance whose target answers "Page not found". */}
        {lifecycle.hasSave && id && canDoEntityAction(me, entity, "update") && (
          <Button
            variant="outline"
            onClick={() =>
              navigate(
                surfacePath(
                  entity.module,
                  entity.plural,
                  encodeURIComponent(
                    getEntityRouteIdentifier(entity, { ...record, id }),
                  ),
                  "edit",
                ),
              )
            }
          >
            <Edit className="size-4 mr-1" />
            Edit
          </Button>
        )}
      </div>

      {/* Field Grid */}
      <div className="rounded-md border">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 p-6">
          {mainFields.map((field) => {
            // For relation fields, show the resolved display name using the
            // related entity's label_field (display_field in YAML), instead of
            // the raw UUID. The API returns a nested resolved object under the
            // alias (e.g. polyclinic_id → polyclinic: {id, name, ...}).
            let value = record[field.name]
            const fieldLabel = entityFieldLabel(entity, field.name)

            if (
              field.type === "relation" &&
              typeof value === "string" &&
              value.length === 36 // UUID length heuristic
            ) {
              const alias = field.name.endsWith("_id")
                ? field.name.slice(0, -3)
                : (field.relation?.resource ?? field.name)
              const resolved = record[alias]
              if (
                resolved &&
                typeof resolved === "object" &&
                !Array.isArray(resolved)
              ) {
                // Determine the related entity's label_field.
                // relation.resource can be "entity" or "module.entity" (cross-module).
                const resourceName = field.relation?.resource ?? alias
                const [relModule, relName] = resolveEntityRef(resourceName, "")
                const relatedEntity = relModule
                  ? entities.find(
                      (e) => e.module === relModule && e.name === relName,
                    )
                  : entities.find((e) => e.name === relName)
                const displayField = relatedEntity?.label_field ?? "name"
                const displayName =
                  (resolved as Record<string, unknown>)[displayField] ??
                  (resolved as Record<string, unknown>).name ??
                  (resolved as Record<string, unknown>).title ??
                  null
                if (displayName) {
                  value = displayName
                }
              }
            }

            return (
              <div key={field.name} className="space-y-1">
                <label className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                  {fieldLabel}
                </label>
                <div className="text-sm">
                  <DetailFieldValue
                    field={field}
                    value={value}
                    fmt={formatter}
                    fileUrl={
                      field.type === "file"
                        ? `/${workspace}/_ui/entity/${entity.module}/${entity.name}/${encodeURIComponent(id ?? "")}/${field.name}`
                        : undefined
                    }
                  />
                </div>
              </div>
            )
          })}
        </div>
      </div>

      {/* Child Tables */}
      {childFields.map((field) => (
        <div key={field.name} className="rounded-md border">
          <div className="border-b px-4 py-2 text-sm font-medium">
            {entityFieldLabel(entity, field.name)}
          </div>
          <div className="p-4">
            {record[field.name] ? (
              <pre className="text-xs text-muted-foreground overflow-auto">
                {JSON.stringify(record[field.name], null, 2)}
              </pre>
            ) : (
              <p className="text-sm text-muted-foreground">No data</p>
            )}
          </div>
        </div>
      ))}

      {/* State Machine Transitions */}
      {visibleTransitions.length > 0 && (
        <div className="space-y-2">
          <h3 className="text-sm font-medium">Actions</h3>
          <div className="flex flex-wrap gap-2">
            {visibleTransitions.map((t) => (
              <Button
                key={t.action}
                variant={
                  t.style === "danger"
                    ? "destructive"
                    : t.style === "primary"
                      ? "default"
                      : "outline"
                }
                disabled={transitioning === t.action}
                onClick={() => handleTransition(t.action, false)}
              >
                {transitioning === t.action ? (
                  <Loader2 className="size-4 mr-1 animate-spin" />
                ) : null}
                {t.label}
              </Button>
            ))}
          </div>
        </div>
      )}

      {/* Confirm Dialog for Transitions */}
      <ConfirmDialog
        open={!!pendingTransition}
        onOpenChange={(open) => {
          if (!open) setPendingTransition(null)
        }}
        title={pendingTransition?.label ?? "Konfirmasi"}
        message={pendingTransition?.confirm ?? ""}
        variant="warning"
        confirmLabel="Konfirmasi"
        onConfirm={() => {
          const pt = pendingTransition
          setPendingTransition(null)
          if (pt) handleTransition(pt.action, true)
        }}
        onCancel={() => setPendingTransition(null)}
      />

      {/* Audit Info */}
      <div className="text-xs text-muted-foreground space-y-0.5">
        {record.created_at ? (
          <div>Created: {formatter.dateTime(record.created_at as string)}</div>
        ) : null}
        {record.modified ? (
          <div>Modified: {formatter.dateTime(record.modified as string)}</div>
        ) : null}
        {typeof record.version === "number" ? (
          <div>Version: {record.version}</div>
        ) : null}
      </div>
    </div>
  )
}

// ── Field Value Display ──

function DetailFieldValue({
  field,
  value,
  fmt,
  fileUrl,
}: {
  field: import("@/types/manifest").Field
  value: unknown
  fmt?: Formatter
  fileUrl?: string
}) {
  if (value == null)
    return <span className="text-muted-foreground italic">-</span>

  if (
    field.type === "enum" ||
    field.name === "doc_status" ||
    field.name === "status"
  ) {
    return <Badge value={String(value)} />
  }

  if (field.type === "boolean") {
    return value ? "Yes" : "No"
  }

  // A multi-value field with a declared choice set renders as labelled chips
  // rather than raw JSON — the same reason the form uses the tag picker: `[1, 2]`
  // tells the reader nothing, `Senin`, `Selasa` does. Placed before the generic
  // json branch below, which would otherwise print the array.
  if (Array.isArray(value) && hasFieldOptions(field)) {
    return <OptionChips value={value} entityField={field} />
  }

  // A single declared value renders its caption (`Senin`), not the stored scalar
  // (`1`) — the same declaration the form and the table cell read, so one field
  // cannot read three ways depending on the surface.
  if (!Array.isArray(value) && hasFieldOptions(field)) {
    const caption = renderOptionCell(value, field)
    if (caption !== null) return <span>{caption}</span>
  }

  if (field.type === "text") {
    return <span className="whitespace-pre-wrap">{String(value)}</span>
  }

  if (field.type === "richtext") {
    return (
      <div
        className="text-sm [&_a]:text-primary [&_a]:underline"
        dangerouslySetInnerHTML={{ __html: sanitizeHTML(String(value)) }}
      />
    )
  }

  if (field.type === "file") {
    const key = Array.isArray(value) ? String(value[0] ?? "") : String(value)
    const name = key.split("/").pop()
    // An image renders as an image (#4) — the download route serves the bytes,
    // so the detail page needs no separate preview widget. Clicking it opens a
    // popup dialog (ImageLightbox) rather than a browser tab, which used to
    // navigate away from the record. Non-images keep the download link.
    if (isImageFile(key) && fileUrl) {
      return (
        <ImageLightbox
          src={fileUrl}
          alt={key}
          className="max-h-64 rounded border object-contain"
        />
      )
    }
    return (
      <a
        href={fileUrl ?? "#"}
        target="_blank"
        rel="noreferrer"
        className="inline-flex items-center gap-1.5 text-primary hover:underline"
      >
        <FileText className="size-4" />
        {name}
      </a>
    )
  }

  if (field.type === "datetime") {
    return (fmt ?? createFormatter()).dateTime(value as string)
  }

  if (field.type === "date") {
    return (fmt ?? createFormatter()).date(value as string)
  }

  if (field.type === "money") {
    // Canonical wire shape is {amount, currency} — format its amount instead of
    // falling through to the JSON branch below.
    const amount = moneyAmount(value)
    if (amount !== undefined) return (fmt ?? createFormatter()).money(amount)
  }

  if (field.type === "decimal" && typeof value === "number") {
    return (fmt ?? createFormatter()).money(value)
  }

  if (field.type === "json" || typeof value === "object") {
    return (
      <pre className="text-xs bg-muted p-2 rounded overflow-auto max-h-32">
        {JSON.stringify(value, null, 2)}
      </pre>
    )
  }

  return String(value)
}
