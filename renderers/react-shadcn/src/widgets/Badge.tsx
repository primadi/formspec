// ─── Badge Widget ───
//
// For enum/status display in tables and detail views.
// Maps status values to semantic colors.

import { cn } from "@/lib/utils"

const STATUS_COLORS: Record<string, string> = {
  draft: "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400",
  submitted: "bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400",
  confirmed: "bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400",
  paid: "bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400",
  cancelled: "bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400",
  active: "bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400",
  inactive: "bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-400",
  pending: "bg-orange-100 text-orange-800 dark:bg-orange-900/30 dark:text-orange-400",
  completed: "bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400",
  failed: "bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400",
}

interface BadgeProps {
  value?: string
  className?: string
}

export function Badge({ value, className }: BadgeProps) {
  if (!value) return null

  const colorClass = STATUS_COLORS[value.toLowerCase()] ?? "bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-400"

  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium",
        colorClass,
        className,
      )}
    >
      {value.charAt(0).toUpperCase() + value.slice(1).replace(/_/g, " ")}
    </span>
  )
}
