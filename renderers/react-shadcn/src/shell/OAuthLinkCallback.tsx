// ─── OAuth Link Callback ───
//
// Landing route after the explicit account-linking flow (todo 5.2.21). The
// backend redirects here (from the provider callback) with the OAuth code in
// the URL fragment (#code=...&provider=...) when the authorize step was
// started with ?mode=link. This component restores the signed-in session,
// POSTs the code to the authenticated link endpoint, and returns to the
// admin surface.

import { useEffect, useState } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { toast } from "sonner"
import { useSessionStore } from "@/stores/session"

export function OAuthLinkCallback() {
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const navigate = useNavigate()
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
      if (!code || !provider) {
        if (!cancelled) {
          setError("Invalid link callback — missing code or provider.")
        }
        return
      }
      // Restore the signed-in session (tokens live in sessionStorage). The
      // link endpoint is authenticated — the user must still be signed in.
      await boot(workspace)
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
          navigate(`/${workspace}/_admin`, { replace: true })
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
