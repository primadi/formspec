// ─── User Menu ───
//
// Signed-in identity control for the auth area (frontend/05-app-kinds.md
// §4.1): an avatar button that opens a dropdown with the user's identity
// (user_id + roles) and a Sign out action. Replaces the bare LogoutButton
// in the header chrome.
//
// Hidden for anonymous and dev-bypass sessions (no token) — same rule as
// LogoutButton.

import * as React from "react"
import { useAppNavigate } from "@/lib/navigation"
import { KeyRound, Link2, LogOut, Repeat, UserRound } from "lucide-react"
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
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { ContextPicker } from "@/shell/ContextPicker"
import { switchSessionContext } from "@/lib/api/switchContext"
import {
  defaultContextChoice,
  readContextPreference,
} from "@/lib/session-context"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { useSurface } from "@/hooks/useSurface"
import { LinkedAccountsDialog } from "./LinkedAccountsDialog"
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
  const navigate = useAppNavigate()
  const token = useSessionStore((s) => s.token)
  const me = useSessionStore((s) => s.me)
  const workspace = useSessionStore((s) => s.workspace)
  const app = useSessionStore((s) => s.app)
  const refreshToken = useSessionStore((s) => s.refreshToken)
  const clearSession = useSessionStore((s) => s.clearSession)
  const chrome = useMetaStore((s) => s.bundle?.app.chrome)
  const { surfacePath } = useSurface()
  const [linkedAccountsOpen, setLinkedAccountsOpen] = React.useState(false)
  const [switchOpen, setSwitchOpen] = React.useState(false)
  const [switchBusy, setSwitchBusy] = React.useState(false)
  const [switchError, setSwitchError] = React.useState<string | null>(null)

  // No real session (anonymous or dev bypass) → nothing to show.
  if (!token) return null

  const handleLogout = () => {
    clearSession()
    // Return to the in-app login page ({surfacePath}/login).
    navigate(surfacePath("login"), { replace: true })
  }

  // Switching context needs somewhere to switch TO. One assignment (or none —
  // an owner/service account) means there is no choice to offer, so the item is
  // hidden rather than shown disabled: a menu entry that can never do anything
  // is noise.
  const choices = me?.context_choices ?? []
  const canSwitch = choices.length > 1

  const handleSwitch = async (assignment: string) => {
    setSwitchBusy(true)
    setSwitchError(null)
    try {
      await switchSessionContext({ workspace, app, refreshToken, assignment })
    } catch (err) {
      setSwitchError(
        err instanceof Error ? err.message : "Could not switch context",
      )
      setSwitchBusy(false)
    }
  }

  const label = me?.username || me?.user_id || "Signed in"
  // What the session is ACTING AS. `roles` says what the principal is; only
  // `context` answers "as which role, in which branch" — the question the
  // switcher exists to make changeable.
  const activeContext = me?.context
  const contextLabel = activeContext
    ? [activeContext.role, activeContext.value].filter(Boolean).join(" · ")
    : ""

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
            {contextLabel ? (
              <p
                className="text-xs font-normal text-muted-foreground"
                data-testid="active-context"
              >
                {contextLabel}
              </p>
            ) : null}
            {workspace ? (
              <p className="text-xs font-normal text-muted-foreground">
                Workspace: {workspace}
              </p>
            ) : null}
          </DropdownMenuLabel>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        {canSwitch ? (
          <DropdownMenuItem
            onClick={() => {
              setSwitchError(null)
              setSwitchOpen(true)
            }}
            className="cursor-pointer"
          >
            <Repeat className="size-4" />
            Switch context
          </DropdownMenuItem>
        ) : null}
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
        <DropdownMenuItem
          onClick={() => navigate(surfacePath("change-password"))}
          className="cursor-pointer"
        >
          <KeyRound className="size-4" />
          Change Password
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => setLinkedAccountsOpen(true)}
          className="cursor-pointer"
        >
          <Link2 className="size-4" />
          Linked Accounts
        </DropdownMenuItem>
        <DropdownMenuItem onClick={handleLogout} className="cursor-pointer">
          <LogOut className="size-4" />
          Sign out
        </DropdownMenuItem>
      </DropdownMenuContent>
      <LinkedAccountsDialog
        open={linkedAccountsOpen}
        onOpenChange={setLinkedAccountsOpen}
      />
      {/* The switcher, not a route: the point is to change context WITHOUT
          leaving the page, and a role switch re-fetches the bundle anyway
          (the switch reloads the surface). */}
      <Dialog open={switchOpen} onOpenChange={setSwitchOpen}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Switch context</DialogTitle>
          </DialogHeader>
          <ContextPicker
            choices={choices}
            defaultId={defaultContextChoice(
              choices,
              readContextPreference(workspace, app),
            )}
            onSubmit={handleSwitch}
            busy={switchBusy}
          />
          {switchError && (
            <p className="text-sm text-destructive">{switchError}</p>
          )}
        </DialogContent>
      </Dialog>
    </DropdownMenu>
  )
}
