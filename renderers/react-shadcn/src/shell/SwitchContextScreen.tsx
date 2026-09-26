// ─── Switch Context Screen ───
//
// Shown when a token refresh answered 409 `CONTEXT_REQUIRED` (backend §8.7):
// the session's assignment was revoked, or the role backing it was deleted, so
// the server will not renew the token under the old boundary — and it never
// picks a new one on the caller's behalf.
//
// This is NOT a logout. The credentials are still valid (the server refuses
// before rotating the session, so the refresh token still works), which is why
// the screen exists at all: expiring the session here would throw away a
// working credential and make the user type their password again to fix
// something they did not do.
//
// The switch goes through `POST /_ui/auth/switch`, which mints a new pair AND
// revokes the old session — never two live contexts from one session.

import { useState } from "react"

import { ContextPicker } from "@/shell/ContextPicker"
import { useAppNavigate } from "@/lib/navigation"
import {
  defaultContextChoice,
  readContextPreference,
  writeContextPreference,
} from "@/lib/session-context"
import { useSessionStore } from "@/stores/session"
import type { ContextChoice } from "@/types/manifest"

interface SwitchContextScreenProps {
  workspace: string
  choices: ContextChoice[]
  app?: string
}

export function SwitchContextScreen({
  workspace,
  choices,
  app,
}: SwitchContextScreenProps) {
  const navigate = useAppNavigate()
  const refreshToken = useSessionStore((s) => s.refreshToken)
  const setSession = useSessionStore((s) => s.setSession)
  const clearPendingContext = useSessionStore((s) => s.clearPendingContext)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const submit = async (assignment: string) => {
    setBusy(true)
    setError(null)
    try {
      const res = await fetch(`/${workspace}/_ui/auth/switch`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          refresh_token: refreshToken,
          assignment,
          ...(app ? { app } : {}),
        }),
      })
      if (!res.ok) {
        // The chosen assignment was rejected too (revoked between the two
        // calls) — the surface must keep asking, not fall back silently.
        const body = (await res.json().catch(() => null)) as {
          error?: { message?: string }
        } | null
        throw new Error(
          body?.error?.message ?? `Could not switch context (${res.status})`,
        )
      }
      const body = (await res.json()) as {
        data: { access_token: string; refresh_token: string }
      }
      writeContextPreference(workspace, app, assignment)
      clearPendingContext()
      // setSession boots nothing by itself — reload so the surface re-runs
      // boot() with the new token and the bundle is refetched under the new
      // context's permissions (a role switch changes what is visible).
      setSession(
        workspace,
        body.data.access_token,
        body.data.refresh_token,
        app,
      )
      window.location.reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not switch context")
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="w-full max-w-sm space-y-6 px-4">
        <div className="text-center">
          <h1 className="text-2xl font-bold tracking-tight">FormSpec</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Your previous context is no longer available. Choose how to
            continue.
          </p>
        </div>

        <ContextPicker
          choices={choices}
          defaultId={defaultContextChoice(
            choices,
            readContextPreference(workspace, app),
          )}
          onSubmit={submit}
          busy={busy}
        />

        {error && <p className="text-sm text-destructive">{error}</p>}

        <p className="text-center text-sm">
          <button
            type="button"
            onClick={() => {
              useSessionStore.getState().clearSession()
              navigate(`/${workspace}/_admin/login`, { replace: true })
            }}
            className="cursor-pointer text-muted-foreground underline"
          >
            Sign in again
          </button>
        </p>
      </div>
    </div>
  )
}
