# 2026-09-10-011 — git-push-and-tag.sh: preflight generated artifacts

## Apa

Menambahkan langkah 0.5 di `scripts/git-push-and-tag.sh` (setelah sync versi,
sebelum test): regenerate `make generate-schema` + `make generate-kind-docs`
lalu fail-fast bila `schemas/` atau `docs/kind/` berubah (drift = artefak
generated stale). Hasil regenerate dibiarkan di tree agar bisa di-commit,
lalu script dijalankan ulang.

## Kenapa

`schemas/` (JSON Schema untuk YAML editor) dan `docs/kind/` (kind reference
docs) di-generate dari `pkg/spec` dan di-commit. Release script sebelumnya
tidak memverifikasi kefresh-annya — kalau `pkg/spec` diubah tanpa jalankan
generator, release membawa schema/docs basi. `publish-schemas` (deploy
schemas.formspec.dev) dan docs-site sengaja tidak diikat ke release script —
keduanya pipeline git-based terpisah dengan kadensi sendiri.

## File terkena dampak

- `scripts/git-push-and-tag.sh` — langkah 0.5 `preflight_generated`
- `docs/guides/releasing.md` — dokumentasi preflight + section baru
  "Yang sengaja di luar script"

## Verifikasi

- `bash -n` OK
- `make generate-schema && make generate-kind-docs` di tree saat ini:
  GEN_OK, tidak ada drift

## Referensi

- `docs_internal/changelog/2026-09-10-006-script-git-push-and-tag.md`
- `docs_internal/changelog/2026-09-10-008-git-push-and-tag-sync-version-refs.md`
