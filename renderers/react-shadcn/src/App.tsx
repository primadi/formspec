// ─── FormSpec App ───
//
// Root component with boot sequence:
// 1. Parse URL → workspace + surface
// 2. Fetch _meta/me + _meta/ui → fill stores
// 3. Build route table from meta bundle
// 4. Render AppShell with router

import { lazy, Suspense, useEffect, useMemo, useState } from "react"
import {
  BrowserRouter,
  Routes,
  Route,
  Navigate,
  useParams,
  useNavigate,
  useSearchParams,
  useLocation,
} from "react-router-dom"
import { Skeleton } from "@/components/ui/skeleton"
import { Toaster } from "sonner"
import { UiHost } from "@/shell/UiHost"
import { DownloadTray } from "@/shell/DownloadTray"

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
import { useAutoLogout } from "@/hooks/useAutoLogout"
import { preloadCommonRenderers } from "@/lib/preload"
import { fetchMetaApps } from "@/lib/api"

const PageRenderer = lazy(() => import("@/kinds/page/PageRenderer"))
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
      <UiHost />
      <DownloadTray />
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
    // root_url "/" owns the whole workspace root — any subpath matches.
    // (startsWith(a.root_url + "/") would test startsWith("//") and never
    // match, silently falling back to _admin for every nested route.)
    if (a.root_url === "/" || rest === a.root_url || rest.startsWith(a.root_url + "/")) {
      if (!best || a.root_url.length > best.root_url.length) best = a
    }
  }
  const publicApp = best && best.access === "public" ? best : undefined

  if (!publicApp) {
    // No public App at this path — fall back to the admin surface.
    return <Navigate to={`/${workspace}/_admin`} replace />
  }

  return <SurfaceShell surface="app" public mountPrefix={`/${workspace}`} />
}

// ── Surface Shell: boot + render for admin or app surface ──
//
// `public` marks an App whose surface boots anonymously (`access: public`).
// Auth is orthogonal to the shell (app_renderer) — a no-nav App can be
// private, and a sidebar-nav App could be public.

