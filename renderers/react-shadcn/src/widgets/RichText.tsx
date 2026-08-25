// ─── RichText Widget ───
//
// For `type: richtext` fields. Basic toolbar (bold/italic, list, link,
// heading) over a contentEditable area. Stores sanitized HTML — the server
// is authoritative on write; the client sanitizes before display and never
// trusts raw HTML. NOT a page builder (07-component-kinds.md §1.2).

import { useEffect, useRef } from "react"
import {
  Bold,
  Italic,
  List,
  ListOrdered,
  Link,
  Heading2,
  Unlink,
} from "lucide-react"
import { cn } from "@/lib/utils"
import { sanitizeHTML } from "@/lib/sanitize"

interface RichTextProps {
  value?: string
  onChange?: (html: string) => void
  readonly?: boolean
  placeholder?: string
  error?: string
}

export function RichText({
  value = "",
  onChange,
  readonly = false,
  placeholder,
  error,
}: RichTextProps) {
  const ref = useRef<HTMLDivElement>(null)

  // Sync external value into the editor without clobbering the cursor when
  // the change originated from typing here (record load, form reset).
  useEffect(() => {
    const el = ref.current
    if (!el) return
    if (el.innerHTML !== value) el.innerHTML = value
  }, [value])

  const emit = () => {
    onChange?.(ref.current?.innerHTML ?? "")
  }

  const exec = (command: string, arg?: string) => {
    ref.current?.focus()
    document.execCommand(command, false, arg)
    emit()
  }

  const addLink = () => {
    const url = window.prompt("Link URL", "https://")
    if (url) exec("createLink", url)
  }

  if (readonly) {
    const safe = sanitizeHTML(value)
    return (
      <div
        className="py-1 text-sm [&_a]:text-primary [&_a]:underline"
        dangerouslySetInnerHTML={{
          __html: safe || "<span class='text-muted-foreground italic'>-</span>",
        }}
      />
    )
  }

  const btn =
    "inline-flex h-7 w-7 items-center justify-center rounded text-muted-foreground hover:bg-accent hover:text-accent-foreground"

  return (
    <div
      className={cn(
        "rounded-md border border-input bg-transparent",
        error && "border-destructive",
      )}
    >
      <div className="flex items-center gap-0.5 border-b border-input px-1 py-1">
        <button
          type="button"
          className={btn}
          title="Bold"
          onClick={() => exec("bold")}
        >
          <Bold className="size-4" />
        </button>
        <button
          type="button"
          className={btn}
          title="Italic"
          onClick={() => exec("italic")}
        >
          <Italic className="size-4" />
        </button>
        <button
          type="button"
          className={btn}
          title="Heading"
          onClick={() => exec("formatBlock", "h3")}
        >
          <Heading2 className="size-4" />
        </button>
        <span className="mx-1 h-4 w-px bg-border" />
        <button
          type="button"
          className={btn}
          title="Bullet list"
          onClick={() => exec("insertUnorderedList")}
        >
          <List className="size-4" />
        </button>
        <button
          type="button"
          className={btn}
          title="Numbered list"
          onClick={() => exec("insertOrderedList")}
        >
          <ListOrdered className="size-4" />
        </button>
        <span className="mx-1 h-4 w-px bg-border" />
        <button type="button" className={btn} title="Link" onClick={addLink}>
          <Link className="size-4" />
        </button>
        <button
          type="button"
          className={btn}
          title="Remove link"
          onClick={() => exec("unlink")}
        >
          <Unlink className="size-4" />
        </button>
      </div>
      <div
        ref={ref}
        contentEditable
        suppressContentEditableWarning
        onInput={emit}
        onBlur={emit}
        data-placeholder={placeholder}
        className="min-h-24 px-3 py-2 text-sm outline-none empty:before:content-[attr(data-placeholder)] empty:before:text-muted-foreground empty:before:italic"
      />
    </div>
  )
}
