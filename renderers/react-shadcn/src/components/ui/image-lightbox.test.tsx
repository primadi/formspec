// @vitest-environment jsdom
//
// ─── Image preview opens a popup dialog, not a new tab (kafe) ───
//
// Reported on the kafe POS menu-item detail page: clicking FOTO MENU opened the
// JPEG in a NEW BROWSER TAB, so the app, sidebar and record were left behind —
// on a cashier/POS surface a stray tab is easy to lose. The image now opens in
// a Dialog (ImageLightbox) at every site that displays one.
//
// Two things must hold:
//   1. the component actually opens/closes a dialog around the image;
//   2. no image site goes back to `target="_blank"` — a re-introduced tab link
//      still "works", which is exactly why it needs pinning.

import { afterEach, describe, expect, it } from "vitest"
import { readFileSync } from "node:fs"
import { fileURLToPath } from "node:url"
import { cleanup, fireEvent, render, screen } from "@testing-library/react"
import "@testing-library/jest-dom/vitest"

import ImageLightbox, { ImageLightboxTrigger } from "./image-lightbox"

const read = (rel: string) =>
  readFileSync(fileURLToPath(new URL(rel, import.meta.url)), "utf8")

afterEach(cleanup)

const SRC = "/kafe/_ui/entity/cafe-master/menu-item/01a0c83e/photo"

describe("ImageLightbox", () => {
  it("shows the thumbnail, closed until it is clicked", () => {
    render(<ImageLightbox src={SRC} alt="kopi-tubruk.jpg" />)

    const trigger = screen.getByRole("button", { name: "View kopi-tubruk.jpg" })
    expect(trigger).toBeInTheDocument()
    expect(trigger.querySelector("img")).toHaveAttribute("src", SRC)
    // Closed: the dialog content (title + preview) must not be mounted yet.
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument()
  })

  it("opens the full image in a dialog on click", async () => {
    render(<ImageLightbox src={SRC} alt="kopi-tubruk.jpg" />)

    fireEvent.click(
      screen.getByRole("button", { name: "View kopi-tubruk.jpg" }),
    )

    const dialog = await screen.findByRole("dialog")
    // The title is the file name — the object key's last segment, since the
    // stored value is a key and not a display name.
    expect(dialog).toHaveTextContent("kopi-tubruk.jpg")
    const preview = dialog.querySelector('img[src="' + SRC + '"]')
    expect(preview).not.toBeNull()
    // Contained in the viewport rather than a fixed size: source photos are
    // portrait and landscape (960×1280 vs 960×643), so a fixed box would crop
    // or overflow one of them.
    expect(preview?.className).toContain("object-contain")
    expect(preview?.className).toContain("max-h-[80vh]")
  })

  it("sizes the panel for the photo, not the dialog default", async () => {
    render(<ImageLightbox src={SRC} alt="kopi-tubruk.jpg" />)
    fireEvent.click(
      screen.getByRole("button", { name: "View kopi-tubruk.jpg" }),
    )
    const dialog = await screen.findByRole("dialog")

    // The default `sm:max-w-sm` (384px) is narrower than the source photos
    // (960px), so the preview would render at thumbnail size; and `w-auto` on a
    // `fixed` left:50% element shrink-to-fits to `viewport − 50%` (measured
    // 462px on a 923px viewport — half the screen unused). `w-full` + a wide
    // cap is the pair that actually fills the viewport.
    expect(dialog.className).toContain("w-full")
    expect(dialog.className).toContain("sm:max-w-[92vw]")
    expect(dialog.className).not.toContain("sm:max-w-sm")
    expect(dialog.className).not.toContain("w-auto")
  })

  it("closes again from the dialog's close control", async () => {
    render(<ImageLightbox src={SRC} alt="kopi-tubruk.jpg" />)

    fireEvent.click(
      screen.getByRole("button", { name: "View kopi-tubruk.jpg" }),
    )
    await screen.findByRole("dialog")

    fireEvent.click(screen.getByRole("button", { name: "Close" }))
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument()
  })
})

