// ─── File field helpers (shared) ───
//
// A `file`/`attachment` field stores the **object key** (`{workspace}/{module}/
// {entity}/{id}/{field}/{uuid}-{name}`) — or an array of keys when
// `max_count > 1`. Everything that needs to show or validate one goes through
// here so the server (`internal/api/file.go`) and the client cannot drift.
//
// `allowed_types` canonical form is a **bare extension without the dot**
// (`jpg`, `png`, `pdf`). Three other spellings are accepted and mean the same
// thing: `.jpg`, an exact MIME type (`image/jpeg`), and a MIME wildcard
// (`image/*`). The bare form used to match nothing on either side — the form the
// spec documents was the one form that never worked (gap #4b).

/** Image extensions that can be rendered inline. */
const IMAGE_EXTENSIONS = ["png", "jpg", "jpeg", "gif", "webp", "svg", "avif"]

/**
 * True when a file field's declared `allowed_types` admit images. Mirrors
 * `spec.StorageAllowsImage` (pkg/spec/storage.go): renderers use it to decide
 * between an inline preview and a plain download link, and a field with no
 * declared types is not assumed to be images-only.
 */
export function storageAllowsImage(storage?: {
  allowed_types?: string[]
}): boolean {
  const allowed = storage?.allowed_types ?? []
  return allowed.some((raw) => {
    const t = raw.trim().toLowerCase()
    if (t.startsWith("image/")) return true
    return IMAGE_EXTENSIONS.includes(t.replace(/^\./, ""))
  })
}

/** The extension of a filename or object key, lowercase, without the dot. */
export function fileExtension(nameOrKey: string): string {
  const base = nameOrKey.split("?")[0]
  const dot = base.lastIndexOf(".")
  if (dot < 0) return ""
  return base.slice(dot + 1).toLowerCase()
}

/** True when a key/filename names an image (by extension). */
export function isImageFile(nameOrKey: string): boolean {
  return IMAGE_EXTENSIONS.includes(fileExtension(nameOrKey))
}

/**
 * Matches an `allowed_types` entry against a file, mirroring the server's
 * `allowedFileType`. Keep the two in step — a divergence means the UI accepts a
 * file the server then rejects (or the reverse).
 */
export function allowedFileType(
  allowed: string[],
  contentType: string,
  filename: string,
): boolean {
  const ext = fileExtension(filename)
  const mime = contentType.toLowerCase()
  for (const raw of allowed) {
    const t = raw.trim().toLowerCase()
    if (!t) continue
    if (t.startsWith(".")) {
      if (ext === t.slice(1)) return true
      continue
    }
    if (t.includes("/")) {
      if (t === mime) return true
      if (t.endsWith("/*") && mime.startsWith(t.slice(0, -1))) return true
      continue
    }
    // Bare extension (canonical).
    if (ext !== "" && ext === t) return true
  }
  return false
}

/**
 * The download URL of a file field. The route resolves the record's field to the
 * object in `ctx.storage`, so this is also the `<img src>` for images.
 */
export function fileDownloadUrl(
  workspace: string,
  module: string,
  entity: string,
  id: string,
  field: string,
): string {
  return `/${workspace}/_ui/entity/${module}/${entity}/${id}/${field}`
}
