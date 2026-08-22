// ─── Logout Button ───
//
// Clears the in-memory session and returns to the login page. Shown only for
// a real authenticated session (non-empty token) — hidden for anonymous and
// dev-bypass (developer identity) sessions where there is nothing to log out
// of.

import { useNavigate } from "react-router-dom"
import { LogOut } from "lucide-react"
import { Button } from "@/components/ui/button"
import { useSessionStore } from "@/stores/session"
import { useSurface } from "@/hooks/useSurface"

export function LogoutButton() {
  const navigate = useNavigate()
  const token = useSessionStore((s) => s.token)
  const clearSession = useSessionStore((s) => s.clearSession)
  const { surfacePath } = useSurface()

  // No real session (anonymous or dev bypass) → nothing to log out of.
  if (!token) return null

  const handleLogout = () => {
    clearSession()
    // Return to the in-app login page ({surfacePath}/login).
    navigate(surfacePath("login"), { replace: true })
  }

  return (
    <Button
      variant="ghost"
      size="icon"
      onClick={handleLogout}
      aria-label="Log out"
      title="Log out"
    >
      <LogOut className="size-4" />
    </Button>
  )
}
