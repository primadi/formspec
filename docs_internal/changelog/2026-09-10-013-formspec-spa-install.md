# 2026-09-10-013 — formspec spa install + spa artifact di release

## Apa

Implementasi plan `docs_internal/plan/spa-install-command.md` (Fase 1–4):

- **F1**: `make release` menambah artifact `spa-<VERSION>.tar.gz`
  (platform-agnostic; layout `spa/` + `manifest.json` berisi versi) — ikut
  masuk `SHA256SUMS.txt` secara otomatis.
- **F2**: subcommand baru `formspec spa install|path|remove`
  (`cmd/formspec/spa.go`) — download artifact dari GitHub Releases **versi
  yang sama dengan binary**, verify SHA256 terhadap `SHA256SUMS.txt`, extract
  ke cache `~/.formspec/spa/<versi>/` (prefix `spa/` di-strip, guard
  path-traversal). Idempotent; `--force` untuk download ulang; base URL bisa
  di-override via `FORMSPEC_SPA_URL`.
- **F3**: fallback chain SPA di `formspec dev` (`dev.go`) ditambah satu
  tingkat: `--dev-ui → --web-dir → auto-detect repo → cache spa install →
embedded/stub`. Placeholder page di-update (opsi pertama:
  `formspec spa install`).
- **F4**: `docs/guides/install.md` — Metode 2 `go install` di-demote jadi
  "CLI-only" dengan penjelasan batasan + dokumentasi `spa`; CLI reference
  `docs/cli-tools/02-formspec-cli.md` menambah section `spa`.

## Kenapa

Binary `go install` tidak memuat embedded SPA (compiler Go tidak bisa
menjalankan npm) — sebelumnya user jalur ini tersangkut di placeholder tanpa
jalur mudah ke UI. Download otomatis diam-diam ditolak (security by
default); command eksplisit + version-locked menutup gap tanpa risiko
supply-chain/version mismatch.

## File terkena dampak

- `Makefile` (spa artifact + fix tab `build-registry`)
- `cmd/formspec/spa.go` (baru), `spa_test.go` (baru), `main.go`, `dev.go`,
  `spa_stub/index.html`
- `cmd/formspec-registry/web/stub/index.html`
- `docs/guides/install.md`, `docs/cli-tools/02-formspec-cli.md`

## Verifikasi

- `go build ./...`, `go vet`, `go test ./cmd/formspec` (termasuk 4 test spa:
  checksum parsing, extract, traversal rejection, cache dir) — hijau
- `make release VERSION=v0.0.99` — `spa-v0.0.99.tar.gz` ter-generate +
  entri di `SHA256SUMS.txt`
- Smoke: `spa path` / `spa install` (dev guard) berperilaku sesuai pesan

## Referensi

- `docs_internal/plan/spa-install-command.md`
- `docs_internal/changelog/2026-09-10-012-untrack-embedded-spa-build-tag.md`
