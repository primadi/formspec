// ─── User Menu ───
//
// Signed-in identity control for the auth area (frontend/05-app-kinds.md
// §4.1): an avatar button that opens a dropdown with the user's identity
// (user_id + roles) and a Sign out action. Replaces the bare LogoutButton
// in the header chrome.
//
// Hidden for anonymous and dev-bypass sessions (no token) — same rule as
// LogoutButton.

import { useNavigate } from "react-router-dom"
import { LogOut, UserRound } from "lucide-react"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { useSurface } from "@/hooks/useSurface"
function initialsOf(id: string): string {
  // Split on whitespace/punctuation AND camelCase boundaries (TestUser →
  // T,U) so mixed-case usernames produce proper two-letter initials.
  const parts = id
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
    .split(/[\s._@-]+/)
    .filter(Boolean)
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase()
  return id.slice(0, 2).toUpperCase()
}

export function UserMenu() {
  const navigate = useNavigate()
  const token = useSessionStore((s) => s.token)
  const me = useSessionStore((s) => s.me)
  const clearSession = useSessionStore((s) => s.clearSession)
  const chrome = useMetaStore((s) => s.bundle?.app.chrome)
  const { surfacePath } = useSurface()

  // No real session (anonymous or dev bypass) → nothing to show.
  if (!token) return null

  const handleLogout = () => {
    clearSession()
    // Return to the in-app login page ({surfacePath}/login).
    navigate(surfacePath("login"), { replace: true })
  }

  const label = me?.username || me?.user_id || "Signed in"
  // Profile route is opt-in via App chrome (`profile_route`) — apps without
  // one simply don't get a Profile item.
  const profileRoute = chrome?.profile_route

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="User menu"
        className="cursor-pointer rounded-full outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <Avatar size="sm">
          <AvatarFallback>{initialsOf(label)}</AvatarFallback>
        </Avatar>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        {/* GroupLabel (base-ui) wajib berada di dalam <Menu.Group> —
            tanpa wrapper ini melempar Base UI error #31 (blank screen). */}
        <DropdownMenuGroup>
          <DropdownMenuLabel>
            <div className="flex items-center gap-2">
              <UserRound className="size-4 text-muted-foreground" />
              <span className="truncate font-medium">{label}</span>
            </div>
            {me?.roles?.length ? (
              <p className="text-xs font-normal text-muted-foreground">
                {me.roles.join(", ")}
              </p>
            ) : null}
          </DropdownMenuLabel>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        {profileRoute ? (
          <DropdownMenuItem
            // profile_route is an app-level route (like page routes) —
            // resolve it against the surface prefix so the workspace
            // segment is included (navigate with a bare "/x" path would
            // escape the workspace and be caught by /:workspace/*).
            onClick={() => navigate(surfacePath(profileRoute))}
            className="cursor-pointer"
          >
            <UserRound className="size-4" />
            Profile
          </DropdownMenuItem>
        ) : null}
        <DropdownMenuItem onClick={handleLogout} className="cursor-pointer">
          <LogOut className="size-4" />
          Sign out
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
