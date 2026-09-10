# 2026-09-10-005 — Perbaiki guard `release-upload` di Makefile

## Apa

Guard `make release-upload` sebelumnya memblokir bila tag `$(VERSION)` sudah
ada di remote (`git ls-remote --tags origin`). Ini salah karena prosedur di
`docs/guides/releasing.md` menganjurkan push tag di langkah 2 **sebelum**
upload artifact di langkah 4 — sehingga guard selalu gagal saat dijalankan
sesuai prosedur. Guard diganti menjadi mengecek apakah **release** dengan tag
tersebut sudah ada di GitHub (`gh release view`), yang merupakan makna sebenarnya
dari "satu tag = satu release".

## Kenapa

Saat release pertama (`v0.0.1`), tag sudah di-push ke remote tapi release belum
dibuat. `make release-upload VERSION=v0.0.1` diblokir padahal step 4 masih
harus dijalankan.

## File terdampak

- `Makefile` — ganti cek `git ls-remote` menjadi `gh release view`; pindahkan
  pengecekan keberadaan `gh` sebelum guard release.
- `docs/guides/releasing.md` — tambahkan catatan perilaku guard setelah langkah 4.
- `.gitignore` — tambahkan `dist/` (artifact `make release` di `dist/release`
  di-upload langsung dari disk oleh `gh`, tidak pernah di-commit).

## Referensi

- `docs/guides/releasing.md` (prosedur release)
- `docs_internal/plan/install-page-plan.md`
