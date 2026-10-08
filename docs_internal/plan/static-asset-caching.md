# Static Asset Caching & Kompresi (SPA renderer)

**Status:** In Progress · **Tanggal:** 2026-10-07 · **Effort:** medium
**Referensi:** `docs/runtimes/05-engine-api-layer.md` §route (static SPA),
`docs/cli-tools/01-formspec-dev.md` §SPA serving, `docs/spec/platform/09-observability.md` §PII
**Changelog:** `docs_internal/changelog/2026-10-07-007-static-asset-cache-kompresi.md`
**Todo:** Fase 8 §8.4

## Masalah

Aset renderer di-serve tanpa pemanfaatan cache HTTP sama sekali:

- `internal/api/router.go` `spaAssetHandler` → `serveFileFS`: hanya `Content-Type`, lalu
  `w.Write(data)`. Tidak ada `Cache-Control`, `ETag`, `Content-Length` (body besar → chunked).
- `spaHandlerFS` memakai `http.FileServer(http.FS(spaFS))`; file `embed.FS` bermodtime nol
  sehingga tanpa validator — setiap muat halaman mengunduh ulang seluruh chunk.
- Mode `--web-dir`: `http.ServeFile` memberi `Last-Modified` + 304, tapi tanpa freshness
  eksplisit; `index.html` bisa basi setelah `npm run build`.
- Tidak ada kompresi: Go tidak mengompres otomatis dan tidak ada middleware gzip.

Padahal build Vite sudah content-hashed (`assets/index-DR5hGT_C.js`,
`vendor-react-b3kFLWLo.js`) sehingga ideal untuk `immutable, max-age=1y`.

## Keputusan (dikunci bersama pemilik proyek, 2026-10-07)

| #   | Keputusan                                                                        | Alasan                                                                                 |
| --- | -------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| D1  | Default `no-cache`; `immutable` hanya bila nama file fingerprint-match           | Predicate salah → kehilangan cache (aman), bukan menyajikan konten basi                |
| D2  | `index.html`/shell SPA = `no-cache` + revalidate via ETag                        | Deploy berulang tidak tertunda 1 menit                                                 |
| D3  | Paritas embed.FS **dan** `--web-dir`                                             | Satu implementasi, dua sumber                                                          |
| D4  | `gzipMinSize = 1024` (1 KiB) — di bawah ambang → identity                        | nginx `gzip_min_length` sejalan; chunk kecil Vite tidak menanggung overhead gzip       |
| D5  | Gate kedua: hanya kirim gzip bila `len(gz) < len(raw)`                           | Respons tidak pernah membengkak                                                        |
| D6  | Warm saat start sebagai **goroutine** (non-blocking)                             | Boot tidak menunggu; request awal tetap benar lewat jalur lazy                         |
| D7  | Kandidat warm: **embed → semua aset kompresibel**; **dir → hanya fingerprinted** | Embed beku selamanya (warm tak pernah sia-sia); dir berubah tiap rebuild               |
| D8  | Memo gzip = **LRU byte-budget 32 MiB**                                           | Entri fingerprint lama tergeser sendiri; memori tidak tumbuh tanpa batas               |
| D9  | `kind` (embed/dir) parameter eksplisit, bukan heuristik tipe                     | `fstest.MapFS` di test bersemantik immutable; heuristik = perubahan perilaku diam-diam |

## Matriks perilaku

| Path                                           | Cache-Control                         | Validator           | Kompresi                     |
| ---------------------------------------------- | ------------------------------------- | ------------------- | ---------------------------- |
| `/assets/*-<hash8>.<ext>` (fingerprinted)      | `public, max-age=31536000, immutable` | — (immutable cukup) | gzip bila ≥ 1 KiB & menyusut |
| `/assets/*` non-fingerprint                    | `no-cache`                            | ETag byte-hash      | gzip bila ≥ 1 KiB & menyusut |
| shell SPA (`index.html` fallback)              | `no-cache`                            | ETag byte-hash      | gzip bila ≥ 1 KiB & menyusut |
| `/favicon.svg`, `/icons.svg`                   | `public, max-age=604800`              | ETag                | gzip bila ≥ 1 KiB & menyusut |
| `/manifest.json`                               | `public, max-age=3600`                | ETag                | gzip bila ≥ 1 KiB & menyusut |
| tipe padat (`woff2`/`png`/`jpeg`/`webp`/`.gz`) | sesuai kelas path                     | ETag                | **tidak pernah**             |

Gate kompresi (semua harus benar, jika tidak → identity): klien menerima gzip dengan q>0,
`len(raw) >= gzipMinSize`, tipe kompresibel, bukan request `Range`, hasil lebih kecil.

## Implementasi

### Phase A — Unifikasi jalur (`internal/api/spaassets.go`, baru)

- `type spaAssets struct { fsys fs.FS; source spaSource; ... }` +
  `newSPAAssets(fsys fs.FS, source spaSource)` / `newSPAAssetsFromDir(dir string)`.
- `ShellHandler()` (fallback index.html untuk client-side route) dan `AssetHandler()`
  (root-level `/assets/*`, `/favicon.svg`, `/icons.svg`, `/manifest.json`).