function SurfaceShell({
  surface,
  public: isPublic = false,
  mountPrefix: mountPrefixOverride,
}: {
  surface: "admin" | "app"
  public?: boolean
  /**
   * Fixed pathname prefix consumed by the OUTER route. Default: admin →
   * "/{ws}/_admin", app → "/{ws}/app". A public App that owns the workspace
   * root (RootSurface, mounted at "/:workspace/*") must pass "/{ws}" — its
   * nested routes are relative to the workspace, not to "/{ws}/app".
   */
  mountPrefix?: string
}) {
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const location = useLocation()
  const sessionLoaded = useSessionStore((s) => s.loaded)
  const sessionError = useSessionStore((s) => s.error)
  const unauthenticated = useSessionStore((s) => s.unauthenticated)
  const token = useSessionStore((s) => s.token)
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
    mountPrefixOverride ??
    (surface === "admin" ? `/${workspace}/_admin` : `/${workspace}/app`)
  // The App's home page (spec.route "/") — rendered as the surface index.
  const homePage = bundle?.pages?.find((p) => p.spec.route === "/")
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
    if (!sessionLoaded && !token) {
      // Boot only when there is no session AND no token in flight. The `!token`
      // guard prevents a re-boot (without a token) when LoginPage's
      // boot(token) briefly resets loaded=false — that would overwrite the
      // authenticated session with an anonymous one.
      boot(workspace).then(() => {
        // Always load the meta bundle — even when unauthenticated — so the
        // resolved App's root_url is known and the auth guard can redirect to
        // the correct in-app login path ({surfacePath}/login).
        const { token } = useSessionStore.getState()
        loadMeta(workspace, surface, token)
      })
    } else if (!bundle && !metaLoading && !metaError && !metaForbidden) {
      // Session already loaded (e.g. navigated here after a login redirect) —
      // boot() ran in LoginPage, so just load the bundle if it's missing.
      // Skip when the bundle was rejected (error/forbidden) to avoid a reload
      // loop (403 → forbidden → effect re-run → reload → 403).
      const { token } = useSessionStore.getState()
      loadMeta(workspace, surface, token)
    }
  }, [
    workspace,
    sessionLoaded,
    token,
    boot,
    loadMeta,
    surface,
    bundle,
    metaLoading,
    metaError,
    metaForbidden,
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

  // Auto-logout: expire the session after a configurable idle timeout. Only
  // armed for a real authenticated session (non-public, not already
  // unauthenticated, token present). On expiry the session store marks the
  // session unauthenticated and the auth guard below redirects to login.
  const autoLogoutEnabled =
    !isPublic && !unauthenticated && sessionLoaded && !!token
  useAutoLogout(autoLogoutEnabled)

  // While loading
  // While loading (session or meta bundle). Non-public surfaces also wait for
  // the bundle — the auth guard needs the resolved App's root_url to build the
  // correct in-app login path ({surfacePath}/login). Escape when the bundle
  // was rejected (403 → forbidden) so the auth guard can redirect to login.
  if (
    !sessionLoaded ||
    metaLoading ||
    (!isPublic && !bundle && !metaError && !metaForbidden)
  ) {
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

  // Error state — before the auth guard so a failed bundle shows the error
  // rather than redirecting to a login path built from a missing root_url.
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

  // In-app login lives at {surfacePath}/login (e.g. /{ws}/app/kafe/login for
  // the app surface, /{ws}/_admin/login for admin). On that route: show the
  // form when unauthenticated, otherwise bounce to the surface root.
  const loginPath = `${surfacePath}/login`
  const isLoginRoute = location.pathname === loginPath

  if (isLoginRoute) {
    if (unauthenticated) {
      return <LoginPage />
    }
    return <Navigate to={surfacePath} replace />
  }

  // Not logged in — redirect to the in-app login page with a returnTo so the
  // user lands back here after authenticating. Public surfaces boot
  // anonymously and never reach this state.
  if (!isPublic && unauthenticated) {
    const returnTo = location.pathname + location.search
    return (
      <Navigate
        to={`${loginPath}?returnTo=${encodeURIComponent(returnTo)}`}
        replace
      />
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
              path={route.path?.replace(`${mountPrefix}/`, "") || "/"}
              Component={route.Component}
            />
          ))}
          {/* Index: the App's home page (spec.route "/") when authored —
              otherwise fall back to the first derived entity. A page route
              "/" strips to an empty relative path that never matches the
              splat remainder, so it must be rendered as the index. */}
          <Route
            index
            element={
              homePage ? (
                <Suspense fallback={null}>
                  <PageRenderer entry={homePage} />
                </Suspense>
              ) : (
                <DefaultRedirect
                  bundle={bundle}
                  workspace={workspace}
                  surface={surface}
                />
              )
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

// App scope for a login URL: /{ws}/app/{app}/... → the segment after "app".
// Empty for the _admin surface / top-level /login. Role management is per-App,
// so the login must carry the app to resolve app-scoped permissions.
function appFromPath(pathname: string): string | undefined {
  const segments = pathname.split("/").filter(Boolean)
  if (segments.length >= 3 && segments[1] === "app") {
    return segments[2]
  }
  return undefined
}

function LoginPage() {
  const navigate = useNavigate()
  // In-app login (rendered inside a /:workspace/... surface) derives the
  // workspace from the URL; the top-level /login route has no param and asks
  // the user for it.
  const { workspace: workspaceParam } = useParams<{ workspace?: string }>()
  const [searchParams] = useSearchParams()
  const boot = useSessionStore((s) => s.boot)
  const app = appFromPath(window.location.pathname)

  const handleLogin = async (
    workspace: string,
    token: string,
    refreshToken?: string,
  ) => {
    await boot(workspace, token, refreshToken, app)
    // The bundle may have been loaded anonymously (empty entities) while on
    // the login route — reset it so it reloads with the authenticated
    // identity's permissions after the redirect.
    useMetaStore.getState().reset()
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

  return (
    <LoginScreen workspace={workspaceParam} app={app} onLogin={handleLogin} />
  )
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
