// ─── No Access State ───
//
// Rendered as the surface INDEX when the shell has nothing to land on: the
// App has no authored home page, no resolved menu, and no derived entity list
// this caller is authorized to open (App.tsx `DefaultRedirect`).
//
// Before this, that case fell back to the first non-summary entity regardless
// of permission, so an anonymous visitor to a public App landed on a derived
// list route that was never registered and read "Page not found" — a 404 for a
// page that was never offered. Worse, the empty case for a *private* App read
// "No entities found. Load a manifest to get started.", which blames the
// manifest when the truth is "this App has nothing for your account"
// (kafe 10.20 / 10.22).
//
// This states the truth and — crucially — keeps a way out: a signed-in session
// gets Sign out, an anonymous visitor gets Sign in. Rendered INSIDE the shell,
// so the chrome (brand bar, session control) stays around it.

import { LogOut } from "lucide-react"
import { Button } from "@/components/ui/button"
import { useAppNavigate } from "@/lib/navigation"
import { useSessionStore } from "@/stores/session"
import { useSurface } from "@/hooks/useSurface"

export function NoAccessState({ appName }: { appName?: string }) {
  const token = useSessionStore((s) => s.token)
  const clearSession = useSessionStore((s) => s.clearSession)
  const { surfacePath } = useSurface()
  const navigate = useAppNavigate()

  const handleLogout = () => {
    clearSession()
    navigate(surfacePath("login"), { replace: true })
  }

  return (
    <div className="flex min-h-[60vh] flex-col items-center justify-center gap-3 text-center">
      <h2 className="text-xl font-semibold">Nothing here for your account</h2>
      <p className="max-w-md text-sm text-muted-foreground">
        {appName ? `“${appName}” does not` : "This app does not"} have a page
        your account can open. If you believe you should have access, ask an
        administrator to grant your role the right permissions.
      </p>
      <div className="mt-2 flex items-center gap-3">
        {token ? (
          <Button variant="outline" onClick={handleLogout}>
            <LogOut className="size-4" />
            Sign out
          </Button>
        ) : (
          <Button onClick={() => navigate(surfacePath("login"))}>
            Sign in
          </Button>
        )}
      </div>
    </div>
  )
}
