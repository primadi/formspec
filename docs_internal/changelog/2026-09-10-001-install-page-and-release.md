# 2026-09-10-001 — Halaman Install (2 metode) + Binary Release Multi-OS

**Plan**: `docs_internal/plan/install-page-plan.md`

## Apa yang diubah

Menyiapkan distribusi FormSpec untuk developer non-Go: halaman install (2 metode)
plus infrastruktur release binary multi-OS.

1. **`cmd/formspec/main.go`** — var `version` (default `dev`, di-stamp via
   `-ldflags -X main.version=`) + subcommand `formspec version` + entri usage.
2. **`Makefile`** — target `release` (SPA dibangun sekali → loop cross-compile
   `{linux,darwin,windows} × {amd64,arm64}`, `CGO_ENABLED=0`, `-trimpath`,
   ldflags version → tar.gz/zip + `SHA256SUMS.txt`) dan `release-upload`
   (via `gh`, draft).
3. **`site/public/install.sh` + `install.ps1`** — installer script user-local
   (no sudo): `~/.local/bin` (macOS/Linux), `%LOCALAPPDATA%\Programs\formspec`
   (Windows); deteksi OS/arch, resolusi versi (latest / `--version`),
   verifikasi checksum SHA256, setup PATH.
4. **`site/src/components/Install.tsx`** (baru) + integrasi `App.tsx` + `Nav.tsx`
   — landing section `#install` dengan tab per OS × 3 metode
   (installer / go install / manual).
5. **`docs/guides/install.md`** (baru) + sidebar VitePress + tabel README guides;
   placeholder wget di `docs/cli-tools/01-formspec-dev.md` §8 diganti installer;
   tautan dari `docs/guides/how-to-run.md`.

## Kenapa

`go install` menuntut Go ≥ 1.26 + compile dari source — berat untuk developer
non-Go. Binary prebuilt + installer satu perintah menutup gap tersebut.
Nama binary di client tetap `formspec` (tanpa versi); versi via `formspec version`.

## Dampak

- Binary baru ter-stamp versinya — verifikasi: `formspec version`.
- Konvensi artifact: `formspec-<os>-<arch>.tar.gz|.zip` + `SHA256SUMS.txt`
  di GitHub Releases.
- Excluded (deferred): CI release workflow, codesigning, brew/scoop, auto-update.

## Update lanjutan

- Prosedur release manual maintainer kini terdokumentasi lengkap di
  `docs/guides/releasing.md` (prasyarat, tag, build, upload draft, verifikasi
  pasca-publish, rollback); di-tautkan dari tabel `docs/guides/README.md`,
  sidebar VitePress, dan komentar Makefile.
