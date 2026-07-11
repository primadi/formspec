// ─── Forma App ───
//
// Root component with boot sequence:
// 1. Parse URL → workspace + surface
// 2. Fetch _meta/me + _meta/ui → fill stores
// 3. Build route table from meta bundle
// 4. Render AppShell with router

import { useEffect } from "react"
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
  const loadMeta = useMetaStore((s) => s.load)

  // Boot: fetch session + meta
  useEffect(() => {
    if (!sessionLoaded) {
      boot(workspace).then(() => {
        const client = useSessionStore.getState().getClient()
        loadMeta(client)
      })
    }
  }, [workspace, sessionLoaded, boot, loadMeta])

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

  // Build routes from meta bundle
  const surfacePath = `/${workspace}/${surface === "admin" ? "_admin" : "app"}`
  const surfaceRoutes = buildRoutes({
    bundle,
    basePath: surfacePath,
  })

  // Apply themes
  const themeBundle = bundle.themes?.[0]
  const themeEntry = themeBundle ? {
    name: themeBundle.name,
    module: themeBundle.module,
    spec: themeBundle.spec,
  } : null

  return (
    <>
      {themeEntry && <ThemeRenderer entry={themeEntry} />}
      <Routes>
        <Route element={<AppShell />}>
        {surfaceRoutes.map((route, idx) => (
          <Route
            key={`${surface}-${idx}`}
            path={route.path?.replace(`${surfacePath}/`, "") ?? ""}
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

function DefaultRedirect({
  bundle,
  workspace,
  surface = "admin",
}: {
  bundle: import("@/types/manifest").MetaBundle
  workspace: string
  surface?: string
}) {
  // Skip summary/read-only entities for the default landing page
  const entity = bundle.entities.find((e) => e.characteristic !== "summary") ?? bundle.entities[0]
  if (entity) {
    const prefix = surface === "admin" ? "_admin" : "app"
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
