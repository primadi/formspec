# 2026-09-18-002 — Visibilitas langkah tag di `make release`

## Apa yang diubah

`make release` sukses (auto-bump `v0.0.9`, 8 artifact di `dist/release/`), lalu
`make release-upload` gagal `❌ Tag v0.0.9 belum ada lokal`. Guard-nya benar —
tag memang tidak pernah dibuat otomatis (`docs_internal/plan/release-version-auto.md`
§Keputusan) — tapi `make release` tidak menyebut langkah tag sama sekali,
sehingga kebutuhannya baru terasa *setelah* SPA build + 6 cross-compile selesai.
Temuan menyertai: `docs/guides/releasing.md` §2 menaruh tag **sebelum** build,
sedangkan keputusan plan menaruh `git tag` **sesudah** `make release`.

Perbaikan berupa visibilitas + konsistensi dokumen — guard tidak diubah:

- `scripts/release-tag-status.sh` (baru) — cetak status tag untuk versi yang akan
  dibangun: belum ada (disertai perintah `git tag` + `git push`), tag ada tapi
  bukan di commit yang dibangun (peringatan artifact↔tag mismatch), atau OK.
  Info saja: selalu exit 0, guard yang menggagalkan tetap di `release-upload`.
- `Makefile` — target `check-release-tag` direprequisite **sebelum** `build-spa`
  di target `release` (status tag terbaca sebelum build mahal) dan dipanggil lagi
  di ringkasan akhir.
- `docs/guides/releasing.md` §2 — urutan tag ↔ build direkonsiliasi: `make release`
  tidak membaca tag, dua urutan sah, syaratnya tag menunjuk commit yang dibangun.

## Kenapa

Gate tag sering disalahartikan sebagai bug ("`make release` sukses, kenapa upload
error?"). Yang sebenarnya hilang adalah **urutan langkah yang terbaca**: kegagalan
muncul di ujung pipeline setelah pekerjaan mahal selesai, dan guide vs plan saling
berkontradiksi soal kapan tag dibuat. Opsi yang ditolak: `release` auto-create tag
(menyimpang dari keputusan plan: auto-bump versi ≠ otorisasi publish) dan
`release-upload` push tag sendiri (mutasi publik diam-diam di dalam target upload).

## File terdampak

`Makefile`, `scripts/release-tag-status.sh` (baru), `docs/guides/releasing.md`,
`docs_internal/plan/release-version-auto.md`, `docs_internal/plan/todo.md`.

## Referensi

- Plan: `docs_internal/plan/release-version-auto.md` §Lanjutan 2026-09-18
- Panduan: `docs/guides/releasing.md` §2/§4
- Changelog terkait: `2026-09-17-002-versi-rilis-otomatis-anti-git-describe.md`
- Konteks: rilis `v0.0.9` (artifact dibangun dari `71bf2fa`)
