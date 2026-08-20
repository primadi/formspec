// ─── FormSpec App ───
//
// Root component with boot sequence:
// 1. Parse URL → workspace + surface
// 2. Fetch _meta/me + _meta/ui → fill stores
// 3. Build route table from meta bundle
// 4. Render AppShell with router

import { useEffect, useMemo, useState } from "react"
import {
  BrowserRouter,
  Routes,
  Route,
  Navigate,
  useParams,
  useNavigate,
  useSearchParams,
} from "react-router-dom"
import { Skeleton } from "@/components/ui/skeleton"
import { Toaster } from "sonner"

import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { usePrefsStore } from "@/stores/prefs"
import {
  AppShell,
  NoNavShell,
  TopNavShell,
  LoginScreen,
  buildRoutes,
} from "@/shell"
import ThemeRenderer from "@/kinds/theme/ThemeRenderer"
import { useTheme } from "@/hooks/useTheme"
import { preloadCommonRenderers } from "@/lib/preload"
import { fetchMetaApps } from "@/lib/api"
import type { AppSummary } from "@/types/manifest"

// ── Shell registry ──
// Maps the App renderer archetype (frontend/05-app-kinds.md) to its concrete
// implementation in this shell. Today react-shadcn fills all three; other
// stack_families register their own implementations later.
const APP_SHELLS: Record<string, React.ComponentType> = {
  "sidebar-nav": AppShell,
  topnav: TopNavShell,
  "no-nav": NoNavShell,
}

// ── Root: Parse URL and route to surface ──

function Root() {
  useTheme()
  preloadCommonRenderers()
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Navigate to="/default" replace />} />
        <Route
          path="/:workspace/_admin/*"
          element={<SurfaceShell surface="admin" />}
        />
        <Route
          path="/:workspace/app/*"
          element={<SurfaceShell surface="app" />}
        />
        {/* Root surface: an `access: public` App owns the workspace root. If
            the workspace resolves to no public App, redirect to _admin. */}
        <Route path="/:workspace/*" element={<RootSurface />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="*" element={<NotFound />} />
      </Routes>
      <Toaster position="top-right" richColors />
    </BrowserRouter>
  )
}

export default Root

// ── Root Surface: detect public App at the workspace root ──

function RootSurface() {
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const [apps, setApps] = useState<AppSummary[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    fetchMetaApps(workspace)
      .then((list) => {
        if (cancelled) return
        setApps(list)
      })
      .catch((err) => {
        if (cancelled) return
        setError(err instanceof Error ? err.message : "Failed to load apps")
      })
    return () => {
      cancelled = true
    }
  }, [workspace])

  if (error) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-sm text-muted-foreground">{error}</p>
      </div>
    )
  }
  if (!apps) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-sm text-muted-foreground">Loading...</p>
      </div>
    )
  }

  // Find the winning App by longest root_url prefix match (same logic as
  // detectAppName in stores/meta.ts). For the workspace root, a public App
  // with root_url "/" wins.
  const segments = window.location.pathname.split("/").filter(Boolean)
  const rest = "/" + segments.slice(1).join("/")
  let best: AppSummary | undefined
  for (const a of apps) {
    if (rest === a.root_url || rest.startsWith(a.root_url + "/")) {
      if (!best || a.root_url.length > best.root_url.length) best = a
    }
  }
  const publicApp = best && best.access === "public" ? best : undefined

  if (!publicApp) {
    // No public App at this path — fall back to the admin surface.
    return <Navigate to={`/${workspace}/_admin`} replace />
  }

  return <SurfaceShell surface="app" public />
}

// ── Surface Shell: boot + render for admin or app surface ──
//
// `public` marks an App whose surface boots anonymously (`access: public`).
// Auth is orthogonal to the shell (app_renderer) — a no-nav App can be
// private, and a sidebar-nav App could be public.

