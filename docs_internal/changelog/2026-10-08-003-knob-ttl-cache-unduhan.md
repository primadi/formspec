# 2026-10-08-003 — Knob TTL cache unduhan + temuan brotli/CDN

**Plan:** `docs_internal/plan/download-cache-ttl-knob.md`
**Todo:** 7.17.11 ✅ ditutup · 8.4.5 ✅ ditutup (keputusan) · 8.4.4 ⏸️ + 7.17.10 ⏸️ temuan diperbarui

Menutup dua item cache terbuka dan memperbarui temuan dua lainnya supaya
penundaannya bisa diperiksa.

## 7.17.11 ✅ — `max-age` unduhan kini bisa dikonfigurasi

`downloadCacheTTL` dulu konstanta tunggal 5 menit tanpa cara mengubahnya. Kini
tiga lapis, mengikuti pola `max_download_mb`/`signed_url_ttl` yang sudah ada
(tanpa mekanisme baru): `storage.download_cache_ttl` per-field →
`FORMSPEC_DOWNLOAD_CACHE_TTL` global → default 5m. **`0s` = `no-cache`** —
escape hatch untuk isi yang harus selalu terlihat segar; ETag tetap dikirim,
jadi paksaan revalidasi itu tetap murah (304, bukan unduh ulang).

Sekalian: `ValidateStorageSpec` kini memvalidasi **durasi** (`download_cache_ttl`,
`signed_url_ttl`, `ttl`). `signed_url_ttl` sebelumnya **tidak divalidasi sama
sekali** — durasi salah ketik ("15min") diam-diam diabaikan saat runtime dan
fallback ke default, kelas bug yang sama dengan `allowed_types` (gap #4b).

**Bukti:** `go test ./...` hijau · 5 sub-test resolusi (global/field/`0s`/
global-`0s`/`1h`) · lint `0 issues` · **live**: `FORMSPEC_DOWNLOAD_CACHE_TTL=45s`
pada dev server kafe → unduhan foto menu menjawab `Cache-Control: private,
max-age=45` (sebelumnya 300). Guard dikalibrasi: mengabaikan deklarasi
per-field → 3 sub-test gagal (`max-age=420, want 60`); melumpuhkan gerbang
durasi → `TestValidateStorageSpec_DurationGates` gagal. Schema diregenerasi
(field baru muncul di `$defs` Entity + alias).

## 8.4.5 ✅ — batas cache API (keputusan, bukan implementasi)

`/api/v1/*` dan `/_ui/entity/*` mengembalikan JSON bergantung
identity/permission; tanpa `Vary`/kunci sadar-auth, cache HTTP di sana adalah
kelas kebocoran lintas-pengguna. Yang sudah cukup: ETag `/_meta/ui` (5.12.1).
Ditutup sebagai keputusan eksplisit di docs — supaya tidak terbaca sebagai
backlog tersembunyi.

## 8.4.4 ⏸️ — temuan brotli berubah (tetap tertunda)

**Koreksi 2026-10-08 (setelah paragraf di bawah ditulis):** dua klaim di sini
gugur saat diuji. (a) **`GOSUMDB=off` tidak diperlukan** — penyebab aslinya
`/go/pkg/mod/sumdb` tak bisa dibuat, jadi cache checksum gagal untuk modul baru
apa pun; `GOPATH` writable saja cukup. (b) **usulan `zstd` dibatalkan**: pada 7
aset nyata (1.618.371 byte raw) zstd default **+4,3% lebih besar** dari gzip-6,
dan `SpeedBestCompression` (−6,0%, 106 ms) masih kalah dari **brotli q5**
(−7,2%, 67 ms); brotli q11 mencapai −16,5%. Jadi usulan pengganti yang benar
adalah **brotli q5 pra-kompresi**, bukan zstd. Angka lengkapnya di item 8.4.4.

Pilihan "pakai `klauspost/compress`" **tidak bisa** untuk brotli: modul itu
tidak punya encoder brotli (isi: `flate, gzip, huff0, s2, snappy, zip, zlib,
zstd, gzhttp`, bukan brotli). Encoder murni-Go = `github.com/andybalholm/brotli`
(diuji di container: bisa diunduh dan jalan), dan `brotli` CLI **tidak ada**,
jadi pre-compress saat build pun butuh tool Go. Karena codec menentukan
`Content-Encoding` yang dikirim ke setiap klien, ini keputusan produk — item
**tidak** ditutup tanpa jawaban.

## 7.17.10 ⏸️ — temuan CDN: tidak ada pemakai

`grep -rn 'cdn: true' examples/ verticals/` → **nol**. `StorageSpec.CDN` adalah
deklarasi spec tanpa pemakai yang menunggu, jadi ini bukan fitur setengah jalan.
Menutupnya butuh keputusan desain (host, path style, invalidation) yang tidak
bisa disimpulkan dari kode. Dicatat di item agar penundaannya jelas disengaja.

## Jebakan (dicatat untuk sesi berikutnya)

Verifikasi regresi memakai `git checkout -- <file>` untuk memulihkan — dan itu
**menghapus perubahan sesi** di file tersebut (validasi durasi hilang, harus
dipasang ulang). Jebakan ini sudah ada di memory repo; yang benar adalah `cp`
ke `/tmp` sebelum injeksi, lalu pulihkan dari salinan itu. Protokol itu dipakai
untuk injeksi berikutnya di sesi yang sama.
