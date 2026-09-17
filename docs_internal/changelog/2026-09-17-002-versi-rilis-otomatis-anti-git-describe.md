# 2026-09-17-002 — Versi rilis otomatis (anti string git-describe)

## Apa yang diubah

Rilis `v0.0.8-4-gceaaf2a` (published 2026-09-17) lahir dari `make release`
**tanpa** `VERSION=`: default `VERSION ?= $(shell git describe --tags …)`
menstamp string git-describe, dan guard `release-upload` meloloskannya karena
regex-nya tidak di-anchor `$`. String itu dibaca komparator semver
`formspec upgrade` sebagai *prerelease v0.0.8*, jadi user di v0.0.8 mendapat
prompt "target lebih lama (rollback)" untuk rilis yang isinya justru 4 commit
lebih baru.

Perbaikan tiga lapis:

- `Makefile` — `VERSION` untuk target rilis kini diisi otomatis: `release` /
  `release-version` memakai patch-bump tag semver tertinggi
  (`scripts/next-version.sh`), `release-upload` memakai versi yang sudah
  tertanam di artifact `dist/release/` (bukan di-derive ulang, agar tag yang
  dibuat di antara dua perintah tidak menggeser hasil auto-bump). `git describe`
  tetap jadi stamp build dev (`DEV_VERSION`). Guard baru
  `check-release-version` / `check-upload-version` + target `release-version`.
- `scripts/check-semver.sh` (baru) — semver murni saja, dipakai target Makefile
  rilis dan `scripts/git-push-and-tag.sh`; menolak string git-describe dengan
  pesan yang menjelaskan akibatnya di `formspec upgrade`.
- `cmd/formspec/semver.go` — suffix git-describe `-<n>-g<hash>` (opsional
  `-dirty`) dikenali sebagai *post-release snapshot* (`classifyPre`):
  `v0.0.8-4-gceaaf2a` > `v0.0.8` dan < `v0.0.9`, sehingga binary/tag describe
  yang sudah beredar tidak lagi memicu prompt rollback palsu.

## Kenapa

Versi rilis harus bisa dibandingkan secara semver oleh `formspec upgrade` dan
installer; string git-describe bukan versi. Auto-bump membuat `make release`
aman tanpa argumen, sementara keputusan semantik (bump minor/major) tetap
eksplisit dan tag tetap dipilih sadar (release-upload masih menuntut tag sudah
ada lokal).

## File terdampak

`Makefile`, `scripts/next-version.sh` (baru), `scripts/check-semver.sh` (baru),
`scripts/git-push-and-tag.sh`, `cmd/formspec/semver.go`,
`cmd/formspec/semver_test.go`, `docs/guides/releasing.md`.

## Referensi

- Plan: `docs_internal/plan/release-version-auto.md`
- Panduan: `docs/guides/releasing.md` §0/§2/§4
- todo: Fase 12 (12.11/12.15)
- Konteks komparator: `docs_internal/plan/formspec-upgrade-command.md` §Fase 2
