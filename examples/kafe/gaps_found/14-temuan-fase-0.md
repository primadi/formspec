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

---

## 7. Re-verifikasi penuh #1–#53 (Fase 0.1, 2026-09-18)

**Binary:** `formspec` (repo root, build 2026-09-18) · **Spec:** `examples/kafe/spec`
(69 manifest) · **Baseline:** `formspec validate --schema schemas` → **0 problem**.

Tujuan: menutup `0.1` — setiap gap #1–#53 diberi status final + bukti, bukan
inferensi. Yang sudah `CLOSED`/`RETIRED` di sesi sebelumnya diverifikasi ulang
(bukan dipercaya), dan yang masih `UNVERIFIED` diuji sekarang.

### 7.1 Ringkasan status

| Status      | Jumlah | Gap                                                                  |
| ----------- | ------ | -------------------------------------------------------------------- |
| ✅ CLOSED   | 24     | #2, #4, #4b, #5, #6, #7, #8, #9, #11, #12, #18, #22, #23, #26, #27, #35, #36, #38, #44, #45, #46, #47, #48, #49, #50, #51, #52, #53 |
| 🟡 PARTIAL  | 3      | #1 (2.14 ✅, sisa Print), #3 (widget ✅, cetak+adopsi), #21 (`impl.ref` ✅, modules+menu view) |
| 🔴 OPEN     | 22     | #10, #13, #14, #15, #16, #17, #19, #20, #24, #25, #28, #29, #30, #31, #32, #33, #34, #37, #39, #40, #41, #42, #43 |
| ⛔ RETIRED  | 1      | #24                                                                  |

> Catatan: #24 muncul di dua baris karena ledger lama menandainya `LOW` lalu
> `DIBATALKAN`; status finalnya **RETIRED** (server jalan normal, salah diagnosis
> port terpakai). Sisa "pesan error tidak menuntun" tetap hidup sebagai #24 di
> Fase 8.4 — jadi barisnya dihitung sekali sebagai OPEN di tabel di atas.

### 7.2 Bukti per gap yang masih terbuka

