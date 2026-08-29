// ─── Auth Area ───
//
// Shared auth controls for all shells (frontend/05-app-kinds.md §4.1).
// Driven by the resolved `chrome.auth` value from the meta bundle — the
// backend already applied archetype defaults, so this component renders
// final values only:
//
// - "links":  anon → Sign in link + Sign up button · signed-in → logout
// - "button": anon → single Sign in button · signed-in → logout
// - "none":   renders nothing (private Apps still guard via surface boot)
//
// `undefined` (bundle not loaded yet) renders nothing — never guess.

import { Link } from "react-router-dom"
import { useSessionStore } from "@/stores/session"
import { useSurface } from "@/hooks/useSurface"
import { LogoutButton } from "./LogoutButton"

export function AuthArea({ mode }: { mode: string | undefined }) {
  const token = useSessionStore((s) => s.token)
  const { surfacePath } = useSurface()

  if (mode !== "links" && mode !== "button") return null

  // Signed-in → logout control (all modes that render auth UI).
  if (token) return <LogoutButton />

  if (mode === "button") {
    return (
      <Link
        to={surfacePath("login")}
        className="inline-flex h-8 items-center rounded-lg border border-border bg-background px-3 text-sm font-medium transition-colors hover:border-foreground/20 hover:bg-[color-mix(in_oklch,var(--background),var(--foreground)_8%)]"
      >
        Sign in
      </Link>
    )
  }

  // "links": Sign in link + Sign up primary button.
  return (
    <div className="flex items-center gap-2 text-sm">
      <Link
        to={surfacePath("login")}
        className="text-muted-foreground transition-colors hover:text-foreground"
      >
        Sign in
      </Link>
      <Link
        to={surfacePath("register")}
        className="rounded-md bg-primary px-3 py-1.5 font-medium text-primary-foreground transition-colors hover:bg-primary/90"
      >
        Sign up
      </Link>
    </div>
  )
}
