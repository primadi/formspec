import { Binary, Boxes, Braces, Cpu, Workflow } from "lucide-react"

const TYPES = [
  {
    name: "native",
    lang: "Go",
    desc: "Business logic in-process dengan performa maksimal. Dipakai untuk hot path dan komputasi berat.",
    icon: Cpu,
    tone: "text-mint-300 bg-mint-500/10 border-mint-500/30",
  },
  {
    name: "script",
    lang: "Starlark",
    desc: "Logika sandboxed yang bisa diedit dari admin panel tanpa redeploy. Ideal untuk aturan yang sering berubah.",
    icon: Braces,
    tone: "text-accent-300 bg-accent-500/10 border-accent-500/30",
  },
  {
    name: "compiled",
    lang: "Go",
    desc: "Handler pre-built yang di-sign. Performa native tanpa exposure source code.",
    icon: Binary,
    tone: "text-amber-300 bg-amber-400/10 border-amber-400/30",
  },
  {
    name: "sidecar",
    lang: "PHP · Py · Node · Java",
    desc: "Polyglot: tulis logika di bahasa favorit lewat pola sidecar terisolasi.",
    icon: Boxes,
    tone: "text-cyan-400 bg-cyan-400/10 border-cyan-400/30",
  },
  {
    name: "wasm",
    lang: "WASM",
    desc: "Sandbox WebAssembly untuk module community — keamanan dengan portabilitas.",
    icon: Workflow,
    tone: "text-rose-400 bg-rose-400/10 border-rose-400/30",
  },
]

export function ImplTypes() {
  return (
    <section id="impl" className="mx-auto max-w-6xl px-5 py-24">
      <div className="mx-auto max-w-2xl text-center">
        <p className="text-sm font-semibold uppercase tracking-widest text-amber-400">
          Business Logic
        </p>
        <h2 className="mt-3 text-3xl font-bold text-white sm:text-4xl">
          Lima cara menulis logika bisnis
        </h2>
        <p className="mt-4 text-zinc-400">
          Pilih sesuai kebutuhan: performa, keamanan, atau fleksibilitas. Semua
          mengakses infrastruktur lewat primitif yang sama.
        </p>
      </div>

      <div className="mt-14 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
        {TYPES.map((t) => (
          <div
            key={t.name}
            className="rounded-2xl border border-white/5 bg-surface-900 p-6 transition-colors hover:border-white/15"
          >
            <div
              className={`mb-4 inline-flex size-11 items-center justify-center rounded-xl border ${t.tone}`}
            >
              <t.icon className="size-5" />
            </div>
            <div className="flex items-baseline gap-2">
              <h3 className="font-mono text-lg font-semibold text-white">
                {t.name}
              </h3>
              <span className="text-xs text-zinc-500">{t.lang}</span>
            </div>
            <p className="mt-2 text-sm leading-relaxed text-zinc-400">
              {t.desc}
            </p>
          </div>
        ))}
        <div className="flex flex-col justify-center rounded-2xl border border-dashed border-white/10 bg-surface-900/50 p-6 text-sm text-zinc-500">
          <p>
            Trust tier{" "}
            <code className="text-mint-300">
              official / verified / community
            </code>{" "}
            menggerbang tipe impl yang boleh berjalan — dari sandbox sampai
            native compiled.
          </p>
        </div>
      </div>
    </section>
  )
}
