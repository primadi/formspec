// ─── Reset Password Screen ───
//
// Landing page for the emailed password-reset link: reads the single-use
// token from the URL (?reset_token=...), lets the user choose a new password,
// and calls POST /{ws}/_ui/auth/reset-password. On success it redirects to
// the login screen.
//
// The query param is `reset_token` (not `token`) because the auth middleware
// reads `?token=` as a JWT for WebSocket handshakes.

import { useState, type FormEvent } from "react"
import { useNavigate, useParams, useSearchParams } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"

export function ResetPasswordScreen() {
  const navigate = useNavigate()
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const [searchParams] = useSearchParams()
  const token = searchParams.get("reset_token") ?? ""

  const [password, setPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [done, setDone] = useState(false)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!token) {
      setError("Missing or invalid reset token")
      return
    }
    if (!password) {
      setError("New password is required")
      return
    }
    if (password.length < 8) {
      setError("Password must be at least 8 characters")
      return
    }
    if (password !== confirmPassword) {
      setError("Passwords do not match")
      return
    }
    setLoading(true)
    setError(null)
    try {
      const res = await fetch(`/${workspace}/_ui/auth/reset-password`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token, password }),
      })
      if (!res.ok) {
        const body = await res.json().catch(() => null)
        throw new Error(body?.error?.message ?? `Reset failed (${res.status})`)
      }
      setDone(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Reset failed")
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
            Choose a new password
          </p>
        </div>

        {done ? (
          <div className="space-y-4">
            <div className="rounded-lg border border-border bg-muted/50 p-3 text-sm text-muted-foreground">
              Your password has been reset. You can now sign in with your new
              password.
            </div>
            <Button
              className="w-full"
              onClick={() => navigate(`/${workspace}/login`)}
            >
              Go to Sign In
            </Button>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2.5">
              <label
                htmlFor="reset-password"
                className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
              >
                New password
              </label>
              <Input
                id="reset-password"
                type="password"
                placeholder="At least 8 characters"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                disabled={loading}
                autoComplete="new-password"
              />
            </div>
            <div className="space-y-2.5">
              <label
                htmlFor="reset-confirm"
                className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
              >
                Confirm new password
              </label>
              <Input
                id="reset-confirm"
                type="password"
                placeholder="Repeat new password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                disabled={loading}
                autoComplete="new-password"
              />
            </div>

            {error && <p className="text-sm text-destructive">{error}</p>}

            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? "Resetting…" : "Reset Password"}
            </Button>
          </form>
        )}
      </div>
    </div>
  )
}
