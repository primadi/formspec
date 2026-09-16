// ─── QrCode Widget ───
//
// Renders a QR code for a string field (gap #3 / S4). Read-only by nature: the
// value IS the payload, so there is nothing to type — the widget exists so a
// table card, a receipt, or a public page can turn a stored token/URL into
// something a phone can scan.
//
// SVG (not canvas) on purpose: it prints crisply at any size, which is what a
// thermal receipt or a table card needs, and it needs no device pixel ratio
// handling.

import { QRCodeSVG } from "qrcode.react"
import { cn } from "@/lib/utils"

export interface QrCodeProps {
  /** The string to encode (a token, or a full URL to the ordering page). */
  value?: string
  /** Rendered size in pixels. */
  size?: number
  /** Shown instead of the QR when the value is empty. */
  emptyText?: string
  className?: string
}

export function QrCode({
  value,
  size = 128,
  emptyText,
  className,
}: QrCodeProps) {
  const payload = value == null ? "" : String(value).trim()
  if (!payload) {
    return (
      <span className="text-xs text-muted-foreground">
        {emptyText ?? "Belum ada isi"}
      </span>
    )
  }
  return (
    <span className={cn("inline-block bg-white p-1", className)}>
      <QRCodeSVG value={payload} size={size} level="M" />
    </span>
  )
}
