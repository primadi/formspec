// ─── Setup Screen ───
//
// First-run setup wizard (self-hosted prod bootstrap). Shown when the
// workspace has no users yet (meta bundle `setup_required: true`). Creates
// the first admin user (roles ["admin"], permissions ["*"]) via the public
// POST /{ws}/_ui/setup endpoint — no formspec-ctl needed. After setup the
// SPA redirects to the in-app login.

import { useEffect, useState, type FormEvent } from "react"
import { useAppNavigate } from "@/lib/navigation"
import { useParams, useSearchParams } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { useMetaStore } from "@/stores/meta"

export function SetupScreen() {
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const navigate = useAppNavigate()
  // `forward` = the URL the visitor originally tried to reach before being
  // redirected here (set by the App.tsx setup redirect). After the first
  // admin is created it is chained through the login screen's `returnTo` so
  // the user lands back on their original page after signing in.
  const [searchParams] = useSearchParams()
  const forward = searchParams.get("forward")
  const [username, setUsername] = useState("")
  const [displayName, setDisplayName] = useState("")
  const [password, setPassword] = useState("")
  const [confirm, setConfirm] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // The login screen this wizard chains into (carries the original URL as
  // `returnTo` so the user lands back where they came from after signing in).
  const loginPath =
    forward &&
    forward.startsWith("/") &&
    !forward.startsWith("//") &&
    forward !== "/"
      ? `/${workspace}/_admin/login?returnTo=${encodeURIComponent(forward)}`
      : `/${workspace}/_admin/login`

  // Setup is one-time: when the workspace already has users, the wizard is a
  // dead end (POST would fail with 409 SETUP_COMPLETE) — send the visitor
  // straight to login instead of showing a form that cannot succeed.
  useEffect(() => {
    let cancelled = false
    fetch(`/${workspace}/_ui/setup`)
      .then((res) => (res.ok ? res.json() : null))
      .then((body) => {
        if (cancelled) return
        if (body?.data?.setup_required === false) {
          // Drop the stale bundle (it still says setup_required=true) so the
          // next surface boots fresh instead of redirecting back here.
          useMetaStore.getState().reset()
          navigate(loginPath, { replace: true })
        }
      })
      .catch(() => {
        // Status endpoint unreachable — leave the form visible; the POST
        // will surface a proper error if setup is indeed complete.
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace])

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!username.trim() || !password) {
      setError("Username and password are required")
      return
    }
    if (password.length < 8) {
      setError("Password must be at least 8 characters")
      return
    }
    if (password !== confirm) {
      setError("Passwords do not match")
      return
    }
    setLoading(true)
    setError(null)
    try {
      const res = await fetch(`/${workspace}/_ui/setup`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          username: username.trim(),
          password,
          display_name: displayName.trim() || username.trim(),
        }),
      })
      if (!res.ok) {
        const body = await res.json().catch(() => null)
        // Setup already done (e.g. the wizard was open in a stale tab while
        // another tab completed it) — login is the right place to be.
        if (body?.error?.code === "SETUP_COMPLETE") {
          navigate(loginPath, { replace: true })
          return
        }
        throw new Error(body?.error?.message ?? `Setup failed (${res.status})`)
      }
      // Setup complete → route the user back where they came from. The
      // original URL is carried as `forward` (set by the App.tsx redirect);
      // it is validated same-origin and chained through the login screen's
      // `returnTo` so the user lands on their original page after signing in.
      // Reset the meta store first: the loaded bundle still says
      // setup_required=true and would redirect the next surface straight
      // back here (setup ↔ login loop).
      useMetaStore.getState().reset()
      navigate(loginPath, { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : "Setup failed")
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
            Set up your first admin account
          </p>
          <p className="mt-1 text-xs text-muted-foreground">
            Workspace: {workspace}
          </p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2.5">
            <label
              htmlFor="setup-username"
              className="text-sm font-medium leading-none"
            >
              Username
            </label>
            <Input
              id="setup-username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="mis. admin"
              autoComplete="username"
            />
          </div>

          <div className="space-y-2.5">
            <label
              htmlFor="setup-display"
              className="text-sm font-medium leading-none"
            >
              Display name
            </label>
            <Input
              id="setup-display"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder="Nama tampilan (opsional)"
            />
          </div>

          <div className="space-y-2.5">
            <label
              htmlFor="setup-password"
              className="text-sm font-medium leading-none"
            >
              Password
            </label>
            <Input
              id="setup-password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Minimal 8 karakter"
              autoComplete="new-password"
            />
          </div>

          <div className="space-y-2.5">
            <label
              htmlFor="setup-confirm"
              className="text-sm font-medium leading-none"
            >
              Confirm password
            </label>
            <Input
              id="setup-confirm"
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              placeholder="Ulangi password"
              autoComplete="new-password"
            />
          </div>

          {error && (
            <p className="text-sm text-destructive" role="alert">
              {error}
            </p>
          )}

          <Button type="submit" className="w-full" disabled={loading}>
            {loading ? "Creating admin..." : "Create Admin Account"}
          </Button>
        </form>
      </div>
    </div>
  )
}
