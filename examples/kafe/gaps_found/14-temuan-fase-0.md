# Temuan Fase 0 — Verifikasi Ulang Ledger

**Tanggal:** 2026-09-14 · **Binary:** `formspec` (repo root, build 2026-09-13) ·
**Spec:** `examples/kafe/spec` (69 manifest)

Tujuan: memisahkan gap yang **masih terbuka** dari yang **sudah tertutup**, dan
menemukan gap yang **belum tercatat**. Semua klaim di bawah diverifikasi dengan
perintah yang bisa gagal, bukan pembacaan kode.

---

## 1. Gap yang sudah tertutup (ledger lama stale)

| Gap                                             | Ledger lama                | Kenyataan                                                | Bukti                                                                                                                                                    |
| ----------------------------------------------- | -------------------------- | -------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **#7** `exclude: [public_api]` belum ditegakkan | HIGH                       | ✅ **TERTUTUP**                                          | `internal/api/fieldsec.go` (`sanitizeData`) menghapus field per surface (`ui` vs `public_api`); changelog `2026-08-20-010-fase6-field-level-security.md` |
| **#24** `formspec dev` "mati"                   | HIGH → sudah dikoreksi LOW | ⛔ **DIBATALKAN** (tetap)                                | `formspec dev` naik normal: 155 route, 4 worker aktif                                                                                                    |
| **#26** `settings.currency` wajib               | HIGH                       | ⚠️ Sebagian: **tidak diterapkan**, tapi juga tidak gagal | lihat #46 di bawah                                                                                                                                       |
| **#18** skill mengajarkan `relation.target`     | MEDIUM                     | ⚠️ Dikoreksi: validator menangkapnya                     | —                                                                                                                                                        |

## 2. Gap yang TERVERIFIKASI masih terbuka

### 2.1 #44 — `create` menghasilkan `draft` yang tidak bisa direferensikan 🔴 BLOCKER

```console
$ curl -X POST .../kafe/_ui/entity/cafe-master/menu-category -d '{"name":"Kopi","sort_order":1}'
HTTP 201 → {"doc_status":"draft", ...}

$ curl -X POST .../kafe/_ui/entity/cafe-master/menu-item -d '{"menu_category_id":"<id>",...}'
HTTP 422
{"error":{"code":"VALIDATION_ERROR","message":"menu-item insert: field validation failed:
 relation target cafe-master.menu-category[<id>] is draft (must be submitted or lifecycle-free)"}}
```

**Masih terbuka.** Rantai: `create` → `draft` → tidak bisa direferensikan →
`submit` butuh permission `update` (D6) → permukaan publik tidak bisa menutupnya.

### 2.2 #46 — `money` tidak divalidasi & tidak dinormalisasi 🔴 HIGH

```console
$ curl -X POST .../cafe-master/promo -d '{"type":"fixed","value":25000,...}'
HTTP 201 → "value": 25000                       ← angka polos, bukan objek Money

$ curl -X POST .../cafe-master/promo -d '{"type":"fixed","value":{"amount":"15000"},...}'
HTTP 201 → "value":{"amount":"15000"}           ← currency TIDAK diisi dari settings.currency
```

Padahal `spec/config/app.yaml` menetapkan `settings.currency.code = IDR`.
**Masih terbuka**, dan **#26 dikonfirmasi**: tidak diterapkan, tidak gagal.
Data tanpa mata uang tersimpan tanpa keluhan.

### 2.3 #22 — `indexes:` di level spec diabaikan 🔴 HIGH

```console
$ ../../formspec migrate plan --spec spec --dsn sqlite:.formspec/tmp.db
── entity:cafe-master/menu-item-price ──
CREATE TABLE cafe_master_menu_item_prices (
  ...
  data        text   NOT NULL DEFAULT '{}'
);                                   ← TIDAK ADA index sama sekali
```

`menu-item-price` mendeklarasikan `indexes: [{fields: [branch_id, menu_item_id], unique: true}]`
di level spec, tetapi DDL menghasilkan **nol index**. Penyebab (kode):
`renderers/jsonb-persist/ddl.go` §4 membaca **hanya** `entity.Persist.Indexes`,
sedangkan spec menaruh di root (`EntitySpec.Indexes`, `pkg/spec/entity.go:87`).

