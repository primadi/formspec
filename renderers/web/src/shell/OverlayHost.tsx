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

import { useCallback } from "react"
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

// ── OverlayHost ──

export function OverlayHost() {
  const [searchParams, _setSearchParams] = useSearchParams()
  const navigate = useNavigate()

  const action = searchParams.get("action")
  const formName = searchParams.get("form")
  const mode = (searchParams.get("mode") ?? "modal") as "modal" | "drawer"

  const getForm = useMetaStore((s) => s.getForm)

  const close = useCallback(() => {
    const next = new URLSearchParams(searchParams)
    next.delete("action")
    next.delete("form")
    next.delete("id")
    next.delete("mode")
    const str = next.toString()
    navigate(str ? `?${str}` : ".", { replace: true })
  }, [searchParams, navigate])

  if (!action || !formName) return null

  const form = getForm(formName)
  if (!form) {
    // Fallback: try to find a derived form
    return null
  }

  const isOpen = action === "create" || action === "edit"

  if (mode === "drawer") {
    return (
      <Sheet open={isOpen} onOpenChange={(open: boolean) => { if (!open) close() }}>
        <SheetContent className="w-full sm:max-w-lg md:max-w-xl lg:max-w-2xl overflow-y-auto">
          <SheetHeader>
            <SheetTitle>
              {action === "create" ? "Create" : "Edit"} {form.spec.entity}
            </SheetTitle>
            <SheetDescription>
              {form.spec.sections?.[0]?.description ?? `Fill in the details for this ${form.spec.entity}.`}
            </SheetDescription>
          </SheetHeader>
          <div className="mt-6">
            {/* Form renderer will go here in Fase 4.F3 */}
            <p className="text-sm text-muted-foreground">
              Form renderer coming in Fase 4.F3
            </p>
          </div>
        </SheetContent>
      </Sheet>
    )
  }

  // Default: modal
  return (
      <Dialog open={isOpen} onOpenChange={(open: boolean) => { if (!open) close() }}>
      <DialogContent className="sm:max-w-lg md:max-w-xl overflow-y-auto max-h-[85vh]">
        <DialogHeader>
          <DialogTitle>
            {action === "create" ? "New" : "Edit"} {form.spec.entity}
          </DialogTitle>
          <DialogDescription>
            {form.spec.sections?.[0]?.description ?? `Fill in the details for this ${form.spec.entity}.`}
          </DialogDescription>
        </DialogHeader>
        <div className="mt-4">
          {/* Form renderer will go here in Fase 4.F3 */}
          <p className="text-sm text-muted-foreground">
            Form renderer coming in Fase 4.F3
          </p>
        </div>
      </DialogContent>
    </Dialog>
  )
}
