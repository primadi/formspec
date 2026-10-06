# Plan — Filter baris per-peran pada grant (10.67 + GAP-08)

Menutup **10.67** (aturan bisnis #1 ditegakkan deklarasi Kanban, bukan API) dan
**GAP-08** (penyaring cabang KDS hanya `fixed_filters` klien), sekaligus
keputusan 10.45 (predikat baris ditegakkan pada jalur tulis).

## Status — SELESAI (2026-10-04)

| Fase | Isi                                                | Status | Bukti                                                                                            |
| ---- | -------------------------------------------------- | ------ | ------------------------------------------------------------------------------------------------ |
| 1    | `FilterSpec.value` literal + fail-closed           | ✅     | `pkg/spec/rowscope.go`, `internal/api/scope.go`                                                  |
| 2    | `row_scope` per-action pada grant, ditegakkan BACA | ✅     | `internal/auth/{grant,materialize,resolver}.go`, `resource/row_scope_grant_e2e_test.go`          |
| 3    | Penegakan TULIS di store (`Update`/`SoftDelete`)   | ✅     | `renderers/jsonb-persist/crud.go`, `row_predicate_test.go`                                       |
| 4    | Validator grant + fail-closed                      | ✅     | `internal/auth/grantshape.go`, `cmd/formspec/validate_seed_grants.go`, `validate_grant_scope.go` |
| 5    | Editor grant (`row_scope` per action)              | ✅     | `renderers/react-shadcn/src/widgets/GrantsEditor.tsx`, `src/lib/grants.ts` + `grants.test.ts`    |
| 6    | Adopsi kafe (10.67 + GAP-08)                       | ✅     | `examples/kafe/spec/modules/formspec.core/seeds/roles.yaml`                                      |
| 7    | Norma & ledger                                     | ✅     | `docs/spec/backend/01-core-basic.md` §1.7, ledger kafe 10.67/GAP-08/10.73/10.74                  |

**Fase 4 dalam bentuk akhirnya lebih luas dari rencana awal**, karena tiga celah
baru terukur saat menutupnya — semuanya kelas yang sama: **batas hilang tanpa
suara**, dan semuanya fail-open (permission bertahan, batasnya lenyap).

1. **Bentuk `grants` dibaca lossy.** `json.Unmarshal` membuang kunci yang tidak
   dikenal, jadi `row_scopes:` menyisakan permission tanpa batas. Kini satu
   pembaca bentuk mentah (`auth.ValidateGrantListShape`) dipakai gerbang deploy
   **dan** loader role runtime; kunci yang salah ketik **ditolak** (403), bukan
   disajikan tanpa batas.
2. **`resource.find`/`resource.fetch` melewati batas.** Script membaca baris yang
   HTTP sembunyikan dari pemanggil yang sama. Kini `scriptRowPredicates` memakai
   batas yang sama (`row_scope` entity **AND** grant, permission `view`), dan
   store punya `FindByFieldsScoped`.
3. **Gate sumber atribut hanya berlaku untuk entity, dan hanya di `validate`.**
   Dua cacat sekaligus, keduanya terukur: (a) grant `from: session` tidak
   diperiksa gate mana pun — menghapus satu-satunya `assignments` di entity
   `employee` membuat tiga grant KDS tanpa sumber yang mungkin, sementara
   `validate` melaporkan **10** penolakan entity dan **nol** tentang grant itu;
   (b) `formspec check` **tidak menjalankan** lapisan scope sama sekali, jadi ia
   melaporkan **0 error / 0 warning** pada pohon yang sama yang oleh `validate`
   dilaporkan **10 problem**. Kini keduanya dijalankan kedua perintah
   (`validateGrantScopeSources` + `validateScopeSources` di `check.go`), dan
   aturannya sama-sama sempit: hanya bentuk implisit (tanpa `attr`) yang dinilai,
   `dimension` **maupun** `field` dari `assignments` dihitung sebagai sumber, dan
   `attr` eksplisit dibiarkan (nilainya bisa datang dari IdP lewat klaim `attrs`).

**Sisa ⏸️:** (a) `ActionGrant.Conditions` tetap inert — putuskan hidupkan
(kendala atribut **tulis**) atau hapus; (b) `row_scope` grant `from: route` tidak
punya padanan di jalur script (sudah fail-closed: ditolak, bukan dilewati) — jadi
tidak ada celah, hanya batas kemampuan yang terdokumentasi.

## Verdict atas dua opsi yang diusulkan pemilik

**Opsi 1 — grant row condition per action → BENAR, dan sudah di-scaffold.**
`internal/auth/grant.go` sudah punya `ActionGrant{Name, Conditions}`; slot
kondisi per-action **sudah ada** tetapi **inert** — `EvaluateGrantConditions`
(`internal/auth/abac.go`) tidak punya pemanggil non-test, dan
`docs_internal/plan/todo.md` item **6.2.6** masih `[⏸️]` ("enforcement penuh di
request di-defer"). Blocker 10.67 sendiri berbunyi _"Filter baris per-peran
belum ada di model grant"_ — jadi ini menutup item yang sudah diakui, bukan
mengarang konstruk baru.

**Opsi 2 — deny entity + allow report terfilter → TIDAK BEKERJA.** Tiga bukti:

1. **Tidak ada endpoint report.** `ReportSpec` (`pkg/spec/frontend.go:592`)
   tidak pernah muncul di `internal/` maupun `cmd/`. SPA memanggil
   `GET /{module}/{entity}` yang sama (`ReportRenderer`), jadi permission-nya
   **sama** dengan list entity — "deny entity" akan ikut mematikan report-nya.
2. **Filter report tak punya padanan server.** `ReportSpec` tidak punya
   `fixed_filters`; `ReportSource.Filter` dideklarasikan (5.13.1a ✅) tetapi
   **tidak dibaca** renderer maupun server. `parameters[]` = input pengguna.
3. **Report membaca entity yang sama**, jadi `row_scope` per-entity berlaku
   identik untuk keduanya → tidak ada penyempitan baris yang didapat.

Bentuk opsi 2 yang **benar-benar bekerja** adalah `characteristic: summary`
(entity tersendiri: nama, plural, permission, `row_scope`, baris sendiri).
Kafe sudah memakainya (`cafe-stock.stock-level`). Biayanya: proyeksi
termaterialisasi + script pemelihara, dan `RebuildSpec` **tidak punya
`Trigger`** sehingga pemicunya harus subscription + script, bukan deklarasi.
Dicatat sebagai alternatif untuk mart laporan, **bukan** untuk 10.67.

## Kenapa `Conditions` saja tidak cukup

`ConditionGrant.Expr` adalah FormSpecExpr terhadap **record yang dikirim**
(`resource` + `params`) — cocok untuk **tulis**, mustahil untuk **baca**:
list harus memuat semua baris lalu mengevaluasi satu per satu, sehingga
paginasi rusak dan tidak ada predikat yang turun ke SQL (index/kolom turunan
tidak terpakai). Maka dua slot, dua semantik berbeda:

| Slot                                      | Semantik                                          | Dipakai             |
| ----------------------------------------- | ------------------------------------------------- | ------------------- |
| `ActionGrant.RowScope []spec.FilterSpec`  | predikat **baris** (baca + tulis), turun ke query | **baru**, paket ini |
| `ActionGrant.Conditions []ConditionGrant` | kendala **atribut** pada payload                  | dibiarkan (inert)   |

`FilterSpec` sudah punya `field`, `op` (closed set lengkap termasuk `in`), dan
sumber nilai `session`/`route`. Yang **hilang** adalah **nilai literal**:
`applyRowScope` (`internal/api/scope.go`) hanya punya `case "session"` dan
`case "route"`; `Default` diabaikan di sana (ia hanya dipakai UI untuk kontrol
`filters`).

## Fase

### 1 — nilai literal di `FilterSpec` (small, mandiri)

- `pkg/spec/frontend.go` — `FilterSpec.Value string` + `@schema`.
- `internal/api/scope.go` — cabang default memakai `sc.Value`;
  **fail-closed** bila `from` **dan** `value` kosong (sekarang `switch` jatuh
  diam → deklarasi yang tidak melakukan apa pun, kelas bug senyap).
- Validator `row_scope` entity: tolak entri tanpa sumber nilai; tolak `from`
  tak dikenal.

### 2 — penegakan BACA: `row_scope` pada grant (medium)

- `internal/auth/grant.go` — `ActionGrant.RowScope []spec.FilterSpec`.
- `internal/auth/materialize.go` — `MaterializeDetailed` mengembalikan
  `{Page, Action, Permission, RowScope}`; `Materialize`/`MaterializePartial`
  mendelegasi sehingga signature lama tetap.
- `internal/auth/resolver.go` — peta **permission → RowScope** dengan cache
  TTL 30 detik (pola `scopeAttrCache`), di-invalidasi lewat `Invalidate`/
  `InvalidateAll` yang sudah ada.
- Pipa yang sudah tersedia dipakai: `api.SetAuthService` (`resource/formspec.go`),
  `authService` global (`internal/api/auth_handler.go`).
- `internal/api/scope.go` — `applyGrantScope(r, permission, filters)`:
  **AND** dengan `row_scope` entity, **tidak pernah melebarkan**. Dipanggil di
  `HandleList` (setelah `applyRowScope`) dan pada jalur id-addressed.
- Per-action, bukan per-page: `list`/`view` dapat predikat; `create` tidak
  (create tidak mencocokkan baris mana pun).

### 3 — penegakan TULIS di store (medium)

Keputusan pemilik: penegakan otoritatif di store (plan
`lapisan-otorisasi.md`); HTTP tetap lapisan gagal-cepat.

- **Titik sisip sudah ada, tanpa query tambahan.** `Update`
  (`renderers/jsonb-persist/crud.go`) sudah membaca baris lama via `GetByID`
  (untuk field `immutable` dan state `from`); predikat dievaluasi di situ.
- `InsertParams`/`UpdateParams` menerima principal + predikat baris yang
  berlaku untuk entity+action ini.
- `SoftDelete` diperluas menjadi `DeleteParams{WorkspaceID, ID, Permissions,
SystemCaller, RowPredicates}`; seluruh pemanggil disesuaikan.
- **Kelas error baru butuh marker sendiri.** `ErrForbidden`/`ForbiddenMarker`
  diklasifikasi `isStoreForbidden` dengan string tertentu; penolakan baris
  memakai pesan berbeda, jadi ia butuh marker + cabang sendiri — tanpa itu
  jalur `resource.save()` jatuh ke **500** (mode kegagalan 10.71).
- Batas yang dinyatakan: `UpsertProjection` (summary, system-only) dan
  `setActive` tetap system; seed/backup/migrasi sudah `SystemCaller`.

### 4 — validasi + fail-closed (small, wajib)

Grant adalah JSON bebas di kolom `grants` Role — **tanpa schema, tanpa
validasi** (dibuktikan 10.47 & 10.53: grant yang gagal materialisasi hilang
senyap). Untuk sebuah filter keamanan, "typo = tanpa filter" berarti
**fail-open**, lebih buruk daripada hari ini. Jadi:

- Validasi grant: page resolve, action ada di footprint, dan untuk `row_scope`:
  field ada di entity target, `op` di closed set, **tepat satu** sumber nilai.
- Runtime: `row_scope` dideklarasikan tetapi tak bisa dipakai → **deny**, bukan
  diam.
- Dilaporkan lewat `SetLogger` resolver (mekanisme yang sudah ada sejak 10.53).

### 5 — editor grant (medium)

- `renderers/react-shadcn/src/widgets/GrantsEditor.tsx` — model TS kini
  `Grant{page, actions?: [{name}], tabs?}`, belum tahu `conditions`/`row_scope`.
  Tambah kontrol: field (dari entity target), operator, sumber nilai
  (literal/`session`/`route`), editor nilai.
- Catatan liputan: `grep examples/** grants-editor` → **0**, jadi editor tidak
  dipakai manifest kafe mana pun (grant kafe datang dari `kind: Seed`).
  Konsekuensinya risiko regresi kafe nol, tetapi liputan kafe untuk editor juga
  nol → editor diuji lewat vitest, dan acceptance kafe tetap lewat jalur seed.

### 6 — adopsi kafe (small)

- `examples/kafe/spec/modules/formspec.core/seeds/roles.yaml`:
  `order-page` **barista** & **dapur** → `row_scope` status
  `in paid,in_kitchen,ready,served`; **pelayan** → `in ready,served,completed`;
  kasir/supervisor/manajer **tanpa** `row_scope` (harus melihat draft-nya).
- Sekaligus **GAP-08**: `{field: branch_id, from: session}` pada grant KDS,
  sehingga cabang ditegakkan API — bukan lagi `fixed_filters` klien
  (`kanbans/order-board-kds.yaml`).

### 7 — norma & ledger (small)

- `docs/spec/backend/01-core-basic.md` §1.7 (row scope) + §8.6 (grant);
  `docs/spec/platform/02-workspace-app-module.md` §3.
- Ledger: 10.67 ✅; GAP-08 diperbarui; sisa bernomor bila ada.

## Verification

1. **Kalibrasi gagal dulu:** tanpa Fase 2, barista `list order` memuat `draft`
   (reproduksi 10.67); dengan Fase 2 hanya paid+.
2. `find` order `draft` sebagai barista → ditolak, bukan 200.
3. **Bukti "UI bukan batas":** panggil API langsung **tanpa** `fixed_filters`
   kanban → barista tetap tidak melihat draft.
4. Kasir `list order` → **masih memuat draft** (membuktikan kasir tidak
   dibutakan).
5. GAP-08: barista cabang A → cabang B 0 baris, tanpa bantuan klien.
6. Fase 3: barista `PATCH` order `draft` → ditolak, dan tetap ditolak pada
   jalur `resource.save()` — bukan 500.
7. Unit: `internal/api/scope_test.go`, `internal/auth/materialize_test.go`,
   validator grant, `renderers/jsonb-persist`, vitest GrantsEditor.
8. `go test ./...` · `gofmt` · `formspec validate` kafe · `formspec check` 0/0
   · `tsc -b`.

## Keputusan yang sudah diambil

1. Granularitas → **per-action** (`ActionGrant.RowScope`).
2. GAP-08 → **digabung** ke paket ini.
3. GrantsEditor → **ikut sekarang**.
4. Penegakan tulis (`Update` + `SoftDelete`) → **ikut sekarang**.
5. Baris di luar scope → **404 NOT_FOUND** (tidak memberi oracle kepemilikan,
   konsisten dengan `list` yang barisnya memang absen). **403** disimpan untuk
   scope yang _tak bisa diselesaikan_ (misconfiguration) dan penolakan
   field-level §5.3 — satu jawaban dipakai jalur HTTP **dan** store.
6. `SoftDelete` → signature diperluas sekarang.

## Risiko

- `from: session` pada grant KDS gagal-closed bila role tak punya assignment →
  validator Fase 4 harus menangkapnya sebelum seed, bukan setelah barista tidak
  bisa membuka kanban (pola gerbang 1.8).
- Cache grant scope harus di-invalidasi saat role diedit.
- `SoftDelete` signature berubah → churn mekanis di banyak situs.
