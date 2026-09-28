// ─── ActionInputDialog ───
//
// ONE generic dialog for every action/transition that declares inputs — the
// mechanism the plan calls for (docs_internal/plan/action-input-contract.md).
//
// It is deliberately not a per-kind renderer and not a new field renderer: the
// resolved inputs are adapted to the `Field` shape and drawn with
// `FormFieldWidget`, the same widget router a Form uses, so a new widget never
// has to be added in two places (widgets/catalog.test.tsx enforces that parity).
//
// Used from every surface that runs an action: a DetailPage transition button, a
// Table row action, a bulk action (one dialog for the whole selection), and a
// Kanban card. Declaring the inputs once on the transition is what makes those
// surfaces consistent — there is no per-surface form to write.

import { useEffect, useMemo, useState } from "react"
import { Loader2 } from "lucide-react"

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { FormFieldWidget } from "@/kinds/form/FormRenderer"
import { buildZodField } from "@/lib/zod-schema"
import {
  evalVisibleWhen,
  evalReadonlyWhen,
  evalRequiredWhen,
  evalCompute,
} from "@/lib/formspec-expr"
import type { EvalContext, RuntimeObject } from "@/lib/formspec-expr"
import type { ResolvedActionInputs } from "@/lib/actionParams"

interface ActionInputDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Dialog heading — the action's button label, so it reads as the same action. */
  title: string
  /** Optional explanatory line under the title (the action's description/confirm). */
  message?: string
  resolved: ResolvedActionInputs
  /** Labels the confirm button; falls back to "Lanjutkan". */
  confirmLabel?: string
  variant?: "default" | "destructive"
  /** Current record values + caller identity, so predicates can reference them. */
  record?: Record<string, unknown>
  user?: unknown
  /** Receives only the values the contract declares and the caller actually filled. */
  onSubmit: (values: Record<string, unknown>) => void | Promise<void>
}

/**
 * Evaluate the input predicates against the values collected so far.
 *
 * The same four-predicate vocabulary a Form field has (visible_when /
 * readonly_when / required_when / compute), read by the same interpreter — which
 * is what makes a transition with several origin states expressible as ONE
 * declaration gated by `required_when`, instead of being split per origin.
 */
function evaluateInputs(
  resolved: ResolvedActionInputs,
  values: Record<string, unknown>,
  record?: Record<string, unknown>,
  user?: unknown,
) {
  const scope: EvalContext = {
    fields: { ...(record ?? {}), ...values } as RuntimeObject,
    user: user as RuntimeObject | undefined,
  }
  return resolved.inputs.map((r) => {
    // `required_when` can only NARROW, never widen past the declaration: an input
    // that is not required and declares no predicate stays optional.
    const required = r.input.required_when
      ? evalRequiredWhen(r.input.required_when, scope) ||
        r.input.required === true
      : r.input.required === true
    return {
      ...r,
      visible: evalVisibleWhen(r.input.visible_when, scope),
      readonly: evalReadonlyWhen(r.input.readonly_when, scope),
      required,
    }
  })
}

export default function ActionInputDialog({
  open,
  onOpenChange,
  title,
  message,
  resolved,
  confirmLabel,
  variant = "default",
  record,
  user,
  onSubmit,
}: ActionInputDialogProps) {
  const [values, setValues] = useState<Record<string, unknown>>({})
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [submitting, setSubmitting] = useState(false)

  // Seed defaults once per opening, so re-opening the dialog for the same action
  // does not carry the previous attempt's values into a fresh run.
  useEffect(() => {
    if (!open) return
    const seeded: Record<string, unknown> = {}
    for (const r of resolved.inputs) {
      if (r.input.default !== undefined) {
        seeded[r.input.name] = r.input.default
      }
    }
    setValues(seeded)
    setErrors({})
    setSubmitting(false)
  }, [open, resolved.inputs])

  // `compute` is applied on every change, mirroring FormRenderer's field loop.
  const computedInputNames = useMemo(
    () =>
      resolved.inputs.filter((r) => r.input.compute).map((r) => r.input.name),
    [resolved.inputs],
  )
  useEffect(() => {
    if (!open || computedInputNames.length === 0) return
    setValues((prev) => {
      const next = { ...prev }
      let changed = false
      for (const r of resolved.inputs) {
        if (!r.input.compute) continue
        const scope: EvalContext = {
          fields: { ...(record ?? {}), ...next } as RuntimeObject,
          user: user as RuntimeObject | undefined,
        }
        const value = evalCompute(r.input.compute, scope)
        if (next[r.input.name] !== value) {
          next[r.input.name] = value
          changed = true
        }
      }
      return changed ? next : prev
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, values, computedInputNames.length])

  const fields = useMemo(
    () => evaluateInputs(resolved, values, record, user),
    [resolved, values, record, user],
  )

  const handleSubmit = async () => {
    // Validate with the SAME zod rule the Form uses, so an input cannot be
    // required in one place and optional in another.
    const nextErrors: Record<string, string> = {}
    for (const r of fields) {
      if (!r.visible) continue
      if (!r.required && !r.input.rules?.length) continue
      const schema = buildZodField(r.descriptor)
      const parsed = schema.safeParse(values[r.input.name])
      if (!parsed.success) {
        nextErrors[r.input.name] =
          parsed.error.issues[0]?.message ?? "Nilai tidak valid"
      }
    }
    if (Object.keys(nextErrors).length > 0) {
      setErrors(nextErrors)
      return
    }

    setErrors({})
    setSubmitting(true)
    try {
      await onSubmit(values)
      onOpenChange(false)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {message ? <DialogDescription>{message}</DialogDescription> : null}
        </DialogHeader>

        <div className="space-y-4 py-2">
          {fields
            .filter((r) => r.visible)
            .map((r) => (
              <FormFieldWidget
                key={r.input.name}
                field={{
                  name: r.input.name,
                  label: r.label,
                  help: r.input.help,
                  widget: r.input.widget,
                }}
                entityField={r.descriptor}
                value={values[r.input.name]}
                error={errors[r.input.name]}
                readonly={r.readonly}
                onChange={(value: unknown) =>
                  setValues((prev) => ({ ...prev, [r.input.name]: value }))
                }
              />
            ))}
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={submitting}
          >
            Batal
          </Button>
          <Button
            variant={variant === "destructive" ? "destructive" : "default"}
            onClick={handleSubmit}
            disabled={submitting}
          >
            {submitting ? (
              <Loader2 className="size-4 mr-1 animate-spin" />
            ) : null}
            {confirmLabel ?? "Lanjutkan"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
