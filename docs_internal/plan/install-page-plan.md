# Plan: Halaman Install FormSpec (2 Metode) + Binary Multi-OS + Installer Script

**Tanggal**: 2026-09-10
**Status**: In Progress

## Latar Belakang

Belum ada halaman/konten "cara install" untuk FormSpec. Ada 2 metode yang harus
didukung:

1. **`go install github.com/primadi/formspec/cmd/formspec@latest`** — untuk Go dev.
2. **Download binary prebuilt** — untuk developer non-Go (Windows, Linux, macOS,
   windows-arm): download → (installer otomatis atau manual) → masuk PATH → bisa
   dipanggil dari folder manapun.

## Keputusan

| Aspek                 | Keputusan                                                                                       |
| --------------------- | ----------------------------------------------------------------------------------------------- |
| Lokasi konten         | Landing section `#install` di `site/` + guide lengkap `docs/guides/install.md`                  |
| Hosting binary        | GitHub Releases (`github.com/primadi/formspec/releases/download/<tag>/...`)                     |
| Release automation    | Script Makefile manual (`make release`), bukan CI/goreleaser                                    |
| Versi                 | Subcommand `formspec version` + stamping `-ldflags -X main.version=`                            |
| Jalur utama cara 2    | Script installer (`install.sh` / `install.ps1`) satu perintah                                   |
| Jalur alternatif      | Manual: tar.gz (linux/darwin) / zip (windows) + SHA256SUMS                                      |
| Hosting installer     | `site/public/` → ter-serve di `formspec.dev/install.sh` & `/install.ps1`                        |
| Lokasi install        | User-local, no sudo: `~/.local/bin` (Linux/macOS), `%LOCALAPPDATA%\Programs\formspec` (Windows) |
| Nama binary di client | `formspec` (tanpa versi) — versi via `formspec version`                                         |

## Fakta Teknis (hasil riset)

- Module: `github.com/primadi/formspec`; SQLite pakai driver pure-Go
  (`modernc.org/sqlite`) → cross-compile `CGO_ENABLED=0` aman tanpa cgo.
- SPA embed: `make build-spa` → copy ke `cmd/formspec/dist` → `go:embed dist/...`
  di `cmd/formspec/main.go`. Identik untuk semua target → build sekali, loop GOOS/GOARCH.
- Saat ini: tidak ada version var, tidak ada subcommand `version`, tidak ada
  release tooling (tanpa CI, tanpa goreleaser).
- `site/`: satu-scroll tanpa router; komponen section di `site/src/components/`;
  `Quickstart.tsx` = pola; `Nav.tsx` = array `LINKS`.
- Belum ada dokumen install di seluruh `docs/`; placeholder
  `wget .../formspec-linux-amd64.tar.gz` di `docs/cli-tools/01-formspec-dev.md` §8.

## Konvensi Artifact

| Item                  | Format                                                                                           |
| --------------------- | ------------------------------------------------------------------------------------------------ |
| Arsip per target      | `formspec-<os>-<arch>.tar.gz` (linux/darwin), `formspec-<os>-<arch>.zip` (windows)               |
| Binary di dalam arsip | `formspec` / `formspec.exe` (tanpa versi)                                                        |
| Target                | `{linux,darwin,windows} × {amd64,arm64}` = 6 artifact                                            |
| Checksums             | `SHA256SUMS.txt` (satu file per release)                                                         |
| Version stamping      | `-ldflags "-s -w -X main.version=$(VERSION)"`, `VERSION ?= git describe --tags --always --dirty` |

## Implementasi (fase & file)

### Fase 1 — Release infra

- `cmd/formspec/main.go`: `var version = "dev"` + case `version`.
- `Makefile`: target `release` (loop cross-compile + packaging + SHA256SUMS),
  target `release-upload` (opsional via `gh`).

### Fase 2 — Installer script

- `site/public/install.sh` (POSIX sh): deteksi OS/arch, resolusi versi
  (`FORMSPEC_VERSION` / `--version`, default `releases/latest`), download dari
  GitHub Releases, ekstrak, install ke `~/.local/bin/formspec`, cek PATH +
  instruksi shell rc, verifikasi `formspec version`.
- `site/public/install.ps1` (PowerShell): deteksi AMD64/ARM64, download zip,
  ekstrak ke `%LOCALAPPDATA%\Programs\formspec`, set user PATH, verifikasi.

### Fase 3 — Landing section

- `site/src/components/Install.tsx` (baru, pola `Quickstart.tsx`): tab per OS;
  3 sub-metode — Installer (recommended), Go install, Manual download.
- `site/src/App.tsx`: insert `<Install />` sebelum `<Quickstart />`.
- `site/src/components/Nav.tsx`: link `#install`.

### Fase 4 — Guide docs

- `docs/guides/install.md` (baru): prasyarat, 3 metode, matriks OS/arch,
  upgrade/uninstall, rollback `--version`, windows-arm64.
- `docs-site/.vitepress/config.mts`: sidebar item `Install` (grup guides).

### Fase 5 — Housekeeping

- `docs/cli-tools/01-formspec-dev.md` §8: placeholder wget → URL GitHub Releases riil.
- `docs/guides/how-to-run.md` + `docs/guides/README.md`: taut ke install.md.
- `docs_internal/changelog/2026-09-10-001-install-page-and-release.md`.
- `docs_internal/plan/todo.md`: task baru + status.

## Referensi Spec

- `docs/cli-tools/01-formspec-dev.md` (dev server & mode jalan)
- `docs/cli-tools/01-formspec-cli.md` (referensi verb CLI — `version` ditambah)

## Excluded (deferred)

- CI/GitHub Actions release workflow (script manual dulu).
- Codesigning/Notarization binary.
- Auto-update, formula brew/scoop.
- Release binary `formspec-registry` / `formspec-operator` (hanya `formspec`).

## Verifikasi

1. `make release VERSION=test` → `dist/release/` berisi 6 arsip + `SHA256SUMS.txt`;
   `shasum -a 256 -c` lolos; binary hasil build `formspec version` mencetak `test`.
2. Uji `install.sh` lokal: default latest-sandbox (FEEDBACK_URL / local file fallback
   saat dev), `--version` override, arch mapping benar, PATH warning muncul bila
   `~/.local/bin` belum di PATH, install ulang idempotent.
3. `cd site && npm run build && npm run typecheck` lolos; `install.sh` & `install.ps1`
   ikut ke `site/dist`; visual section `#install`.
4. `cd docs-site && npx vitepress build` lolos; halaman install di sidebar.
5. `go test ./...` tetap hijau.
