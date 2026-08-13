import { ArrowUpRight, Menu, X } from "lucide-react"
import { useState } from "react"
import { GithubIcon } from "./GithubIcon"

const LINKS = [
  { label: "Dokumentasi", href: "https://docs.formspec.dev" },
  { label: "Schema", href: "https://schemas.formspec.dev" },
  { label: "Registry", href: "https://registry.formspec.dev" },
]

export function Logo() {
  return (
    <a
      href="/"
      className="flex items-center gap-2 font-semibold tracking-tight"
    >
      <svg width="28" height="28" viewBox="0 0 32 32" fill="none" aria-hidden>
        <rect width="32" height="32" rx="8" fill="url(#lg)" />
        <path
          d="M9 21V11l7 8 7-8v10"
          stroke="white"
          strokeWidth="2.6"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <defs>
          <linearGradient id="lg" x1="0" y1="0" x2="32" y2="32">
            <stop stopColor="#6366f1" />
            <stop offset="1" stopColor="#10b981" />
          </linearGradient>
        </defs>
      </svg>
      <span className="text-lg">
        Form<span className="text-accent-400">Spec</span>
      </span>
    </a>
  )
}

export function Nav() {
  const [open, setOpen] = useState(false)

  return (
    <header className="sticky top-0 z-50 border-b border-white/5 bg-surface-950/80 backdrop-blur-md">
      <nav className="mx-auto flex h-16 max-w-6xl items-center justify-between px-5">
        <Logo />

        <div className="hidden items-center gap-7 md:flex">
          {LINKS.map((l) => (
            <a
              key={l.label}
              href={l.href}
              className="text-sm text-zinc-400 transition-colors hover:text-white"
            >
              {l.label}
            </a>
          ))}
        </div>

        <div className="hidden items-center gap-3 md:flex">
          <a
            href="https://github.com/primadi/formspec"
            className="inline-flex items-center gap-1.5 text-sm text-zinc-400 transition-colors hover:text-white"
          >
            <GithubIcon className="size-4" />
            GitHub
          </a>
          <a
            href="#quickstart"
            className="inline-flex items-center gap-1.5 rounded-lg bg-accent-500 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-600"
          >
            Mulai
            <ArrowUpRight className="size-4" />
          </a>
        </div>

        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          className="text-zinc-300 md:hidden"
          aria-label="Menu"
        >
          {open ? <X className="size-6" /> : <Menu className="size-6" />}
        </button>
      </nav>

      {open && (
        <div className="border-t border-white/5 bg-surface-900 px-5 py-4 md:hidden">
          <div className="flex flex-col gap-4">
            {LINKS.map((l) => (
              <a
                key={l.label}
                href={l.href}
                className="text-sm text-zinc-300"
                onClick={() => setOpen(false)}
              >
                {l.label}
              </a>
            ))}
            <a
              href="#quickstart"
              className="text-sm font-medium text-accent-300"
              onClick={() => setOpen(false)}
            >
              Mulai →
            </a>
          </div>
        </div>
      )}
    </header>
  )
}
