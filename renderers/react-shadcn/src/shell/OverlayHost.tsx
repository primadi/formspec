// ─── Overlay Host ───
//
// Renders modal or drawer forms controlled by URL query parameters.
//
// Design-time locking (Frontend §1.6):
//   `Form.render.mode` = "modal" | "drawer" | "separate_page"
//   - modal → shadcn Dialog
//   - drawer → shadcn Sheet (right side)
//   - separate_page → rendered as a route (not handled here)
//
// URL pattern:
//   ?action=create&form=order-quick-edit
//   ?action=edit&id=123&form=order-edit&mode=modal
//
// Back button closes overlay. URL is shareable.

import { lazy, Suspense, useCallback } from "react"
import { useSearchParams, useNavigate } from "react-router-dom"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet"
import { useMetaStore } from "@/stores/meta"
import { resolveEntityRef } from "@/engine/entityRef"
import { titleCase } from "@/lib/utils"
import { Skeleton } from "@/components/ui/skeleton"

const FormRenderer = lazy(() => import("@/kinds/form/FormRenderer"))

// ── OverlayHost ──

export function OverlayHost() {
  const [searchParams, _setSearchParams] = useSearchParams()
  const navigate = useNavigate()

  const action = searchParams.get("action")
  const formName = searchParams.get("form")
  // Fallback for entities with no authored Form manifest — TableRenderer
  // sends `entity=module.name` instead of `form` and OverlayHost derives the
  // form spec the same way a standalone FormRenderer route would.
  const entityParam = searchParams.get("entity")
  const id = searchParams.get("id") ?? undefined
  const mode = (searchParams.get("mode") ?? "modal") as "modal" | "drawer"

  const getForm = useMetaStore((s) => s.getForm)
  const getEntity = useMetaStore((s) => s.getEntity)

  const close = useCallback(() => {
    const next = new URLSearchParams(searchParams)
    next.delete("action")
    next.delete("form")
    next.delete("entity")
    next.delete("id")
    next.delete("mode")
    const str = next.toString()
    navigate(str ? `?${str}` : ".", { replace: true })
  }, [searchParams, navigate])

  if (!action || (!formName && !entityParam)) return null

  const form = formName ? getForm(formName) : undefined
  if (formName && !form) return null

  // Resolve entity: from the authored form's spec.entity ("module.entity"),
  // or directly from the `entity` param in the derived-form fallback.
  // resolveEntityRef splits at the last dot so dotted module names
  // (e.g. "formspec.core.role") resolve correctly.
  const entityRef = form ? form.spec.entity : entityParam!
  const [module, entityName] = resolveEntityRef(entityRef, form?.module ?? "")
  const entity = getEntity(module, entityName)
  if (!entity) return null

  // Determine overlay mode: authored spec > URL param > default modal
  const overlayMode = form?.spec.render?.mode ?? mode ?? "modal"
  const isOpen = action === "create" || action === "edit"
  const formMode = action === "edit" ? "edit" : "create"

  const overlayContent = (
    <Suspense
      fallback={
        <div className="flex justify-center py-8">
          <Skeleton className="h-32 w-full max-w-md" />
        </div>
      }
    >
      <FormRenderer
        entity={entity}
        mode={formMode}
        id={id}
        formRef={formName ?? undefined}
        inOverlay
        onClose={close}
      />
    </Suspense>
  )

  if (overlayMode === "drawer") {
    return (
      <Sheet
        open={isOpen}
        onOpenChange={(open: boolean) => {
          if (!open) close()
        }}
      >
        <SheetContent className="w-full sm:max-w-lg md:max-w-xl lg:max-w-2xl overflow-y-auto">
          <SheetHeader>
            <SheetTitle>
              {action === "create" ? "New" : "Edit"} {titleCase(entity.name)}
            </SheetTitle>
            <SheetDescription>
              {form?.spec.sections?.[0]?.description ??
                `Fill in the details for this ${entity.name}.`}
            </SheetDescription>
          </SheetHeader>
          <div className="mt-6">{overlayContent}</div>
        </SheetContent>
      </Sheet>
    )
  }

  // Default: modal
  return (
    <Dialog
      open={isOpen}
      onOpenChange={(open: boolean) => {
        if (!open) close()
      }}
    >
      <DialogContent className="sm:max-w-lg md:max-w-xl overflow-y-auto max-h-[85vh]">
        <DialogHeader>
          <DialogTitle>
            {action === "create" ? "New" : "Edit"} {titleCase(entity.name)}
          </DialogTitle>
          <DialogDescription>
            {form?.spec.sections?.[0]?.description ??
              `Fill in the details for this ${entity.name}.`}
          </DialogDescription>
        </DialogHeader>
        <div className="mt-4">{overlayContent}</div>
      </DialogContent>
    </Dialog>
  )
}
