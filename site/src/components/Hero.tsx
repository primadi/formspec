import { ArrowRight, BookOpen, Sparkles } from "lucide-react"
import { GithubIcon } from "./GithubIcon"

const MANIFEST = `apiVersion: formspec.dev/v1alpha1
kind: Entity
metadata:
  name: invoice
  module: billing
spec:
  fields:
    - name: number
      type: string
    - name: customer
      type: relation
      target: customer
    - name: total
      type: decimal
  state_machine:
    states: [draft, issued, paid, void]
    transitions:
      - from: draft
        to: issued
  actions:
    - name: mark_paid
      state_machine: paid`

export function Hero() {
  return (
    <section className="relative overflow-hidden">
      <div className="bg-grid absolute inset-0" />
      <div className="glow absolute inset-0" />

      <div className="relative mx-auto grid max-w-6xl gap-12 px-5 pb-24 pt-20 lg:grid-cols-2 lg:items-center lg:pt-28">
        <div>
          <div className="mb-5 inline-flex items-center gap-2 rounded-full border border-accent-500/30 bg-accent-500/10 px-3 py-1 text-xs font-medium text-accent-300">
            <Sparkles className="size-3.5" />
            Spec adalah kontrak · Renderer adalah implementasi
          </div>

          <h1 className="text-4xl font-bold leading-tight tracking-tight text-white sm:text-5xl lg:text-6xl">
            Tulis kontrak{" "}
            <span className="bg-linear-to-r from-accent-400 to-mint-400 bg-clip-text text-transparent">
              sekali
            </span>
            , dapatkan aplikasi bisnis utuh.
          </h1>

          <p className="mt-6 max-w-xl text-lg leading-relaxed text-zinc-400">
            FormSpec adalah ekosistem spec-first untuk aplikasi bisnis. Dari
            satu YAML manifest — API, UI, permissions, state machine, dan events
            diturunkan otomatis. Go untuk performa, Starlark untuk logika
            sandboxed, sidecar untuk bahasa apa pun.
          </p>

          <div className="mt-8 flex flex-wrap items-center gap-4">
            <a
              href="#quickstart"
              className="inline-flex items-center gap-2 rounded-lg bg-accent-500 px-5 py-3 text-sm font-semibold text-white shadow-lg shadow-accent-500/25 transition-colors hover:bg-accent-600"
            >
              Coba Sekarang
              <ArrowRight className="size-4" />
            </a>
            <a
              href="https://docs.formspec.dev"
              className="inline-flex items-center gap-2 rounded-lg border border-white/10 bg-white/5 px-5 py-3 text-sm font-semibold text-zinc-200 transition-colors hover:bg-white/10"
            >
              <BookOpen className="size-4" />
              Baca Dokumentasi
            </a>
            <a
              href="https://github.com/primadi/formspec"
              className="inline-flex items-center gap-2 rounded-lg border border-white/10 bg-white/5 px-5 py-3 text-sm font-semibold text-zinc-200 transition-colors hover:bg-white/10"
            >
              <GithubIcon className="size-4" />
              Star di GitHub
            </a>
          </div>

          <div className="mt-10 flex flex-wrap gap-6 text-sm text-zinc-500">
            <div>
              <span className="font-semibold text-zinc-200">1 spec</span> → API,
              UI, docs
            </div>
            <div>
              <span className="font-semibold text-zinc-200">
                6 ctx primitives
              </span>
              , closed set
            </div>
            <div>
              <span className="font-semibold text-zinc-200">5 tipe impl</span>{" "}
              native · Starlark
            </div>
            <div>
              <span className="font-semibold text-zinc-200">2 plane</span>{" "}
              control + resource
            </div>
          </div>
        </div>

        <div className="relative">
          <div className="absolute -inset-4 rounded-3xl bg-linear-to-br from-accent-500/20 via-transparent to-mint-500/20 blur-2xl" />
          <div className="relative overflow-hidden rounded-2xl border border-white/10 bg-surface-900/90 shadow-2xl">
            <div className="flex items-center gap-1.5 border-b border-white/5 px-4 py-3">
              <span className="size-2.5 rounded-full bg-rose-400/80" />
              <span className="size-2.5 rounded-full bg-amber-400/80" />
              <span className="size-2.5 rounded-full bg-mint-400/80" />
              <span className="ml-3 text-xs text-zinc-500">
                spec/entities/invoice.yaml
              </span>
            </div>
            <pre className="overflow-x-auto p-5 text-[13px] leading-relaxed">
              <code className="font-mono text-zinc-300">{MANIFEST}</code>
            </pre>
          </div>
          <p className="mt-3 text-center text-xs text-zinc-600">
            Satu manifest Entity → CRUD API, admin UI, menu, dan event otomatis.
          </p>
        </div>
      </div>
    </section>
  )
}