- Traversal aman: normalisasi wildcard chi → path `fs.ValidPath` (tolak `..`/absolut),
  direktori → fallback shell.
- Hapus `spaHandler`, `spaHandlerFS`, `spaAssetHandler`, `spaAssetHandlerDir`;
  `mimeTypeByExtension` dipertahankan (dipindah/dipakai dari `spaassets.go`).

### Phase B — Cache header + ETag/304

- `staticCacheControl(name, fingerprinted)` + `isFingerprinted(name)` (regex
  `-[A-Za-z0-9_-]{8}\.[A-Za-z0-9]+$`, hanya untuk path di bawah `assets/`).
- ETag = SHA-256 16 byte pertama, heksadesimal; entri gzip memakai sufiks `-gzip`
  (representasi berbeda = ETag berbeda, sesuai RFC 7232). Dimemo per `(name,size,modtime)`.
- Respons dikirim lewat `http.ServeContent` → stdlib menangani `If-None-Match`/
  `If-Modified-Since` → 304, `Range` → 206, dan `Content-Length`.

### Phase C — gzip + memo LRU + warm

- `compress/gzip` stdlib; hasil dimemo di `byteLRU` (budget 32 MiB) dengan key
  `(name, size, modtime)` — key mismatch otomatis berarti "kompres ulang", jadi warm
  tidak pernah bisa menyajikan bytes basi.
- `Vary: Accept-Encoding` diset untuk semua tipe yang _bisa_ dikompres (termasuk saat
  respons identity karena di bawah min-size).
- `Warm()`: walk FS, pilih kandidat (D7), tolak `> warmMaxFile` (2 MiB) dan
  `total > warmMaxTotal` (64 MiB) → skip tanpa error; kompres paralel
  (`GOMAXPROCS` worker); idempotent; satu log ringkas (jumlah + bytes + durasi, tanpa
  nama file bisnis — patuh 8.2.2).
- Dipanggil sebagai goroutine dari `resource/formspec.go` setelah
  `SetWebDir`/`SetWebFS` dan dari `ReloadSpec`.

### Phase D — Test (`internal/api/spaassets_test.go`, baru)

Table test yang sama dijalankan terhadap `fstest.MapFS` (embed) dan `t.TempDir()` (dir)
sebagai bukti paritas: Cache-Control per kelas path, 304 via `If-None-Match`, gzip +
`Vary` + dekompresi identik, `gzip;q=0` → identity, boundary min-size (1023 B identity /
1024 B gzip), gate menyusut, `woff2` tak pernah dikompres, `Range` → 206 identity,
tulis ulang file di mode dir → ETag/bytes baru, `Content-Length` selalu ada.

Warm: kandidat embed vs dir berbeda, request pertama tidak mengompres ulang, idempotensi,
`warmMaxTotal` skip, LRU eviction ≤ budget dan tetap benar setelah eviksi.

### Phase E — Docs

- `docs/runtimes/05-engine-api-layer.md` — tabel route + kolom caching/kompresi.
- `docs/cli-tools/01-formspec-dev.md` — catatan perilaku SPA serving.
- `docs_internal/plan/todo.md` Fase 8 §8.4 (8.4.1 cache+ETag, 8.4.2 gzip+min-size,
  8.4.3 pre-compress saat start).

## Dependensi antar task

A → (B ∥ C) → D → E. Tidak ada dependensi pada paket lain; tidak menyentuh frontend.

## File terdampak

- `internal/api/spaassets.go` (baru), `internal/api/spaassets_test.go` (baru)
- `internal/api/router.go` (switch `webFS`/`webDir`, route asset, hapus 4 handler lama)
- `resource/formspec.go` (wire `Warm` di boot + reload)
- `internal/api/router_spa_test.go`, `app_workspace_scope_test.go` (verifikasi tidak rusak)

## Verifikasi

- `go build ./...`, `go test ./internal/api/ ./resource/`, `go test ./...`
- `gofmt -l ./internal/api`; `golangci-lint run ./internal/api/...`
- Manual: `curl -I` pada `/assets/index-*.js` (immutable + Content-Length),
  `/{ws}/_admin` (no-cache + ETag), `If-None-Match` → 304, `Accept-Encoding: gzip` →
  `Content-Encoding: gzip`; ulangi dengan `--web-dir renderers/react-shadcn/dist`.

## Risiko

| Risiko                                    | Mitigasi                                                                      |
| ----------------------------------------- | ----------------------------------------------------------------------------- |
| Predicate fingerprint salah               | Default `no-cache` → hanya kehilangan cache; guard test walk `dist/assets/**` |
| ETag basi di mode dir setelah rebuild     | Key `(name,size,modtime)`; test tulis-ulang file                              |
| Memori naik karena gzip warm              | LRU byte-budget + `warmMaxTotal`; test eviction                               |
| `os.DirFS` menggantikan `http.FileServer` | Listing direktori tidak dipakai; `Range` tetap via `http.ServeContent`        |

## Out of scope

Brotli (butuh encoder non-stdlib / pre-compress di build), cache HTTP untuk API
(`/api/v1`, `/_ui/*` sudah punya ETag `_meta`), service worker/precache,
cache-aside `formspec-registry` (13.5.7 ⏸️), Range untuk respons terkompresi.
