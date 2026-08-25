// ─── Preferences Store ───
//
// Zustand store for user preferences that persist to localStorage.
// Currently limited to UI chrome: sidebar collapse state and theme.

import { create } from "zustand"
import { persist } from "zustand/middleware"

export type ThemeMode = "light" | "dark" | "system"

// ── Color Presets ──
// Each preset overrides primary/accent/radius CSS vars via data attribute.
export interface ColorPreset {
  name: string
  label: string
  primary: string
  "primary-foreground": string
  accent?: string
  radius?: string
}

export const COLOR_PRESETS: Record<string, ColorPreset> = {
  neutral: {
    name: "neutral",
    label: "Neutral (default)",
    primary: "oklch(0.205 0 0)",
    "primary-foreground": "oklch(0.985 0 0)",
  },
  blue: {
    name: "blue",
    label: "Biru",
    primary: "oklch(0.546 0.245 262.881)",
    "primary-foreground": "oklch(0.985 0 0)",
  },
  green: {
    name: "green",
    label: "Hijau",
    primary: "oklch(0.527 0.154 149.784)",
    "primary-foreground": "oklch(0.985 0 0)",
  },
  violet: {
    name: "violet",
    label: "Violet",
    primary: "oklch(0.541 0.281 293.009)",
    "primary-foreground": "oklch(0.985 0 0)",
  },
  orange: {
    name: "orange",
    label: "Orange",
    primary: "oklch(0.646 0.222 41.116)",
    "primary-foreground": "oklch(0.985 0 0)",
  },
  rose: {
    name: "rose",
    label: "Rose",
    primary: "oklch(0.592 0.221 25.332)",
    "primary-foreground": "oklch(0.985 0 0)",
  },
}

/** Default auto-logout idle timeout in minutes (0 = disabled). */
export const DEFAULT_SESSION_TIMEOUT_MINUTES = 30

/**
 * A user's saved widget layout for one customizable dashboard. Mirrors
 * `DashboardWidget` from the manifest but is stored as a runtime preference
 * (5.7.3) — never written back to YAML.
 */
export interface DashboardLayoutPref {
  ref: string
  layout: { x: number; y: number; w: number; h: number }
  config?: Record<string, unknown>
}

export interface PrefsState {
  sidebarCollapsed: boolean
  theme: ThemeMode
  colorPreset: string
  /** Name of the active manifest theme, or null for no theme (use index.css defaults) */
  activeTheme: string | null
  /** Auto-logout idle timeout in minutes (0 = disabled). Set on the login screen. */
  sessionTimeoutMinutes: number
  /** Per-dashboard saved widget layouts (5.7.3) — keyed by dashboard name. */
  dashboardLayouts: Record<string, DashboardLayoutPref[]>
  // ── Actions ──
  toggleSidebar: () => void
  setSidebarCollapsed: (collapsed: boolean) => void
  setTheme: (theme: ThemeMode) => void
  setColorPreset: (name: string) => void
  setActiveTheme: (name: string | null) => void
  setSessionTimeoutMinutes: (minutes: number) => void
  setDashboardLayout: (name: string, widgets: DashboardLayoutPref[]) => void
}

export const usePrefsStore = create<PrefsState>()(
  persist(
    (set) => ({
      sidebarCollapsed: false,
      theme: "system",
      colorPreset: "neutral",
      activeTheme: null,
      sessionTimeoutMinutes: DEFAULT_SESSION_TIMEOUT_MINUTES,
      dashboardLayouts: {},

      toggleSidebar: () =>
        set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
      setSidebarCollapsed: (collapsed) => set({ sidebarCollapsed: collapsed }),
      setTheme: (theme) => set({ theme }),
      setColorPreset: (name) => set({ colorPreset: name }),
      setActiveTheme: (name) => set({ activeTheme: name }),
      setSessionTimeoutMinutes: (minutes) =>
        set({ sessionTimeoutMinutes: minutes }),
      setDashboardLayout: (name, widgets) =>
        set((s) => ({
          dashboardLayouts: { ...s.dashboardLayouts, [name]: widgets },
        })),
    }),
    {
      name: "formspec-prefs",
    },
  ),
)
