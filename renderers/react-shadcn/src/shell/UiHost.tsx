// ─── UiHost — renders formspec.ui confirm/dialog/drawer requests ───
//
// Subscribes to lib/ui confirm/drawer events and renders the UI, resolving
// the promise when the user acts. Mounted once in App.tsx.

import { useEffect, useState } from "react"
import ConfirmDialog from "@/components/ui/confirm-dialog"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import {
  onConfirm,
  onDrawer,
  type UiDialogOptions,
  type UiDrawerOptions,
} from "@/lib/ui"

interface PendingConfirm {
  id: number
  options: UiDialogOptions
  resolve: (value: boolean) => void
}

interface PendingDrawer {
  id: number
  options: UiDrawerOptions
  resolve: (value: boolean) => void
}

export function UiHost() {
  const [confirmReq, setConfirmReq] = useState<PendingConfirm | null>(null)
  const [drawerReq, setDrawerReq] = useState<PendingDrawer | null>(null)

  useEffect(() => {
    const offConfirm = onConfirm((req) => setConfirmReq(req))
    const offDrawer = onDrawer((req) => setDrawerReq(req))
    return () => {
      offConfirm()
      offDrawer()
    }
  }, [])

  return (
    <>
      <ConfirmDialog
        open={!!confirmReq}
        onOpenChange={(open) => {
          if (!open && confirmReq) {
            confirmReq.resolve(false)
            setConfirmReq(null)
          }
        }}
        title={confirmReq?.options.title ?? ""}
        message={confirmReq?.options.message ?? ""}
        variant={
          confirmReq?.options.variant === "destructive"
            ? "destructive"
            : confirmReq?.options.variant === "warning"
              ? "warning"
              : "default"
        }
        confirmLabel={confirmReq?.options.confirmLabel}
        cancelLabel={confirmReq?.options.cancelLabel}
        onConfirm={() => {
          confirmReq?.resolve(true)
          setConfirmReq(null)
        }}
        onCancel={() => {
          confirmReq?.resolve(false)
          setConfirmReq(null)
        }}
      />
      <Sheet
        open={!!drawerReq}
        onOpenChange={(open) => {
          if (!open && drawerReq) {
            drawerReq.resolve(true)
            setDrawerReq(null)
          }
        }}
      >
        <SheetContent side={drawerReq?.options.side ?? "right"}>
          <SheetHeader>
            <SheetTitle>{drawerReq?.options.title}</SheetTitle>
          </SheetHeader>
          <div className="px-4 pb-4">{drawerReq?.options.content}</div>
        </SheetContent>
      </Sheet>
    </>
  )
}
