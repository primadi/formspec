import { ArrowUpRight } from "lucide-react"
import { Logo } from "./Nav"

const RESOURCES = [
  { label: "Dokumentasi", href: "https://docs.formspec.dev" },
  { label: "JSON Schema", href: "https://schemas.formspec.dev" },
  { label: "Module Registry", href: "https://registry.formspec.dev" },
  { label: "Status", href: "https://status.formspec.dev" },
]

const COMMUNITY = [
  { label: "GitHub", href: "https://github.com/primadi/formspec" },
  { label: "Remote MCP", href: "https://mcp.formspec.dev" },
  { label: "API", href: "https://api.formspec.dev" },
]

export function Footer() {
  return (
    <footer className="border-t border-white/5 bg-surface-950">
      <div className="mx-auto max-w-6xl px-5 py-14">
        <div className="grid gap-10 md:grid-cols-4">
          <div className="md:col-span-2">
            <Logo />
            <p className="mt-4 max-w-sm text-sm text-zinc-500">
              Ekosistem spec-first untuk aplikasi bisnis. Spec adalah kontrak;
              renderer adalah implementasi. Standar terbuka (CC0) dengan
              implementasi referensi.
            </p>
          </div>

          <div>
            <h4 className="text-sm font-semibold text-zinc-300">Resources</h4>
            <ul className="mt-4 space-y-2.5">
              {RESOURCES.map((l) => (
                <li key={l.label}>
                  <a
                    href={l.href}
                    className="inline-flex items-center gap-1 text-sm text-zinc-500 transition-colors hover:text-white"
                  >
                    {l.label}
                    <ArrowUpRight className="size-3" />
                  </a>
                </li>
              ))}
            </ul>
          </div>

          <div>
            <h4 className="text-sm font-semibold text-zinc-300">Community</h4>
            <ul className="mt-4 space-y-2.5">
              {COMMUNITY.map((l) => (
                <li key={l.label}>
                  <a
                    href={l.href}
                    className="inline-flex items-center gap-1 text-sm text-zinc-500 transition-colors hover:text-white"
                  >
                    {l.label}
                    <ArrowUpRight className="size-3" />
                  </a>
                </li>
              ))}
            </ul>
          </div>
        </div>

        <div className="mt-12 flex flex-col items-start justify-between gap-3 border-t border-white/5 pt-6 text-xs text-zinc-600 sm:flex-row sm:items-center">
          <span>
            © {new Date().getFullYear()} FormSpec · Standar terbuka (CC0)
          </span>
          <span className="font-mono">apiVersion: formspec.dev/v1alpha1</span>
        </div>
      </div>
    </footer>
  )
}
