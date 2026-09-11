import { Download, Terminal } from "lucide-react"
import { useState } from "react"

type Method = {
  label: string
  note: string
  code: string
}

const METHODS: Record<string, Method[]> = {
  "macOS / Linux": [
    {
      label: "Installer (recommended)",
      note: "Tanpa Go. Otomatis deteksi OS/arch, pasang di ~/.local/bin, cek PATH.",
      code: "curl -fsSL https://formspec.dev/install.sh | sh",
    },
    {
      label: "Go install",
      note: "Butuh Go ≥ 1.26 di mesin Anda",
      code: "go install github.com/primadi/formspec/cmd/formspec@latest",
    },
    {
      label: "Manual",
      note: "Download dari GitHub Releases, ekstrak, taruh di PATH",
      code: "github.com/primadi/formspec/releases",
    },
  ],
  Windows: [
    {
      label: "Installer (recommended)",
      note: "Tanpa admin. Pasang di %LOCALAPPDATA%\\Programs\\formspec + user PATH",
      code: "irm https://formspec.dev/install.ps1 | iex",
    },
    {
      label: "Go install",
      note: "Butuh Go ≥ 1.26 di mesin Anda",
      code: "go install github.com/primadi/formspec/cmd/formspec@latest",
    },
    {
      label: "Manual",
      note: "Download .zip (amd64 / arm64) dari GitHub Releases",
      code: "github.com/primadi/formspec/releases",
    },
  ],
  Linux: [
    {
      label: "Installer (recommended)",
      note: "Tanpa sudo. Otomatis deteksi amd64/arm64, pasang di ~/.local/bin",
      code: "curl -fsSL https://formspec.dev/install.sh | sh",
    },
    {
      label: "Go install",
      note: "Butuh Go ≥ 1.26 di mesin Anda",
      code: "go install github.com/primadi/formspec/cmd/formspec@latest",
    },
    {
      label: "Manual",
      note: "Download .tar.gz dari GitHub Releases, ekstrak ke PATH",
      code: "github.com/primadi/formspec/releases",
    },
  ],
}

const OS_TABS = ["macOS / Linux", "Windows", "Linux"]

export function Install() {
  const [tab, setTab] = useState(OS_TABS[0])
  const methods = METHODS[tab]

  return (
    <section
      id="install"
      className="border-y border-white/5 bg-surface-900/40 py-24"
    >
      <div className="mx-auto max-w-6xl px-5">
        <div className="mx-auto max-w-2xl text-center">
          <p className="flex items-center justify-center gap-2 text-sm font-semibold uppercase tracking-widest text-mint-400">
            <Download className="size-4" />
            Install
          </p>
          <h2 className="mt-3 text-3xl font-bold text-white sm:text-4xl">
            Satu binary. Semua platform.
          </h2>
          <p className="mt-4 text-zinc-400">
            Tidak pakai Go? Tidak masalah — satu perintah shell, tanpa sudo. Go
            developer bisa pakai{" "}
            <code className="rounded bg-surface-800 px-1.5 py-0.5 font-mono text-sm text-mint-300">
              go install
            </code>
            .
          </p>
        </div>

        <div className="mx-auto mt-10 max-w-3xl">
          {/* OS tabs */}
          <div className="flex justify-center gap-1 rounded-xl border border-white/10 bg-surface-900 p-1">
            {OS_TABS.map((os) => (
              <button
                key={os}
                type="button"
                onClick={() => setTab(os)}
                className={
                  "flex-1 rounded-lg px-4 py-2 text-sm font-medium transition-colors " +
                  (tab === os
                    ? "bg-accent-500 text-white"
                    : "text-zinc-400 hover:text-white")
                }
              >
                {os}
              </button>
            ))}
          </div>

          {/* Terminal utama — installer */}
          <div className="relative mt-6">
            <div className="absolute -inset-4 rounded-3xl bg-linear-to-br from-mint-500/20 via-transparent to-accent-500/20 blur-2xl" />
            <div className="relative overflow-hidden rounded-2xl border border-white/10 bg-black/60 shadow-2xl">
              <div className="flex items-center gap-2 border-b border-white/5 px-4 py-3">
                <Terminal className="size-4 text-zinc-500" />
                <span className="text-xs text-zinc-500">{tab} — installer</span>
              </div>
              <pre className="overflow-x-auto p-5 text-[13px] leading-relaxed">
                <code className="font-mono">
                  <span className="text-zinc-600">$</span>{" "}
                  <span className="text-mint-300">{methods[0].code}</span>
                  {"\n"}
                  <span className="text-zinc-500">✓ Version: v0.0.6</span>
                  {"\n"}
                  <span className="text-zinc-500">
                    ✓ Installed: ~/.local/bin/formspec
                  </span>
                  {"\n"}
                  <span className="text-zinc-500">✓ Checksum SHA256 OK</span>
                  {"\n\n"}
                  <span className="text-zinc-600">$</span>{" "}
                  <span className="text-mint-300">formspec version</span>
                  {"\n"}
                  <span className="text-zinc-500">formspec v0.0.6</span>
                </code>
              </pre>
            </div>
          </div>

          {/* Alternatif lain */}
          <div className="mt-6 grid gap-4 sm:grid-cols-2">
            {methods.slice(1).map((m) => (
              <div
                key={m.label}
                className="rounded-xl border border-white/10 bg-surface-900 p-4"
              >
                <p className="text-sm font-semibold text-zinc-200">{m.label}</p>
                <p className="mt-1 text-xs text-zinc-500">{m.note}</p>
                <code className="mt-3 block overflow-x-auto rounded-lg bg-black/50 px-3 py-2 font-mono text-xs text-zinc-300">
                  {m.code}
                </code>
              </div>
            ))}
          </div>

          <p className="mt-6 text-center text-sm text-zinc-500">
            Panduan lengkap (upgrade, rollback, uninstall, windows-arm):{" "}
            <a
              href="https://docs.formspec.dev/guides/install.html"
              className="text-mint-300 hover:underline"
            >
              docs.formspec.dev/guides/install
            </a>
          </p>
        </div>
      </div>
    </section>
  )
}
