// ─── Icon Resolver ───
//
// Shared utility to resolve lucide-react icon names to components at runtime.
// Used by ActionIcon component and Sidebar (and any future consumer).
//
// Why namespace import (`import *`):
//   - FormSpec is a manifest-driven framework — icon names come from YAML
//     at runtime and can't be known at compile-time
//   - Sidebar already bundles all lucide-react via this pattern,
//     so no additional bundle cost

import * as LucideIcons from "lucide-react"
import type { LucideIcon } from "lucide-react"

export function resolveIcon(name: string): LucideIcon | null {
  const key = name
    .split(/[-_]/)
    .map((s) => s.charAt(0).toUpperCase() + s.slice(1))
    .join("")
  return (LucideIcons as unknown as Record<string, LucideIcon>)[key] ?? null
}
