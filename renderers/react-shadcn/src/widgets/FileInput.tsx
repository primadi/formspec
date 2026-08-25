// ─── FileInput Widget ───
//
// For `type: file` fields. Uploads to the object store via the file upload
// route (todo 7.17.1) and stores the returned object key as the field value.
// Shows a preview for images/PDF and enforces size/type from StorageSpec.

import { useRef, useState } from "react"
import { useParams } from "react-router-dom"
import { Upload, X, FileText, Loader2 } from "lucide-react"
import { useSessionStore } from "@/stores/session"
import { cn } from "@/lib/utils"

interface FileInputProps {
  value?: string // object key
  onChange?: (key: string | null) => void
  readonly?: boolean
  error?: string
  entityModule?: string
  entityName?: string
  recordId?: string
  fieldName?: string
  maxSizeMB?: number
  allowedTypes?: string[]
}

function allowedFileType(
  allowed: string[],
  contentType: string,
  filename: string,
): boolean {
  const ext = filename.toLowerCase().split(".").pop() ?? ""
  for (const a of allowed) {
    const t = a.trim().toLowerCase()
    if (!t) continue
    if (t.startsWith(".")) {
      if (ext === t.slice(1)) return true
      continue
    }
    if (t === contentType) return true
    if (t.endsWith("/*") && contentType.startsWith(t.slice(0, -1))) return true
  }
  return false
}

export function FileInput({
  value,
  onChange,
  readonly = false,
  error,
  entityModule,
  entityName,
  recordId,
  fieldName,
  maxSizeMB,
  allowedTypes,
}: FileInputProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [uploading, setUploading] = useState(false)
  const [uploadError, setUploadError] = useState<string | null>(null)
  const getClient = useSessionStore((s) => s.getClient)
  const { workspace = "default" } = useParams<{ workspace: string }>()

  const canUpload =
    !readonly && !!entityModule && !!entityName && !!recordId && !!fieldName
  const downloadUrl =
    entityModule && entityName && recordId && fieldName
      ? `/${workspace}/_ui/entity/${entityModule}/${entityName}/${recordId}/${fieldName}`
      : null

  const isImage = value ? /\.(png|jpe?g|gif|webp|svg)$/i.test(value) : false

  const handleFile = async (file: File) => {
    if (!canUpload) return
    if (maxSizeMB && file.size > maxSizeMB * 1024 * 1024) {
      setUploadError(`File exceeds max_size_mb=${maxSizeMB}`)
      return
    }
    if (
      allowedTypes &&
      allowedTypes.length > 0 &&
      !allowedFileType(allowedTypes, file.type, file.name)
    ) {
      setUploadError("File type not allowed")
      return
    }
    setUploading(true)
    setUploadError(null)
    try {
      const fd = new FormData()
      fd.append("file", file)
      const client = getClient()
      const res = await client.post(
        `${entityModule}/${entityName}/${recordId}/${fieldName}`,
        { body: fd },
      )
      const body = (await res.json()) as { key: string }
      onChange?.(body.key)
    } catch (e) {
      setUploadError(e instanceof Error ? e.message : "Upload failed")
    } finally {
      setUploading(false)
    }
  }

  // Readonly / view mode: show preview or placeholder.
  if (readonly) {
    if (!value) {
      return <div className="py-1 text-sm text-muted-foreground italic">-</div>
    }
    return (
      <div className="py-1">
        {isImage && downloadUrl ? (
          <a href={downloadUrl} target="_blank" rel="noreferrer">
            <img
              src={downloadUrl}
              alt={value}
              className="max-h-32 rounded border"
            />
          </a>
        ) : (
          <a
            href={downloadUrl ?? "#"}
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-1.5 text-sm text-primary hover:underline"
          >
            <FileText className="size-4" />
            {value.split("/").pop()}
          </a>
        )}
      </div>
    )
  }

  return (
    <div className={cn("space-y-1.5", error && "text-destructive")}>
      {value ? (
        <div className="flex items-center gap-2">
          {isImage && downloadUrl ? (
            <img
              src={downloadUrl}
              alt={value}
              className="h-12 w-12 rounded border object-cover"
            />
          ) : (
            <FileText className="size-5 text-muted-foreground" />
          )}
          <a
            href={downloadUrl ?? "#"}
            target="_blank"
            rel="noreferrer"
            className="truncate text-sm text-primary hover:underline"
          >
            {value.split("/").pop()}
          </a>
          <button
            type="button"
            className="ml-auto inline-flex items-center gap-1 rounded border px-2 py-1 text-xs hover:bg-accent"
            onClick={() => inputRef.current?.click()}
          >
            <Upload className="size-3.5" /> Replace
          </button>
          <button
            type="button"
            className="inline-flex items-center gap-1 rounded border px-2 py-1 text-xs hover:bg-accent"
            onClick={() => onChange?.(null)}
          >
            <X className="size-3.5" /> Remove
          </button>
        </div>
      ) : (
        <button
          type="button"
          disabled={!canUpload || uploading}
          className="inline-flex items-center gap-1.5 rounded-md border border-dashed px-3 py-2 text-sm text-muted-foreground hover:bg-accent disabled:cursor-not-allowed disabled:opacity-50"
          onClick={() => inputRef.current?.click()}
        >
          {uploading ? (
            <Loader2 className="size-4 animate-spin" />
          ) : (
            <Upload className="size-4" />
          )}
          {uploading ? "Uploading…" : "Upload file"}
        </button>
      )}
      {!canUpload && !value && (
        <p className="text-xs text-muted-foreground">
          Save the record first to upload files.
        </p>
      )}
      {uploadError && <p className="text-xs text-destructive">{uploadError}</p>}
      <input
        ref={inputRef}
        type="file"
        className="hidden"
        onChange={(e) => {
          const f = e.target.files?.[0]
          if (f) void handleFile(f)
          e.target.value = ""
        }}
      />
    </div>
  )
}
