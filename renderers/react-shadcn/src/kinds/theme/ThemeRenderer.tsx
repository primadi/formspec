// ─── Theme Renderer ───
//
// Theme tokens → CSS custom properties via a <style> element.
//
// CRITICAL DESIGN: we inject tokens as a proper <style> block with :root
// selector, NOT as inline styles on <html>. Inline styles have the highest
// CSS specificity and would override .dark class rules — breaking the
// light/dark/system toggle entirely (Frontend Spec §10, D35).
//
// With a <style> block:
//   :root { --primary: #8B5E3C; ... }     ← theme tokens (light)
//   .dark { --primary: #D4A76A; ... }      ← theme stylesheet (dark)
//   .dark { --primary: oklch(0.922…); }    ← index.css (fallback)
//
// Cascade wins: stylesheet's .dark > index.css .dark due to insertion order.
// The user's light/dark/system toggle (via .dark class) works correctly.
//
// Design doc §5.5 Theme kind (F5)

import { useEffect, useRef } from "react"
import type { Entry, ThemeSpec } from "@/types/manifest"

// ── Token → CSS variable mapping ──

const FONT_MAP: Record<string, string> = {
  family: "sans",
  heading: "heading",
  mono: "mono",
};

function tokenToCssVar(key: string): string {
  if (key.startsWith("--")) return key
  if (key.startsWith("color.")) return `--${key.slice("color.".length)}`
  if (key.startsWith("radius.")) return "--radius"
  if (key.startsWith("font.")) {
    const name = key.slice("font.".length)
    const mapped = FONT_MAP[name]
    return mapped ? `--${mapped}` : `--font-${name}`
  }
  return `--${key}`
}

interface ThemeRendererProps {
  entry: Entry<ThemeSpec> | null
}

export default function ThemeRenderer({ entry }: ThemeRendererProps) {
  // Use a ref to track the style element for cleanup
  const styleRef = useRef<HTMLStyleElement | null>(null)
  const sheetRef = useRef<HTMLStyleElement | null>(null)

  useEffect(() => {
    // Cleanup previous style elements
    if (styleRef.current) {
      styleRef.current.remove()
      styleRef.current = null
    }
    if (sheetRef.current) {
      sheetRef.current.remove()
      sheetRef.current = null
    }

    if (!entry) return

    // ── 1. Build token CSS ──
    const tokens = entry.spec.tokens
    const rules: string[] = []

    if (tokens) {
      const vars: string[] = []
      for (const [key, value] of Object.entries(tokens)) {
        vars.push(`  ${tokenToCssVar(key)}: ${value};`)
      }
      if (vars.length > 0) {
        rules.push(`:root {\n${vars.join("\n")}\n}`)
      }
    }

    // ── 2. Inject token <style> ──
    if (rules.length > 0) {
      const styleEl = document.createElement("style")
      styleEl.id = `formspec-theme-${entry.name}-tokens`
      styleEl.textContent = rules.join("\n")
      document.head.appendChild(styleEl)
      styleRef.current = styleEl
    }

    // ── 3. Inject stylesheet if provided ──
    if (entry.spec.stylesheet) {
      const sheetEl = document.createElement("style")
      sheetEl.id = `formspec-theme-${entry.name}-sheet`
      sheetEl.textContent = entry.spec.stylesheet
      document.head.appendChild(sheetEl)
      sheetRef.current = sheetEl
    }

    return () => {
      if (styleRef.current) styleRef.current.remove()
      if (sheetRef.current) sheetRef.current.remove()
    }
  }, [entry])

  return null
}
