// ─── Theme Switcher ───
//
// Compact dropdown: mode horizontal, theme compact list, warna grid.
// Theme descriptions disembunyikan — cukup nama + color dot.
// Color presets hanya muncul saat tidak ada manifest theme aktif (Opsi A).
//
// Frontend Spec §10 — Theme kind.

import { useState, useRef, useEffect } from "react"
import { Moon, Sun, Monitor, Check } from "lucide-react"
import { cn } from "@/lib/utils"
import { usePrefsStore, COLOR_PRESETS, type ThemeMode } from "@/stores/prefs"
import { useMetaStore } from "@/stores/meta"
import { Button } from "@/components/ui/button"

export function ThemeSwitcher() {
  const theme = usePrefsStore((s) => s.theme)
  const colorPreset = usePrefsStore((s) => s.colorPreset)
  const activeTheme = usePrefsStore((s) => s.activeTheme)
  const setTheme = usePrefsStore((s) => s.setTheme)
  const setColorPreset = usePrefsStore((s) => s.setColorPreset)
  const setActiveTheme = usePrefsStore((s) => s.setActiveTheme)

  const bundle = useMetaStore((s) => s.bundle)
  const manifestThemes = bundle?.themes ?? []

  const [open, setOpen] = useState(false)
  const panelRef = useRef<HTMLDivElement>(null)
  const btnRef = useRef<HTMLButtonElement>(null)

  // Close on click outside
  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (
        panelRef.current &&
        !panelRef.current.contains(e.target as Node) &&
        btnRef.current &&
        !btnRef.current.contains(e.target as Node)
      ) {
        setOpen(false)
      }
    }
    document.addEventListener("mousedown", handler)
    return () => document.removeEventListener("mousedown", handler)
  }, [open])

  const ModeIcon = { light: Sun, dark: Moon, system: Monitor }[theme]

  return (
    <div className="relative">
      <Button
        ref={btnRef}
        variant="ghost"
        size="icon"
        onClick={() => setOpen(!open)}
        title="Theme settings"
      >
        <ModeIcon className="size-4" />
      </Button>

      {open && (
        <div
          ref={panelRef}
          className="absolute right-0 top-full mt-1 z-50 w-64 rounded-lg border bg-popover p-3 shadow-lg"
        >
          {/* ── Mode: horizontal icon row ── */}
          <div className="flex items-center justify-center gap-1 bg-muted/50 rounded-lg p-1">
            {(["light", "dark", "system"] as ThemeMode[]).map((mode) => {
              const Icon = { light: Sun, dark: Moon, system: Monitor }[mode]
              return (
                <button
                  key={mode}
                  onClick={() => setTheme(mode)}
                  title={mode}
                  className={cn(
                    "flex items-center justify-center rounded-md p-2 text-sm transition-colors cursor-pointer flex-1",
                    theme === mode
                      ? "bg-background text-foreground shadow-xs"
                      : "text-muted-foreground hover:text-foreground",
                  )}
                >
                  <Icon className="size-4" />
                </button>
              )
            })}
          </div>

          {/* ── Divider ── */}
          <div className="my-2 border-t" />

          {/* ── Manifest Themes ── */}
          <div className="space-y-0.5 max-h-48 overflow-y-auto">
            {/* None = default index.css */}
            <button
              onClick={() => setActiveTheme(null)}
              className={cn(
                "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-xs transition-colors cursor-pointer",
                activeTheme === null
                  ? "bg-accent text-accent-foreground"
                  : "hover:bg-accent/50 text-foreground",
              )}
            >
              <span className="size-3 rounded-full border shrink-0 flex items-center justify-center text-[6px] font-bold text-muted-foreground">
                —
              </span>
              <span className="font-medium">Default</span>
              {activeTheme === null && <Check className="size-2.5 ml-auto shrink-0" />}
            </button>

            {manifestThemes.map((t) => (
              <button
                key={t.name}
                onClick={() => setActiveTheme(t.name)}
                className={cn(
                  "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-xs transition-colors cursor-pointer",
                  activeTheme === t.name
                    ? "bg-accent text-accent-foreground"
                    : "hover:bg-accent/50 text-foreground",
                )}
              >
                <span
                  className="size-3 rounded-full border shrink-0"
                  style={{ backgroundColor: t.spec.tokens?.["color.primary"] ?? "var(--primary)" }}
                />
                <span className="truncate">{t.name}</span>
                {activeTheme === t.name && <Check className="size-2.5 ml-auto shrink-0" />}
              </button>
            ))}
          </div>

          {/* ── Color Presets (only when no manifest theme active) ── */}
          {!activeTheme && (
            <>
              <div className="my-2 border-t" />
              <div>
                <p className="px-1 py-1 text-[10px] font-medium text-muted-foreground uppercase tracking-wider">Warna</p>
                <div className="flex flex-wrap gap-1.5 px-1">
                  {Object.values(COLOR_PRESETS).map((preset) => (
                    <button
                      key={preset.name}
                      onClick={() => setColorPreset(preset.name)}
                      title={preset.label}
                      className={cn(
                        "size-6 rounded-full border-2 transition-all cursor-pointer shrink-0",
                        colorPreset === preset.name
                          ? "border-foreground scale-110"
                          : "border-transparent hover:scale-110",
                      )}
                      style={{ backgroundColor: preset.primary }}
                    />
                  ))}
                </div>
              </div>
            </>
          )}
        </div>
      )}
    </div>
  )
}
