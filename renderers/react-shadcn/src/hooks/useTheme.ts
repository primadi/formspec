// ─── useTheme Hook ───
//
// Applies theme mode (light/dark/system) and color preset to <html>.
// Reads from usePrefsStore and syncs to DOM reactively.
//
// When a manifest theme is active (activeTheme !== null), color presets
// are skipped — the theme's YAML tokens already set all CSS variables via
// ThemeRenderer's <style> block. The user can still toggle light/dark/system
// independently (Frontend Spec §10, Opsi A).

import { useEffect } from "react"
import { usePrefsStore, COLOR_PRESETS } from "@/stores/prefs"

export function useTheme() {
  const theme = usePrefsStore((s) => s.theme)
  const colorPreset = usePrefsStore((s) => s.colorPreset)
  const activeTheme = usePrefsStore((s) => s.activeTheme)

  useEffect(() => {
    const root = document.documentElement

    // 1. Resolve effective theme
    const isDark =
      theme === "dark" || (theme === "system" && window.matchMedia("(prefers-color-scheme: dark)").matches)

    root.classList.toggle("dark", isDark)
    root.style.colorScheme = isDark ? "dark" : "light"

    // 2. Apply color preset (only when no manifest theme is active)
    //    When a theme is active, its <style> block handles all colors.
    if (!activeTheme) {
      const preset = COLOR_PRESETS[colorPreset]
      if (preset) {
        // For the neutral (black) preset in dark mode, skip overriding
        // --primary and --primary-foreground because the .dark class
        // in index.css already sets contrast-appropriate values:
        //   --primary:             oklch(0.922 0 0)  (light gray)
        //   --primary-foreground:  oklch(0.205 0 0)  (near-black)
        // Without this guard, the inline style overrides .dark and
        // text-primary resolves to near-black on near-black background
        // — invisible text. Saturated presets (blue, green, etc.) are
        // unaffected because their hues maintain contrast on dark
        // backgrounds; they still get overridden as before.
        if (isDark && colorPreset === "neutral") {
          root.style.removeProperty("--primary")
          root.style.removeProperty("--primary-foreground")
        } else {
          root.style.setProperty("--primary", preset.primary)
          root.style.setProperty("--primary-foreground", preset["primary-foreground"])
        }
        if (preset.accent) root.style.setProperty("--accent", preset.accent)
        if (preset.radius) root.style.setProperty("--radius", preset.radius)
      }
    }

    // Cleanup: remove preset overrides
    return () => {
      if (!activeTheme && colorPreset !== "neutral") {
        root.style.removeProperty("--primary")
        root.style.removeProperty("--primary-foreground")
        root.style.removeProperty("--accent")
        root.style.removeProperty("--radius")
      }
    }
  }, [theme, colorPreset, activeTheme])

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
