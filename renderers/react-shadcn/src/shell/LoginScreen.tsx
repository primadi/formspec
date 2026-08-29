// ─── Login Screen ───
//
// Login screen shown when no session exists (dev-auth / prod mode).
// Username/password against the backend login endpoint. API tokens are used
// when the app is accessed programmatically from another app (not via this
// form). Navigation after login is the parent's responsibility (LoginPage
// handles the `returnTo` redirect) — this screen only authenticates.

import { useState, type FormEvent } from "react"
import { Link, useSearchParams } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select } from "@/components/ui/select"
import { loginWithPassword } from "@/lib/api"
import { usePrefsStore } from "@/stores/prefs"

interface LoginScreenProps {
  /** Pre-filled workspace from the URL (in-app login) — hides the workspace field */
  workspace?: string
  /** App scope for this login (role management is per-App); empty = workspace-level */
  app?: string
  onLogin: (
    workspace: string,
    token: string,
    refreshToken?: string,
  ) => Promise<void>
}

export function LoginScreen({
  workspace: workspaceProp,
  app,
  onLogin,
  mode: modeProp,
}: LoginScreenProps & { mode?: "login" | "register" }) {
  const [searchParams] = useSearchParams()
  const mode = (modeProp ??
    (searchParams.get("mode") === "register" ? "register" : "login")) as
    | "login"
    | "register"
  const [workspace, setWorkspace] = useState(workspaceProp ?? "")
  const [username, setUsername] = useState("")
  const [displayName, setDisplayName] = useState("")
  const [password, setPassword] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Auto-logout idle timeout preference (persisted; 0 = never).
  const sessionTimeoutMinutes = usePrefsStore((s) => s.sessionTimeoutMinutes)
  const setSessionTimeoutMinutes = usePrefsStore(
    (s) => s.setSessionTimeoutMinutes,
  )

  // Workspace comes from the URL when provided (in-app login); otherwise the
  // user types it (top-level /login).
  const effectiveWorkspace = workspaceProp ?? workspace.trim()

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!effectiveWorkspace) {
      setError("Workspace is required")
      return
    }
    if (!username.trim() || !password) {
      setError("Username and password are required")
      return
    }
    setLoading(true)
    setError(null)
    try {
      if (mode === "register") {
        // Self-service registration (portal sign-up) — then auto-login.
        const res = await fetch(
          `/${effectiveWorkspace}/_ui/auth/register`,
          {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              username: username.trim(),
              password,
              display_name: displayName.trim() || username.trim(),
            }),
          },
        )
        if (!res.ok) {
          const body = await res.json().catch(() => null)
          throw new Error(body?.error?.message ?? `Registration failed (${res.status})`)
        }
      }
      const { accessToken, refreshToken } = await loginWithPassword(
        effectiveWorkspace,
        username.trim(),
        password,
        app,
      )
      await onLogin(effectiveWorkspace, accessToken, refreshToken)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Authentication failed")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="w-full max-w-sm space-y-6 px-4">
        <div className="text-center">
          <h1 className="text-2xl font-bold tracking-tight">FormSpec</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {mode === "register"
              ? "Create your account"
              : "Sign in to your workspace"}
          </p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          {!workspaceProp && (
            <div className="space-y-2.5">
              <label
                htmlFor="workspace"
                className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
              >
                Workspace
              </label>
              <Input
                id="workspace"
                placeholder="acme"
                value={workspace}
                onChange={(e) => setWorkspace(e.target.value)}
                disabled={loading}
                autoComplete="off"
              />
            </div>
          )}

          {mode === "register" && (
            <div className="space-y-2.5">
              <label
                htmlFor="display_name"
                className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
              >
                Display Name
              </label>
              <Input
                id="display_name"
                placeholder="Your Name"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                disabled={loading}
                autoComplete="name"
              />
            </div>
          )}

          <div className="space-y-2.5">
            <label
              htmlFor="username"
              className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
            >
              Username
            </label>
            <Input
              id="username"
              placeholder="admin"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              disabled={loading}
              autoComplete="username"
            />
          </div>
          <div className="space-y-2.5">
            <label
              htmlFor="password"
              className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
            >
              Password
            </label>
            <Input
              id="password"
              type="password"
              placeholder="••••••••"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              disabled={loading}
              autoComplete="current-password"
            />
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}

          <Button type="submit" className="w-full" disabled={loading}>
            {loading
              ? mode === "register"
                ? "Creating account..."
                : "Signing in..."
              : mode === "register"
                ? "Create Account"
                : "Sign In"}
          </Button>
        </form>

        <p className="text-center text-sm text-muted-foreground">
          {mode === "register" ? (
            <>
              Already have an account?{" "}
              <Link
                to={window.location.pathname.replace(/\/register$/, "/login")}
                className="text-foreground underline"
              >
                Sign in
              </Link>
            </>
          ) : (
            <>
              Don't have an account?{" "}
              <Link
                to={window.location.pathname.replace(/\/login$/, "") + "/register"}
                className="text-foreground underline"
              >
                Sign up
              </Link>
            </>
          )}
        </p>

        <div className="space-y-2.5">
          <label
            htmlFor="session-timeout"
            className="text-sm font-medium leading-none"
          >
            Auto logout after
          </label>
          <Select
            value={String(sessionTimeoutMinutes)}
            onChange={(v) => setSessionTimeoutMinutes(Number(v))}
            options={[
              { value: "5", label: "5 minutes" },
              { value: "15", label: "15 minutes" },
              { value: "30", label: "30 minutes" },
              { value: "60", label: "60 minutes" },
              { value: "0", label: "Never (stay signed in)" },
            ]}
            disabled={loading}
          />
          <p className="text-xs text-muted-foreground">
            You&apos;ll be signed out automatically after this period of
            inactivity.
          </p>
        </div>
      </div>
    </div>
  )
}
