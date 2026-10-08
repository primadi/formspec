# Brotli q11 pre-compress post-build (8.4.4)

**Status:** In Progress · **Tanggal:** 2026-10-08 · **Effort:** medium
**Referensi:** `docs_internal/plan/static-asset-caching.md` §Phase C/D,
`docs/runtimes/05-engine-api-layer.md` §2.2
**Changelog:** `docs_internal/changelog/2026-10-08-004-brotli-q11-precompress.md`
**Todo:** Fase 8 §8.4.4

## Keputusan (berdasarkan pengukuran, bukan dugaan)

Diukur pada 7 aset nyata `renderers/react-shadcn/dist/assets` (1.618.371 byte raw):

| Codec                     | Wire        | vs gzip-6  | Encode | **Decode**             |
| ------------------------- | ----------- | ---------- | ------ | ---------------------- |
| gzip-6 (runtime sekarang) | 442.244     | —          | 43 ms  | 5,64 ms (287 MB/s)     |
| zstd SpeedDefault         | 461.045     | **+4,3%**  | 42 ms  | 6,01 ms                |
| zstd SpeedBestCompression | 415.882     | −6,0%      | 106 ms | —                      |
| **brotli q5**             | 410.276     | −7,2%      | 67 ms  | 6,20 ms (261 MB/s)     |
| **brotli q11**            | **369.093** | **−16,5%** | 2,79 s | **6,04 ms (268 MB/s)** |

Keputusan yang diambil dari tabel itu:

- **q11, bukan q5.** Encode 2,79 s dibayar **sekali saat build**; hemat 73 KB
  lebih banyak (−16,5% vs −7,2%) mengalahkan itu untuk bundle yang diunduh
  setiap pengguna.
- **q11 tidak membebani klien.** Decode q11 (6,04 ms) setara/pada praktiknya
  sama dengan q5 (6,20 ms) dan gzip-6 (5,64 ms) → `q` adalah knob _encode_;
  decoder tidak mengulang pencarian match. Angka ini dari decoder pure-Go,
  jadi browser (decoder C) hanya lebih cepat — kesimpulannya konservatif.
- **`LGWin` 22 (4 MiB, default brotli).** Decoder mengalokasikan window per
  stream; 4 MiB sudah lebih besar dari aset terbesar (630 KB), jadi window
  lebih besar tidak membeli apa pun. **Bukan 24** — itu 16 MiB/stream.
- **zstd dibatalkan** (didominasi brotli; default-nya malah lebih besar dari
  gzip).
- **Gzip runtime tetap ada** sebagai fallback universal (klien tanpa `br`,
  atau `--web-dir` yang belum di-compress).

## Desain

### 1. Predikat kompresi: satu sumber, bukan dua

Kalau langkah `.br` ditulis di Node/bash, aturan "tipe kompresibel, ≥ ambang
ukuran, hanya bila lebih kecil" menjadi **dua implementasi** yang bisa drift —
kelas bug yang sudah dikenal di repo ini (lexer FormSpecExpr, todo 5.11.7).
Karena itu kompresi dilakukan oleh **kode Go yang sama** yang menyajikan:

- `minCompressSize` (dulu `gzipMinSize`) — ambang bersama gzip dan brotli.
- `isCompressible(name)` — daftar tipe yang sama.
- `auditCompressible` — keputusan "layak" diekstrak agar `CompressStaticTree`
  dan jalur HTTP tidak bisa berpisah.

Diekspos sebagai `api.CompressStaticTree(dir, opts)`, dipanggil CLI
`formspec spa compress --dir <dist>`.

### 2. Kapan dijalankan: **sekali, setelah `build-spa`**

`make build-spa` = `npm run build` **+** langkah kompresi. Urutannya
load-bearing: ada **tiga** konsumen yang meng-copy `dist/`
(`build-formspec` → `cmd/formspec/dist`, `build-registry` →
`cmd/formspec-registry/web/dist`, `release` → tarball `spa-<versi>.tar.gz`).
Kompresi harus selesai **sebelum** copy pertama, kalau tidak salah satu build
kehilangan `.br` **tanpa error** (hanya kehilangan 7–16% byte).

`web-build` (untuk `--web-dir` dev) juga dikompresi supaya dev punya paritas.

### 3. Penyajian: `.br` diutamakan, dengan fallback berlapis

Preferensi per request: **br → gzip → identity**, dengan gerbang yang sama
(Range → identity, ambang ukuran, tipe kompresibel, `Vary: Accept-Encoding`).

- ETag `.br` bersufiks `-br` (representasi berbeda → validator berbeda,
  pola yang sudah dipakai `-gzip`).
- Body `.br` dimemoikan di LRU yang sama (kunci `…|br`), dimuat saat `Warm`
  (hanya baca berkas — tidak ada kompresi saat boot).
- **Berkas `.br`/`.gz` tidak boleh disajikan sebagai aset** (404): ia artefak
  internal, bukan berkas yang bisa diunduh sebagai JS.

### 4. Kebersihan

`CompressStaticTree` menghapus `.br` basi bila sumbernya tidak lagi layak
(terlalu kecil / tidak kompresibel / tidak menyusut). Vite sudah mengosongkan
`dist/` tiap build, jadi ini sabuk pengaman, bukan mekanisme utama.

## Verifikasi

- `go test ./internal/api/` (+ `./...`), lint, `gofmt`.
- Test: `.br` dibuat hanya untuk berkas layak; `.br` basi dihapus; `.br` tidak
  disajikan (404); br diutamakan atas gzip; ETag br ≠ gzip ≠ identity; Range →
  identity; fallback gzip tetap bekerja tanpa `.br`.
- Kalibrasi: suntikkan regresi (preferensi br dilewati / predikat layak
  dilonggarkan) → test harus gagal.
- Live: `make web-build` lalu `curl -H 'Accept-Encoding: br'` pada
  `/assets/*.js` → `Content-Encoding: br` + `Content-Length` = ukuran `.br`;
  `Accept-Encoding: gzip` → tetap gzip.
