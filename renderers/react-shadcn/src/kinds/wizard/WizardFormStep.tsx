// ─── WizardFormStep ───
//
// Renders a wizard step that references a Form manifest (step.form).
// Loads the form spec from the meta store, resolves field types from the
// entity schema, fetches relation options via API, and handles depends_on
// for dependent dropdowns (e.g. doctor filtered by polyclinic).
//
// Used by WizardRenderer when step.form is set and layout is not search_select.

import { useState, useEffect, useRef, useCallback, useMemo } from "react"
import type { KyInstance } from "ky"

import type { WizardStep, Entry, FormSpec, FormField } from "@/types/manifest"
import { useMetaStore } from "@/stores/meta"
import { resolveEntityRef } from "@/engine/entityRef"
import { apiList } from "@/lib/api"
import { Input } from "@/components/ui/input"

interface WizardFormStepProps {
  step: WizardStep
  module: string
  stepData: Record<string, unknown>
  onFieldChange: (field: string, value: unknown) => void
  getClient: () => KyInstance
}

interface OptionItem {
  id: string
  name: string
  [key: string]: unknown
}

export default function WizardFormStep({
  step,
  module,
  stepData,
  onFieldChange,
  getClient,
}: WizardFormStepProps) {
  const getForm = useMetaStore((s) => s.getForm)
  const getEntity = useMetaStore((s) => s.getEntity)

  // Look up the form spec — memoized to avoid infinite effect loops
  const form: Entry<FormSpec> | undefined = useMemo(
    () => (step.form ? getForm(step.form) : undefined),
    [step.form, getForm],
  )
  // form.spec.entity is resolved relative to the FORM manifest's own module
  // (form.module) — not the wizard's module, which may differ for a
  // cross-module Form referenced by step.form.
  const entityName = form?.spec.entity
  const entity = useMemo(() => {
    if (!entityName || !form) return undefined
    const [entityModule, entityLocalName] = resolveEntityRef(entityName, form.module)
    return getEntity(entityModule, entityLocalName)
  }, [entityName, form, getEntity])

  // Gather all fields from all sections — stable reference
  const allFields: FormField[] = useMemo(
    () => (form ? form.spec.sections.flatMap((s) => s.fields) : []),
    [form],
  )

  // ── Load options for relation fields ──
  // Use ref to avoid re-render loops (infinite update depth exceeded)
  const optionsRef = useRef<Record<string, OptionItem[]>>({})
  const [loadingOptions, setLoadingOptions] = useState<Record<string, boolean>>({})
  // Re-render trigger when new options arrive
  const [refreshTick, setRefreshTick] = useState(0)

  // Memoize the current options from ref + tick
  const optionsCache = useMemo(
    () => optionsRef.current,
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [refreshTick],
  )

  const loadOptions = useCallback(async (resource: string) => {
    if (optionsRef.current[resource]) return // already loaded
    setLoadingOptions((prev) => ({ ...prev, [resource]: true }))
    try {
      const client = getClient()
      // relation.resource is resolved relative to the entity that declares
      // the relation field (entity.module) — falling back to the wizard's
      // own module only if the entity itself hasn't resolved yet.
      const [resModule, resName] = resolveEntityRef(resource, entity?.module ?? module)
      const resEntity = getEntity(resModule, resName)
      if (!resEntity) {
        optionsRef.current = { ...optionsRef.current, [resource]: [] }
        setRefreshTick((t) => t + 1)
        return
      }
      const path = `${resEntity.module}/${resEntity.name}`
      const { items } = await apiList<OptionItem>(client, path, { per_page: "100" })
      optionsRef.current = { ...optionsRef.current, [resource]: items ?? [] }
      setRefreshTick((t) => t + 1)
    } catch {
      optionsRef.current = { ...optionsRef.current, [resource]: [] }
      setRefreshTick((t) => t + 1)
    } finally {
      setLoadingOptions((prev) => ({ ...prev, [resource]: false }))
    }
  }, [module, entity, getClient, getEntity])

  // Load options for relation fields on mount
  useEffect(() => {
    if (!entity) return
    for (const field of allFields) {
      // Find matching entity field to check type
      const entityField = entity.fields.find((f) => f.name === field.name)
      if (entityField?.type === "relation" && entityField.relation?.resource) {
        loadOptions(entityField.relation.resource)
      }
    }
  }, [entity, allFields, loadOptions])

  // ── Depends_on: re-fetch options when dependency changes ──
  const dependsOnField = step.depends_on
  const dependsOnValue = dependsOnField ? stepData[dependsOnField] : undefined

  // ── Render a single form field ──

  const renderField = (field: FormField) => {
    if (!entity) return null

    const entityField = entity.fields.find((f) => f.name === field.name)
    if (!entityField) return null

    const value = (stepData[field.name] as string) ?? ""
    const label = field.label ?? entityField.description ?? field.name

    // Handle relation fields → dropdown
    if (entityField.type === "relation" && entityField.relation?.resource) {
      const resource = entityField.relation.resource
      const options = optionsCache[resource] ?? []
      const isLoading = loadingOptions[resource] ?? false
      // This field is the *dependent* one (e.g. doctor_id) when a depends_on
      // is declared and it isn't the trigger field itself (e.g. polyclinic_id)
      // — the trigger field's own options are never filtered by themselves.
      const isDependent = dependsOnField !== undefined && field.name !== dependsOnField
      const dependentId = isDependent ? dependsOnValue : undefined

      // Filter options if this field depends on another
      let filteredOptions = options
      if (isDependent && dependentId && options.length > 0) {
        // For belongs_to relations, filter by the dependency field on the target entity
        // e.g. doctor.polyclinic_id === selected polyclinic_id
        filteredOptions = options.filter((opt) => {
          // The target entity (e.g. doctor) has a relation field matching the dependency
          // We check if the option has a field matching the dependency name
          const depFieldName = dependsOnField ?? ""
          return String(opt[depFieldName] ?? "") === String(dependentId)
        })
      }

      return (
        <div key={field.name} className="space-y-2.5">
          <label className="text-sm font-medium">{label}</label>
          {field.help && (
            <p className="text-xs text-muted-foreground">{field.help}</p>
          )}
          <select
            autoComplete="nope"
            className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs"
            value={value}
            onChange={(e) => {
              const id = e.target.value
              onFieldChange(field.name, id)
              // Also store the full record under the resource name (e.g.
              // "polyclinic"/"doctor") so wizard summary steps can resolve
              // dotted paths like "polyclinic.name" — mirrors how
              // SearchSelect stores the full patient record.
              const full = filteredOptions.find((o) => o.id === id)
              onFieldChange(resource, full ?? null)
            }}
          >
            <option value="">Pilih {label}</option>
            {isLoading ? (
              <option disabled>Memuat...</option>
            ) : (
              filteredOptions.map((opt) => (
                <option key={opt.id} value={opt.id}>
                  {opt.name ?? opt.id}
                </option>
              ))
            )}
          </select>
        </div>
      )
    }

    // Handle enum fields → dropdown
    if (entityField.type === "enum" && entityField.enum_values?.length) {
      return (
        <div key={field.name} className="space-y-2.5">
          <label className="text-sm font-medium">{label}</label>
          <select
            autoComplete="nope"
            className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs"
            value={value}
            onChange={(e) => onFieldChange(field.name, e.target.value)}
          >
            <option value="">Pilih {label}</option>
            {entityField.enum_values.map((opt) => (
              <option key={opt} value={opt}>{opt}</option>
            ))}
          </select>
        </div>
      )
    }

    // Handle boolean fields
    if (entityField.type === "boolean") {
      return (
        <div key={field.name} className="flex items-center gap-2">
          <input
            type="checkbox"
            id={`wf-${field.name}`}
            checked={value === "true"}
            onChange={(e) =>
              onFieldChange(field.name, e.target.checked ? "true" : "false")
            }
            className="size-4 rounded border border-input"
          />
          <label htmlFor={`wf-${field.name}`} className="text-sm font-medium">
            {label}
          </label>
        </div>
      )
    }

    // Handle date fields
    if (entityField.type === "date" || entityField.type === "datetime") {
      return (
        <div key={field.name} className="space-y-2.5">
          <label className="text-sm font-medium">{label}</label>
          <Input
            type={entityField.type === "datetime" ? "datetime-local" : "date"}
            value={value}
            onChange={(e) => onFieldChange(field.name, e.target.value)}
          />
        </div>
      )
    }

    // Handle number types
    if (entityField.type === "integer" || entityField.type === "decimal" || entityField.type === "number") {
      return (
        <div key={field.name} className="space-y-2.5">
          <label className="text-sm font-medium">{label}</label>
          <Input
            type="number"
            step={entityField.type === "decimal" ? "0.01" : "1"}
            placeholder={field.placeholder}
            value={value}
            onChange={(e) => onFieldChange(field.name, e.target.value)}
          />
        </div>
      )
    }

    // Default: string text input
    return (
      <div key={field.name} className="space-y-2.5">
        <label className="text-sm font-medium">{label}</label>
        <Input
          placeholder={field.placeholder}
          value={value}
          onChange={(e) => onFieldChange(field.name, e.target.value)}
        />
      </div>
    )
  }

  if (!form) {
    return (
      <p className="text-sm text-muted-foreground py-4">
        Form "{step.form}" tidak ditemukan
      </p>
    )
  }

  return (
    <div className="space-y-6">
      {form.spec.sections.map((section, si) => (
        <div key={si} className="space-y-3">
          {section.title && (
            <h3 className="text-md font-semibold">{section.title}</h3>
          )}
          {section.description && (
            <p className="text-xs text-muted-foreground">{section.description}</p>
          )}
          {section.fields.map(renderField)}
        </div>
      ))}
    </div>
  )
}
