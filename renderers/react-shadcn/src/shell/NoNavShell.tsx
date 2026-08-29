// ─── No-Nav Shell ───
//
// Minimal chrome for a `no-nav` App (frontend/05-app-kinds.md §3): brand bar
// + optional public nav + footer, then the page content via Outlet. No
// sidebar, no breadcrumb. Auth is a separate axis (App access) — this shell
// renders both public and private no-nav Apps; the surface boot handles auth.
//
// The nav is derived from the App's resolved menu, filtered to leaves with a
// route. Falls back to no nav when the App declares no menu.

import { Link, Outlet, useParams } from "react-router-dom"
import { useMetaStore } from "@/stores/meta"
import { LogoutButton } from "./LogoutButton"
import type { MenuItem } from "@/types/manifest"

// ── Nav derivation ──
// Walk the resolved menu tree and collect leaf routes. We keep it shallow
// (top-level groups + their leaves) to avoid a heavy nested nav in the bare
// chrome.
function collectPublicLinks(
  items: MenuItem[] | null | undefined,
  depth = 0,
): { label: string; route: string }[] {
  if (!items?.length) return []
  const out: { label: string; route: string }[] = []
  for (const item of items) {
    if (item.route) {
      out.push({ label: item.label || item.route, route: item.route })
    } else if (item.children?.length && depth < 1) {
      out.push(...collectPublicLinks(item.children, depth + 1))
    }
  }
  return out
}

export function NoNavShell() {
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const bundle = useMetaStore((s) => s.bundle)
  const rootUrl = bundle?.app.root_url ?? "/"
  // Base app path (root_url may be "/" for a public App — strip the trailing
  // slash so concatenating a leading-slash route never yields a double slash).
  const base = `/${workspace}${rootUrl}`.replace(/\/+$/, "")
  const links = collectPublicLinks(bundle?.menu)

  return (
    <div className="flex min-h-screen flex-col">
      {/* Brand bar */}
      <header className="sticky top-0 z-40 border-b bg-background/95 backdrop-blur supports-backdrop-filter:bg-background/60">
        <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4">
          <Link to={base} className="flex items-center gap-2 font-semibold">
            <span className="text-lg tracking-tight">
              {bundle?.app.name ?? "FormSpec"}
            </span>
          </Link>
          <div className="flex items-center gap-2">
            {links.length > 0 && (
              <nav className="hidden items-center gap-6 text-sm md:flex">
                {links.map((link) => (
                  <Link
                    key={link.route}
                    to={`${base}${link.route}`}
                    className="text-muted-foreground transition-colors hover:text-foreground"
                  >
                    {link.label}
                  </Link>
                ))}
              </nav>
            )}
            <LogoutButton />
          </div>
        </div>
      </header>

      {/* Page content — same container as header/footer so the page aligns
          with the brand bar instead of hugging the viewport edge. */}
      <main className="flex-1">
        <div className="mx-auto max-w-6xl px-4 py-6">
          <Outlet />
        </div>
      </main>

      {/* Footer */}
      <footer className="border-t py-6">
        <div className="mx-auto max-w-6xl px-4 text-center text-sm text-muted-foreground">
          © {new Date().getFullYear()} {bundle?.app.name ?? "FormSpec"}
        </div>
      </footer>
    </div>
  )
}
