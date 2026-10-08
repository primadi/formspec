// ─── OAuth Callback ───
//
// Landing route after an external auth (OAuth/OIDC, auth redesign Fase 5)
// provider redirects back. The backend delivers the token pair in the URL
// fragment (#token=...&refresh_token=...) — never in the query string, so it
// is not sent to the server or leaked into logs. This component reads the
// fragment, boots the session, and navigates to the surface root.

import { useEffect, useState } from "react"
import { useAppNavigate } from "@/lib/navigation"
import { useParams } from "react-router-dom"
import { useSessionStore } from "@/stores/session"
import { ContextPicker } from "@/shell/ContextPicker"
import {
  defaultContextChoice,
  readContextPreference,
} from "@/lib/session-context"
import {
  oauthAuthorizeURL,
  parseOAuthContextHash,
  type OAuthContextResume,
} from "@/lib/oauthContext"

export function OAuthCallback() {
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const navigate = useAppNavigate()
  const boot = useSessionStore((s) => s.boot)
  const [error, setError] = useState<string | null>(null)
  // Set when the round-trip came back asking for a context, with the App the
  // flow was started from (needed to resume it).
  const [resume, setResume] = useState<
    (OAuthContextResume & { app: string }) | null
  >(null)

  useEffect(() => {
    let cancelled = false
    const run = async () => {
      // Parse the fragment (#token=...&refresh_token=...&app=...).
      const hash = window.location.hash.replace(/^#/, "")
      const params = new URLSearchParams(hash)
      const token = params.get("token")
      const refreshToken = params.get("refresh_token")
      // The App the session is scoped to — the backend put it in the fragment
      // because login is per-App (plan app-scoped-login.md D1). Without it the
      // session cannot be booted under the right App.
      const app = params.get("app") ?? ""
      // Several session contexts and no way to choose one DURING a provider
      // round-trip: the backend sends the choices here rather than picking a
      // boundary on the caller's behalf (backend §8.7). Handled right here
      // rather than bounced to the login screen, because a pure-OAuth account
      // has no password to fall back to — the flow has to RESUME.
      const resume = parseOAuthContextHash(window.location.hash)
      if (resume) {
        if (!cancelled) setResume({ ...resume, app })
        return
      }
      if (params.get("oauth") === "error" || !token || !app) {
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
        await boot({
          workspace,
          app,
          token,
          refreshToken: refreshToken ?? undefined,
        })
        if (!cancelled) navigate(`/${workspace}`, { replace: true })
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
        {resume ? (
          <>
            <p className="text-sm text-muted-foreground">
              You can act in more than one context. Choose the one to continue
              in — signing in again is not needed.
            </p>
            <ContextPicker
              choices={resume.choices}
              defaultId={defaultContextChoice(
                resume.choices,
                readContextPreference(workspace, resume.app),
              )}
              onSubmit={(assignment) => {
                // Resume the SAME flow, now carrying the choice in `state` so
                // it survives the provider round-trip.
                window.location.href = oauthAuthorizeURL(
                  workspace,
                  resume.app,
                  resume.provider,
                  assignment,
                )
              }}
            />
          </>
        ) : error ? (
          <p className="text-sm text-destructive">{error}</p>
        ) : (
          <p className="text-sm text-muted-foreground">Completing sign in...</p>
        )}
      </div>
    </div>
  )
}
