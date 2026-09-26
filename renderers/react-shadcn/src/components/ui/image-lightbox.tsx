// ─── ImageLightbox ───
//
// An image value from a `file` field used to open in a NEW BROWSER TAB
// (`target="_blank"`), which drops the user out of the app onto a bare JPEG:
// the surface, the menu and the record are all gone, and on a POS/kiosk shell
// a stray tab is easy to lose. One shared component shows the full image in a
// Dialog instead, so every image the renderer displays offers the same
// affordance.
//
// Non-image files keep their download link — opening a PDF/CSV in a tab is
// what a download link is for; only the image branch changed.

import { useState } from "react"
import type { ReactNode } from "react"

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog"

interface ImageLightboxProps {
  /** The file field's download route — the same URL the thumbnail uses. */
  src: string
  /** File name or object key; the last path segment becomes the title. */
  alt: string
  /** Thumbnail classes. The dialog image is always contained in the viewport. */
  className?: string
  /**
   * Suppress the default wrapper (a `<button>` around the image) so the caller
   * can render its own trigger.
   *
   * Needed wherever the thumbnail already sits inside a clickable parent — a
   * Table row (`<tr onClick>`) or a catalog card (`<button onClick>`). A nested
   * `<button>` is invalid HTML there, and it would swallow the parent's click,
   * so "click the image to enlarge" would silently take over "click the row to
   * open" / "click the card to add to the order". `renderTrigger` receives an
   * `open` callback, letting the caller place a small control in a corner where
   * it does not compete with the parent's hit target.
   */
  renderTrigger?: (open: () => void) => ReactNode
  /** Classes for the control produced when `renderTrigger` is given. */
  triggerClassName?: string
}

export default function ImageLightbox({
  src,
  alt,
  className,
  renderTrigger,
  triggerClassName,
}: ImageLightboxProps) {
  const [open, setOpen] = useState(false)
  const name = alt.split("/").pop() || alt

  return (
    <>
      {renderTrigger ? (
        renderTrigger(() => setOpen(true))
      ) : (
        <button
          type="button"
          onClick={() => setOpen(true)}
          aria-label={`View ${name}`}
          className="inline-block cursor-zoom-in"
        >
          <img src={src} alt={name} className={className} />
        </button>
      )}

      <Dialog open={open} onOpenChange={setOpen}>
        {/* The dialog's default `sm:max-w-sm` (384px) is smaller than the
            source photos (960px), so it would render the preview at thumbnail
            size. Keep `w-full` and widen the cap instead — `w-auto` is NOT an
            option here: a `fixed` element at `left: 50%` shrink-to-fits to
            `viewport − 50%`, which measured 462px on a 923px viewport, i.e.
            half the screen left unused. */}
        <DialogContent
          className={`w-full gap-2 p-3 sm:max-w-[92vw] ${triggerClassName ?? ""}`}
        >
          <DialogTitle className="pr-8 text-sm font-medium break-all">
            {name}
          </DialogTitle>
          <DialogDescription className="sr-only">
            Image preview
          </DialogDescription>
          <img
            src={src}
            alt={name}
            className="mx-auto max-h-[80vh] max-w-full rounded object-contain"
          />
        </DialogContent>
      </Dialog>
    </>
  )
}

/**
 * The small "enlarge" control used when the thumbnail is inside a clickable
 * parent. Absolutely positioned over the image's corner so it does not compete
 * with the parent's hit target, and it stops the event from reaching the parent
 * — clicking it must open the preview, not navigate.
 */
export function ImageLightboxTrigger({
  src,
  alt,
  className,
  thumbClassName,
}: {
  src: string
  alt: string
  className?: string
  thumbClassName?: string
}) {
  return (
    <ImageLightbox
      src={src}
      alt={alt}
      renderTrigger={(open) => (
        <span className={className ?? "relative block"}>
          <img
            src={src}
            alt={alt.split("/").pop() || alt}
            className={thumbClassName}
            loading="lazy"
          />
          <button
            type="button"
            aria-label={`View ${alt.split("/").pop() || alt}`}
            onClick={(e) => {
              // Without this the parent row/card would also act on the click:
              // the row would navigate and the catalog card would add the item
              // to the order, so "enlarge" would be indistinguishable from
              // "select".
              e.stopPropagation()
              e.preventDefault()
              open()
            }}
            className="absolute right-1 bottom-1 rounded bg-background/80 px-1 text-xs leading-5 shadow-sm hover:bg-background"
          >
            ⤢
          </button>
        </span>
      )}
    />
  )
}
