// ─── ActionIcon ───
//
// Generic icon component for runtime-resolved icons (from YAML manifest).
// Uses shared resolveIcon from lib/icon-resolver.
//
// Props:
//   iconName  – lucide-react icon name (kebab-case or PascalCase)
//   className – optional Tailwind classes (default: "size-4")

import { resolveIcon } from "@/lib/icon-resolver"
import { cn } from "@/lib/utils"

export function ActionIcon({
  iconName,
  className = "size-4",
}: {
  iconName: string
  className?: string
}) {
  const Icon = resolveIcon(iconName)
  if (!Icon) {
    return <span className={cn("text-xs", className)}>{iconName.charAt(0).toUpperCase()}</span>
  }
  return <Icon className={cn(className)} />
}