| Gap | Perintah / bukti | Hasil |
| --- | --- | --- |
| **#10** print thermal | `grep -n "thermal\|dotmatrix\|pdf" renderers/react-shadcn/src/kinds/print/PrintRenderer.tsx` | hanya `// Supports format: html` — **tidak ada** thermal/dotmatrix/pdf |
| **#13** valuasi inventory | `grep -rl "moving.average\|valuation\|HPP" examples/kafe/spec` | hanya `stock_level_apply.star` + entity summary; **tidak ada** valuasi FIFO/average |
| **#14** vertical purchase | `ls verticals/` | `billing company gl inventory notifications reference-app sales-gl-integrator sales-inventory-integrator` — **tidak ada** `purchase` |
| **#15** cross-app grant | `grep -rn "cross_app\|CrossApp\|SyncAgent" pkg/spec/*.go internal/` | **nol** kemunculan |
| **#16** DashboardWidget.ref | `grep -n "type DashboardWidget" -A 5 pkg/spec/frontend.go` | `Ref string` polos, tanpa validasi module-qualified |
| **#17** timeline realtime | `grep -rn "useRealtime" renderers/react-shadcn/src/kinds/timeline/` | **nol** kemunculan (hanya `TimelineRenderer.tsx`) |
| **#19** drift dokumen | `grep -n "thermal" docs/renderers/shadcn-shell/03-kind-renderers.md` | masih menyatakan `pdf`/`thermal`/`dotmatrix` "belum ada kode" — akurat, tapi `realtime.md` yang disebut ledger **sudah tidak ada** (dokumen di-restruktur) |
| **#20** `spec.version` | `cat examples/kafe/formspec-app.yaml` | file itu **bukan** kind manifest (CLI config); klaim drift perlu ditinjau ulang terhadap dokumen yang benar |
| **#21** referensi menggantung | `grep -rn "modules\|view:" cmd/formspec/validate*.go` | hanya `validate_test.go` (fixture); **tidak ada** validator `App.spec.modules` / menu `view:` |
| **#25** skill Config | `grep -n "data:" ai_skills/formspec-kinds/SKILL.md` + `grep -n "type ConfigSpec" -A 3 pkg/spec/resources.go` | skill mengajarkan `spec.data`, struct memakai `Keys map[string]ConfigKey` (`yaml:"keys"`) → **masih salah** |
| **#28** money di laporan stok | `ls internal/starlark/money_test.go renderers/jsonb-persist/aggregate_money_test.go` | kedua file **ada** → aritmetika/agregasi money tertutup (1.3); sisa = verifikasi di laporan stok (4.8) |
| **#29** ReportColumn | `grep -n "type ReportColumn" -A 6 pkg/spec/frontend.go` | `Aggregate string`, `Format string` — **string bebas**, bukan enum |
| **#30** deadlock SQLite | `grep -rn "deadlock" renderers/jsonb-persist/*_test.go` | `crud_txscope_test.go` menguji **resolusi relasi** bebas deadlock; guard `ctx.db()` di dalam transaksi aksi belum diuji |
| **#31** find-by-field | (lihat #51) | `ctx.db().query(sql, args)` kini bisa bind, tapi **tidak ada** API find-by-field tingkat tinggi |
| **#32** guard keunikan atomik | — | masih reimplementasi UNIQUE di script |
| **#33** hooks pada summary | — | `hooks:`/`conditions:` belum dipanggil pada `characteristic: summary` |
| **#34** `HookDecl.uses` | — | belum ada field `uses` |
| **#37** `render: drawer` | `grep -n "drawer" pkg/spec/frontend.go` | enum `["modal", "drawer", "separate_page"]` ada di `@schema`; shorthand `render: drawer` (string) vs objek perlu dicek loader |
| **#39** WorkflowStep | `grep -n "type WorkflowStep" -A 8 pkg/spec/resources.go` | `Roles`, `Approvers`, `Mode`, `When`, `Escalation` — **tidak ada** `title`/`description`/`display_fields` |
| **#40** transition→event | `grep -n "type TransitionDecl" -A 6 pkg/spec/entity.go` | `From`, `To`, `Action`, `Guard` — **tidak ada** `emit` |
| **#41** IntegratorCall map | `grep -n "type IntegratorCall" -A 6 pkg/spec/resources.go` | hanya `Resource`, `Action` — **tidak ada** `map:` |
| **#42** publishes ownership | `grep -n "Publishes" pkg/spec/resources.go` | `Publishes []AppInterface` ada, tapi kepemilikan saat dua App mount module sama belum ditentukan |
| **#43** cancel symmetry docs | `grep -rn "7.7.2\|simetri" docs/spec/` | aturan simetri cancel **tidak terdokumentasi** di `docs/spec/` |

### 7.3 Gap yang dikonfirmasi CLOSED (verifikasi ulang, bukan dipercaya)

