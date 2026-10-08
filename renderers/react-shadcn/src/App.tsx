// ─── FormSpec App ───
//
// Root component with boot sequence:
// 1. Parse URL → workspace + surface
// 2. Fetch _meta/me + _meta/ui → fill stores
// 3. Build route table from meta bundle
// 4. Render RegionShell with router

import { lazy, Suspense, useEffect, useMemo, useState } from "react"
import {
  BrowserRouter,
  Routes,
  Route,
  Navigate,
  useParams,
  useLocation,
} from "react-router-dom"
import { Skeleton } from "@/components/ui/skeleton"
import { Toaster } from "sonner"
import { UiHost } from "@/shell/UiHost"
import { DownloadTray } from "@/shell/DownloadTray"

import { useSessionStore } from "@/stores/session"
import { useIntakeStore } from "@/stores/intake"
import { detectApp, useMetaStore } from "@/stores/meta"
import { usePrefsStore } from "@/stores/prefs"
import { usePageTransitionEffect } from "@/lib/navigation"
import {
  RegionShell,
  OAuthLinkCallback,
  AuthPage,
  NoAccessState,
  buildRoutes,
} from "@/shell"
import { SwitchContextScreen } from "@/shell/SwitchContextScreen"
import { ChallengeScreen } from "@/shell/ChallengeScreen"
import { pickLandingEntity } from "@/shell/landing"
import ThemeRenderer from "@/kinds/theme/ThemeRenderer"
import { useTheme } from "@/hooks/useTheme"
import { useAutoLogout } from "@/hooks/useAutoLogout"
import { preloadCommonRenderers } from "@/lib/preload"
import { fetchMetaApps } from "@/lib/api"

const PageRenderer = lazy(() => import("@/kinds/page/PageRenderer"))
import type { AppSummary } from "@/types/manifest"

// ── Shell registry ──
// All three App archetypes now render through RegionShell, which composes the
// resolved chrome REGIONS (frontend/05-app-kinds.md §4.2): the archetype is a
// preset over one region map, so `sidebar-nav` is `no-nav` + a filled sidebar.
// The per-archetype components are gone; the archetype is passed through as
// the preset selector.
const APP_SHELLS: Record<string, React.ComponentType> = {
  "sidebar-nav": RegionShell,
  topnav: RegionShell,
  "no-nav": RegionShell,
}

// ── Root: Parse URL and route to surface ──

function Root() {
  useTheme()
  // Mirror App.spec.page_transition onto <html data-page-transition> so the
  // ::view-transition-* CSS in index.css targets the configured animation.
  usePageTransitionEffect()
  preloadCommonRenderers()
  return (
    // useTransitions={false}: BrowserRouter otherwise wraps its state update
    // in React.startTransition, which flushSync cannot flush synchronously
    // (transition lane ≠ sync lane). Page transitions (lib/navigation.tsx)
    // need the DOM update to land INSIDE the startViewTransition callback,
    // or the "new" snapshot is identical to the old one and the animation
    // is invisible.
    <BrowserRouter useTransitions={false}>
      <Routes>
        <Route path="/" element={<Navigate to="/default" replace />} />
        {/* Framework routes (plan app-scoped-login.md D5). These are auth
            infrastructure, not tabs of an entity admin panel: first-run setup,
            the OAuth callbacks and change-password stay reachable even when the
            workspace has no usable App yet. No entity browsing lives under
            `_admin` any more (D4) — the derived panel is gone. */}
        <Route
          path="/:workspace/_admin/setup"
          element={<AuthPage slot="setup_page" />}
        />
        <Route
          path="/:workspace/_admin/change-password"
          element={<AuthPage slot="change_password_page" />}
        />
        <Route
          path="/:workspace/_admin/oauth/callback"
          element={<AuthPage slot="oauth_callback_page" />}
        />
        {/* OAuth link callback — explicit account linking (todo 5.2.21):
            reads the code from the URL fragment and POSTs it to the
            authenticated link endpoint. */}
        <Route
          path="/:workspace/_admin/oauth/link-callback"
          element={<OAuthLinkCallback />}
        />
        {/* Password reset — landing page for the emailed reset link
            (?token=...). Standalone, no session needed. */}
        <Route
          path="/:workspace/reset-password"
          element={<AuthPage slot="reset_password_page" />}
        />
        {/* Every other path belongs to an App. The owning App is resolved by
            longest root_url prefix (root_url is a free-form mount inside the
            workspace), which is why there is no static `/app/*` route: an App
            may own `/{ws}` itself. No App claims the path → an honest 404; the
            old fallback to the `_admin` surface no longer exists (D4). */}
        <Route path="/:workspace/*" element={<WorkspaceRoute />} />
        <Route path="*" element={<NotFound />} />
      </Routes>
      <Toaster position="top-right" richColors />
      <UiHost />
      <DownloadTray />
    </BrowserRouter>
  )
}

