# 2026-09-14-002 — Fase 0 kafe: rute lifecycle di surface UI + permission plural

Melanjutkan verifikasi kafe (`examples/kafe/gaps_found/TODO.md` Fase 0). Ditemukan
**BLOCKER #52** yang menjelaskan mengapa seluruh aplikasi tidak bisa berjalan, dan
**#53** yang membuat sebagian rute selalu menolak.

**#52 — aksi lifecycle tidak punya rute di surface UI.** `GenerateUIRoutes`
(`internal/api/generator.go`) hanya mengirim `[list, find, create, update, delete]`
ke `generateRESTRoutes`, sehingga `submit`/`cancel`/`amend` tidak pernah terdaftar.
Akibatnya rute wildcard file upload/download `/{module}/{entity}/{id}/{field}`
(`internal/api/router.go:451-452`) **menelan** `/{id}/submit` dan membalas
`403 missing permission: {module}.{entity}.update` — terverifikasi dengan aksi
karangan (`POST /{id}/zzz`) yang memberi error **identik**, jadi permintaan memang
tidak pernah mencapai handler aksi. Dampak: master data tidak akan pernah bisa
`submit` → karena record `draft` tidak _referenceable_ (#44), **tidak ada satu pun
transaksi yang bisa dibuat**.

**#53 — rute file memakai nama entity singular untuk permission.**
`internal/api/file.go:451` (`module + "." + entity + "." + action`) dan enam pesan
errornya memakai nama entity **singular**, sedangkan registry
(`internal/entity/registry.go:218`) dan generator (`generator.go:173`) memakai
**plural** → pengguna yang diberi `cafe-master.menu-categories.update` tetap ditolak
pada rute upload/download. Kanonik = plural (D5).

Yang diubah:

- `internal/api/generator.go` — `GenerateUIRoutes` menambahkan `submit`/`cancel`/`amend`
  ke `uiActions` (di-skip untuk `characteristic: summary`, dan untuk aksi yang sudah
  punya `impl:` karena aksi itu ditangani `GenerateUICustomActionRoutes` — mendaftarkan
  keduanya akan membuat rute ganda dan menutupi handler kustom; regresi ini sempat
  muncul pada `examples/Clinic-UI-Showcase` dan sudah ditangani).
- `internal/api/file.go` — `HandlerFactory.permName()` (plural, fallback `{entity}s`)
  dipakai oleh `can()` dan pesan error upload/download; validasi field dipindah
  **sebelum** pemeriksaan permission pada handler upload/download, dan jawabannya
  **404** (bukan 400/403) bila `{field}` bukan field `file`/`attachment` — supaya
  jalur aksi yang tidak ada tidak menyamar sebagai kesalahan izin.
- `internal/api/file_test.go`, `internal/api/link_test.go` — identitas uji memakai
  bentuk plural (perilaku singular yang ter-enkode di test memang yang keliru).
- `internal/api/api_test.go` — test baru `TestGenerateUIRoutes_LifecycleActions`
  (memuat spec kafe asli: `cafe-master/menu-category` harus punya rute
  `submit/cancel/amend` dengan permission `cafe-master.menu-categories.submit`) dan
  `TestGenerateUIRoutes_SummaryNoLifecycle` (`cafe-stock/stock-level` tidak punya).

Diverifikasi: `go build ./...` OK; `go test ./...` → 35 paket `ok`, nol `FAIL`.
Runtime: `POST /kafe/_ui/entity/cafe-master/menu-category/{id}/submit` → **401
authentication required** (rute ada; sebelumnya 403 `.update` menyesatkan);
`POST …/zzz` → **404**.

Referensi: `examples/kafe/gaps_found/TODO.md` (2.12, 2.13),
`examples/kafe/gaps_found/14-temuan-fase-0.md`, `decisions-needed.md`.
Sisa BLOCKER kafe berikutnya: **#44** (lifecycle/draft) dan **#46** (money).