describe("image display sites", () => {
  // Every `<ImageLightbox … />` element in a file, taken up to its own `/>`.
  // A slice bounded by the element (not a fixed-width window) is what keeps a
  // neighbouring NON-image download link — correctly `target="_blank"` — from
  // being read as a regression.
  const lightboxElements = (src: string): string[] => {
    const out: string[] = []
    let idx = src.indexOf("<ImageLightbox")
    while (idx >= 0) {
      const end = src.indexOf("/>", idx)
      out.push(src.slice(idx, end))
      idx = src.indexOf("<ImageLightbox", end)
    }
    return out
  }

  const assertNoTab = (src: string, expected: number) => {
    const blocks = lightboxElements(src)
    expect(blocks).toHaveLength(expected)
    for (const b of blocks) {
      expect(b).not.toContain("target=")
      expect(b).not.toContain("<a")
    }
    // The old shape in both files: an anchor with `target="_blank"` wrapping
    // the preview `<img>`. Non-image download links stay as they are.
    expect(src).not.toMatch(/<a\b[^>]*target="_blank"[^>]*>\s*<img/)
  }

  it("DetailPage routes image fields through the lightbox", () => {
    const src = read("../../kinds/page/DetailPage.tsx")
    assertNoTab(src, 1)
    // Not just present — inside the `file` branch.
    expect(src.slice(src.indexOf('field.type === "file"'))).toContain(
      "<ImageLightbox",
    )
  })

  it("FileInput routes both image previews through the lightbox", () => {
    const src = read("../../widgets/FileInput.tsx")
    // Readonly preview + edit-mode thumbnail.
    assertNoTab(src, 2)
  })

  it("table cells use the corner trigger, not a wrapping button (5.21.2)", () => {
    // A cell image sits inside a clickable row (`<tr onClick>` opens the
    // record). Wrapping the thumbnail in a `<button>` would swallow that click,
    // so the cell must use the corner control instead — and the trigger itself
    // must not carry a tab link.
    const src = read("../../lib/renderCell.tsx")
    const idx = src.indexOf("<ImageLightboxTrigger")
    expect(idx).toBeGreaterThan(-1)
    const element = src.slice(idx, src.indexOf("/>", idx))
    expect(element).not.toContain("target=")
    expect(element).not.toContain("<a")
    // The `<a target="_blank">` that remains below is the NON-image fallback,
    // which is the documented behaviour ("opening a PDF/CSV in a tab is what a
    // download link is for") — it must stay, so it is not asserted away.
    expect(src.slice(idx)).toContain('target="_blank"')
  })

  it("catalog cards use the corner trigger, not the photo (5.21.2)", () => {
    // The card IS a `<button>` that adds the item to the order (measured:
    // clicking the photo totalled Rp18.000). "Click the photo to enlarge" would
    // therefore be indistinguishable from "add to order".
    const src = read("../../kinds/form/PickerPanel.tsx")
    expect(src).toContain("<ImageLightboxTrigger")
  })
})

describe("ImageLightboxTrigger — sits inside a clickable parent", () => {
  it("opens the preview and stops the click from reaching the parent", async () => {
    const parentClicks: number[] = []
    render(
      <button type="button" onClick={() => parentClicks.push(1)}>
        <ImageLightboxTrigger
          src={SRC}
          alt="kopi-tubruk.jpg"
          thumbClassName="h-10 w-10"
        />
      </button>,
    )

    fireEvent.click(
      screen.getByRole("button", { name: "View kopi-tubruk.jpg" }),
    )

    // The dialog opened...
    expect(await screen.findByRole("dialog")).toBeInTheDocument()
    // ...and the parent did NOT act. Without `stopPropagation` this is where
    // the row would navigate / the card would be added to the cart.
    expect(parentClicks).toHaveLength(0)
  })

  it("still renders the thumbnail when the dialog is closed", () => {
    render(
      <ImageLightboxTrigger
        src={SRC}
        alt="kopi-tubruk.jpg"
        thumbClassName="h-10 w-10"
      />,
    )
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument()
    expect(document.querySelector(`img[src="${SRC}"]`)).not.toBeNull()
  })
})