export default Root

// ── Workspace Route: resolve the App owning this path ──
//
// Login and rendering are both App-scoped (plan app-scoped-login.md D1): the
// App is the unit the URL belongs to, so it is resolved ONCE here — from
// /_meta/apps (a public endpoint, available before any session) — and handed to
// SurfaceShell. The session is then booted for that App's name, which is what
// makes switching App switch sessions instead of reusing whatever token was in
// the tab.

function WorkspaceRoute() {
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

  // Find the winning App by longest root_url prefix match (delegated to
  // detectApp so the router, the login screen and the meta store cannot drift
  // apart).
  const best = detectApp(window.location.pathname, apps)

  if (!best) {
    // No App claims this path. There used to be a fallback to the `_admin`
    // surface here; it is gone (D4), and inventing a replacement would hide a
    // dead link. State the truth.
    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-2">
        <h2 className="text-xl font-semibold">Page not found</h2>
        <p className="max-w-md text-center text-sm text-muted-foreground">
          No app is mounted at this path in workspace{" "}
          <span className="font-mono">{workspace}</span>.
        </p>
      </div>
    )
  }

  // mountPrefix is the fixed prefix consumed by the outer `/:workspace/*`
  // route. root_url is a free-form mount INSIDE the workspace, so every nested
  // route is derived from `surfacePath − mountPrefix`, never from surfacePath
  // (which for root_url "/app/pos" would double the "app" segment).
  return <SurfaceShell app={best} mountPrefix={`/${workspace}`} />
}

// ── Surface Shell: boot + render one App ──
//
// `app` is the resolved App this path belongs to; its `root_url` is the surface
// path and its `access` decides whether the surface boots anonymously.

