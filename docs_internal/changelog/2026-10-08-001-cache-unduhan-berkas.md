# 2026-10-08-001 — Caching unduhan berkas (field `file` + link consume)

**Plan:** `docs_internal/plan/file-download-caching.md`
**Todo:** Fase 7 §7.17.9

Unduhan berkas tidak membawa satu pun header cache: `GET
/{ws}/_ui/entity/{module}/{entity}/{id}/{field}` dan `GET
/{ws}/_ui/storage/link/{token}` hanya menyetel `Content-Type` +
`Content-Disposition`, lalu `w.Write(data)`. Tanpa `Cache-Control`, `ETag`,
`Last-Modified`, atau bahkan `Content-Length` (respons jadi chunked), browser
menganggap respons sudah basi begitu diterima — setiap navigasi/reload
mengunduh ulang seluruh objek. **Terukur** di dev server kafe pada foto menu
136.398 byte (`visibility: public`): staf ber-sesi dan tamu anonim sama-sama
menerima 200 penuh tanpa header cache.

**Yang dikerjakan.** Satu helper `serveFileBody` di `internal/api/file.go`
kini melayani **kedua** situs penyajian — sebelumnya tiap situs menulis pasangan
header sendiri, dan duplikasi itulah yang melahirkan bug ini. Kontraknya:

- `ETag` diturunkan dari **object key** (`fileETag`), bukan dari hash body.
  `ObjectKey` menyematkan UUID baru tiap unggah, jadi unggah ulang otomatis
  memberi validator berbeda **tanpa membaca satu byte pun**; `If-None-Match`
  yang cocok dijawab `304` tanpa body.
- `Cache-Control: private, max-age=300` untuk `public`/`private`. `private`
  wajib karena respons bergantung permission pemanggil (atau dibaca anonim),
  jadi cache bersama tidak boleh mencampurnya. Jendela **5 menit**, bukan
  setahun: byte objek memang immutable per key, tetapi URL-nya tidak berubah
  saat baris di-repoint ke key baru — `immutable` akan memakukan foto basi.
- `visibility: signed` + route link-consume → `no-store` **tanpa** ETag: body
  hanya terjangkau lewat kredensial sekali-pakai, dan validator hanya akan
  mengundang permintaan kondisional yang pasti gagal di `Consume` berikutnya.
- `Content-Length` selalu diset (chunked hilang).
- `If-None-Match` ditambahkan ke `Access-Control-Allow-Headers`: tanpanya
  revalidasi lintas-origin tidak pernah sampai ke handler.

**Bukti.** `go test ./internal/api/` hijau (8 test baru: 304, validator basi,
`*`, `W/`, `private` vs `public`, ETag berubah setelah unggah ulang, `no-store`
pada link consume, `etagMatches`) · `go test ./...` hijau · lint `0 issues`.
**Live** di dev server kafe — foto menu: `Cache-Control: private, max-age=300`

- `Content-Length: 136398` + ETag, dan `If-None-Match` → **304** dengan **nol**
  header body (`Content-Length`/`Transfer-Encoding` tak ada). Guard dikalibrasi
  dengan regresi disuntikkan: ETag dari `r.URL.Path` (stabil lintas unggah ulang)
  → `TestFileDownloadCaching_ReuploadChangesETag` gagal (`304, want 200` —
  persis foto basi); `cacheable=true` pada jalur link → test `no-store` gagal.

**Berkas:** `internal/api/file.go`, `internal/api/middleware.go`,
`internal/api/filedownload_cache_test.go` (baru),
`docs/spec/backend/05-field-types.md`, `docs/runtimes/05-engine-api-layer.md`.

**Sisa:** `max-age` 300 detik adalah pilihan konservatif, bukan hasil
pengukuran pola akses; `visibility: signed` + `cdn: true` (passthrough CDN di
spec) masih belum diimplementasikan — tidak berubah dari sebelumnya, dan tidak
diklaim tertutup di sini.
