// ─── useTheme Hook ───
//
// Applies theme mode (light/dark/system) and color preset to <html>.
// Reads from usePrefsStore and syncs to DOM reactively.

import { useEffect } from "react"
import { usePrefsStore, COLOR_PRESETS } from "@/stores/prefs"

export function useTheme() {
  const theme = usePrefsStore((s) => s.theme)
  const colorPreset = usePrefsStore((s) => s.colorPreset)

  useEffect(() => {
    const root = document.documentElement

    // 1. Resolve effective theme
    const isDark =
      theme === "dark" || (theme === "system" && window.matchMedia("(prefers-color-scheme: dark)").matches)

    root.classList.toggle("dark", isDark)
    root.style.colorScheme = isDark ? "dark" : "light"

    // 2. Apply color preset
    const preset = COLOR_PRESETS[colorPreset]
    if (preset) {
      root.style.setProperty("--primary", preset.primary)
      root.style.setProperty("--primary-foreground", preset["primary-foreground"])
      if (preset.accent) root.style.setProperty("--accent", preset.accent)
      if (preset.radius) root.style.setProperty("--radius", preset.radius)
    }

    // Cleanup: remove preset overrides when switching back to default
    return () => {
      if (colorPreset !== "neutral") {
        root.style.removeProperty("--primary")
        root.style.removeProperty("--primary-foreground")
        root.style.removeProperty("--accent")
        root.style.removeProperty("--radius")
      }
    }
  }, [theme, colorPreset])

  // Listen for system color scheme changes when in "system" mode
  useEffect(() => {
    if (theme !== "system") return

    const mq = window.matchMedia("(prefers-color-scheme: dark)")
    const handler = (e: MediaQueryListEvent) => {
      const root = document.documentElement
      root.classList.toggle("dark", e.matches)
      root.style.colorScheme = e.matches ? "dark" : "light"
    }
    mq.addEventListener("change", handler)
    return () => mq.removeEventListener("change", handler)
  }, [theme])
}