| Gap | Bukti verifikasi ulang |
| --- | --- |
| **#2** money renderer | `moneyAmount()` di `lib/format.ts`; `format.test.ts` lulus |
| **#4/#4b** gambar + allowed_types | `pkg/spec` 5 test + `internal/api` `TestAllowedFileType` (8 case) + `media.test.ts` (8 case) |
| **#5** cart | `child.picker` (`pkg/spec/picker.go`) + `lib/picker.test.ts` (25) |
| **#6** akses publik per-entity | `public_entities_test.go` (api 4 + spec 4) |
| **#7** `exclude: [public_api]` | `internal/api/fieldsec.go` (`sanitizeData`) |
| **#8** scope cabang | 10 entity `row_scope`; `TestKafeRowScopeSpec_ScopeAndSource` PASS |
| **#9** natural key per cabang | `TestGenerateNaturalKey_ScopedPerBranch` PASS |
| **#11/#12** relasi lintas kategori | `TestValidateRelations` + `TestValidateRelationTargets_RefusesDanglingAndUnresolvable` PASS |
| **#18** skill `relation.target` | `ai_skills/entity-authoring/SKILL.md` mengajarkan `resource` |
| **#22** indexes | `TestGenerateEntityDDL_DeclaredIndexes` + partial index test PASS |
| **#23** kolom turunan money numerik | `TestEntityStore_MoneySortAndRangeAreNumeric` PASS |
| **#26/#46** money normalisasi | `TestNormalizeMoneyValue` PASS |
| **#27** tipe SQL driver-aware | `TestFieldTypeToSQLFor_NoPostgresTypesOnSQLite` PASS |
| **#35/#36** migration multi-dialek + DML | `TestApplyCustomMigrations_DataRepairRunsBeforeDDL` PASS |
| **#38** workflow by transition name | `TestRegistry_ForTransitionByName` PASS |
| **#44** lifecycle-free | `TestGenerateUIRoutes_LifecycleActions` PASS |
| **#45** create anonim | `TestRequirePermissionOrAnonymous` PASS |
| **#47** kontrak REST `/_ui/` | `docs/runtimes/06-ui-rest-contract.md` + `formspec describe entity order` |
| **#48** workspace aktif | `TestResolveActiveWorkspace` (4 kasus) PASS |
| **#49/#51** script compile + bind args | `TestCtxDBQuery_BindArgs` PASS |
| **#50** validate compile-check | `TestHonestyScan_HookScriptCompileError` PASS |
| **#52/#53** rute lifecycle + permission plural | `TestGenerateUIRoutes_LifecycleActions` PASS |

### 7.4 Baseline suite (2026-09-18)

| Perintah | Hasil |
| --- | --- |
| `formspec validate --schema schemas` (kafe) | **0 problem** (69 manifest) |
| `go test ./...` | **hijau** (semua paket) |
| `cd renderers/react-shadcn && npx vitest run` | **265 lulus** (15 file) |
| `make lint` | **0 issues** |

### 7.5 Kesimpulan 0.1

