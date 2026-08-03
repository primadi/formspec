// ─── Forma App ───
//
// Root component with boot sequence:
// 1. Parse URL → workspace + surface
// 2. Fetch _meta/me + _meta/ui → fill stores
// 3. Build route table from meta bundle
// 4. Render AppShell with router

import { useEffect, useMemo } from "react"
import {
  BrowserRouter,
  Routes,
  Route,
  Navigate,
  useParams,
  useNavigate,
} from "react-router-dom"
import { Skeleton } from "@/components/ui/skeleton"
import { Toaster } from "sonner"

import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { usePrefsStore } from "@/stores/prefs"
import { AppShell, LoginScreen, buildRoutes } from "@/shell"
import ThemeRenderer from "@/kinds/theme/ThemeRenderer"
import { useTheme } from "@/hooks/useTheme"
import { preloadCommonRenderers } from "@/lib/preload"

// ── Root: Parse URL and route to surface ──

function Root() {
  useTheme()
  preloadCommonRenderers()
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Navigate to="/default/_admin" replace />} />
        <Route path="/:workspace" element={<Navigate to="_admin" replace />} />
        <Route path="/:workspace/_admin/*" element={<SurfaceShell surface="admin" />} />
        <Route path="/:workspace/app/*" element={<SurfaceShell surface="app" />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="*" element={<NotFound />} />
      </Routes>
      <Toaster position="top-right" richColors />
    </BrowserRouter>
  )
}

export default Root

// ── Surface Shell: boot + render for admin or app surface ──

function SurfaceShell({ surface }: { surface: "admin" | "app" }) {
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
  // bare "/app" before the bundle (and thus root_url) is known. This is the
  // full absolute prefix used to build/link every route (basePath below,
  // Sidebar hrefs, DefaultRedirect targets).
  const surfacePath =
    surface === "admin" ? `/${workspace}/_admin` : `/${workspace}${bundle?.app.root_url ?? "/app"}`
  // mountPrefix is the FIXED prefix actually consumed by the outer splat
  // route ("/:workspace/app/*" or "/:workspace/_admin/*"). The nested
  // <Routes> below only ever sees the remainder after that fixed prefix —
  // root_url is NOT part of it, even though it IS part of surfacePath — so
  // relative route paths must strip mountPrefix, never surfacePath.
  const mountPrefix = `/${workspace}/${surface === "admin" ? "_admin" : "app"}`
  const surfaceRoutes = useMemo(
    () => (bundle ? buildRoutes({ bundle, basePath: surfacePath }) : []),
    [bundle, surfacePath],
  )

  // Boot: fetch session + meta, then start spec version polling
  useEffect(() => {
    if (!sessionLoaded) {
      boot(workspace).then(() => {
        const { token } = useSessionStore.getState()
        loadMeta(workspace, surface, token)
      })
    }
  }, [workspace, sessionLoaded, boot, loadMeta, surface])

  // Dev-mode only: listen for Vite HMR 'forma:spec-reloaded' events.
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
    hot.on("forma:spec-reloaded", handler)
    return () => {
      hot.off("forma:spec-reloaded", handler)
    }
  }, [workspace, surface])

  // While loading
  if (!sessionLoaded || metaLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="space-y-4 w-full max-w-sm px-4">
          <div className="text-center">
            <h1 className="text-2xl font-bold">Forma</h1>
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
          Make sure the Forma server is running.
        </p>
      </div>
    )
  }

  // No bundle = can't render
  if (!bundle) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-sm text-muted-foreground">
          Loading manifests...
        </p>
      </div>
    )
  }

  // Apply selected theme from user preference.
  // When activeTheme is null, no manifest theme is applied (use index.css defaults).
  const themeEntry = activeTheme
    ? bundle.themes.find((t) => t.name === activeTheme) ?? null
    : null

  return (
    <>
      <ThemeRenderer entry={themeEntry} />
      <Routes>
        <Route element={<AppShell />}>
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
            <DefaultRedirect bundle={bundle} workspace={workspace} surface={surface} />
          }
        />
        {/* Catch-all: redirect to root */}
        <Route
          path="*"
          element={
            <DefaultRedirect bundle={bundle} workspace={workspace} surface={surface} />
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
function firstMenuRoute(items: import("@/types/manifest").MenuItem[] | null | undefined): string | undefined {
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
  const prefix = surface === "admin" ? "_admin" : bundle.app.root_url.replace(/^\//, "")

  // App surface: land on the App's own first authored menu item (e.g. its
  // Dashboard) rather than an arbitrary derived entity list.
  if (surface === "app") {
    const menuRoute = firstMenuRoute(bundle.menu)
    if (menuRoute) {
      return <Navigate to={`/${workspace}/${prefix}${menuRoute}`} replace />
    }
  }

  // Fallback (admin surface, or an app with no menu at all): first
  // non-summary/read-only entity's derived list.
  const entity = bundle.entities.find((e) => e.characteristic !== "summary") ?? bundle.entities[0]
  if (entity) {
    return (
      <Navigate
        to={`/${workspace}/${prefix}/${entity.module}/${entity.plural}`}
        replace
      />
    )
  }
  return (
    <div className="flex min-h-[60vh] items-center justify-center">
      <div className="text-center">
        <h2 className="text-xl font-semibold">Welcome to Forma</h2>
        <p className="mt-2 text-sm text-muted-foreground">
          No entities found. Load a manifest to get started.
        </p>
      </div>
    </div>
  )
}

// ── Login Page ──

function LoginPage() {
  const navigate = useNavigate()
  const boot = useSessionStore((s) => s.boot)

  const handleLogin = async (workspace: string, token: string) => {
    await boot(workspace, token)
    navigate(`/${workspace}/_admin`)
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
