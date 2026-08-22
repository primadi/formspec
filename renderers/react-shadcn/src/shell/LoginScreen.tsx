// ─── Login Screen ───
//
// Login screen shown when no session exists (dev-auth / prod mode).
// Username/password against the backend login endpoint. API tokens are used
// when the app is accessed programmatically from another app (not via this
// form). Navigation after login is the parent's responsibility (LoginPage
// handles the `returnTo` redirect) — this screen only authenticates.

import { useState, type FormEvent } from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { loginWithPassword } from "@/lib/api"

interface LoginScreenProps {
  /** Pre-filled workspace from the URL (in-app login) — hides the workspace field */
  workspace?: string
  onLogin: (workspace: string, token: string) => Promise<void>
}

export function LoginScreen({
  workspace: workspaceProp,
  onLogin,
}: LoginScreenProps) {
  const [workspace, setWorkspace] = useState(workspaceProp ?? "")
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

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
      const { accessToken } = await loginWithPassword(
        effectiveWorkspace,
        username.trim(),
        password,
      )
      await onLogin(effectiveWorkspace, accessToken)
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
            Sign in to your workspace
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
            {loading ? "Signing in..." : "Sign In"}
          </Button>
        </form>
      </div>
    </div>
  )
}
