// ─── media helpers (gap #4b) ───
//
// The client half of the `allowed_types` contract. It must agree with the
// server's `allowedFileType` (internal/api/file.go): a divergence means the
// upload button accepts a file the server then rejects, or rejects one it would
// have accepted. The canonical form is a bare extension — the documented one —
// and it used to match nothing at all on either side.

import { describe, expect, it } from "vitest"
import {
  allowedFileType,
  fileDownloadUrl,
  fileExtension,
  isImageFile,
  storageAllowsImage,
} from "@/lib/media"

describe("fileExtension / isImageFile", () => {
  it("reads the extension of a filename or object key", () => {
    expect(
      fileExtension("kafe/cafe-master/menu-item/1/photo/abc-foto.JPG"),
    ).toBe("jpg")
    expect(fileExtension("menu.pdf")).toBe("pdf")
    expect(fileExtension("no-extension")).toBe("")
    expect(fileExtension("foto.png?v=2")).toBe("png")
  })

  it("recognises image keys", () => {
    expect(isImageFile(".../foto.webp")).toBe(true)
    expect(isImageFile(".../struk.pdf")).toBe(false)
  })
})

describe("allowedFileType", () => {
  it("accepts the canonical bare extension", () => {
    expect(allowedFileType(["jpg", "png"], "image/jpeg", "foto.JPG")).toBe(true)
  })

  it("still accepts dotted, exact-mime, and wildcard forms", () => {
    expect(allowedFileType([".jpg"], "image/jpeg", "foto.jpg")).toBe(true)
    expect(allowedFileType(["image/jpeg"], "image/jpeg", "foto.bin")).toBe(true)
    expect(allowedFileType(["image/*"], "image/webp", "foto.webp")).toBe(true)
  })

  it("rejects an unlisted type", () => {
    expect(allowedFileType(["jpg"], "image/gif", "anim.gif")).toBe(false)
    expect(allowedFileType(["pdf"], "image/jpeg", "foto.jpg")).toBe(false)
  })
})

describe("storageAllowsImage", () => {
  it("detects image fields from either spelling", () => {
    expect(storageAllowsImage({ allowed_types: ["jpg", "png"] })).toBe(true)
    expect(storageAllowsImage({ allowed_types: ["image/*"] })).toBe(true)
    expect(storageAllowsImage({ allowed_types: [".webp"] })).toBe(true)
  })

  it("does not claim a non-image field is an image", () => {
    expect(storageAllowsImage({ allowed_types: ["pdf", "docx"] })).toBe(false)
    expect(storageAllowsImage({})).toBe(false)
    expect(storageAllowsImage(undefined)).toBe(false)
  })
})

describe("fileDownloadUrl", () => {
  it("points at the entity file route", () => {
    expect(
      fileDownloadUrl("kafe", "cafe-master", "menu-item", "42", "photo"),
    ).toBe("/kafe/_ui/entity/cafe-master/menu-item/42/photo")
  })
})
