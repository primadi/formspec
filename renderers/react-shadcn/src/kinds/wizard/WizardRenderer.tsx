// ─── Wizard Renderer ───
//
// Multi-step business process (kind: Wizard).
// Step state tracked via `?step=N` query parameter.
//
// Design doc §5.5 Wizard kind (F4)

import { useState, useEffect } from "react"
import { useSearchParams, useNavigate } from "react-router-dom"
import { useSurface } from "@/hooks/useSurface"
import { ArrowLeft, ArrowRight, Check, X } from "lucide-react"
import { toast } from "@/lib/ui"

import type { Entry, WizardSpec } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { resolveEntityRef } from "@/engine/entityRef"
import { apiPost } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import SearchSelect from "./SearchSelect"
import WizardFormStep from "./WizardFormStep"

interface WizardRendererProps {
  entry: Entry<WizardSpec>
}

export default function WizardRenderer({ entry }: WizardRendererProps) {
  const navigate = useNavigate()
  const { adminPath } = useSurface()
  const [searchParams, setSearchParams] = useSearchParams()
  const getClient = useSessionStore((s) => s.getClient)
  const getEntity = useMetaStore((s) => s.getEntity)

  const steps = entry.spec.steps
  const currentStep = parseInt(searchParams.get("step") ?? "0", 10)

  // Each open wizard is identified by an instance id in the URL so that
  // ordinary multi-tab use (Ctrl+click) and page refresh don't clobber or
  // lose in-progress step data — no server-side draft row needed.
  const [instance] = useState<string>(
    () => searchParams.get("instance") ?? crypto.randomUUID(),
  )
  const storageKey = `wizard:${entry.module}.${entry.name}:${instance}`

  useEffect(() => {
    if (searchParams.get("instance") !== instance) {
      const next = new URLSearchParams(searchParams)
      next.set("instance", instance)
      setSearchParams(next, { replace: true })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [instance])

  const [stepData, setStepData] = useState<Record<string, unknown>>(() => {
    try {
      const raw = localStorage.getItem(storageKey)
      return raw ? JSON.parse(raw) : {}
    } catch {
      return {}
    }
  })
  const [submitting, setSubmitting] = useState(false)
  // Response of the most recent successful submit — kept around so
  // `on_complete.banner` can resolve `response.*` paths after a restart
  // clears `stepData` itself.
  const [completion, setCompletion] = useState<Record<string, unknown> | null>(
    null,
  )

  useEffect(() => {
    try {
      localStorage.setItem(storageKey, JSON.stringify(stepData))
    } catch {
      // best-effort autosave — ignore quota/serialization errors
    }
  }, [stepData, storageKey])

  const totalSteps = steps.length
  const isFirst = currentStep === 0
  const isLast = currentStep === totalSteps - 1
  const progress = ((currentStep + 1) / totalSteps) * 100

  const step = steps[currentStep]
  const canProceed =
    !step.required?.length ||
    step.required.every((f) => {
      const v = stepData[f]
      return v !== undefined && v !== null && v !== ""
    })

  // Summary items reference fields as dotted paths (e.g. "patient.name",
  // resolving into stepData.patient.name) since search_select steps store
  // the full selected/created record under the entity name. A "response."
  // prefix resolves against the last submit's response instead (for
  // on_complete.banner, since stepData is cleared on restart).
  const resolveField = (path: string): string => {
    const fromResponse = path.startsWith("response.")
    const key = fromResponse ? path.slice("response.".length) : path
    const source: unknown = fromResponse ? completion : stepData
    const value = key
      .split(".")
      .reduce<unknown>(
        (acc, k) =>
          acc && typeof acc === "object"
            ? (acc as Record<string, unknown>)[k]
            : undefined,
        source,
      )
    return value == null ? "-" : String(value)
  }

  const goToStep = (step: number) => {
    const next = new URLSearchParams(searchParams)
    next.set("step", String(Math.max(0, Math.min(step, totalSteps - 1))))
    setSearchParams(next)
  }

  // Step hooks (on_enter/on_next/on_prev) are best-effort — a failing hook
  // never blocks navigation.
  const runHook = async (action?: string) => {
    if (!action) return
    try {
      const client = getClient()
      await client.post(action, { json: stepData })
    } catch {
      // ignore — hooks are fire-and-forget
    }
  }

  useEffect(() => {
    runHook(steps[currentStep]?.on_enter)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentStep])

  const handleNext = async () => {
    await runHook(step.on_next)
    goToStep(currentStep + 1)
  }

  const handlePrev = async () => {
    await runHook(step.on_prev)
    goToStep(currentStep - 1)
  }

  const handleSubmit = async () => {
    setSubmitting(true)
    try {
      const client = getClient()
      let response: Record<string, unknown> = {}
      if (entry.spec.action) {
        // A custom commit action — expected to be an entity-scoped action
        // (POST .../{id}/{action}), for wizards that finalize an existing
        // draft record rather than create one from scratch.
        response = await apiPost<Record<string, unknown>>(
          client,
          entry.spec.action,
          stepData,
        )
      } else if (entry.spec.entity) {
        // No action declared: every field the target entity needs was
        // already resolved during the wizard's steps (e.g. patient_id from
        // an eager patient.create in step 1), so the final commit is just a
        // normal entity create — no custom script/action required.
        const [entityModule, entityName] = resolveEntityRef(
          entry.spec.entity,
          entry.module,
        )
        const entitySchema = getEntity(entityModule, entityName)
        if (!entitySchema) {
          throw new Error(`entity ${entityModule}.${entityName} not found`)
        }
        const payload: Record<string, unknown> = {
          transaction_date: new Date().toISOString().slice(0, 10),
        }
        for (const [key, value] of Object.entries(stepData)) {
          if (entitySchema.fields.some((f) => f.name === key)) {
            payload[key] = value
          }
        }
        response = await apiPost<Record<string, unknown>>(
          client,
          `${entitySchema.module}/${entitySchema.name}`,
          payload,
        )
      }
      toast.success("Wizard completed successfully")

      const onComplete = entry.spec.on_complete
      if (onComplete?.restart) {
        setCompletion(response)
        setStepData({})
        goToStep(0)
      } else if (onComplete?.redirect) {
        navigate(onComplete.redirect)
      } else {
        navigate(adminPath())
      }
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Wizard submission failed",
      )
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      {/* Completion banner (on_complete.restart flow) */}
      {completion && entry.spec.on_complete?.banner?.length ? (
        <div className="flex items-start justify-between gap-4 rounded-md border border-primary/30 bg-primary/5 p-4">
          <div>
            <p className="text-sm font-medium mb-1">Wizard completed</p>
            <dl className="space-y-0.5 text-sm">
              {entry.spec.on_complete.banner.map((item) => (
                <div key={item.field} className="flex gap-2">
                  <dt className="text-muted-foreground">{item.label}:</dt>
                  <dd className="font-medium">{resolveField(item.field)}</dd>
                </div>
              ))}
            </dl>
          </div>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setCompletion(null)}
            className="shrink-0"
          >
            <X className="size-4" />
          </Button>
        </div>
      ) : null}

      {/* Progress bar */}
      <div className="space-y-2">
        <div className="flex items-center justify-between text-sm">
          <h1 className="text-2xl font-bold tracking-tight">
            {entry.spec.title}
          </h1>
          <span className="text-muted-foreground">
            Step {currentStep + 1} of {totalSteps}
          </span>
        </div>
        <div className="h-2 bg-muted rounded-full overflow-hidden">
          <div
            className="h-full bg-primary transition-all duration-300"
            style={{ width: `${progress}%` }}
          />
        </div>
      </div>

      {/* Step indicators */}
      <div className="flex items-center gap-2">
        {steps.map((step, idx) => (
          <div key={idx} className="flex items-center gap-2">
            <button
              onClick={() => goToStep(idx)}
              className={cn(
                "flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium transition-colors cursor-pointer",
                idx === currentStep
                  ? "bg-primary text-primary-foreground"
                  : idx < currentStep
                    ? "bg-primary/10 text-primary hover:bg-primary/20"
                    : "bg-muted text-muted-foreground hover:bg-muted/80",
              )}
            >
              {idx < currentStep ? (
                <Check className="size-3" />
              ) : (
                <span>{idx + 1}</span>
              )}
              <span className="hidden sm:inline">{step.title}</span>
            </button>
            {idx < totalSteps - 1 && <div className="h-px w-4 bg-border" />}
          </div>
        ))}
      </div>

      {/* Current step content */}
      <div className="rounded-md border p-6">
        <h2 className="text-lg font-semibold mb-1">
          {steps[currentStep].title}
        </h2>
        {steps[currentStep].description && (
          <p className="text-sm text-muted-foreground mb-4">
            {steps[currentStep].description}
          </p>
        )}

        {/* ── Step content: dispatch by layout ── */}
        {steps[currentStep].layout === "search_select" ? (
          <SearchSelect
            step={steps[currentStep]}
            module={entry.module}
            stepData={stepData}
            onSelect={(field, value) =>
              setStepData((d) => ({ ...d, [field]: value }))
            }
            getClient={getClient}
          />
        ) : steps[currentStep].fields?.length ? (
          <div className="space-y-3">
            {steps[currentStep].fields?.map((field) => (
              <div key={field.name} className="space-y-1">
                <label className="text-sm font-medium">
                  {field.label ?? field.name}
                </label>
                <input
                  autoComplete="nope"
                  className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs"
                  placeholder={field.placeholder}
                  value={(stepData[field.name] as string) ?? ""}
                  onChange={(e) =>
                    setStepData((d) => ({ ...d, [field.name]: e.target.value }))
                  }
                />
              </div>
            ))}
          </div>
        ) : steps[currentStep].form ? (
          <WizardFormStep
            step={steps[currentStep]}
            module={entry.module}
            stepData={stepData}
            onFieldChange={(field, value) =>
              setStepData((d) => ({ ...d, [field]: value }))
            }
            getClient={getClient}
          />
        ) : steps[currentStep].component ? (
          <p className="text-sm text-muted-foreground py-4">
            Custom component: {steps[currentStep].component}
          </p>
        ) : (
          <p className="text-sm text-muted-foreground py-4">
            No fields configured for this step.
          </p>
        )}

        {/* Summary step */}
        {steps[currentStep].summary?.length ? (
          <div className="mt-4 rounded-md bg-muted p-4">
            <h4 className="text-sm font-medium mb-2">Summary</h4>
            <dl className="space-y-1 text-sm">
              {steps[currentStep].summary.map((item) => (
                <div key={item.field} className="flex justify-between">
                  <dt className="text-muted-foreground">{item.label}</dt>
                  <dd className="font-medium">{resolveField(item.field)}</dd>
                </div>
              ))}
            </dl>
          </div>
        ) : null}
      </div>

      {/* Navigation buttons */}
      <div className="flex items-center justify-between">
        <Button variant="outline" disabled={isFirst} onClick={handlePrev}>
          <ArrowLeft className="size-4 mr-1" />
          Previous
        </Button>

        {isLast ? (
          <Button onClick={handleSubmit} disabled={submitting || !canProceed}>
            {submitting ? "Submitting..." : "Complete"}
            <Check className="size-4 ml-1" />
          </Button>
        ) : (
          <Button onClick={handleNext} disabled={!canProceed}>
            Next
            <ArrowRight className="size-4 ml-1" />
          </Button>
        )}
      </div>
    </div>
  )
}