function SurfaceShell({
  app,
  mountPrefix: mountPrefixOverride,
}: {
  app: AppSummary
  /**
   * Fixed pathname prefix consumed by the OUTER route. Defaults to the App's
   * own root_url — an App that owns the workspace root (root_url "/") collapses
   * to "/{ws}".
   */
  mountPrefix?: string
}) {
  const isPublic = app.access === "public"
  // Whether this App takes a login at all — the server's own test
  // (chrome.auth !== none), used to decide whether a login route exists
  // (plan app-scoped-login.md D3).
  const acceptsLogin = app.accepts_login !== false
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const location = useLocation()
  const sessionLoaded = useSessionStore((s) => s.loaded)
  const sessionError = useSessionStore((s) => s.error)
  const unauthenticated = useSessionStore((s) => s.unauthenticated)
  const token = useSessionStore((s) => s.token)
  const boot = useSessionStore((s) => s.boot)
  const pendingContext = useSessionStore((s) => s.pendingContext)
  const intakeSolving = useIntakeStore((s) => s.solving)
  const storedBundle = useMetaStore((s) => s.bundle)
  const metaApp = useMetaStore((s) => s.loadedApp)
  // The meta store holds ONE bundle for the whole SPA. A bundle resolved for a
  // DIFFERENT App is stale here (each App has its own modules, menu and
  // permissions) — treat it as absent so the boot effect reloads for this App
  // and the guards never misread another App's bundle.
  const bundle = metaApp === app.name ? storedBundle : null
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
  const themeTouched = usePrefsStore((s) => s.themeTouched)
  const setActiveTheme = usePrefsStore((s) => s.setActiveTheme)
  // The surface path is this App's own root_url (Core §4.4). One workspace may
  // resolve several Apps, each mounted at its own root_url; the App that owns
  // the current path was already resolved by WorkspaceRoute, so the path does
  // not depend on the bundle having loaded. A public App that owns the
  // workspace root (root_url "/") collapses to "/{ws}".
  const surfacePath = `/${workspace}${app.root_url ?? "/app"}`.replace(
    /\/+$/,
    "",
  )
  // mountPrefix is the FIXED prefix consumed by the outer `/:workspace/*`
  // route — root_url is a free-form mount INSIDE the workspace, so it is part
  // of surfacePath but NOT of mountPrefix. Nested route paths must therefore
  // strip mountPrefix, never surfacePath.
  const mountPrefix = mountPrefixOverride ?? `/${workspace}`
  // The App's home page (spec.route "/") — rendered as the surface index.
  const homePage = bundle?.pages?.find((p) => p.spec.route === "/")
  // This surface's own root expressed RELATIVE to mountPrefix: "app/pos" for
  // root_url "/app/pos", "" for an App that owns the workspace root
  // (root_url "/"). The nested <Routes> only ever sees the remainder after
  // mountPrefix, so every path registered here (the home route and the auth
  // screens below) is derived from surfacePath minus mountPrefix — never from
  // surfacePath itself, which for root_url "/app/pos" would double the "app"
  // segment.
  const surfaceRelative = surfacePath.startsWith(mountPrefix)
    ? surfacePath.slice(mountPrefix.length).replace(/^\/+/, "")
    : ""
  // Path of the change-password auth screen inside this surface. It is
  // registered per-surface because an App's mount is free-form (root_url) and
  // cannot be declared statically: the top-level /{ws}/_admin/change-password
  // route covers only the framework path, so on /{ws}{root_url} the user
  // menu's "Change Password" item would fall through to the catch-all 404.
  const changePasswordPath = surfaceRelative
    ? `${surfaceRelative}/change-password`
    : "change-password"
  // Login lives at {surfacePath}/login — the App's own root + "/login"
  // (e.g. /{ws}/app/pos/login). Register is the same mount + "/register":
  // self-service sign-up is App-scoped too (the account lands in this
  // workspace, then signs into THIS App).
  //
  // An App with no auth entry point (a public catalog) has NO login route at
  // all (plan app-scoped-login.md D3): the server refuses an App-scoped login
  // there and the surface boots anonymously, so a form would only offer a
  // button that 400s.
  const loginPath = `${surfacePath}/login`
  const registerPath = `${surfacePath}/register`
  const surfaceRoutes = useMemo(
    () =>
      bundle
        ? buildRoutes({
            bundle,
            basePath: surfacePath,
            surfacePublic: isPublic,
          })
        : [],
    [bundle, surfacePath, isPublic],
  )

  // Document title: App display title (fallback name) — pages refine it.
  useEffect(() => {
    if (bundle) {
      document.title = bundle.app.title ?? bundle.app.name
    }
  }, [bundle])

  // Theme binding (frontend/05-app-kinds.md §6): auto-apply the App's
  // theme_ref as the default theme — but only until the user picks a theme
  // (or color preset) themselves (prefs.themeTouched). Mode toggles
  // (light/dark/system) do NOT count as a pick, so switching mode never
  // cancels the App's theme.
  useEffect(() => {
    if (!bundle || themeTouched) return
    const ref = bundle.app.theme
    if (ref && bundle.themes.some((t) => t.name === ref)) {
      setActiveTheme(ref)
    }
  }, [bundle, themeTouched, setActiveTheme])

  // Boot: restore THIS App's session (if any), then load its bundle.
  //
  // "Loaded" is checked PER APP, not per workspace. `useSessionStore.loaded`
  // is global (one store), but a session belongs to exactly one App — reusing
  // it on another App's path attached the wrong token and the server answered
  // 403 APP_MISMATCH, so a public catalog rendered empty. `sessionApp` is the
  // App the loaded session belongs to; the effect re-boots whenever the path
  // moves to a different App.
  const appName = app.name
  const sessionApp = useSessionStore((s) => s.app)
  const sessionReady = sessionLoaded && sessionApp === appName
  useEffect(() => {
    if (isPublic) {
      if (!sessionReady) {
        boot({ workspace, app: appName }).then(() => {
          const { token } = useSessionStore.getState()
          loadMeta(workspace, token)
        })
      } else if (!bundle && !metaLoading) {
        const { token } = useSessionStore.getState()
        loadMeta(workspace, token)
      }
      return
    }
    if (!sessionReady && !token) {
      // Boot only when this App has no session AND no token is in flight. The
      // `!token` guard prevents a re-boot (without a token) when LoginPage's
      // boot() briefly resets loaded=false — that would overwrite the
      // authenticated session with an anonymous one.
      boot({ workspace, app: appName }).then(() => {
        const { token } = useSessionStore.getState()
        loadMeta(workspace, token)
      })
    } else if (!bundle && !metaLoading && !metaError && !metaForbidden) {
      // Session already loaded for this App (e.g. navigated here after a login
      // redirect) — boot() ran in LoginPage, so just load the bundle if it's
      // missing. Skip when the bundle was rejected (error/forbidden) to avoid a
      // reload loop (403 → forbidden → effect re-run → reload → 403).
      const { token } = useSessionStore.getState()
      loadMeta(workspace, token)
    }
  }, [
    workspace,
    appName,
    sessionReady,
    token,
    boot,
    loadMeta,
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
        state.refresh(workspace, token)
      }
    }
    hot.on("formspec:spec-reloaded", handler)
    return () => {
      hot.off("formspec:spec-reloaded", handler)
    }
  }, [workspace])
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

  // A token refresh answered 409 CONTEXT_REQUIRED: the session's assignment
  // (role × branch) was revoked or its role was deleted, so the server will
  // not renew it under the old boundary (backend §8.7). The credentials are
  // still good — this is a question, not a logout — so block the surface and
  // ask which context to continue in, instead of bouncing to the login form.
  if (pendingContext && pendingContext.length > 0) {
    return (
      <SwitchContextScreen
        workspace={workspace}
        choices={pendingContext}
        app={appName}
        loginPath={loginPath}
      />
    )
  }

  // Anonymous intake gate (plan intake-challenge-pow.md): while a
  // proof-of-work challenge is being solved the surface blocks, mirroring the
  // `pendingContext` screen above. A dismissible dialog would not be a gate,
  // and an unexplained multi-second pause reads as a broken page.
  if (intakeSolving) {
    return <ChallengeScreen />
  }

  // First-run setup: the workspace has no users yet → show the setup wizard
  // (self-hosted prod bootstrap without formspec-ctl). Only for anonymous
  // visitors — a signed-in user implies setup already happened. The original
  // URL is carried as `forward` so the setup wizard can route the user back
  // to where they were headed after the first admin is created.
  if (bundle?.setup_required && !token) {
    const forward = location.pathname + location.search
    return (
      <Navigate
        to={`/${workspace}/_admin/setup?forward=${encodeURIComponent(forward)}`}
        replace
      />
    )
  }

  // Login/register routes: show the form when unauthenticated, otherwise
  // bounce to the App root. Neither route exists for an App with no auth entry
  // point (e.g. a public catalog), so an anonymous visitor there is never
  // redirected to a form that cannot succeed.
  const isLoginRoute = acceptsLogin && location.pathname === loginPath
  const isRegisterRoute = acceptsLogin && location.pathname === registerPath

  // A page explicitly marked `public: true` is reachable anonymously even in
  // a private App (auth redesign Fase 3 — App access is the default, pages
  // override per-page). Match the current path against public page routes
  // (param segments like `:id` match any value).
  const currentPageIsPublic = bundle?.pages?.some((p) => {
    if (p.spec.public !== true) return false
    const route = p.spec.route.startsWith("/")
      ? p.spec.route
      : `/${p.spec.route}`
    const full = `${surfacePath}${route}`
    const fullSegs = full.split("/").filter(Boolean)
    const pathSegs = location.pathname.split("/").filter(Boolean)
    if (fullSegs.length !== pathSegs.length) return false
    return fullSegs.every(
      (seg, i) => seg.startsWith(":") || seg === pathSegs[i],
    )
  })

  if (isLoginRoute || isRegisterRoute) {
    // First-run guard: while the workspace has no users, the register form is
    // a trap — it would create a non-admin user and lock the setup wizard
    // (409 SETUP_COMPLETE) with no admin left. Send the visitor to the setup
    // wizard instead; it chains back through login afterwards.
    if (isRegisterRoute && bundle?.setup_required && !token) {
      const forward = location.pathname + location.search
      return (
        <Navigate
          to={`/${workspace}/_admin/setup?forward=${encodeURIComponent(forward)}`}
          replace
        />
      )
    }
    if (unauthenticated) {
      return (
        <AuthPage
          slot={isRegisterRoute ? "login_page" : "login_page"}
          mode={isRegisterRoute ? "register" : "login"}
        />
      )
    }
    return <Navigate to={surfacePath} replace />
  }

  // Not logged in — redirect to the App's own login page with a returnTo so
  // the user lands back here after authenticating. An App with no auth entry
  // point (a public catalog) boots anonymously and never reaches this state;
  // a private App still renders routes when the current page is explicitly
  // public (the per-page guard in buildRoutes keeps the other routes gated).
  if (acceptsLogin && !isPublic && unauthenticated && !currentPageIsPublic) {
    const returnTo = location.pathname + location.search
    return (
      <Navigate
        to={`${loginPath}?returnTo=${encodeURIComponent(returnTo)}`}
        replace
      />
    )
  }

  // Forbidden: authenticated, but the bundle was refused (403). With the
  // unscoped admin bundle gone (plan app-scoped-login.md D4) this means the
  // session's App does not permit this request.
  if (metaForbidden) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-4">
        <h1 className="text-2xl font-bold">Access Denied</h1>
        <p className="text-sm text-muted-foreground">
          You don&apos;t have permission to access this app.
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
  // picks the chrome preset for the whole surface — all three archetypes are
  // composed by RegionShell. Falls back to the sidebar preset for any
  // unknown/absent archetype.
  const archetype = bundle.app.app_renderer ?? "sidebar-nav"
  const Shell = APP_SHELLS[archetype] ?? RegionShell

  return (
    <>
      <ThemeRenderer entry={themeEntry} />
      <Routes>
        <Route element={<Shell />}>
          {surfaceRoutes.map((route, idx) => (
            <Route
              key={`${appName}-${idx}`}
              path={route.path?.replace(`${mountPrefix}/`, "") || "/"}
              Component={route.Component}
            />
          ))}
          {/* The App's own root (its `root_url`): the home page when authored,
              otherwise the sensible default landing view.

              This is a PATH, not an `index` route. The outer mount is
              `/:workspace/*`, so for the App root the nested <Routes> sees the
              remainder after mountPrefix — for `/kafe/app/pos` with
              mountPrefix `/kafe` that is the string "app/pos", never "". An
              `index` route never matches a non-empty remainder, so the root
              fell through to the catch-all and rendered 404. Deriving the path
              from surfacePath minus mountPrefix yields "app/pos" here (and ""
              for mountPrefixOverride, where it degrades to the index case). */}
          {(() => {
            const element = homePage ? (
              <Suspense fallback={null}>
                <PageRenderer entry={homePage} />
              </Suspense>
            ) : (
              <DefaultRedirect bundle={bundle} workspace={workspace} />
            )
            return surfaceRelative ? (
              <Route path={surfaceRelative} element={element} />
            ) : (
              <Route index element={element} />
            )
          })()}
          {/* Change Password — the resolved auth change-password page
              (default: formspec.core/change-password, overridable via
              App.spec.auth.change_password_page). Registered inside EVERY
              surface, so the user menu's item works on the App surface too
              (the admin-only top-level route originally left
              /{ws}{root_url}/change-password to the catch-all). Admin keeps
              rendering the top-level route: React Router ranks it above
              /:workspace/_admin/*, so this one is simply unreachable there. */}
          <Route
            path={changePasswordPath}
            element={<AuthPage slot="change_password_page" />}
          />
          {/* Catch-all: the route does not exist FOR THIS SESSION.
              This used to silently redirect to the surface root, which made a
              dead link indistinguishable from a working one: an authored menu
              entry whose target entity is not in the bundle (the entity is
              filtered out by permission — e.g. "Pelanggan" pointing at
              `cafe-master/members`, which the user has no grant for) rendered
              the FIRST entity's list instead. The user believed they had opened
              the page they clicked. A 404 states the truth. */}
          <Route
            path="*"
            element={
              <div className="flex min-h-[60vh] flex-col items-center justify-center gap-2">
                <h2 className="text-xl font-semibold">Page not found</h2>
                <p className="max-w-md text-center text-sm text-muted-foreground">
                  This page does not exist, or you don&apos;t have access to the
                  data behind it.
                </p>
              </div>
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
}: {
  bundle: import("@/types/manifest").MetaBundle
  workspace: string
}) {
  // Normalize: an App that owns the workspace root (root_url "/") collapses to
  // "/{ws}" — never "/{ws}//...".
  const base = `/${workspace}${bundle.app.root_url}`.replace(/\/+$/, "")

  // Land on the App's own first authored menu item (e.g. its Dashboard or home
  // hero) rather than an arbitrary derived entity list.
  const menuRoute = firstMenuRoute(bundle.menu)
  if (menuRoute) {
    return <Navigate to={`${base}${menuRoute}`} replace />
  }

  // Fallback for an App with no menu at all: the first entity whose derived
  // LIST route is actually registered for this caller.
  //
  // `authorized_actions` is resolved server-side with the same checker that
  // decides whether the entity ships (kafe 10.23). Landing on the first entity
  // REGARDLESS of it — the old behaviour — sent an anonymous visitor to a
  // public App to a list route that was never registered (the grant was
  // `find` only), i.e. straight to "Page not found" (kafe 10.20).
  const entity = pickLandingEntity(bundle.entities)
  if (entity) {
    return <Navigate to={`${base}/${entity.module}/${entity.plural}`} replace />
  }

  // Nothing to land on: say so honestly, inside the shell, and keep a way out
  // (sign out / sign in). Never "No entities found. Load a manifest" — that
  // blames the manifest when the truth is a permission gap (kafe 10.20/10.22).
  return <NoAccessState appName={bundle.app.title ?? bundle.app.name} />
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
