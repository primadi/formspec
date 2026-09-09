// ─── Login Page (self-contained) ───
//
// Self-contained login/register page — the built-in renderer for the default
// `formspec.core/login` auth page (plan auth-screens-spec-driven). Wraps
// LoginScreen with the session-boot + returnTo redirect logic (previously
// inline in App.tsx). Rendered via the built-in auth asset registry when the
// resolved auth page is the framework default; App.spec.auth.login_page
// overrides render through the normal PageRenderer instead.

import { useAppNavigate } from "@/lib/navigation"
import { useParams, useSearchParams } from "react-router-dom"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { LoginScreen } from "./LoginScreen"

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

export function LoginPage({ mode = "login" }: { mode?: "login" | "register" }) {
  const navigate = useAppNavigate()
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
    navigate(
      returnTo &&
        returnTo.startsWith("/") &&
        !returnTo.startsWith("//") &&
        returnTo !== "/"
        ? returnTo
        : `/${workspace}`,
      { replace: true },
    )
  }

  return (
    <LoginScreen
      workspace={workspaceParam}
      app={app}
      onLogin={handleLogin}
      mode={mode}
    />
  )
}
