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
import { switchSessionContext } from "@/lib/api/switchContext"
import { useAppNavigate } from "@/lib/navigation"
import {
  defaultContextChoice,
  readContextPreference,
} from "@/lib/session-context"
import { useSessionStore } from "@/stores/session"
import type { ContextChoice } from "@/types/manifest"

interface SwitchContextScreenProps {
  workspace: string
  choices: ContextChoice[]
  /** The App this session is scoped to — login is per-App (D1). */
  app: string
  /** In-App login path, for the "sign in again" escape hatch. */
  loginPath: string
}

export function SwitchContextScreen({
  workspace,
  choices,
  app,
  loginPath,
}: SwitchContextScreenProps) {
  const navigate = useAppNavigate()
  // The refresh token proves which session is being replaced, and it lives in
  // the store rather than in the route: it is the only credential the switch
  // needs, and the props already carry the workspace/App it belongs to.
  const refreshToken = useSessionStore((s) => s.refreshToken)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // One implementation for both callers (this screen and the user-menu
  // switcher): the switch revokes the previous session, so the order of
  // operations — and the reload that re-fetches the bundle — must not differ.
  const submit = async (assignment: string) => {
    setBusy(true)
    setError(null)
    try {
      await switchSessionContext({ workspace, app, refreshToken, assignment })
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
              navigate(loginPath, { replace: true })
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
