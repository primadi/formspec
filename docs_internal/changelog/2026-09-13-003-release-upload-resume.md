# 2026-09-13-003 — `release-upload` idempotent (resume upload yang terputus)

## Apa

Target `make release-upload` dipecah menjadi _create-or-resume_. Sebelumnya
target ini membuat draft release **sekaligus** meng-upload 8 asset dalam satu
perintah `gh release create`, sehingga bila upload terhenti di tengah (baru
sebagian asset naik) perintah tidak bisa diulang: guard `gh release view`
menganggap "release sudah ada" dan menolak, sementara draft sudah terlanjur
terbentuk dengan asset sebagian. Sekarang:

- Release belum ada → draft dibuat dulu (`--draft --generate-notes`, tanpa
  asset), lalu asset di-upload menyusul satu per file.
- Release **draft** sudah ada → upload di-resume: asset yang `name` + `size`-nya
  sudah cocok di GitHub dilewati, sisanya di-upload ulang dengan
  `gh release upload --clobber`.
- Release **published** → tetap ditolak ("satu tag = satu release", alasan di
  `docs/guides/releasing.md` §Rollback rilis).

Upload dilakukan satu file per file (bukan paralel) supaya progres granular dan
file yang gagal bisa diulang tanpa mengulang yang sudah naik. Di akhir, jumlah
asset di GitHub diverifikasi sama dengan jumlah file di `dist/release/`.

## Kenapa

Kejadian nyata pada rilis `v0.0.7`: `gh release create` (8 asset paralel +
`--generate-notes`) macet > 8 menit dan baru `SHA256SUMS.txt` yang ter-upload,
sementara 7 arsip (~85 MB) belum naik. Mengulang `make release-upload
VERSION=v0.0.7` gagal dengan "Release v0.0.7 sudah ada di GitHub", sehingga
pemulihannya harus manual (`gh release delete` + ulangi, atau `gh release
upload --clobber` per file). Upload satu per file terbukti jauh lebih stabil
(~3 menit untuk 85 MB).

## File terdampak

- `Makefile` — target `release-upload` dipecah create/resume + guard `gh` >= 2.18
  (`--clobber`), cek `dist/release/` tidak kosong, dan verifikasi jumlah asset.
- `docs/guides/releasing.md` — §4 menjelaskan perilaku idempotent & jalur resume;
  §0 diselaraskan (guard menolak hanya untuk release published); §"Hapus tag pada
  release yang masih DRAFT" menegaskan draft belum lengkap tidak perlu dihapus.
- `scripts/git-push-and-tag.sh` — komentar langkah 5 (cara melanjutkan bila
  upload terputus di tengah pipeline).

## Referensi

- `docs/guides/releasing.md` §4, §Rollback rilis, §Hapus tag pada release DRAFT
- `docs_internal/changelog/2026-09-10-005-fix-guard-release-upload.md` — guard
  `gh release view` sebelumnya
- `docs_internal/plan/todo.md` fase 12 (release pipeline)