Tambahan: tabel tidak punya kolom turunan `_branch_id`/`_menu_item_id` →
field `relation` memang tidak dapat kolom turunan (index atas relasi mustahil).

Band **shift** juga tidak punya partial unique `(branch_id, cashier_id) WHERE status='open'`:

```console
── entity:cafe-order/shift ──
CREATE INDEX idx_cafe_order_shifts_transaction_date ON ... (_transaction_date);
CREATE INDEX idx_cafe_order_shifts_status ON ... (_status);
                                 ← tidak ada unique parsial (aturan bisnis #10 tidak dijaga DB)
```

### 2.4 #27 — tipe PostgreSQL bocor ke DDL SQLite 🔴 LOW–MEDIUM

```console
_transaction_date timestamptz GENERATED ALWAYS AS (json_extract(data, '$.transaction_date')) STORED
```

`timestamptz` adalah tipe PostgreSQL, muncul di DDL SQLite. Sebagian entity
(`shift`) memakai `date`, sebagian (`table-session`, `point-entry`) `timestamptz`
— tidak konsisten, membuktikan pemetaan tipe tidak driver-aware.

### 2.5 #10 / S10 / #21 / #17 — diverifikasi lewat kode

| Gap                       | Status  | Bukti                                                                                                            |
| ------------------------- | ------- | ---------------------------------------------------------------------------------------------------------------- |
| #10 print thermal         | 🔴 OPEN | `docs/renderers/shadcn-shell/03-kind-renderers.md:63` — "`pdf`/`thermal`/`dotmatrix` belum ada kode sama sekali" |
| S10 widget enum           | 🔴 OPEN | `schemas/formspec.schema.json` → `Field.widget` = `{"type":"string"}`, tanpa `enum`                              |
| #21 referensi menggantung | 🔴 OPEN | tidak ada validasi `App.spec.modules` / menu `view:` / `impl.ref` di `internal/manifest/`; validate hijau        |
| #17 timeline realtime     | 🔴 OPEN | nol kemunculan `realtime` di `renderers/react-shadcn/src/kinds/timeline/`                                        |

---

## 3. Gap BARU (belum tercatat di ledger) 🔴

### #49 — Tiga guard script Starlark tidak bisa dikompilasi 🔴 BLOCKER

**Ini temuan paling penting Fase 0.** Ketiga guard script kafe memakai
**implicit adjacent string-literal concatenation** (kebiasaan Python) yang
**tidak didukung** oleh dialek Starlark:

```console
$ curl -X POST .../cafe-master/menu-item-price -d '{...}'
HTTP 422
{"error":{"code":"HOOK_ABORTED","message":"action cafe-master.create (script_ref, 797µs):
 script failed: script compile error:
 spec/modules/cafe-master/scripts/guard_menu_item_price_unique.star:83:31: got string literal, want ','"}}

$ curl -X POST .../cafe-order/shift -d '{...}'
HTTP 422
{"error":{"code":"HOOK_ABORTED","message":"... cafe-order.create (script_ref, 298µs):
 script compile error:
 spec/modules/cafe-order/scripts/guard_shift_open_unique.star:45:31: got string literal, want ','"}}
```

Pola yang sama di **ketiga** script:

| Script                                                  | Baris gagal | Menghambat                          |
| ------------------------------------------------------- | ----------- | ----------------------------------- |
| `cafe-master/scripts/guard_menu_item_price_unique.star` | :83         | `create`/`update` `menu-item-price` |
| `cafe-order/scripts/guard_shift_open_unique.star`       | :45         | `create`/`update` `shift`           |
| `cafe-stock/scripts/guard_stock_level_unique.star`      | :88         | tulis `stock-level`                 |

