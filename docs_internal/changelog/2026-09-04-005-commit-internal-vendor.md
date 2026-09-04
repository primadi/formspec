# 2026-09-04-005-commit-internal-vendor.md

## Commit `internal/vendor` agar `go install @latest` berfungsi

**Masalah:** `go install github.com/primadi/formspec/cmd/formspec@latest` gagal
dengan `module ... does not contain package github.com/primadi/formspec/internal/vendor`.

**Akar masalah:** pola `.gitignore` `vendor/` (tanpa anchor) cocok dengan
direktori bernama `vendor` di level mana pun — termasuk `internal/vendor/`,
paket Go asli (module vendoring: lockfile, install, registry client, signing,
overrides) yang dipakai `resource/formspec.go` dan CLI. Karena ter-ignore,
paket tidak pernah di-commit sehingga tidak ada di module yang dipublish.

**Perbaikan:**
- `.gitignore`: `vendor/` → `/vendor/` (root-anchored, hanya ignore Go vendor
  dir di root), tambah `sdk/php/vendor/` eksplisit (composer deps tetap ignore).
- Commit `internal/vendor/` (11 file) — commit `15e663d`.

**Verifikasi:** `go build ./...` OK; `go install ./cmd/formspec` OK.

**Catatan:** `@latest` baru melihat perbaikan setelah push ke remote.

Ref: `docs_internal/plan/fase13-vendoring.md`