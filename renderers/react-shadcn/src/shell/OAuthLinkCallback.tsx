// ─── OAuth Link Callback ───
//
// Landing route after the explicit account-linking flow (todo 5.2.21). The
// backend redirects here (from the provider callback) with the OAuth code in
// in the URL fragment (#code=...&provider=...&app=...) when the authorize step
// was started with ?mode=link. This component restores the signed-in session
// for that App, POSTs the code to the authenticated link endpoint, and returns
// to the App surface.

import { useEffect, useState } from "react"
import { useAppNavigate } from "@/lib/navigation"
import { useParams } from "react-router-dom"
import { toast } from "sonner"
import { useSessionStore } from "@/stores/session"

export function OAuthLinkCallback() {
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const navigate = useAppNavigate()
  const boot = useSessionStore((s) => s.boot)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    const run = async () => {
      // Parse the fragment (#code=...&provider=...).
      const hash = window.location.hash.replace(/^#/, "")
      const params = new URLSearchParams(hash)
      const code = params.get("code")
      const provider = params.get("provider")
      const app = params.get("app") ?? ""
      if (!code || !provider || !app) {
        if (!cancelled) {
          setError("Invalid link callback — missing code, provider or app.")
        }
        return
      }
      // Restore the signed-in session for this App (tokens live in
      // sessionStorage, one slot per App). The link endpoint is authenticated
      // — the user must still be signed in.
      await boot({ workspace, app })
      const token = useSessionStore.getState().token
      if (!token) {
        if (!cancelled) {
          setError("You must be signed in to link an account.")
        }
        return
      }
      try {
        const res = await fetch(
          `/${workspace}/_ui/auth/oauth/${provider}/link`,
          {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
              Authorization: `Bearer ${token}`,
            },
            body: JSON.stringify({ code }),
          },
        )
        if (!res.ok) {
          const body = await res.json().catch(() => null)
          throw new Error(
            body?.error?.message ?? `Failed to link account (${res.status})`,
          )
        }
        if (!cancelled) {
          toast.success(
            `Linked ${provider.charAt(0).toUpperCase() + provider.slice(1)} account`,
          )
          // Return to the App the user was in. The App's surface path is
          // unknown here (root_url is free-form), so land on the workspace and
          // let WorkspaceRoute resolve the App from the path.
          navigate(`/${workspace}`, { replace: true })
        }
      } catch (err) {
        if (!cancelled) {
          setError(
            err instanceof Error ? err.message : "Failed to link account",
          )
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
          <p className="text-sm text-muted-foreground">Linking account...</p>
        )}
      </div>
    </div>
  )
}
