# Caching unduhan berkas (`file` field + link consume)

**Status:** ✅ Selesai · **Tanggal:** 2026-10-07 (selesai 2026-10-08) · **Effort:** small
**Referensi:** `docs/spec/backend/05-field-types.md` §Storage Spec,
`docs/runtimes/05-engine-api-layer.md` §2
**Changelog:** `docs_internal/changelog/2026-10-08-001-cache-unduhan-berkas.md`
**Todo:** Fase 7 §7.17.9

## Masalah (terukur)

`GET /{ws}/_ui/entity/{module}/{entity}/{id}/{field}` dan
`GET /{ws}/_ui/storage/link/{token}` hanya menyetel `Content-Type` +
`Content-Disposition`, lalu `w.Write(data)` — tidak ada `Cache-Control`,
`ETag`, `Last-Modified`, maupun `Content-Length`.

Terukur di dev server kafe (foto menu, `visibility: public`, 136.398 byte):

```
HTTP/1.1 200 OK
Content-Disposition: inline; filename="...-kopi-susu.jpg"
Content-Type: image/jpeg
Transfer-Encoding: chunked
```

Identik untuk staf ber-sesi dan tamu anonim. Karena browser tidak menerima
satu pun header cache, respons dianggap basi begitu diterima — setiap
navigasi/reload mengunduh ulang penuh 136 KB (dan satu `Stat` ke object
store). Untuk katalog menu bergambar, biayanya berlipat per gambar.

## Keputusan

| #   | Keputusan                                                          | Alasan                                                                                                        |
| --- | ------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------- |
| D1  | `ETag` diturunkan dari **object key**, tanpa membaca/menghash body | `ObjectKey` menyematkan UUID baru setiap unggah → unggah ulang memberi ETag berbeda **gratis**                |
| D2  | `Content-Length` selalu diset                                      | Menghapus chunked; progres + deteksi transfer terpotong                                                       |
| D3  | `visibility: public`/`private` → `private, max-age=300`            | `private` melarang cache bersama (respons bergantung permission); 5 menit, **bukan** setahun                  |
| D4  | `visibility: signed` + route link-consume → `no-store`             | Token-scoped, bisa `one_time`; body tidak boleh tinggal di cache                                              |
| D5  | Tidak pernah `immutable`                                           | URL stabil (`.../{id}/photo`) sementara baris bisa di-repoint ke key baru → `immutable` = foto basi selamanya |
| D6  | `If-None-Match` → `304`                                            | Idempotent, murah, dan aman bahkan untuk `public`                                                             |

**Kenapa `max-age` pendek, bukan panjang.** Isi objek **immutable per key**,
tetapi **URL-nya tidak**: unggah ulang menulis key baru dan memperbarui baris,
sedangkan URL tetap sama. Enforcement permission juga hanya terjadi saat
request, jadi server tidak bisa menarik kembali apa yang sudah di-cache klien.
`private` + jendela pendek + ETag adalah titik seimbangnya: repeat view murah
(304 tanpa body), dan foto baru terlihat maksimal 5 menit setelah revalidasi.

## Implementasi

- `internal/api/file.go`:
  - konstanta `downloadCacheTTL = 5 * time.Minute`;
  - satu helper `(f *HandlerFactory) serveFileBody(w, r, key, data, disposition, cacheable)` yang menangani ETag/304/`Cache-Control`/`Content-Length` — dipakai **kedua** situs penyajian (unduhan entity + link consume), karena keduanya sebelumnya menduplikasi blok header yang sama dan duplikasi itulah yang melahirkan bug ini;
  - `fileETag(key)` (SHA-256 → hex 16 byte) dan `etagMatches(header, etag)` (menangani `*`, daftar berkoma, prefiks `W/`).
- `internal/api/middleware.go`: `If-None-Match` masuk `Access-Control-Allow-Headers`
  (butuh preflight lintas-origin; tanpa itu revalidasi tak pernah terjadi).
- Docs: `docs/spec/backend/05-field-types.md` §Storage Spec (paragraf caching) +
  `docs/runtimes/05-engine-api-layer.md` (tabel header).

## Test

`internal/api/file_test.go` — `TestFileDownloadCaching`:

1. ETag ada; `If-None-Match` cocok → **304** tanpa body;
2. `If-None-Match` tidak cocok → 200 + body lengkap;
3. `Cache-Control` = `private, max-age=300` untuk `private`;
4. `public` → 200 anonim **tanpa** identity, `Cache-Control` sama;
5. `signed` + `one_time` → `no-store`, **tanpa** ETag;
6. `Content-Length` = panjang body;
7. ETag **berubah** setelah unggah ulang (key baru) — guard anti-foto-basi;
8. `visibility: signed` tanpa `link_token` → 401 (tidak berubah).

Guard dikalibrasi dengan menyuntikkan regresi (ETag dari path alih-alih key →
test 7 gagal) sebelum dianggap selesai.
