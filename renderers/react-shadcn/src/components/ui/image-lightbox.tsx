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
}

export default function ImageLightbox({
  src,
  alt,
  className,
}: ImageLightboxProps) {
  const [open, setOpen] = useState(false)
  const name = alt.split("/").pop() || alt

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        aria-label={`View ${name}`}
        className="inline-block cursor-zoom-in"
      >
        <img src={src} alt={name} className={className} />
      </button>

      <Dialog open={open} onOpenChange={setOpen}>
        {/* The dialog's default `sm:max-w-sm` (384px) is smaller than the
            source photos (960px), so it would render the preview at thumbnail
            size. Keep `w-full` and widen the cap instead — `w-auto` is NOT an
            option here: a `fixed` element at `left: 50%` shrink-to-fits to
            `viewport − 50%`, which measured 462px on a 923px viewport, i.e.
            half the screen left unused. */}
        <DialogContent className="w-full gap-2 p-3 sm:max-w-[92vw]">
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
