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
import { useMetaStore } from "@/stores/meta"

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
    | "forgot"
  const [forgotMode, setForgotMode] = useState(false)
  const [email, setEmail] = useState("")
  const [forgotSent, setForgotSent] = useState(false)
  const [workspace, setWorkspace] = useState(workspaceProp ?? "")
  const [username, setUsername] = useState("")
  const [displayName, setDisplayName] = useState("")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Configured external auth providers (auth redesign Fase 5) — a button per
  // provider redirects to the authorize endpoint.
  const oauthProviders = useMetaStore((s) => s.bundle?.oauth_providers ?? [])

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
        // Email is optional but recommended: the account starts unverified
        // and a verification email is sent (account pre-hijacking
        // protection — an unverified email can never be OAuth-linked).
        const res = await fetch(`/${effectiveWorkspace}/_ui/auth/register`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            username: username.trim(),
            email: email.trim() || undefined,
            password,
            display_name: displayName.trim() || username.trim(),
          }),
        })
        if (!res.ok) {
          const body = await res.json().catch(() => null)
          throw new Error(
            body?.error?.message ?? `Registration failed (${res.status})`,
          )
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

  const handleForgotSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!effectiveWorkspace) {
      setError("Workspace is required")
      return
    }
    if (!email.trim()) {
      setError("Email is required")
      return
    }
    setLoading(true)
    setError(null)
    try {
      // Always succeeds (200) — the endpoint never reveals whether the email
      // is registered. If the address exists, a reset link is emailed.
      const res = await fetch(
        `/${effectiveWorkspace}/_ui/auth/forgot-password`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ email: email.trim() }),
        },
      )
      if (!res.ok) {
        const body = await res.json().catch(() => null)
        throw new Error(
          body?.error?.message ?? `Request failed (${res.status})`,
        )
      }
      setForgotSent(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Request failed")
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
            {forgotMode
              ? "Reset your password"
              : mode === "register"
                ? "Create your account"
                : "Sign in to your workspace"}
          </p>
        </div>

        {forgotMode ? (
          <form onSubmit={handleForgotSubmit} className="space-y-4">
            {!workspaceProp && (
              <div className="space-y-2.5">
                <label
                  htmlFor="forgot-workspace"
                  className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
                >
                  Workspace
                </label>
                <Input
                  id="forgot-workspace"
                  placeholder="acme"
                  value={workspace}
                  onChange={(e) => setWorkspace(e.target.value)}
                  disabled={loading}
                  autoComplete="off"
                />
              </div>
            )}
            <div className="space-y-2.5">
              <label
                htmlFor="forgot-email"
                className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
              >
                Email
              </label>
              <Input
                id="forgot-email"
                type="email"
                placeholder="you@example.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                disabled={loading}
                autoComplete="email"
              />
            </div>

            {forgotSent ? (
              <div className="rounded-lg border border-border bg-muted/50 p-3 text-sm text-muted-foreground">
                If an account exists for that email, a password reset link has
                been sent. Check your inbox (and spam folder).
              </div>
            ) : null}

            {error && <p className="text-sm text-destructive">{error}</p>}

            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? "Sending…" : "Send Reset Link"}
            </Button>
            <p className="text-center text-sm text-muted-foreground">
              Remembered it?{" "}
              <button
                type="button"
                onClick={() => {
                  setForgotMode(false)
                  setError(null)
                }}
                className="cursor-pointer text-foreground underline"
              >
                Back to sign in
              </button>
            </p>
          </form>
        ) : (
          <>
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

              {mode === "register" && (
                <div className="space-y-2.5">
                  <label
                    htmlFor="email"
                    className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
                  >
                    Email
                  </label>
                  <Input
                    id="email"
                    type="email"
                    placeholder="you@example.com"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    disabled={loading}
                    autoComplete="email"
                  />
                  <p className="text-xs text-muted-foreground">
                    Optional — a verification email is sent to confirm you own
                    this address.
                  </p>
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
              {mode === "login" && (
                <p className="text-center text-sm">
                  <button
                    type="button"
                    onClick={() => {
                      setForgotMode(true)
                      setError(null)
                    }}
                    className="cursor-pointer text-muted-foreground underline"
                  >
                    Forgot password?
                  </button>
                </p>
              )}
            </form>

            {/* External auth providers (auth redesign Fase 5) — a button per
            configured provider redirects to the authorize endpoint. */}
            {mode === "login" && oauthProviders.length > 0 && (
              <div className="space-y-2">
                <div className="flex items-center gap-3">
                  <div className="h-px flex-1 bg-border" />
                  <span className="text-xs text-muted-foreground">or</span>
                  <div className="h-px flex-1 bg-border" />
                </div>
                {oauthProviders.map((name) => (
                  <a
                    key={name}
                    href={`/${effectiveWorkspace}/_ui/auth/oauth/${name}/authorize`}
                    className="flex w-full items-center justify-center gap-2 rounded-lg border border-border bg-background px-3 py-2 text-sm font-medium transition-colors hover:bg-muted/50"
                  >
                    Sign in with {name.charAt(0).toUpperCase() + name.slice(1)}
                  </a>
                ))}
              </div>
            )}

            <p className="text-center text-sm text-muted-foreground">
              {mode === "register" ? (
                <>
                  Already have an account?{" "}
                  <Link
                    to={window.location.pathname.replace(
                      /\/register$/,
                      "/login",
                    )}
                    className="text-foreground underline"
                  >
                    Sign in
                  </Link>
                </>
              ) : (
                <>
                  Don't have an account?{" "}
                  <Link
                    to={
                      window.location.pathname.replace(/\/login$/, "") +
                      "/register"
                    }
                    className="text-foreground underline"
                  >
                    Sign up
                  </Link>
                </>
              )}
            </p>
          </>
        )}

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