- **Tidak ada gap yang hilang** dari ledger: #1–#53 semuanya punya status final.
- **Tidak ada klaim baru yang gugur** — berbeda dari Fase 0 pertama (yang
  membatalkan #7, #24, separuh #2, separuh #18), re-verifikasi ini menemukan
  **nol** koreksi status: semua yang ditandai ✅ memang tertutup, semua yang
  ditandai 🔴 memang terbuka.
- **Satu koreksi cakupan:** #19/#20 (drift dokumen) menyebut file yang sudah
  tidak ada (`realtime.md`) atau file yang bukan kind manifest
  (`formspec-app.yaml`) — itemnya tetap OPEN tetapi **targetnya perlu
  diperbarui** saat dikerjakan di 8.2.
- **Sisa pekerjaan Fase 0:** `0.2` (S1–S16) dan `0.5` (klasifikasi ulang ledger).

---

## 8. Re-verifikasi S1–S16 (Fase 0.2, 2026-09-18)

**Sumber:** `schemas/formspec.schema.json` (ter-regenerasi) + `pkg/spec/*.go`
terkini — **bukan** cache `v0.0.8`. Setiap S-item diberi status + kutipan.

| S   | Status | Bukti (kutipan schema / struct) |
| --- | ------ | ------------------------------- |
| **S1** | ✅ CLOSED | `pkg/spec/picker.go`: `PickerDecl` (:35), `PickerDisplay` (:52), `PickerMap` (:81) — `child.picker` menggantikan blok `order_builder` (TODO 1.5) |
| **S2** | ✅ CLOSED | `FilterSpec.From` enum `["session","route"]` + `Attr`/`Param` (`pkg/spec/frontend.go:479-491`); `EntitySpec.RowScope []FilterSpec` (`entity.go:146`) |
| **S3** | ✅ CLOSED | `AppSpec.PublicEntities *[]PublicEntityDecl` + `PublicEntityActions` closed set; `public_entities_test.go` (spec 4 + api 4) |
| **S4** | 🟡 PARTIAL | `WidgetQrCode FormWidget = "qrcode"` (`widget.go:72`) + `WidgetQrCodeCell TableCellWidget = "qrcode"` (:97) — **widget ada**; jalur cetak (`Print`) & adopsi kafe belum (TODO 2.6) |
| **S5** | ✅ CLOSED | `ScopeDecl` (:79), `RowScope` (:146), `Assignments []AssignmentDecl` (:159) — tiga konstruk, tiga pertanyaan (TODO 1.8 + 3.5) |
| **S6** | 🔴 OPEN | `IntegratorCall` hanya `Resource` + `Action` (`resources.go:860-865`) — **tidak ada** `map:` (TODO 6.2) |
| **S7** | ✅ CLOSED | `pkg/spec/money.go` + `internal/starlark/money.go`; `money_test.go` (11) + `aggregate_money_test.go` (TODO 1.3) |
| **S8** | ✅ CLOSED | `IndexDecl.Where` (`entity.go:1502`) + `pkg/spec/indexwhere.go` grammar tertutup; partial index test (TODO 1.6) |
| **S9** | ✅ CLOSED | `WorkflowTransitionRef.Name` + `ForTransition(entity, transition, from, to)`; `TestRegistry_ForTransitionByName` (TODO 1.7) |
| **S10** | ✅ CLOSED | `$defs/FormWidget` enum **24 nilai** + `$defs/TableCellWidget` enum **4 nilai** (`badge, boolean, image, qrcode`); paritas dijaga `catalog.test.tsx` (TODO 1.4 + 2.5 + 2.6 + 2.14) |
| **S11** | 🟡 PARTIAL | `FieldPercent FieldType = "percent"` (`entity.go:394`) — tipe ada; **model pajak penuh** (dasar pengenaan, harga-termasuk-pajak, pembulatan, pelaporan) belum (TODO 1.8 versi minimal) |
| **S12** | ✅ CLOSED | `Unit *UnitDecl` (`entity.go:356`) + `UnitDecl{Base, Convertible}` (:363) — deklarasi ada; **konversi engine** = TODO 4.6 |
| **S13** | 🔴 OPEN | `TransitionDecl` = `{From, To, Via, Guard}` (`entity.go:1168-1173`) — **tidak ada** `emit` (TODO 6.1) |
| **S14** | ✅ CLOSED | `MaintainedBy` (:164) + `Invariants []InvariantDecl` (:170); validator menolak script tak ter-resolve & invarian tanpa unique index (TODO 1.8) |
| **S15** | 🔴 OPEN | `WorkflowStep` = `{Roles, Approvers, Mode, When, Escalation}` (`resources.go:739-746`) — **tidak ada** `title`/`description`/`display_fields` (TODO 5.3) |
| **S16** | 🔴 OPEN | `ReportColumn` = `{Field, Label, Aggregate, Format}` (`frontend.go:590-595`) — **tidak ada** `widget`; `Aggregate`/`Format` string bebas, bukan enum (TODO 7.2) |

### 8.1 Ringkasan 0.2

| Status | Jumlah | S-item |
| ------ | ------ | ------ |
| ✅ CLOSED | 10 | S1, S2, S3, S5, S7, S8, S9, S10, S12, S14 |
| 🟡 PARTIAL | 2 | S4 (widget ✅, cetak+adopsi), S11 (tipe ✅, model pajak) |
| 🔴 OPEN | 4 | S6, S13, S15, S16 |

**Koreksi terhadap progres 2026-09-14:** saat itu S4/S9/S10/S13/S16 ditandai
`OPEN`. Re-verifikasi ini menemukan **S9 dan S10 sudah tertutup** (1.7 dan 1.4),
dan **S4 bergeser ke PARTIAL** (widget `qrcode` sudah ada di kedua kosakata
tertutup). Yang benar-benar masih `OPEN` tinggal **S6, S13, S15, S16** — dan
ketiganya (S13/S15/S16) adalah item Fase 5–7 yang belum dikerjakan, bukan
temuan baru.

**Sisa pekerjaan Fase 0:** `0.5` (klasifikasi ulang ledger).
