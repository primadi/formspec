import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * Title-case a kebab/snake_case identifier for display — e.g. "otc-sale"
 * → "Otc Sale", "consultation-board" → "Consultation Board". Capitalizes
 * every word, not just the first, unlike the naive
 * `s.charAt(0).toUpperCase() + s.slice(1)` pattern this replaces.
 */
export function titleCase(s: string): string {
  return s
    .replace(/[-_]+/g, " ")
    .trim()
    .replace(/\b\w/g, (c) => c.toUpperCase())
}
