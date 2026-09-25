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
import { detectApp, useMetaStore } from "@/stores/meta"
import { fetchMetaApps } from "@/lib/api/meta"
import type { AppSummary } from "@/types/manifest"
import { useEffect, useState } from "react"
import { LoginScreen } from "./LoginScreen"

// App scope for a login request, resolved from the URL by matching the longest
// `root_url` prefix — the SAME rule the app bundle uses (`detectApp`), so the
// session and the bundle can never disagree.
//
// WHY NOT the URL segment after `/app/`: the URL is `/{ws}/app/{segment}/...`
// and the segment is `pos` while the App is named `kafe-pos`. Roles are
// declared `app: kafe-pos`, so a session scoped to `pos` matches NOTHING:
// `PermissionResolver` skips every role whose `app` differs from the session's,
// the user ends up with zero permissions, and an `access: private` App then
// serves an EMPTY bundle — the menu still renders (it is built from the App
// manifest, not from permissions) while every list under it is blank. That is
// exactly how it looked: a sidebar full of items over an empty page.
//
// The resolved root_url is ALSO the post-login landing path. Redirecting to
// `/{workspace}` instead drops the `/app/<segment>` prefix, so the bundle is
// then resolved for whatever App owns the root (for kafe: the PUBLIC `kafe-qr`)
// and the signed-in user is shown a different App than the one they logged into.

export function LoginPage({ mode = "login" }: { mode?: "login" | "register" }) {
  const navigate = useAppNavigate()
  // In-app login (rendered inside a /:workspace/... surface) derives the
  // workspace from the URL; the top-level /login route has no param and asks
  // the user for it.
  const { workspace: workspaceParam } = useParams<{ workspace?: string }>()
  const [searchParams] = useSearchParams()
  const boot = useSessionStore((s) => s.boot)
  // The app list is needed to turn the URL into an App NAME. The login screen
  // may render before any bundle is loaded (that is the point of a login
  // screen), so it fetches the list itself rather than reading it from the meta
  // store — which is also why the store's `load()` cannot be reused for this.
  const [apps, setApps] = useState<AppSummary[]>([])
  useEffect(() => {
    if (!workspaceParam) return
    fetchMetaApps(workspaceParam)
      .then(setApps)
      .catch(() => setApps([]))
  }, [workspaceParam])
  const resolvedApp = detectApp(window.location.pathname, apps)

  const handleLogin = async (
    workspace: string,
    token: string,
    refreshToken?: string,
  ) => {
    await boot(workspace, token, refreshToken, resolvedApp?.name)
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
        // Land on the App's own root_url, not on `/{workspace}` — see the note
        // above about the prefix loss. Falls back to workspace root only when
        // the app list is unavailable.
        : `/${workspace}${resolvedApp && resolvedApp.root_url !== "/" ? resolvedApp.root_url : ""}`,
      { replace: true },
    )
  }

  return (
    <LoginScreen
      workspace={workspaceParam}
      app={resolvedApp?.name}
      onLogin={handleLogin}
      mode={mode}
    />
  )
}
