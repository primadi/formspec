import { BookOpen, Send } from "lucide-react"
import { GithubIcon } from "./GithubIcon"

export function CTA() {
  return (
    <section className="mx-auto max-w-6xl px-5 py-24">
      <div className="relative overflow-hidden rounded-3xl border border-accent-500/30 bg-linear-to-br from-surface-800 to-surface-900 px-6 py-16 text-center sm:px-12">
        <div className="glow absolute inset-0" />
        <div className="relative">
          <h2 className="mx-auto max-w-2xl text-3xl font-bold text-white sm:text-4xl">
            Kontrak sekali. Dapatkan platform di mana pun.
          </h2>
          <p className="mx-auto mt-4 max-w-xl text-zinc-400">
            FormSpec adalah standar terbuka (CC0) dengan implementasi referensi.
            Jelajahi dokumentasi, coba di GitHub, atau bergabung dengan
            ekosistem.
          </p>
          <div className="mt-8 flex flex-wrap items-center justify-center gap-4">
            <a
              href="https://docs.formspec.dev"
              className="inline-flex items-center gap-2 rounded-lg bg-accent-500 px-6 py-3 text-sm font-semibold text-white transition-colors hover:bg-accent-600"
            >
              <BookOpen className="size-4" />
              Baca Dokumentasi
            </a>
            <a
              href="https://github.com/primadi/formspec"
              className="inline-flex items-center gap-2 rounded-lg border border-white/10 bg-white/5 px-6 py-3 text-sm font-semibold text-zinc-200 transition-colors hover:bg-white/10"
            >
              <GithubIcon className="size-4" />
              GitHub
            </a>
            <a
              href="https://formspec.dev/schemas"
              className="inline-flex items-center gap-2 rounded-lg border border-white/10 bg-white/5 px-6 py-3 text-sm font-semibold text-zinc-200 transition-colors hover:bg-white/10"
            >
              <Send className="size-4" />
              Schema JSON
            </a>
          </div>
        </div>
      </div>
    </section>
  )
}
