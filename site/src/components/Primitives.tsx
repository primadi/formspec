import {
  Boxes,
  Database,
  KeyRound,
  Lock,
  MessagesSquare,
  Workflow,
} from "lucide-react"

const PRIMITIVES = [
  {
    name: "ctx.db",
    desc: "Datastore transaksional. Relasi, query, dan consistency di-handle engine — tanpa SQL mentah di handler.",
    icon: Database,
  },
  {
    name: "ctx.cache",
    desc: "Cache terdistribusi dengan invalidation berbasis event. Read hot-path jadi murah tanpa boilerplate.",
    icon: Boxes,
  },
  {
    name: "ctx.lock",
    desc: "Distributed lock untuk anti double-processing. Kritikal untuk job, payment, dan stock.",
    icon: Lock,
  },
  {
    name: "ctx.queue",
    desc: "Antrian kerja (work queue) dengan retry & dead-letter. Pecah pekerjaan berat jadi async.",
    icon: Workflow,
  },
  {
    name: "ctx.pubsub",
    desc: "Event bus untuk cross-module reaction. Tulis sekali, subscribe dari mana saja.",
    icon: MessagesSquare,
  },
  {
    name: "ctx.storage",
    desc: "Object storage dengan metadata. Upload file, attachment, dan artifact dengan aman.",
    icon: KeyRound,
  },
]

export function Primitives() {
  return (
    <section id="primitives" className="mx-auto max-w-6xl px-5 py-24">
      <div className="mx-auto max-w-2xl text-center">
        <p className="text-sm font-semibold uppercase tracking-widest text-accent-400">
          Closed Primitives
        </p>
        <h2 className="mt-3 text-3xl font-bold text-white sm:text-4xl">
          Enam primitif infrastruktur. Himpunan tertutup.
        </h2>
        <p className="mt-4 text-zinc-400">
          Semua infrastruktur aplikasi diakses lewat enam primitif{" "}
          <code className="rounded bg-surface-800 px-1.5 py-0.5 font-mono text-sm text-mint-300">
            ctx.*
          </code>
          . Tidak ada SQL langsung, tidak ada SDK yang bercabang-cabang.
          Konsisten di semua bahasa dan environment.
        </p>
      </div>

      <div className="mt-14 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
        {PRIMITIVES.map((p) => (
          <div
            key={p.name}
            className="group rounded-2xl border border-white/5 bg-surface-900 p-6 transition-colors hover:border-accent-500/40 hover:bg-surface-800"
          >
            <div className="mb-4 inline-flex size-11 items-center justify-center rounded-xl bg-accent-500/15 text-accent-300">
              <p.icon className="size-5" />
            </div>
            <h3 className="font-mono text-lg font-semibold text-white">
              {p.name}
            </h3>
            <p className="mt-2 text-sm leading-relaxed text-zinc-400">
              {p.desc}
            </p>
          </div>
        ))}
      </div>
    </section>
  )
}