function SurfaceShell({
  surface,
  public: isPublic = false,
}: {
  surface: "admin" | "app"
  public?: boolean
}) {
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const sessionLoaded = useSessionStore((s) => s.loaded)
  const sessionError = useSessionStore((s) => s.error)
  const boot = useSessionStore((s) => s.boot)
  const bundle = useMetaStore((s) => s.bundle)
  const metaLoading = useMetaStore((s) => s.loading)
  const metaError = useMetaStore((s) => s.error)
  const metaForbidden = useMetaStore((s) => s.forbidden)
  const loadMeta = useMetaStore((s) => s.load)

  // Build routes from meta bundle.
  // Memoized: buildRoutes() constructs a fresh Component closure per route
  // on every call, and React Router treats a changed Component reference as
  // a different element type — remounting it and wiping any local state
  // (e.g. a Wizard's step data) on every re-render, including re-renders
  // triggered by navigation events unrelated to the bundle itself (like a
  // wizard's own setSearchParams() step change). Keying on bundle/surfacePath
  // keeps the same Component references across those re-renders.
  // Must run before any early return below — Hooks can't be conditional.
  const activeTheme = usePrefsStore((s) => s.activeTheme)
  // The app surface uses this resolved App's own root_url (Core §4.4) —
  // a workspace can resolve to more than one App, each mounted at its own
  // root_url under the shared /{ws}/app/* renderer SPA. Falls back to a
  // bare "/app" before the bundle (and thus root_url) is known. A public App
  // that owns the workspace root (root_url "/") collapses to "/{ws}".
  const surfacePath =
    surface === "admin"
      ? `/${workspace}/_admin`
      : `/${workspace}${bundle?.app.root_url ?? "/app"}`.replace(/\/+$/, "")
  // mountPrefix is the FIXED prefix actually consumed by the outer splat
  // route ("/:workspace/app/*" or "/:workspace/_admin/*"). The nested
  // <Routes> below only ever sees the remainder after that fixed prefix —
  // root_url is NOT part of it, even though it IS part of surfacePath — so
  // relative route paths must strip mountPrefix, never surfacePath.
  const mountPrefix =
    surface === "admin" ? `/${workspace}/_admin` : `/${workspace}/app`
  const surfaceRoutes = useMemo(
    () => (bundle ? buildRoutes({ bundle, basePath: surfacePath }) : []),
    [bundle, surfacePath],
  )

  // Boot: fetch session + meta, then start spec version polling.
  // A `public` surface is anonymous — no session boot, meta fetched without
  // a token (the server serves the public bundle for an `access: public`
  // App).
  useEffect(() => {
    if (isPublic) {
      if (!sessionLoaded) {
        // Mark session loaded so the loading gate below doesn't spin forever.
        useSessionStore.getState().setSession(workspace, "")
      }
      if (!bundle && !metaLoading) {
        loadMeta(workspace, "app")
      }
      return
    }
    if (!sessionLoaded) {
      boot(workspace).then(() => {
        const { token } = useSessionStore.getState()
        loadMeta(workspace, surface, token)
      })
    }
  }, [
    workspace,
    sessionLoaded,
    boot,
    loadMeta,
    surface,
    bundle,
    metaLoading,
    isPublic,
  ])

  // Dev-mode only: listen for Vite HMR 'formspec:spec-reloaded' events.
  // When the backend reloads YAML specs, the Vite dev server broadcasts
  // this event and we re-fetch the meta bundle — no polling needed.
  useEffect(() => {
    const hot = import.meta.hot
    if (!hot) return

    const handler = () => {
      const state = useMetaStore.getState()
      if (state.bundle) {
        const { token } = useSessionStore.getState()
        state.refresh(workspace, surface, token)
      }
    }
    hot.on("formspec:spec-reloaded", handler)
    return () => {
      hot.off("formspec:spec-reloaded", handler)
    }
  }, [workspace, surface])

  // While loading
  if (!sessionLoaded || metaLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="space-y-4 w-full max-w-sm px-4">
          <div className="text-center">
            <h1 className="text-2xl font-bold">FormSpec</h1>
          </div>
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-3/4" />
          <Skeleton className="h-8 w-full" />
        </div>
      </div>
    )
  }

  // Forbidden: authenticated, but lacks _admin.access (or equivalent gate)
  if (metaForbidden) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-4">
        <h1 className="text-2xl font-bold">Access Denied</h1>
        <p className="text-sm text-muted-foreground">
          You don&apos;t have permission to access this area.
        </p>
      </div>
    )
  }

  // Error state
  if (sessionError || metaError) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-4">
        <h1 className="text-2xl font-bold">Connection Error</h1>
        <p className="text-sm text-muted-foreground">
          {sessionError ?? metaError}
        </p>
        <p className="text-xs text-muted-foreground">
          Make sure the FormSpec server is running.
        </p>
      </div>
    )
  }

  // No bundle = can't render
  if (!bundle) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-sm text-muted-foreground">Loading manifests...</p>
      </div>
    )
  }

  // Apply selected theme from user preference.
  // When activeTheme is null, no manifest theme is applied (use index.css defaults).
  const themeEntry = activeTheme
    ? (bundle.themes.find((t) => t.name === activeTheme) ?? null)
    : null

  // Shell selection (frontend/05-app-kinds.md): the App renderer archetype
  // picks the chrome for the whole surface. Falls back to the sidebar shell
  // for any unknown/absent archetype.
  const archetype = bundle.app.app_renderer ?? "sidebar-nav"
  const Shell = APP_SHELLS[archetype] ?? AppShell

  return (
    <>
      <ThemeRenderer entry={themeEntry} />
      <Routes>
        <Route element={<Shell />}>
          {surfaceRoutes.map((route, idx) => (
            <Route
              key={`${surface}-${idx}`}
              path={route.path?.replace(`${mountPrefix}/`, "") ?? ""}
              Component={route.Component}
            />
          ))}
          {/* Default: redirect to first derived entity */}
          <Route
            index
            element={
              <DefaultRedirect
                bundle={bundle}
                workspace={workspace}
                surface={surface}
              />
            }
          />
          {/* Catch-all: redirect to root */}
          <Route
            path="*"
            element={
              <DefaultRedirect
                bundle={bundle}
                workspace={workspace}
                surface={surface}
              />
            }
          />
        </Route>
      </Routes>
    </>
  )
}

