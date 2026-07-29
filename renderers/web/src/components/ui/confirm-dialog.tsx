// ─── ConfirmDialog ───
//
// Reusable confirmation dialog component to replace native window.confirm().
// Uses shadcn/ui Dialog (base-ui) + lucide-react icons + tailwindcss-animate.
//
// Variants:
//   - default:   AlertTriangle (amber/icon-warning)
//   - destructive: XCircle (red)
//   - warning:   TriangleAlert (orange)

import * as React from "react"
import { AlertTriangle, XCircle, TriangleAlert } from "lucide-react"

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogTitle,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

type ConfirmVariant = "default" | "destructive" | "warning"

interface ConfirmDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  message: string
  variant?: ConfirmVariant
  confirmLabel?: string
  cancelLabel?: string
  icon?: React.ReactNode
  onConfirm: () => void
  onCancel?: () => void
}

const variantConfig: Record<
  ConfirmVariant,
  {
    icon: React.ReactNode
    bgClass: string
    iconClass: string
    buttonVariant: "default" | "destructive" | "outline"
  }
> = {
  default: {
    icon: <AlertTriangle className="size-5" />,
    bgClass: "bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400",
    iconClass: "",
    buttonVariant: "default",
  },
  destructive: {
    icon: <XCircle className="size-5" />,
    bgClass: "bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-400",
    iconClass: "",
    buttonVariant: "destructive",
  },
  warning: {
    icon: <TriangleAlert className="size-5" />,
    bgClass: "bg-orange-100 text-orange-600 dark:bg-orange-900/30 dark:text-orange-400",
    iconClass: "",
    buttonVariant: "default",
  },
}

export default function ConfirmDialog({
  open,
  onOpenChange,
  title,
  message,
  variant = "default",
  confirmLabel = "Konfirmasi",
  cancelLabel = "Batal",
  icon: customIcon,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const config = variantConfig[variant]
  const displayIcon = customIcon ?? config.icon

  const handleConfirm = () => {
    onConfirm()
  }

  const handleCancel = () => {
    onCancel?.()
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        className="sm:max-w-sm gap-0 p-0"
      >
        {/* Icon */}
        <div className="flex justify-center pt-6 pb-2">
          <div
            className={cn(
              "flex size-10 items-center justify-center rounded-full ring-1 ring-inset ring-current/20 animate-in zoom-in-0 fade-in-0 duration-200 delay-75",
              config.bgClass,
            )}
          >
            {displayIcon}
          </div>
        </div>

        {/* Title */}
        <DialogTitle className="text-center px-6 pb-1">
          {title}
        </DialogTitle>

        {/* Message */}
        <DialogDescription className="text-center px-6 pb-4 text-sm">
          {message}
        </DialogDescription>

        {/* Actions */}
        <DialogFooter className="px-6 py-4 gap-2">
          <Button
            variant="outline"
            onClick={handleCancel}
            className="flex-1 sm:flex-none"
          >
            {cancelLabel}
          </Button>
          <Button
            variant={config.buttonVariant}
            onClick={handleConfirm}
            className="flex-1 sm:flex-none"
          >
            {confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
