import { Terminal } from "lucide-react"

const STEPS = [
  {
    title: "Init project",
    code: "formspec init myapp",
  },
  {
    title: "Tulis Entity",
    code: "formspec new document invoice",
  },
  {
    title: "Validasi",
    code: "formspec validate --spec myapp/spec",
  },
  {
    title: "Jalankan",
    code: "formspec dev",
  },
]

export function Quickstart() {
  return (
    <section
      id="quickstart"
      className="border-y border-white/5 bg-surface-900/40 py-24"
    >
      <div className="mx-auto grid max-w-6xl items-center gap-12 px-5 lg:grid-cols-2">
        <div>
          <p className="text-sm font-semibold uppercase tracking-widest text-mint-400">
            Quickstart
          </p>
          <h2 className="mt-3 text-3xl font-bold text-white sm:text-4xl">
            Dari nol ke aplikasi berjalan dalam beberapa perintah
          </h2>
          <p className="mt-4 text-zinc-400">
            CLI{" "}
            <code className="rounded bg-surface-800 px-1.5 py-0.5 font-mono text-sm text-mint-300">
              formspec
            </code>{" "}
            menangani scaffold, validasi dua lapis (engine + JSON Schema),
            generasi, dan dev server. Manifes tetap di git sebagai source of
            truth.
          </p>

          <div className="mt-8 space-y-4">
            {STEPS.map((s, i) => (
              <div key={s.title} className="flex items-center gap-4">
                <span className="inline-flex size-7 shrink-0 items-center justify-center rounded-full bg-accent-500/15 font-mono text-sm text-accent-300">
                  {i + 1}
                </span>
                <div>
                  <p className="text-sm font-medium text-zinc-300">{s.title}</p>
                  <code className="font-mono text-sm text-zinc-500">
                    {s.code}
                  </code>
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className="relative">
          <div className="absolute -inset-4 rounded-3xl bg-linear-to-br from-mint-500/20 via-transparent to-accent-500/20 blur-2xl" />
          <div className="relative overflow-hidden rounded-2xl border border-white/10 bg-black/60 shadow-2xl">
            <div className="flex items-center gap-2 border-b border-white/5 px-4 py-3">
              <Terminal className="size-4 text-zinc-500" />
              <span className="text-xs text-zinc-500">
                terminal — formspec dev
              </span>
            </div>
            <pre className="overflow-x-auto p-5 text-[13px] leading-relaxed">
              <code className="font-mono">
                <span className="text-zinc-600">$</span>{" "}
                <span className="text-mint-300">formspec init myapp</span>
                {"\n"}
                <span className="text-zinc-500">
                  ✓ Workspace created · manifest scaffolded
                </span>
                {"\n\n"}
                <span className="text-zinc-600">$</span>{" "}
                <span className="text-mint-300">
                  formspec new document invoice
                </span>
                {"\n"}
                <span className="text-zinc-500">
                  ✓ Entity invoice scaffolded (CRUD + admin UI derived)
                </span>
                {"\n\n"}
                <span className="text-zinc-600">$</span>{" "}
                <span className="text-mint-300">formspec dev</span>
                {"\n"}
                <span className="text-zinc-500">
                  🌐 API: http://localhost:8080/api/v1
                </span>
                {"\n"}
                <span className="text-zinc-500">
                  🖥️ Admin: http://localhost:8080/_admin
                </span>
                {"\n"}
                <span className="text-zinc-500">
                  ✓ 12 manifests loaded · 0 problems
                </span>
              </code>
            </pre>
          </div>
        </div>
      </div>
    </section>
  )
}
