// ─── SearchSelect ───
//
// Wizard step layout: search_select (§11 spec frontend).
// Renders a search input that queries an entity by the specified search_fields,
// displays results for selection, and optionally allows creating a new record.
//
// Used by WizardRenderer when step.layout === "search_select".

import { useState, useEffect, useRef, useCallback } from "react"
import { Search, Check, Loader2, UserPlus } from "lucide-react"
import type { KyInstance } from "ky"

import type { WizardStep, FormSpec, Entry, FormField } from "@/types/manifest"
import { useMetaStore } from "@/stores/meta"
import { apiList, apiPost } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog"
import { toast } from "sonner"

interface SearchSelectProps {
  step: WizardStep
  module: string // owning module from the wizard entry
  stepData: Record<string, unknown>
  onSelect: (field: string, value: unknown) => void
  getClient: () => KyInstance
}

interface SearchResult {
  id: string
  [key: string]: unknown
}

export default function SearchSelect({ step, module, stepData, onSelect, getClient }: SearchSelectProps) {
  const [query, setQuery] = useState("")
  const [results, setResults] = useState<SearchResult[]>([])
  const [loading, setLoading] = useState(false)
  // Rehydrate from stepData on mount — covers both page refresh (autosaved
  // stepData restored from localStorage) and navigating back to this step.
  const [selected, setSelected] = useState<SearchResult | null>(
    () => (stepData[step.entity ?? "selected"] as SearchResult | undefined) ?? null,
  )
  const [createOpen, setCreateOpen] = useState(false)
  const [formData, setFormData] = useState<Record<string, string>>({})
  const [creating, setCreating] = useState(false)
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(undefined)

  // Look up the quick-create form spec from meta store
  const getForm = useMetaStore((s) => s.getForm)
  const createForm: Entry<FormSpec> | undefined = step.form ? getForm(step.form) : undefined

  // Look up the entity schema from the meta store
  const getEntity = useMetaStore((s) => s.getEntity)
  const entity = step.entity ? getEntity(module, step.entity) : undefined

  // Derived: module & plural for API path
  const moduleName = entity?.module ?? module
  const pluralName = entity?.plural ?? `${step.entity}s`

  // ── Search ──

  const doSearch = useCallback(async (q: string) => {
    if (!q.trim() || !step.search_fields?.length) {
      setResults([])
      return
    }

    setLoading(true)
    try {
      const client = getClient()
      const path = `${moduleName}/${pluralName}`
      const { items } = await apiList<SearchResult>(client, path, {
        search: q.trim(),
        per_page: "10",
      })
      setResults(items ?? [])
    } catch {
      setResults([])
    } finally {
      setLoading(false)
    }
  }, [moduleName, pluralName, step.search_fields, getClient])

  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => doSearch(query), 300)
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
    }
  }, [query, doSearch])

  // ── Select / Deselect ──

  const handleSelect = (item: SearchResult) => {
    setSelected(item)
    setResults([])
    setQuery("")
    // Store the full record (for summary display, e.g. "patient.name") and
    // its id under "{entity}_id" (the field name the target entity's create
    // payload expects, e.g. visit.patient_id).
    onSelect(step.entity ?? "selected", item)
    onSelect(`${step.entity ?? "selected"}_id`, item.id)
  }

  const handleClear = () => {
    setSelected(null)
    onSelect(step.entity ?? "selected", null)
    onSelect(`${step.entity ?? "selected"}_id`, null)
  }

  // ── Create form handler ──

  const handleCreate = useCallback(async () => {
    setCreating(true)
    try {
      const client = getClient()
      const path = `${moduleName}/${pluralName}`
      const record = await apiPost<SearchResult>(client, path, formData)

      // Select the newly created record — created eagerly, on dialog save,
      // regardless of whether the wizard is completed or abandoned (the
      // patient record is real master data, not wizard-scoped draft state).
      setSelected(record)
      onSelect(step.entity ?? "selected", record)
      onSelect(`${step.entity ?? "selected"}_id`, record.id)
      setCreateOpen(false)
      setFormData({})
      toast.success("Pasien berhasil didaftarkan")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Gagal menyimpan pasien")
    } finally {
      setCreating(false)
    }
  }, [moduleName, pluralName, formData, getClient, step.entity, onSelect])

  // Reset form when dialog opens
  useEffect(() => {
    if (createOpen) {
      setFormData({})
    }
  }, [createOpen])

  // ── Render helpers ──

  const highlightField = (item: SearchResult, field: string) => {
    return String(item[field] ?? "")
  }

  /** Render a single form field input */
  const renderField = (field: FormField) => {
    const value = formData[field.name] ?? ""
    const label = field.label ?? field.name

    // Determine input type from the field name / entity field type
    const entityField = entity?.fields.find((f) => f.name === field.name)
    const fieldType = entityField?.type ?? "string"

    switch (fieldType) {
      case "date":
        return (
          <div key={field.name} className="space-y-1">
            <label className="text-sm font-medium">{label}</label>
            <Input
              type="date"
              value={value}
              onChange={(e) => setFormData((d) => ({ ...d, [field.name]: e.target.value }))}
            />
          </div>
        )
      case "enum": {
        const options = entityField?.enum_values ?? []
        return (
          <div key={field.name} className="space-y-1">
            <label className="text-sm font-medium">{label}</label>
            <select
              className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs"
              value={value}
              onChange={(e) => setFormData((d) => ({ ...d, [field.name]: e.target.value }))}
            >
              <option value="">Pilih {label}</option>
              {options.map((opt) => (
                <option key={opt} value={opt}>{opt}</option>
              ))}
            </select>
          </div>
        )
      }
      case "boolean":
        return (
          <div key={field.name} className="flex items-center gap-2">
            <input
              type="checkbox"
              id={`field-${field.name}`}
              checked={value === "true"}
              onChange={(e) =>
                setFormData((d) => ({ ...d, [field.name]: e.target.checked ? "true" : "false" }))
              }
              className="size-4 rounded border border-input"
            />
            <label htmlFor={`field-${field.name}`} className="text-sm font-medium">
              {label}
            </label>
          </div>
        )
      case "integer":
      case "decimal":
      case "number":
        return (
          <div key={field.name} className="space-y-1">
            <label className="text-sm font-medium">{label}</label>
            <Input
              type="number"
              step={fieldType === "decimal" ? "0.01" : "1"}
              value={value}
              onChange={(e) => setFormData((d) => ({ ...d, [field.name]: e.target.value }))}
            />
          </div>
        )
      default:
        return (
          <div key={field.name} className="space-y-1">
            <label className="text-sm font-medium">{label}</label>
            <Input
              placeholder={field.placeholder}
              value={value}
              onChange={(e) => setFormData((d) => ({ ...d, [field.name]: e.target.value }))}
            />
          </div>
        )
    }
  }

  return (
    <div className="space-y-4">
      {/* Selected item badge */}
      {selected ? (
        <div className="flex items-center gap-3 rounded-md border border-primary/30 bg-primary/5 p-3">
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium truncate">
              {selected[step.search_fields?.[0] ?? "name"] as string ?? "Selected"}
            </p>
            <p className="text-xs text-muted-foreground truncate">
              {step.search_fields?.slice(1).map((f) => `${f}: ${selected[f] ?? "-"}`).join(" · ")}
            </p>
          </div>
          <Button variant="ghost" size="sm" onClick={handleClear} className="shrink-0">
            <span className="text-xs">Change</span>
          </Button>
          <Check className="size-4 text-primary shrink-0" />
        </div>
      ) : (
        <>
          {/* Search input */}
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
            <Input
              className="pl-9"
              placeholder={
                step.search_fields
                  ? `Cari berdasarkan ${step.search_fields.join(", ")}...`
                  : "Cari..."
              }
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              autoFocus
            />
            {loading && (
              <Loader2 className="absolute right-3 top-1/2 -translate-y-1/2 size-4 animate-spin text-muted-foreground" />
            )}
          </div>

          {/* Search results */}
          {results.length > 0 && (
            <div className="rounded-md border divide-y max-h-60 overflow-y-auto">
              {results.map((item) => (
                <button
                  key={item.id ?? String(item[step.search_fields?.[0] ?? "name"])}
                  type="button"
                  onClick={() => handleSelect(item)}
                  className="w-full text-left px-3 py-2.5 text-sm hover:bg-muted transition-colors cursor-pointer"
                >
                  <p className="font-medium">
                    {step.search_fields?.map((f, i) => (
                      <span key={f}>
                        {i > 0 && <span className="text-muted-foreground mx-1">·</span>}
                        {highlightField(item, f)}
                      </span>
                    ))}
                  </p>
                  {item.description ? (
                    <p className="text-xs text-muted-foreground mt-0.5">{String(item.description)}</p>
                  ) : null}
                </button>
              ))}
            </div>
          )}

          {/* Empty state */}
          {query.trim() && !loading && results.length === 0 && (
            <p className="text-sm text-muted-foreground py-2 text-center">
              Tidak ditemukan hasil untuk "{query}"
            </p>
          )}

          {/* Create new button */}
          {step.allow_create && (
            <>
              <Button
                variant="outline"
                className="w-full gap-2 mt-2"
                onClick={() => setCreateOpen(true)}
              >
                <UserPlus className="size-4" />
                Pasien Baru
              </Button>
              <Dialog open={createOpen} onOpenChange={setCreateOpen}>
                <DialogContent className="sm:max-w-lg">
                  <DialogTitle className="text-lg font-semibold">
                    {createForm?.spec.sections[0]?.title ?? "Daftarkan Pasien Baru"}
                  </DialogTitle>

                  {createForm ? (
                    <form
                      onSubmit={(e) => {
                        e.preventDefault()
                        handleCreate()
                      }}
                      className="space-y-4 py-2"
                    >
                      {createForm.spec.sections.map((section, si) => (
                        <div key={si} className="space-y-3">
                          {section.fields.map(renderField)}
                        </div>
                      ))}

                      <div className="flex items-center justify-end gap-2 pt-2">
                        <Button
                          type="button"
                          variant="outline"
                          onClick={() => setCreateOpen(false)}
                        >
                          Batal
                        </Button>
                        <Button type="submit" disabled={creating}>
                          {creating && <Loader2 className="size-4 animate-spin" />}
                          {createForm.spec.submit?.label ?? "Simpan"}
                        </Button>
                      </div>
                    </form>
                  ) : (
                    <p className="py-4 text-sm text-muted-foreground">
                      Form "{step.form}" tidak ditemukan
                    </p>
                  )}
                </DialogContent>
              </Dialog>
            </>
          )}
        </>
      )}
    </div>
  )
}