**Dampak:** bukan "guard yang berisiko deadlock di dev" seperti dugaan GAP-30,
melainkan **guard yang selalu gagal** — seluruh penulisan harga menu, shift, dan
saldo stok **diblokir total** dengan pesan error yang membingungkan. GAP-30
(deadlock) bahkan **tidak pernah tercapai** karena kompilasi gagal lebih dulu.

**Klasifikasi:** campuran — bagian **SPEC/PROJECT** (script bisa diperbaiki:
gabungkan literal dengan `+` atau satu literal) dan **DOC** (dialek Starlark vs
Python tidak didokumentasikan, sehingga penulis spec wajar memakai kebiasaan
Python).

**Perbaikan:**

1. **Spec kafe (cepat):** ganti konkatenasi implisit → satu literal atau `+`.
2. **Dokumentasi/skill:** catat batasan dialek Starlark (tidak ada implicit
   string concatenation) di `ai_skills/**` + `docs/spec/backend/`.
3. **Engine (lebih baik):** jangan sampai script gagal _saat runtime pertama kali_;
   validasi saat load (lihat #50).

### #50 — `formspec validate` tidak mendeteksi script yang gagal kompilasi 🔴 HIGH

```console
$ ../../formspec validate --spec spec
69 manifest(s) validated, 0 problem(s) found        ← HIJAU
EXIT=0
```

...padahal tiga script yang dirujuk `hooks:` di spec itu **tidak bisa
dikompilasi**. `validate` punya opsi `-fix` (honesty scan `uses`) tetapi **tidak
ada pemeriksaan kompilasi Starlark**, dan tidak ada flag untuk itu.

**Dampak:** satu-satunya gerbang otomatis proyek ini (dijanjikan di
`docs/architecture.md` §0) tidak dapat membedakan spec yang sehat dari spec yang
setiap penulisannya gagal. Ini persis pola kegagalan yang mendasari seluruh
ledger: _"fitur belum ada"_ vs _"saya salah tulis"_ tidak terbedakan.

**Perbaikan:** tambahkan tahap compile-check Starlark ke `validate`/`check`
(muat tiap script yang direferensikan `impl.ref`/`hooks`/`conditions` dan
kompilasi dengan runtime yang sama), plus gerbang di Fase 0.

### #51 — `ctx.db().query()` menolak bind parameter 🔴 HIGH

Ditemukan **setelah #49 diperbaiki**: kompilasi lolos, tapi script gagal saat jalan.

```console
$ curl -X POST .../cafe-master/menu-item-price -d '{...}'
HTTP 422
{"error":{"code":"HOOK_ABORTED","message":"action cafe-master.create (script_ref, 357µs):
 script failed: script runtime error: query: got 2 arguments, want at most 1"}}
```

**Kontrak vs implementasi:**

| Sumber                                            | Menyatakan                                                           |
| ------------------------------------------------- | -------------------------------------------------------------------- |
| `internal/starlark/primitive.go:21`               | `// Querier serves ctx.db().query(sql, args...)`                     |
| `internal/starlark/context.go:11`                 | `ctx.db().query("SELECT ...")`                                       |
| `internal/starlark/primitive.go` (`builtinQuery`) | `UnpackArgs("query", args, kwargs, "sql", &sql)` — **hanya `sql`**   |
| lanjutan                                          | `q.Query(threadContext(thread), sql)` — **argumen tidak diteruskan** |

Jadi bind parameter tidak pernah ada di layer builtin, meskipun interface
`Querier` mendeklarasikan `args ...any`. Dampak: **setiap** guard/validasi yang
butuh membandingkan nilai harus menginterpolasi ke dalam teks SQL (risiko
injeksi + tidak bisa memakai placeholder), atau tidak bisa ditulis sama sekali.
Ini adalah **akar** dari kelas masalah yang di ledger lama terlihat seperti
"tidak ada API find-by-field" (#31) dan "deadlock" (#30) — keduanya baru bisa
dievaluasi setelah query parameter bisa jalan.

**Perbaikan (dikerjakan):** `builtinQuery` menerima argumen opsional kedua
(`args?`) sebagai list/tuple, konversi lewat `fromStarlark()`, dan meneruskan ke
`Querier.Query(ctx, sql, params...)`. Test regresi
`TestCtxDBQuery_BindArgs` (`internal/starlark/primitive_test.go`).

---

### #52 — Aksi lifecycle (submit/cancel/amend) **tidak punya rute di surface UI**; rute file menelannya 🔴 BLOCKER

Temuan paling merusak untuk aplikasi kafe. Diuji dengan **aksi yang tidak ada** sebagai kontrol:

```console
$ POST /kafe/_ui/entity/cafe-master/menu-category/<id>/submit
HTTP 403  {"error":{"code":"FORBIDDEN","message":"missing permission: cafe-master.menu-category.update"}}

$ POST /kafe/_ui/entity/cafe-master/menu-category/<id>/zzz      # aksi karangan
HTTP 403  {"error":{"code":"FORBIDDEN","message":"missing permission: cafe-master.menu-category.update"}}   ← IDENTIK

$ GET  /kafe/_ui/entity/cafe-master/menu-category/<id>/submit
HTTP 403  {"error":{"code":"FORBIDDEN","message":"missing permission: cafe-master.menu-category.view"}}
```

Aksi karangan menghasilkan error **yang sama** ⇒ permintaan tidak pernah mencapai handler aksi;
ya ditangkap rute _wildcard_ file upload/download:

```go
// internal/api/router.go:451-452 (surface UI)
r.Post("/{module}/{entity}/{id}/{field}", b.factory.HandleFileUpload())
r.Get("/{module}/{entity}/{id}/{field}", b.factory.HandleFileDownload())
```

Sebab rute aksi tidak ada: `GenerateUIRoutes` (`internal/api/generator.go:88-95`) hanya
mengirim `uiActions = [list, find, create, update, delete]` (+deactivate/reactivate) ke
`generateRESTRoutes`, sehingga `submit`/`cancel`/`amend` **di-skip** (mereka bukan bagian
dari daftar `allowed`, dan cabang _"skip lifecycle"_ hanya untuk `useAll`).

**Dampak ke aplikasi kafe: BLOCKER.** Master data **tidak akan pernah bisa** `submit` →
karena #44, record tetap `draft` → tidak bisa jadi target relasi → **tidak ada satu transaksi pun
bisa dibuat**. Selain itu seluruh custom action yang tidak mendeklarasikan `impl:` juga tidak
punya rute di surface UI (mis. `close-shift` pada `shift` bila `impl` kosong).

**Catatan penting:** ini juga **membatalkan** penjelasan lama GAP-44/D6. Ledger lama menyimpulkan
_"submit butuh permission update"_; yang sebenarnya terjadi adalah **rute submit tidak pernah ada**,
dan pesan `update` berasal dari rute file yang memakai nama entity **singular** (#53).

**Perbaikan:**

1. `GenerateUIRoutes` harus mengikutsertakan `submit`/`cancel`/`amend` bila lifecycle entity aktif
   (selaras dengan `StandardRESTActions` + gating `TransitiveDisabled`), persis seperti surface external.
2. Rute wildcard file sebaiknya memvalidasi bahwa `{field}` memang field `file`/`attachment`
   pada entity → jika bukan, **404** (bukan 403 yang menyesatkan).

### #53 — Rute file memakai nama entity **singular** untuk permission 🟡 MEDIUM

```go
// internal/api/file.go:451
return identity.HasPermission(module + "." + entity + "." + action)   // singular
```

Registry dan generator memakai **plural**:

```go
// internal/entity/registry.go:218   /   internal/api/generator.go:173
perm := module + "." + plural + "." + action
```

Akibat: pengguna yang diberi `cafe-master.menu-categories.update` **tetap ditolak** pada rute
upload/download (dan sebaliknya). Enam situs di `file.go` (:149, :356, :560, :721, :803, :863)
menghasilkan pesan yang menyesatkan. Kanonik = **plural** (D5).

---

## 4. Yang terverifikasi BEKERJA (jangan dibangun ulang)

| Aspek                        | Bukti                                                                                                                             |
| ---------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `formspec dev` naik normal   | 155 route, SPA dari `renderers/react-shadcn/dist`, 4 worker (outbox/workflow-escalation/subscription-stream/subscription-dynamic) |
| Workspace flag dihormati     | `--workspace-id kafe` → `tenant_id: "kafe"` pada record; `GET /kafe/...` hidup                                                    |
| Guard referenceability jalan | menolak target `draft` dengan pesan jelas (#44)                                                                                   |
| `soft_deactivate`            | `is_active: true` otomatis pada record baru                                                                                       |
| `characteristic: summary`    | tulis `stock-level` → **405** (create diblokir di level route)                                                                    |
| Envelope respons             | `{data, meta}` / `{error:{code,message}, meta}` konsisten                                                                         |
| Unique natural key           | `CREATE UNIQUE INDEX ... (tenant_id, _code) WHERE deleted_at IS NULL` untuk `branch`/`menu-item`/`table-session`                  |

---

## 5. Dampak ke rencana

1. **#49 menaikkan prioritas "script bisa jalan" di atas semua Fase 1** — sekarang
   ini yang paling menghambat (`menu-item-price`, `shift`, `stock-level` tidak
   bisa ditulis sama sekali). Checklist diperbarui: `0.6` + `2.9`.
2. **#50 menambah pekerjaan validator** — masuk Fase 8 (sekarang `8.7`).
3. **#22 dikonfirmasi dua kali** (index diabaikan **dan** relasi tidak dapat
   kolom turunan) → Fase 3.1/3.2 tetap, tapi kini punya bukti DDL.
4. **#7 ditutup** → beban Fase 1 berkurang.
5. Estimasi: dari 48 gap, 1 tertutup dan 2 baru ditemukan; sisanya bertahan
   kecuali S-item yang belum diuji ulang (`0.2` masih berjalan).

---

## 6. Perbaikan yang sudah dikerjakan di Fase 0

| Item    | Aksi                                                                                     | Verifikasi                                                              |
| ------- | ---------------------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| **#49** | Konkatenasi implisit → `+` di 3 guard script (`cafe-master`, `cafe-order`, `cafe-stock`) | Guard `menu-item-price` **berjalan**; tidak lagi `script compile error` |
| **#51** | `builtinQuery` menerima & meneruskan bind args; test regresi `TestCtxDBQuery_BindArgs`   | `go test ./internal/starlark/ -run TestCtxDBQuery` → PASS               |
| **0.4** | `validate-baseline.md` dibuat (baseline: 0 problem)                                      | `formspec validate` → 0 problem                                         |

**Belum dikerjakan:** #50 (validator compile-check) → `TODO.md` 8.7.
**Efek samping:** setelah #51, guard benar-benar mengeksekusi query. Kegagalan
berikutnya pada jalur `menu-item-price` adalah **#44** (target relasi `draft`),
sesuai yang didokumentasikan — jadi #49/#51 tidak menyembunyikan gap lain,
melainkan membuka jalan untuk mengujinya.

### Temuan tambahan sesi yang sama

| Item      | Aksi                                                                                                | Verifikasi                                                                                                                                                                                    |
| --------- | --------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **#52**   | Rute aksi lifecycle di surface UI (submit/cancel/amend) + wildcard file → 404 bila bukan field file | ✅ **Diperbaiki 2026-09-14** — `POST …/menu-category/{id}/submit` → **401 auth** (sebelumnya 403 `.update` menyesatkan); `POST …/zzz` → **404**. Test `TestGenerateUIRoutes_LifecycleActions` |
| **#53**   | `internal/api/file.go` memakai `plural` untuk permission                                            | ✅ **Diperbaiki 2026-09-14** — `HandlerFactory.permName()`; test file/link diselaraskan                                                                                                       |
| **0.3**   | `decisions-needed.md` — D1–D7 dijawab dari kode                                                     | ✅ 2026-09-14                                                                                                                                                                                 |
| **D4/D6** | Koreksi: `on_*` kanonik; `submit` punya permission sendiri (bukan `update`)                         | ✅ tercatat di `decisions-needed.md`                                                                                                                                                          |
