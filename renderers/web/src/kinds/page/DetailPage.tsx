// ─── Detail Page ───
//
// Readonly field grid + child tables + state machine transition buttons
// + lifecycle patterns (Frontend §1.7).
//
// Design doc §5.5 Detail page (F3)

import { useEffect, useState, useMemo } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { useSurface } from "@/hooks/useSurface"
import { toast } from "sonner"
import { ArrowLeft, Edit, Loader2 } from "lucide-react"

import type { EntitySchema } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { can as checkPermission } from "@/engine/permissions"
import { deriveDetailFields } from "@/engine/derive"
import { getLifecycle, getAvailableTransitions } from "@/engine/lifecycle"
import { apiGet } from "@/lib/api"
import { Badge } from "@/widgets/Badge"
import { Button } from "@/components/ui/button"
import ConfirmDialog from "@/components/ui/confirm-dialog"

interface DetailPageProps {
  entity: EntitySchema
}

export default function DetailPage({ entity }: DetailPageProps) {
  const navigate = useNavigate()
  const { workspace = "default", id } = useParams<{ workspace: string; id: string }>()
  const { surfacePath } = useSurface()
  const me = useSessionStore((s) => s.me)
  const getClient = useSessionStore((s) => s.getClient)

  const { mainFields, childFields } = useMemo(() => deriveDetailFields(entity), [entity])
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
          `${entity.module}/${entity.name}/${id}`,
        )
        setRecord(data)
      } catch (err) {
        toast.error("Failed to load record")
        navigate(surfacePath(entity.module, entity.plural))
      } finally {
        setLoading(false)
      }
    }
    loadRecord()
  }, [id, entity, getClient, navigate, workspace])

  // All entity schemas from the meta bundle — used to resolve relation display fields
  const entities = useMetaStore((s) => s.bundle?.entities ?? [])

  const currentState = record?.[entity.state_machine?.field ?? ""] as string | undefined
  const transitions = useMemo(
    () => (currentState ? getAvailableTransitions(entity, currentState) : []),
    [entity, currentState],
  )

  const handleTransition = async (
    action: string,
    skipConfirm = false,
  ) => {
    if (!me) return
    const perm = `${entity.module}.${entity.plural}.${action}`
    if (!checkPermission(perm, me.permissions)) {
      toast.error("You don't have permission")
      return
    }

    // Find transition to check for confirm message
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
      await client.post(`${entity.module}/${entity.name}/${id}/${action}`)
      toast.success("State updated")
      // Reload
      const data = await apiGet<Record<string, unknown>>(
        client,
        `${entity.module}/${entity.name}/${id}`,
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
            {entity.name.charAt(0).toUpperCase() + entity.name.slice(1)}
          </h1>
          {entity.state_machine && currentState && (
            <Badge value={currentState} />
          )}
        </div>

        {lifecycle.hasSave && id && (
          <Button
            variant="outline"
            onClick={() =>
              navigate(
                surfacePath(entity.module, entity.plural, id, "edit"),
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
            const fieldLabel = field.name.replace(/_/g, " ")

            if (
              field.type === "relation" &&
              typeof value === "string" &&
              value.length === 36 // UUID length heuristic
            ) {
              const alias = field.name.endsWith("_id")
                ? field.name.slice(0, -3)
                : field.relation?.resource ?? field.name
              const resolved = record[alias]
              if (resolved && typeof resolved === "object" && !Array.isArray(resolved)) {
                // Determine the related entity's label_field.
                // relation.resource can be "entity" or "module.entity" (cross-module).
                const resourceName = field.relation?.resource ?? alias
                const parts = resourceName.split(".")
                const relatedEntity = parts.length === 2
                  ? entities.find((e) => e.module === parts[0] && e.name === parts[1])
                  : entities.find((e) => e.name === parts[0])
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
                  <DetailFieldValue field={field} value={value} />
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
            {field.name.replace(/_/g, " ")}
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
      {transitions.length > 0 && (
        <div className="space-y-2">
          <h3 className="text-sm font-medium">Actions</h3>
          <div className="flex flex-wrap gap-2">
            {transitions.map((t) => (
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
          <div>Created: {new Date(record.created_at as string).toLocaleString()}</div>
        ) : null}
        {record.modified ? (
          <div>Modified: {new Date(record.modified as string).toLocaleString()}</div>
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
}: {
  field: import("@/types/manifest").Field
  value: unknown
}) {
  if (value == null) return <span className="text-muted-foreground italic">-</span>

  if (field.type === "enum" || field.name === "doc_status" || field.name === "status") {
    return <Badge value={String(value)} />
  }

  if (field.type === "boolean") {
    return value ? "Yes" : "No"
  }

  if (field.type === "datetime") {
    return new Date(value as string).toLocaleString()
  }

  if (field.type === "date") {
    return new Date(value as string).toLocaleDateString()
  }

  if (field.type === "decimal" && typeof value === "number") {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: "USD",
    }).format(value)
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

