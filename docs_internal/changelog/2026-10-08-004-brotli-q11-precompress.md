# 2026-10-08-004 — Brotli q11 pre-compress post-build (8.4.4)

**Plan:** `docs_internal/plan/brotli-precompress-build.md`
**Todo:** Fase 8 §8.4.4 ✅ ditutup

Item 8.4.4 ditutup: aset renderer kini disajikan dengan **brotli** kalau ada,
gzip sebagai fallback. Sidecar `.br` dibuat oleh `formspec spa compress`,
dipanggil otomatis dari `make build-spa` dan `make web-build` — **sekali setelah
`npm run build`, sebelum siapa pun meng-copy `dist/`**.

## Angka (yang menentukan pilihan q11)

Diukur pada 7 aset nyata `dist/assets` (1.618.371 byte raw):

| Codec                     | Wire        | vs gzip-6  | Encode | **Decode**  |
| ------------------------- | ----------- | ---------- | ------ | ----------- |
| gzip-6 (runtime)          | 442.244     | —          | 43 ms  | 5,64 ms     |
| zstd SpeedDefault         | 461.045     | **+4,3%**  | 42 ms  | 6,01 ms     |
| zstd SpeedBestCompression | 415.882     | −6,0%      | 106 ms | —           |
| brotli q5                 | 410.276     | −7,2%      | 67 ms  | 6,20 ms     |
| **brotli q11**            | **369.093** | **−16,5%** | 2,79 s | **6,04 ms** |

**q11, bukan q5:** encode 2,79 s dibayar sekali saat build; −16,5% vs −7,2%
mengalahkan itu untuk bundle yang diunduh setiap pengguna. **q11 tidak membebani
klien** — decode-nya setara q5 (6,04 vs 6,20 ms) karena `q` adalah knob _encode_;
decoder tidak mengulang pencarian match. Diukur dengan decoder pure-Go, jadi
browser (decoder C) hanya lebih cepat → kesimpulannya konservatif. **zstd
dibatalkan**: didominasi brotli di setiap titik, dan default-nya lebih besar
dari gzip. Ini koreksi atas usulan zstd sebelumnya, yang tidak terukur.

**Terukur pada bundle kafe sesudah implementasi:** 1.839.789 byte raw → gzip
608.324 → **brotli 438.994 (−27,8% vs gzip, −76,1% vs raw)**. Per berkas,
`vendor-icons-*.js`: 630.784 → 185.968 (gzip) → **120.539 (brotli)**.

## Desain

- **Satu sumber predikat.** `isCompressible` + `minCompressSize` (dulu
  `gzipMinSize` — nama lama menyesatkan begitu dipakai dua codec) + `compressEligible`
  hidup di `internal/api`, dipakai baik oleh langkah build maupun jalur HTTP.
  Salinan shell/Node akan jadi sumber kedua yang bisa drift, dan drift di sini
  **senyap** (hanya kehilangan kompresi).
- **Urutan Makefile load-bearing.** Ada tiga konsumen yang meng-copy `dist/`
  (`build-formspec`, `build-registry`, `release`). Kompresi setelah salah satu
  copy = build itu kehilangan `.br` **tanpa error**.
- **Preferensi per request: br → gzip → identity**, dengan gerbang sama
  (Range → identity, ambang ukuran, tipe kompresibel, `Vary: Accept-Encoding`).
  ETag ber-sufiks `-br`/`-gzip` sehingga cache tidak pernah tertukar representasi.
- **Sidecar tidak dapat diunduh** (404): ia dipilih lewat `Accept-Encoding`,
  bukan lewat nama — kalau tidak, `/assets/app.js.br` menyajikan stream brotli
  ber-Content-Type octet-stream.
- **Warm jadi lebih murah, bukan lebih mahal.** Bila `.br` ada, warm hanya
  **membaca** berkas alih-alih mengompres gzip: terukur boot 90 ms → **19 ms**.

## Bug yang ketemu saat verifikasi

1. **`Elapsed` selalu 0.** `defer func(){ stats.Elapsed = … }()` tidak mencapai
   pemanggil karena `return stats, nil` menyalin ke slot hasil **tak bernama**
   sebelum deferred berjalan. Diperbaiki dengan named results; test
   `stats.Elapsed <= 0` yang mengungkapnya.
2. **Sidecar ditulis mode `0600`.** `os.CreateTemp` membuat 0600 dan `rename`
   mempertahankannya → `dist/` yang berfungsi untuk user yang mem-build dan
   403 untuk siapa pun selainnya (kontainer, user berbeda, tree yang disalin).
   Kini sidecar mewarisi bit permission **sumber**; dipin
   `TestBrotliCompressTree_SidecarInheritsSourceMode`.

## Guard yang dibuktikan menggigit

Regresi disuntikkan lalu **dipulihkan dari salinan `/tmp`** (bukan
`git checkout` — jebakan yang sudah tercatat):

- preferensi br dilumpuhkan (`if false && …`) →
  `TestSpaAssets_BrotliPreferredOverGzip` + `TestSpaAssets_BrotliETagIsDistinct` gagal;
- gerbang "harus menyusut" dilumpuhkan → awalnya **tidak ada test yang gagal**,
  karena fixture `.bin` sudah tersaring oleh ekstensi dan tidak pernah mencapai
  gerbang itu. Ditambahkan `TestBrotliCompressTree_RefusesNonShrinkingFile`
  (byte acak bernama `.js`); dengan regresi yang sama kini **gagal**, hijau
  setelah dipulihkan. Kelas temuan yang sama dengan guard prefiks `assets/`
  sebelumnya: injeksi yang membongkar celah test, bukan sekadar mengonfirmasi.

## Berkas

`internal/api/spaassets_compress.go` (baru) · `internal/api/spaassets.go`
(preferensi br, memo probe sidecar, `acceptsEncoding`, `isSidecar`) ·
`internal/api/spaassets_brotli_test.go` (baru) · `cmd/formspec/spa_compress.go`
(baru) + `cmd/formspec/spa.go` (dispatch) · `Makefile` · `go.mod`/`go.sum`
(+`andybalholm/brotli`) · `docs/runtimes/05-engine-api-layer.md` §2.2 ·
`docs/cli-tools/02-formspec-cli.md`.

## Catatan dependency

Menambah modul baru di container ini butuh `GOSUMDB=off` **satu kali**, karena
`/go/pkg/sumdb` tidak bisa dibuat dan verifikasi checksum gagal untuk modul yang
belum ada di `go.sum`. Perintah yang dipakai: `GOFLAGS=-mod=mod GOSUMDB=off go
mod tidy`. Hasilnya `go.sum` memuat hash nyata dari proxy, jadi build berikutnya
terverifikasi normal. (Klaim awal bahwa `GOSUMDB=off` adalah syarat brotli
adalah **salah** — penyebabnya lingkungan, bukan codec.)
