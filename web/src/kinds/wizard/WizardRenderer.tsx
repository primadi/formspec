// ─── Wizard Renderer ───
//
// Multi-step business process (kind: Wizard).
// Step state tracked via `?step=N` query parameter.
//
// Design doc §5.5 Wizard kind (F4)

import { useState } from "react"
import { useSearchParams, useNavigate, useParams } from "react-router-dom"
import { ArrowLeft, ArrowRight, Check } from "lucide-react"
import { toast } from "sonner"

import type { Entry, WizardSpec } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
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
  const { workspace = "default" } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const getClient = useSessionStore((s) => s.getClient)

  const steps = entry.spec.steps
  const currentStep = parseInt(searchParams.get("step") ?? "0", 10)
  const [stepData, setStepData] = useState<Record<string, unknown>>({})
  const [submitting, setSubmitting] = useState(false)

  const totalSteps = steps.length
  const isFirst = currentStep === 0
  const isLast = currentStep === totalSteps - 1
  const progress = ((currentStep + 1) / totalSteps) * 100

  const goToStep = (step: number) => {
    const next = new URLSearchParams(searchParams)
    next.set("step", String(Math.max(0, Math.min(step, totalSteps - 1))))
    setSearchParams(next)
  }

  const handleNext = async () => {
    // If current step has an action, execute it
    const step = steps[currentStep]
    if (step.action) {
      try {
        const client = getClient()
        await client.post(
          `${step.action}`,
          { json: stepData },
        )
      } catch {
        // Continue to next step even if action fails
      }
    }
    goToStep(currentStep + 1)
  }

  const handleSubmit = async () => {
    setSubmitting(true)
    try {
      const client = getClient()
      if (entry.spec.action) {
        await apiPost(client, entry.spec.action, stepData)
      }
      toast.success("Wizard completed successfully")
      navigate(`/${workspace}/_admin`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Wizard submission failed")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      {/* Progress bar */}
      <div className="space-y-2">
        <div className="flex items-center justify-between text-sm">
          <h1 className="text-2xl font-bold tracking-tight">{entry.spec.title}</h1>
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
            {idx < totalSteps - 1 && (
              <div className="h-px w-4 bg-border" />
            )}
          </div>
        ))}
      </div>

      {/* Current step content */}
      <div className="rounded-md border p-6">
        <h2 className="text-lg font-semibold mb-1">{steps[currentStep].title}</h2>
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
                <label className="text-sm font-medium">{field.label ?? field.name}</label>
                <input
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
                  <dd className="font-medium">
                    {(stepData[item.field] as string) ?? "-"}
                  </dd>
                </div>
              ))}
            </dl>
          </div>
        ) : null}
      </div>

      {/* Navigation buttons */}
      <div className="flex items-center justify-between">
        <Button
          variant="outline"
          disabled={isFirst}
          onClick={() => goToStep(currentStep - 1)}
        >
          <ArrowLeft className="size-4 mr-1" />
          Previous
        </Button>

        {isLast ? (
          <Button onClick={handleSubmit} disabled={submitting}>
            {submitting ? "Submitting..." : "Complete"}
            <Check className="size-4 ml-1" />
          </Button>
        ) : (
          <Button onClick={handleNext}>
            Next
            <ArrowRight className="size-4 ml-1" />
          </Button>
        )}
      </div>
    </div>
  )
}
