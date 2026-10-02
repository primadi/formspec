// ─── Auth Area ───
//
// Shared auth controls for all shells (frontend/05-app-kinds.md §4.1).
// Driven by the resolved `chrome.auth` value from the meta bundle — the
// backend already applied archetype defaults, so this component renders
// final values only:
//
// - "links":  anon → Sign in link + Sign up button · signed-in → user menu
// - "button": anon → single Sign in button · signed-in → user menu
// - "none":   anon → nothing · signed-in → STILL the user menu (see below)
//
// `chrome.auth` tunes the ANONYMOUS entry points only. It never removes the
// exit: a signed-in session always gets its user menu (→ Sign out), whatever
// the mode. `no-nav` resolves `auth: none`, and the old "none → render nothing"
// rule left a signed-in visitor on a public App with no Sign out at all, and
// no way back in once signed out (kafe 10.18).
//
// `undefined` (bundle not loaded yet) renders nothing — never guess.

import { AppLink as Link } from "@/lib/navigation"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { useSurface } from "@/hooks/useSurface"
import { AssetRenderer } from "./AssetRenderer"
import { UserMenu } from "./UserMenu"

export function AuthArea({ mode }: { mode: string | undefined }) {
  const token = useSessionStore((s) => s.token)
  const { surfacePath } = useSurface()
  // Custom chrome auth area (App.spec.auth.chrome_auth) — a component asset
  // replaces the default Sign in/Sign up/logout/user menu entirely.
  const chromeAuth = useMetaStore((s) => s.bundle?.app.auth?.chrome_auth)

  if (chromeAuth) {
    return <AssetRenderer asset={chromeAuth} />
  }

  // Signed-in → avatar user menu (Profile identity + Sign out).
  //
  // This runs BEFORE the mode check on purpose: a session must always have an
  // exit. `chrome.auth` (incl. its `none` default on `no-nav`) governs the
  // anonymous Sign in / Sign up affordances, never the signed-in exit.
  if (token) return <UserMenu />

  if (mode !== "links" && mode !== "button") return null

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
