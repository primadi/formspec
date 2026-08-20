// ─── Login Screen ───
//
// Token/JWT input screen shown in prod mode when no session exists.
// In dev mode, the boot sequence auto-creates a synthetic identity.

import { useState, type FormEvent } from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"

interface LoginScreenProps {
  onLogin: (workspace: string, token: string) => Promise<void>
}

export function LoginScreen({ onLogin }: LoginScreenProps) {
  const [workspace, setWorkspace] = useState("")
  const [token, setToken] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!workspace.trim() || !token.trim()) {
      setError("Workspace and token are required")
      return
    }
    setLoading(true)
    setError(null)
    try {
      // Navigation after login is the parent's responsibility (LoginPage
      // handles the `returnTo` redirect) — this screen only authenticates.
      await onLogin(workspace.trim(), token.trim())
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

          <div className="space-y-2.5">
            <label
              htmlFor="token"
              className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
            >
              API Token
            </label>
            <Input
              id="token"
              type="password"
              placeholder="Paste your JWT token"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              disabled={loading}
            />
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}

          <Button type="submit" className="w-full" disabled={loading}>
            {loading ? "Signing in..." : "Sign In"}
          </Button>
        </form>

        <p className="text-center text-xs text-muted-foreground">
          In development mode, authentication is automatic.
        </p>
      </div>
    </div>
  )
}
