// ─── DownloadTray — shows downloaded files (todo 5.9.4) ───
//
// A floating panel listing files added via formspec.files. Mounted once in
// App.tsx.

import { useEffect, useState } from "react"
import { Download, X, FileText } from "lucide-react"
import {
  onFilesChange,
  removeDownloadedFile,
  type DownloadedFile,
} from "@/lib/files"

export function DownloadTray() {
  const [files, setFiles] = useState<DownloadedFile[]>([])
  const [open, setOpen] = useState(false)

  useEffect(() => onFilesChange(setFiles), [])

  if (files.length === 0) return null

  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col items-end gap-2">
      {open && (
        <div className="w-72 rounded-lg border border-border bg-popover p-2 shadow-lg">
          <div className="flex items-center justify-between px-2 py-1">
            <span className="text-sm font-medium">Downloads</span>
            <button
              type="button"
              onClick={() => setOpen(false)}
              className="text-muted-foreground hover:text-foreground"
            >
              <X className="size-4" />
            </button>
          </div>
          <div className="space-y-1">
            {files.map((f) => (
              <div
                key={f.id}
                className="flex items-center gap-2 rounded px-2 py-1 text-sm hover:bg-accent"
              >
                <FileText className="size-4 shrink-0 text-muted-foreground" />
                <a
                  href={f.url}
                  target="_blank"
                  rel="noreferrer"
                  className="truncate text-primary hover:underline"
                >
                  {f.name}
                </a>
                <button
                  type="button"
                  onClick={() => removeDownloadedFile(f.id)}
                  className="ml-auto text-muted-foreground hover:text-foreground"
                >
                  <X className="size-3.5" />
                </button>
              </div>
            ))}
          </div>
        </div>
      )}
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="inline-flex items-center gap-1.5 rounded-full border border-border bg-popover px-3 py-2 text-sm shadow-lg hover:bg-accent"
      >
        <Download className="size-4" />
        {files.length}
      </button>
    </div>
  )
}
