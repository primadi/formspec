// ─── Linked Accounts Dialog ───
//
// Self-service "link an external sign-in method" dialog opened from the user
// menu (todo 5.2.21). Lists every configured OAuth provider with its link
// state:
//
//   - Linked provider → "Unlink" button (two-step inline confirm). Unlinking
//     POSTs to the authenticated unlink endpoint and refreshes the identity.
//     A pure-OAuth account (no password) is rejected by the backend — the
//     user must set a password first.
//   - Unlinked provider → "Link {provider}" button that starts the explicit
//     linking flow (?mode=link) — full-page navigation to the provider, then
//     back to the OAuthLinkCallback route which POSTs the code to the
//     authenticated link endpoint.

import * as React from "react"
import { toast } from "sonner"
import { Link2, ShieldCheck } from "lucide-react"

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { useSessionStore } from "@/stores/session"
import { useMetaStore } from "@/stores/meta"
import { fetchMe } from "@/lib/api"

interface LinkedAccountsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function LinkedAccountsDialog({
  open,
  onOpenChange,
}: LinkedAccountsDialogProps) {
  const workspace = useSessionStore((s) => s.workspace)
  const token = useSessionStore((s) => s.token)
  const me = useSessionStore((s) => s.me)
  // Configured external auth providers (auth redesign Fase 5) — same list the
  // login screen renders as sign-in buttons. Select the raw value (stable
  // reference) and fall back to [] outside the selector — a `?? []` inside
  // would return a fresh array every render and loop forever.
  const oauthProviders = useMetaStore((s) => s.bundle?.oauth_providers) ?? []

  const linkedProvider = me?.oauth_provider ?? ""

  // Provider name awaiting the second (confirm) click of the unlink flow.
  const [confirmUnlink, setConfirmUnlink] = React.useState<string | null>(null)
  const [unlinking, setUnlinking] = React.useState(false)

  // Reset the confirm state each time the dialog opens.
  React.useEffect(() => {
    if (open) {
      setConfirmUnlink(null)
      setUnlinking(false)
    }
  }, [open])

  const handleUnlink = async (provider: string) => {
    if (confirmUnlink !== provider) {
      // First click — arm the confirm state.
      setConfirmUnlink(provider)
      return
    }
    setUnlinking(true)
    try {
      const res = await fetch(
        `/${workspace}/_ui/auth/oauth/${provider}/unlink`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
          },
        },
      )
      if (!res.ok) {
        const body = await res.json().catch(() => null)
        throw new Error(
          body?.error?.message ?? `Failed to unlink account (${res.status})`,
        )
      }
      // Refresh the identity so the dialog reflects the change.
      const fresh = await fetchMe(workspace, token)
      if (fresh) useSessionStore.setState({ me: fresh })
      toast.success(
        `Unlinked ${provider.charAt(0).toUpperCase() + provider.slice(1)}`,
      )
      setConfirmUnlink(null)
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to unlink account",
      )
      setConfirmUnlink(null)
    } finally {
      setUnlinking(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Link2 className="size-4" />
            Linked Accounts
          </DialogTitle>
          <DialogDescription>
            Link or unlink an external sign-in method for this account.
          </DialogDescription>
        </DialogHeader>

        {oauthProviders.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No external sign-in providers are configured for this workspace.
          </p>
        ) : (
          <div className="space-y-2">
            {oauthProviders.map((name) => {
              const label = name.charAt(0).toUpperCase() + name.slice(1)
              const isLinked = linkedProvider === name
              return (
                <div
                  key={name}
                  className="flex items-center justify-between rounded-lg border border-border px-3 py-2"
                >
                  <span className="text-sm font-medium">{label}</span>
                  {isLinked ? (
                    <div className="flex items-center gap-2">
                      <span className="inline-flex items-center gap-1.5 text-xs font-medium text-emerald-600">
                        <ShieldCheck className="size-4" />
                        Linked
                      </span>
                      <Button
                        size="sm"
                        variant={
                          confirmUnlink === name ? "destructive" : "outline"
                        }
                        disabled={unlinking}
                        onClick={() => handleUnlink(name)}
                      >
                        {confirmUnlink === name ? "Confirm unlink?" : "Unlink"}
                      </Button>
                    </div>
                  ) : (
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => {
                        // Full-page navigation (consistent with the login
                        // flow) — the provider page is a cross-origin hop.
                        window.location.href = `/${workspace}/_ui/auth/oauth/${name}/authorize?mode=link`
                      }}
                    >
                      Link {label}
                    </Button>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
