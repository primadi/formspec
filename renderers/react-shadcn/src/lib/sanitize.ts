// Lightweight client-side HTML sanitizer for richtext display.
//
// Mirrors the server-side sanitizer (renderers/jsonb-persist/crud.go
// sanitizeHTML) — defense-in-depth only. The server remains the authoritative
// sanitizer on write; the client never trusts raw HTML when rendering.

const RE_SCRIPT = /<script[\s\S]*?<\/script>/gi
const RE_STYLE = /<style[\s\S]*?<\/style>/gi
const RE_DANGER =
  /<\s*\/?\s*(iframe|object|embed|form|input|button|link|meta)\b[^>]*>/gi
const RE_EVENT = /\son\w+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)/gi
const RE_JS = /\s(href|src)\s*=\s*["']?\s*javascript:[^"'\s>]*["']?/gi

export function sanitizeHTML(html: string): string {
  return html
    .replace(RE_SCRIPT, "")
    .replace(RE_STYLE, "")
    .replace(RE_DANGER, "")
    .replace(RE_EVENT, "")
    .replace(RE_JS, "")
}
