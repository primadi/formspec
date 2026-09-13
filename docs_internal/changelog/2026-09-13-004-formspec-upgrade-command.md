# 2026-09-13-004 — `formspec upgrade` (self-update binary)

Ditambahkan verb `formspec upgrade` — self-update binary dari GitHub Releases
tanpa install ulang. Sebelumnya satu-satunya jalur upgrade adalah menjalankan
ulang installer (`curl … install.sh | sh`) atau `go install …@latest`; user
tidak punya cara untuk cek versi terbaru atau mengganti binary di tempat.

Alur: resolve versi target (`releases/latest` atau `--version <tag>`) → cek
direktori binary writable → download artifact `formspec-<os>-<arch>.tar.gz|.zip`

- `SHA256SUMS.txt` → verifikasi checksum → extract ke temp **di direktori
  binary** → smoke test (`<binary-baru> version`) → swap atomik. Flags `--check`,
  `--dry-run`, `--force`, `--yes`. Windows memakai rename-ke-`.old` (`.exe` yang
  berjalan tidak bisa di-overwrite); Unix memakai `os.Rename` atomik. Prinsip
  sama dengan `spa install`: eksplisit (bukan auto-update), rilis resmi saja,
  tanpa `sudo`, tanpa dependency baru.

File baru: `cmd/formspec/release.go` (helper bersama download/verify/extract —
`spa.go` di-refactor memakainya), `cmd/formspec/semver.go` (comparator in-repo),
`cmd/formspec/upgrade.go` (+ test untuk ketiganya). Terdampak: `main.go`
(dispatch + usage), `spa.go` (hapus duplikasi `spaHTTPGet`/`spaChecksumFromSums`/
`spaExtract`), `spa_test.go`, dan docs (`docs/cli-tools/02-formspec-cli.md`,
`docs/guides/install.md`, `docs/guides/releasing.md` §kontrak publik,
`.github/skills/formspec-cli/SKILL.md`, `site/src/components/Install.tsx`).

Referensi: `docs_internal/plan/formspec-upgrade-command.md`, todo 3.9.1.
Diverifikasi E2E: build palsu v0.0.6 → `formspec upgrade` → v0.0.7 (checksum +
smoke test OK), idempotent saat sudah terbaru, dan rollback `--version v0.0.6`.
