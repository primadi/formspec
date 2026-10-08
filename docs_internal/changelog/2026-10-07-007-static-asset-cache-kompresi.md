# 2026-10-07-007 — Caching + kompresi aset renderer (SPA static delivery)

**Plan:** `docs_internal/plan/static-asset-caching.md`
**Todo:** Fase 8 §8.4 (8.4.1, 8.4.2, 8.4.3)

Aset renderer sebelumnya disajikan tanpa pemanfaatan cache HTTP sama sekali:
`spaAssetHandler`/`serveFileFS` hanya menulis `Content-Type` lalu
`w.Write(data)` — tanpa `Cache-Control`, `ETag`, maupun `Content-Length`, dan
tanpa kompresi. Akibatnya setiap muat halaman mengunduh ulang seluruh bundle,
termasuk chunk ber-hash yang isinya tidak mungkin berubah.

**Yang dikerjakan.** Empat handler lama (`spaHandler`, `spaHandlerFS`,
`spaAssetHandler`, `spaAssetHandlerDir`) digantikan satu implementasi,
`spaAssets` (`internal/api/spaassets.go`), yang melayani `embed.FS` dan
`--web-dir` dari satu jalur. Sumber FS menjadi parameter eksplisit, bukan
ditebak dari tipe `fs.FS` — sebab perilaku pre-compress berbeda. Setiap berkas
kini membawa `Cache-Control` per kelasnya (fingerprinted → `immutable`
setahun; sisanya `no-cache` + `ETag` SHA-256), respons dikirim lewat
`http.ServeContent` (304/Range/`Content-Length` ditangani stdlib), dan gzip
dikenakan hanya bila seluruh gerbang lolos — **termasuk `gzipMinSize = 1024`**
dan syarat "hasil lebih kecil dari aslinya". Saat start, aset immutable
di-precompress di goroutine latar (memo LRU ber-byte-budget), sehingga request
pertama tidak ikut menanggung biaya kompresi.

**Terukur di dev server kafe** (`--web-dir`): warm memproses **31 berkas,
1.839.789 → 608.324 byte dalam 98 ms**; `vendor-icons-*.js` **630.784 →
185.968 byte (-70%)**; shell `/_admin` menjawab `no-cache` + ETag dan
**304** pada `If-None-Match`; `woff2` tetap identity tanpa `Vary`; dan
`/assets/nope.js` menjawab **404 JSON**, bukan shell HTML.

**Guard yang dibuktikan menggigit** (regresi disuntikkan lalu dipulihkan):
menghapus lantai `gzipMinSize` → `TestSpaAssets_GzipMinSizeBoundary/1023_bytes`
gagal; melumpuhkan prefiks `assets/` pada predikat fingerprint →
`TestSpaAssets_FingerprintPredicate` + `TestSpaAssets_HashLikeRootFileIsNotImmutable`
gagal (kasus terakhir **ditambahkan** justru karena injeksi pertama membongkar
celah test: nama root ber-hash-palsu seperti `report-20240101.js` dulu akan
mendapat `immutable` — bug staleness setahun yang kini dipin).

**Berkas:** `internal/api/spaassets.go` (baru), `internal/api/spaassets_test.go`
(baru), `internal/api/router.go` (handler lama dicabut, wiring `spaAssets` +
`WarmStaticAssets`), `resource/formspec.go` (warm di boot + reload),
`docs/runtimes/05-engine-api-layer.md` §2.2, `docs/cli-tools/01-formspec-dev.md` §6.
