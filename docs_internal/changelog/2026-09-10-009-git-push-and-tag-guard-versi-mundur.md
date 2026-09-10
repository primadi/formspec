# 2026-09-10-009 — git-push-and-tag.sh: guard versi mundur

## Apa

Menambahkan guard di `scripts/git-push-and-tag.sh`: `VERSION` harus lebih
tinggi dari tag tertinggi yang sudah ada (dibandingkan via `sort -V` terhadap
`git tag --sort=-v:refname | head -1`). Versi mundur ditolak sebelum tag dibuat.

## Kenapa

Guard lama hanya mengecek keberadaan tag, bukan urutan. `v0.0.10` setelah
`v0.1.0` akan lolos semua validasi dan release tetap terbuat. Dampaknya:

1. `sync_version_refs` menulis mundur versi contoh di site/docs
   (`v0.1.0` → `v0.0.10`).
2. GitHub menentukan release "latest" berdasarkan yang terakhir di-publish
   (bukan semver tertinggi), sehingga `install.sh` — yang resolve versi via
   GitHub API — akan men-downgrade user ke `v0.0.10`.

## File terkena dampak

- `scripts/git-push-and-tag.sh` — guard anti-downgrade setelah cek tag exists
- `docs/guides/releasing.md` — dokumentasi guard baru

## Referensi

- Changelog terkait: `2026-09-10-006-script-git-push-and-tag.md`,
  `2026-09-10-008-git-push-and-tag-sync-version-refs.md`
