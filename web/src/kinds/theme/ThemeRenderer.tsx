// ─── Theme Renderer ───
//
// Theme tokens → CSS custom properties.
// Applies theme variables via useEffect when the manifest changes.
//
// Design doc §5.5 Theme kind (F5)

import { useEffect } from "react"
import type { Entry, ThemeSpec } from "@/types/manifest"

interface ThemeRendererProps {
  entry: Entry<ThemeSpec>
}

export default function ThemeRenderer({ entry }: ThemeRendererProps) {
  useEffect(() => {
    const tokens = entry.spec.tokens
    if (!tokens) return

    const root = document.documentElement
    const applied: string[] = []

    for (const [key, value] of Object.entries(tokens)) {
      // Support both --prefix and bare names
      const cssVar = key.startsWith("--") ? key : `--${key}`
      root.style.setProperty(cssVar, value)
      applied.push(cssVar)
    }

    // Apply stylesheet if provided
    if (entry.spec.stylesheet) {
      const styleId = `forma-theme-${entry.name}`
      let styleEl = document.getElementById(styleId)
      if (!styleEl) {
        styleEl = document.createElement("style")
        styleEl.id = styleId
        document.head.appendChild(styleEl)
      }
      styleEl.textContent = entry.spec.stylesheet
    }

    return () => {
      // Cleanup: remove applied tokens
      for (const cssVar of applied) {
        root.style.removeProperty(cssVar)
      }
      // Remove stylesheet
      if (entry.spec.stylesheet) {
        const styleEl = document.getElementById(`forma-theme-${entry.name}`)
        if (styleEl) styleEl.remove()
      }
    }
  }, [entry.spec, entry.name])

  // This renderer doesn't produce visible UI
  return null
}
