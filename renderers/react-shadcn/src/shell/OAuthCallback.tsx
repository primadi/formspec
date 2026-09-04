// ─── OAuth Callback ───
//
// Landing route after an external auth (OAuth/OIDC, auth redesign Fase 5)
// provider redirects back. The backend delivers the token pair in the URL
// fragment (#token=...&refresh_token=...) — never in the query string, so it
// is not sent to the server or leaked into logs. This component reads the
// fragment, boots the session, and navigates to the surface root.

import { useEffect, useState } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { useSessionStore } from "@/stores/session"

export function OAuthCallback() {
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const navigate = useNavigate()
  const boot = useSessionStore((s) => s.boot)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    const run = async () => {
      // Parse the fragment (#token=...&refresh_token=...).
      const hash = window.location.hash.replace(/^#/, "")
      const params = new URLSearchParams(hash)
      const token = params.get("token")
      const refreshToken = params.get("refresh_token")
      if (params.get("oauth") === "error" || !token) {
        if (!cancelled) setError("Authentication failed. Please try again.")
        return
      }
      // Account pre-hijacking cases (the backend redirects with a distinct
      // fragment so the SPA can explain what happened).
      if (params.get("oauth") === "email_unverified") {
        if (!cancelled) {
          setError(
            "This email is registered but not yet verified. Sign in with your password and verify your email before linking a Google account.",
          )
        }
        return
      }
      if (params.get("oauth") === "link_required") {
        if (!cancelled) {
          setError(
            "An account with this email already exists. Sign in with your password to link your Google account.",
          )
        }
        return
      }
      try {
        await boot(workspace, token, refreshToken ?? undefined)
        if (!cancelled) navigate(`/${workspace}/_admin`, { replace: true })
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Authentication failed")
        }
      }
    }
    run()
    return () => {
      cancelled = true
    }
  }, [workspace, boot, navigate])

  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="w-full max-w-sm space-y-4 px-4 text-center">
        <h1 className="text-2xl font-bold tracking-tight">FormSpec</h1>
        {error ? (
          <p className="text-sm text-destructive">{error}</p>
        ) : (
          <p className="text-sm text-muted-foreground">Completing sign in...</p>
        )}
      </div>
    </div>
  )
}
