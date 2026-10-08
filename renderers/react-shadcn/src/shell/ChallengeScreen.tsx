import { Loader2 } from "lucide-react"

/**
 * Blocking screen shown while an anonymous intake challenge is being solved
 * (plan docs_internal/plan/intake-challenge-pow.md).
 *
 * It exists because a proof-of-work solve is seconds of CPU on a phone, and an
 * unexplained pause looks like a broken page — the guest's next move is to
 * reload, which only adds load. The copy says what is happening and why, and
 * promises no action is required, because none is: the request retries itself
 * once the solution is found.
 *
 * Deliberately a full-surface early return (see `SurfaceShell`), not a dialog:
 * `OverlayHost`/`UiHost` are dismissible by design, and a gate the user can
 * dismiss is not a gate.
 */
export function ChallengeScreen() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 px-6 text-center">
      <Loader2
        className="size-8 animate-spin text-muted-foreground"
        aria-hidden
      />
      <h1 className="text-lg font-semibold">Memverifikasi perangkat Anda</h1>
      <p className="max-w-sm text-sm text-muted-foreground">
        Server sedang sibuk, jadi kami memastikan permintaan ini datang dari
        browser sungguhan. Tidak ada yang perlu Anda lakukan — halaman akan
        lanjut sendiri.
      </p>
      <p className="text-xs text-muted-foreground" role="status">
        Mohon jangan tutup atau muat ulang halaman ini.
      </p>
    </div>
  )
}
