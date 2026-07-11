// ─── Theme Switcher ───
//
// Dropdown for switching theme mode (light/dark/system) and color preset.
// Renders as an icon button in the header with a popover panel.

import { useState, useRef, useEffect } from "react"
import { Moon, Sun, Monitor, Check } from "lucide-react"
import { cn } from "@/lib/utils"
import { usePrefsStore, COLOR_PRESETS, type ThemeMode } from "@/stores/prefs"
import { Button } from "@/components/ui/button"

export function ThemeSwitcher() {
  const theme = usePrefsStore((s) => s.theme)
  const colorPreset = usePrefsStore((s) => s.colorPreset)
  const setTheme = usePrefsStore((s) => s.setTheme)
  const setColorPreset = usePrefsStore((s) => s.setColorPreset)

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

  const themeIcon = {
    light: Sun,
    dark: Moon,
    system: Monitor,
  }[theme]

  const ThemeIcon = themeIcon

  return (
    <div className="relative">
      <Button
        ref={btnRef}
        variant="ghost"
        size="icon"
        onClick={() => setOpen(!open)}
        title="Theme settings"
      >
        <ThemeIcon className="size-4" />
      </Button>

      {open && (
        <div
          ref={panelRef}
          className="absolute right-0 top-full mt-1 z-50 w-56 rounded-lg border bg-popover p-2 shadow-lg"
        >
          {/* Theme Mode */}
          <div className="space-y-1">
            <p className="px-2 py-1 text-xs font-medium text-muted-foreground">Mode</p>
            {(["light", "dark", "system"] as ThemeMode[]).map((mode) => {
              const Icon = { light: Sun, dark: Moon, system: Monitor }[mode]
              return (
                <button
                  key={mode}
                  onClick={() => setTheme(mode)}
                  className={cn(
                    "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm transition-colors cursor-pointer",
                    theme === mode
                      ? "bg-accent text-accent-foreground"
                      : "hover:bg-accent/50 text-foreground",
                  )}
                >
                  <Icon className="size-4" />
                  <span className="capitalize">{mode}</span>
                  {theme === mode && <Check className="size-3 ml-auto" />}
                </button>
              )
            })}
          </div>

          {/* Divider */}
          <div className="my-2 border-t" />

          {/* Color Presets */}
          <div className="space-y-1">
            <p className="px-2 py-1 text-xs font-medium text-muted-foreground">Warna</p>
            {Object.values(COLOR_PRESETS).map((preset) => (
              <button
                key={preset.name}
                onClick={() => setColorPreset(preset.name)}
                className={cn(
                  "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm transition-colors cursor-pointer",
                  colorPreset === preset.name
                    ? "bg-accent text-accent-foreground"
                    : "hover:bg-accent/50 text-foreground",
                )}
              >
                <span
                  className="size-3.5 rounded-full border"
                  style={{ backgroundColor: preset.primary }}
                />
                <span>{preset.label}</span>
                {colorPreset === preset.name && <Check className="size-3 ml-auto" />}
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
