# 2026-09-10-007 — Fail-fast loop `make release` + deteksi artifact linux korup

## Apa

Temuan: arsip `dist/release/formspec-linux-{amd64,arm64}.tar.gz` sebesar 45
bytes — tar berisi binary 0 byte (gzip dari tar zero-block), sedangkan target
darwin/windows normal (~19 MB). Penyebab: loop build di target `make release`
tidak fail-fast — recipe recipe make dijalankan tanpa `set -e`, jadi jika
`go build` salah satu target gagal, `tar`/`zip` tetap membungkus file rusak dan
loop lanjut ke target berikutnya.

Perbaikan Makefile:

- `set -e` di awal loop build → kegagalan `go build`/`tar`/`zip` menghentikan
  `make release` seketika.
- Sanity check `test -s` (binary tidak 0 byte) setelah setiap build, sebelum
  packaging.

`docs/guides/releasing.md` verifikasi step 3 ditambah cek `ls -lh` (arsip < 1
MB = artifact korup) dan catatan bahwa checksum "OK" tidak membuktikan
validitas artifact.

## Kenapa

Checksum `SHA256SUMS.txt` dihitung dari file yang dihasilkan target sendiri,
jadi artifact korup tetap lolos `shasum -c`. Tanpa guard ini artifact rusak
bisa ter-upload ke GitHub Releases dan dibagikan ke user.

## File terdampak

- `Makefile` — `set -e` + sanity check binary dalam loop `release`.
- `docs/guides/releasing.md` — verifikasi step 3 diperkuat.

## Referensi

- `docs/guides/releasing.md` §3 (verifikasi sebelum upload)
- `scripts/git-push-and-tag.sh` (changelog 2026-09-10-006) — otomatis memanggil `make release`, ikut diuntungkan guard ini.
