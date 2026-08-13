import { ArrowDown, Layers, Shield, Server } from "lucide-react"

export function Architecture() {
  return (
    <section
      id="architecture"
      className="border-y border-white/5 bg-surface-900/40 py-24"
    >
      <div className="mx-auto max-w-6xl px-5">
        <div className="mx-auto max-w-2xl text-center">
          <p className="text-sm font-semibold uppercase tracking-widest text-mint-400">
            Arsitektur
          </p>
          <h2 className="mt-3 text-3xl font-bold text-white sm:text-4xl">
            Dua plane, satu tujuan
          </h2>
          <p className="mt-4 text-zinc-400">
            Governance dan eksekusi dipisah dengan tegas. Control Plane mengatur
            kebijakan; Resource Plane menjalankan bisnis. Caller tidak pernah
            tahu di mana resource berjalan.
          </p>
        </div>

        <div className="mt-14 grid gap-8 lg:grid-cols-2">
          {/* Control plane */}
          <div className="rounded-2xl border border-accent-500/25 bg-surface-900 p-7">
            <div className="flex items-center gap-3">
              <div className="inline-flex size-11 items-center justify-center rounded-xl bg-accent-500/15 text-accent-300">
                <Shield className="size-5" />
              </div>
              <div>
                <h3 className="text-lg font-semibold text-white">
                  Control Plane
                </h3>
                <p className="text-xs font-mono text-zinc-500">formspec-ctl</p>
              </div>
            </div>
            <ul className="mt-6 space-y-3 text-sm text-zinc-400">
              <li className="flex gap-2">
                <Layers className="mt-0.5 size-4 shrink-0 text-accent-400" />
                Governance: Environment, Policy, Datastore, kunci, kontrak
              </li>
              <li className="flex gap-2">
                <Layers className="mt-0.5 size-4 shrink-0 text-accent-400" />
                Transparency log — jejak keputusan governance tercatat
              </li>
              <li className="flex gap-2">
                <Layers className="mt-0.5 size-4 shrink-0 text-accent-400" />
                Tidak pernah membaca data bisnis / mengeksekusi logika
              </li>
            </ul>
          </div>

          {/* Resource plane */}
          <div className="rounded-2xl border border-mint-500/25 bg-surface-900 p-7">
            <div className="flex items-center gap-3">
              <div className="inline-flex size-11 items-center justify-center rounded-xl bg-mint-500/15 text-mint-300">
                <Server className="size-5" />
              </div>
              <div>
                <h3 className="text-lg font-semibold text-white">
                  Resource Plane
                </h3>
                <p className="text-xs font-mono text-zinc-500">
                  formspec-resource
                </p>
              </div>
            </div>
            <ul className="mt-6 space-y-3 text-sm text-zinc-400">
              <li className="flex gap-2">
                <Layers className="mt-0.5 size-4 shrink-0 text-mint-400" />
                Engine: CRUD, Action, State Machine, Event/Outbox
              </li>
              <li className="flex gap-2">
                <Layers className="mt-0.5 size-4 shrink-0 text-mint-400" />
                Spec Resolution API → render ke Shell mana pun
              </li>
              <li className="flex gap-2">
                <Layers className="mt-0.5 size-4 shrink-0 text-mint-400" />
                PersistBackend pluggable: Postgres, SQLite
              </li>
            </ul>
          </div>
        </div>

        <div className="mt-8 flex items-center justify-center gap-3 text-sm text-zinc-500">
          <span className="font-mono text-accent-300">formspec apply</span>
          <ArrowDown className="size-4" />
          <span className="text-zinc-400">
            dua-tahap: registrasi → pull policy (mTLS, tanpa write-back)
          </span>
        </div>
      </div>
    </section>
  )
}
