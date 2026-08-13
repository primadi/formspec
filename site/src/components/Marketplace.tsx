import { Package, Store, BadgeCheck, ShieldCheck, Users } from "lucide-react"

export function Marketplace() {
  return (
    <section id="marketplace" className="mx-auto max-w-6xl px-5 py-24">
      <div className="mx-auto max-w-2xl text-center">
        <p className="text-sm font-semibold uppercase tracking-widest text-rose-400">
          Ekosistem
        </p>
        <h2 className="mt-3 text-3xl font-bold text-white sm:text-4xl">
          Marketplace & Registry
        </h2>
        <p className="mt-4 text-zinc-400">
          Satu registry untuk Module, App template, Renderer, dan PersistBackend
          — semua artifact bertanda tangan. Marketplace adalah lapisan listing &
          metering di atasnya.
        </p>
      </div>

      <div className="mt-14 grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
        <div className="rounded-2xl border border-white/5 bg-surface-900 p-6">
          <Package className="mb-4 size-6 text-rose-400" />
          <h3 className="font-semibold text-white">Artifact</h3>
          <p className="mt-2 text-sm text-zinc-400">
            Module, Renderer, PersistBackend, Theme — semuanya artifact bertanda
            tangan di registry yang sama.
          </p>
        </div>
        <div className="rounded-2xl border border-white/5 bg-surface-900 p-6">
          <BadgeCheck className="mb-4 size-6 text-mint-400" />
          <h3 className="font-semibold text-white">Trust Tier</h3>
          <p className="mt-2 text-sm text-zinc-400">
            official / verified / community. Verified Badge wajib untuk listing
            berbayar.
          </p>
        </div>
        <div className="rounded-2xl border border-white/5 bg-surface-900 p-6">
          <Store className="mb-4 size-6 text-accent-400" />
          <h3 className="font-semibold text-white">Pricing</h3>
          <p className="mt-2 text-sm text-zinc-400">
            Vocabulary tertutup: free, one-time, subscription, per-seat,
            per-call.
          </p>
        </div>
        <div className="rounded-2xl border border-white/5 bg-surface-900 p-6">
          <ShieldCheck className="mb-4 size-6 text-amber-400" />
          <h3 className="font-semibold text-white">Signing</h3>
          <p className="mt-2 text-sm text-zinc-400">
            Signature ed25519; salinan yang di-rename tidak bisa memalsukan
            provenance.
          </p>
        </div>
      </div>

      <div className="mt-8 flex items-center justify-center gap-2 text-sm text-zinc-500">
        <Users className="size-4" />
        <span>
          Vertical apps: billing, inventory, general ledger, notifications —{" "}
          <span className="text-zinc-300">lihat ekosistem nyata di GitHub</span>
        </span>
      </div>
    </section>
  )
}