// ── Default Redirect ──

// Depth-first search for the first navigable leaf in a resolved menu tree
// (bundle.menu — Core §4.4, routes already resolved server-side).
function firstMenuRoute(
  items: import("@/types/manifest").MenuItem[] | null | undefined,
): string | undefined {
  if (!items?.length) return undefined
  for (const item of items) {
    if (item.route) return item.route
    if (item.children?.length) {
      const found = firstMenuRoute(item.children)
      if (found) return found
    }
  }
  return undefined
}

function DefaultRedirect({
  bundle,
  workspace,
  surface = "admin",
}: {
  bundle: import("@/types/manifest").MetaBundle
  workspace: string
  surface?: string
}) {
  const prefix =
    surface === "admin" ? "_admin" : bundle.app.root_url.replace(/^\//, "")
  // Normalize: a public App owns the workspace root (root_url "/"), so
  // prefix is "" and base collapses to "/{ws}" — never "/{ws}//...".
  const base = `/${workspace}/${prefix}`.replace(/\/+$/, "")

  // App surface: land on the App's own first authored menu item (e.g. its
  // Dashboard or home hero) rather than an arbitrary derived entity list.
  if (surface === "app") {
    const menuRoute = firstMenuRoute(bundle.menu)
    if (menuRoute) {
      return <Navigate to={`${base}${menuRoute}`} replace />
    }
  }

  // Fallback (admin surface, or an app with no menu at all): first
  // non-summary/read-only entity's derived list.
  const entity =
    bundle.entities.find((e) => e.characteristic !== "summary") ??
    bundle.entities[0]
  if (entity) {
    return <Navigate to={`${base}/${entity.module}/${entity.plural}`} replace />
  }
  return (
    <div className="flex min-h-[60vh] items-center justify-center">
      <div className="text-center">
        <h2 className="text-xl font-semibold">Welcome to FormSpec</h2>
        <p className="mt-2 text-sm text-muted-foreground">
          No entities found. Load a manifest to get started.
        </p>
      </div>
    </div>
  )
}

// ── Login Page ──
//
// Workspace-scoped login with a `returnTo` redirect: after a successful login
// the user returns to the page they originally tried to reach. `returnTo` is
// validated to be same-origin (prevents open redirect); missing/invalid
// values fall back to the workspace admin surface.

function LoginPage() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const boot = useSessionStore((s) => s.boot)

  const handleLogin = async (workspace: string, token: string) => {
    await boot(workspace, token)
    const returnTo = searchParams.get("returnTo")
    // Same-origin guard: only accept a path starting with "/" that is not
    // "//" (protocol-relative) and not a bare "/" (which would loop).
    if (
      returnTo &&
      returnTo.startsWith("/") &&
      !returnTo.startsWith("//") &&
      returnTo !== "/"
    ) {
      navigate(returnTo, { replace: true })
      return
    }
    navigate(`/${workspace}/_admin`, { replace: true })
  }

  return <LoginScreen onLogin={handleLogin} />
}

// ── 404 ──

function NotFound() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-2">
      <h1 className="text-4xl font-bold">404</h1>
      <p className="text-muted-foreground">Page not found</p>
    </div>
  )
}
