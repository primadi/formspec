# Item cache terbuka: knob TTL unduhan + batas cache API

**Status:** ✅ Selesai (7.17.11 + 8.4.5) · 8.4.4/7.17.10 tetap ⏸️ · **Tanggal:** 2026-10-08 · **Effort:** small
**Referensi:** `docs/spec/backend/05-field-types.md` §Storage Spec,
`docs/runtimes/05-engine-api-layer.md` §2.2
**Changelog:** `docs_internal/changelog/2026-10-08-003-knob-ttl-cache-unduhan.md`
**Todo:** Fase 7 §7.17.11 (tutup), Fase 8 §8.4.5 (tutup), §8.4.4 + §7.17.10 (tetap ⏸️, temuan diperbarui)

## Ruang lingkup

Empat item cache terbuka; dua bisa ditutup sekarang, dua tetap terblokir dan
temuannya diperbarui supaya penundaan itu bisa diperiksa (bukan prosa).

### 7.17.11 — knob `max-age` unduhan (dikerjakan)

`downloadCacheTTL = 5 * time.Minute` adalah konstanta tunggal tanpa knob.
Pola yang sudah ada di repo dipakai apa adanya, tanpa mekanisme baru:

| Lapis        | Precedent                                                    | Yang ditambahkan                               |
| ------------ | ------------------------------------------------------------ | ---------------------------------------------- |
| Per-field    | `storage.max_download_mb` (7.17.7), `storage.signed_url_ttl` | `storage.download_cache_ttl` (durasi Go)       |
| Global (env) | `FORMSPEC_DOWNLOAD_MAX_MB`                                   | `FORMSPEC_DOWNLOAD_CACHE_TTL` (default `5m`)   |
| Resolusi     | `effectiveDownloadLimitMB`                                   | `effectiveDownloadCacheTTL` (per-field menang) |

**`download_cache_ttl: 0s` = escape hatch** — dijawab `no-cache` (tetap dengan
ETag, jadi revalidasi tetap murah). Ini berguna untuk field yang isinya sering
berubah dan pembacanya tidak boleh melihat versi lama sama sekali.

Validasi (fail-loud) ditambahkan di `ValidateStorageSpec` untuk
`download_cache_ttl` dan `signed_url_ttl` — yang kedua **belum divalidasi sama
sekali** hari ini, jadi durasi salah ketik di sana diam-diam diabaikan saat
runtime (fallback ke default). Sekelas dengan mismatch `allowed_types` (gap #4b)
yang sudah ditutup.

### 8.4.5 — batas cache API (ditutup sebagai keputusan)

Tidak diimplementasikan, dan itu keputusannya: `GET /api/v1/...` dan
`/_ui/entity/...` mengembalikan JSON yang bergantung pada identity/permission.
Tanpa `Vary` yang benar atau kunci cache sadar-auth, cache HTTP di situ adalah
kelas bug kebocoran lintas-pengguna. Yang sudah ada dan cukup: ETag pada
`/_meta/ui` (5.12.1). Dicatat di docs sebagai batas eksplisit, bukan backlog.

### 8.4.4 — brotli (tetap ⏸️, temuan diperbarui)

Temuan baru yang mengubah gambaran:

- **`klauspost/compress` TIDAK punya encoder brotli.** Isi modul:
  `flate, gzip, huff0, s2, snappy, zip, zlib, zstd, gzhttp, dict, fse` —
  tidak ada `brotli`. Jadi "pakai klauspost" (pilihan awal) **tidak bisa**
  untuk brotli.
- Encoder brotli murni-Go = `github.com/andybalholm/brotli` (dipakai
  `klauspost/compress` hanya sebagai dependency **test**). Diuji di container:
  bisa diunduh (`v1.2.6`) tapi butuh `GOSUMDB=off` karena `/go/pkg/mod/sumdb`
  tidak ada — artinya penambahan dependency ini butuh keputusan infra
  supply-chain, bukan sekadar `go get`.
- **Tidak ada `brotli` CLI** di container → pre-compress saat build pun butuh
  tool Go (bukan satu baris Makefile).

Alternatif yang perlu keputusan pemilik: **`zstd`** — encoder murni-Go **sudah**
ada di `klauspost/compress` (dependency transitif, tinggal promosi ke direct),
dan `Content-Encoding: zstd` sudah didukung browser arus utama. Rasio mendekati
brotli, nol dependency baru. Karena pilihan codec adalah keputusan produk (ia
menentukan `Content-Encoding` yang dikirim ke setiap klien), item ini **tidak
ditutup tanpa jawaban**.

### 7.17.10 — `cdn: true` (tetap ⏸️, temuan diperbarui)

`grep -rn 'cdn: true' examples/ verticals/` → **nol manifest** yang mengaktifkan
passthrough CDN. Jadi ini bukan fitur yang setengah jalan dengan pemakai yang
menunggu; ia deklarasi spec tanpa pemakai. Menutupnya butuh keputusan desain
(host CDN, path style, siapa yang meng-invalidate) yang tidak bisa disimpulkan
dari kode. Temuan ini dicatat di item agar pembaca tahu bahwa penundaannya
disengaja, bukan lupa.

## Verifikasi

- `go test ./pkg/spec/ ./internal/api/` (+ `go test ./...`)
- `make generate-schema` + `make generate-kind-docs` (spec berubah)
- Live: `curl -D-` unduhan foto menu → `private, max-age=300` (default tak
  berubah oleh perubahan ini); dengan `download_cache_ttl: 60s` di manifest →
  `max-age=60`; dengan `0s` → `no-cache`.
