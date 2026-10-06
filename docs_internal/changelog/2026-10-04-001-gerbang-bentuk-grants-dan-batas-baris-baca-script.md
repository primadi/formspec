# 2026-10-04-001 — Gerbang bentuk `grants` + batas baris pada baca script

Menutup **kafe 10.73** (`grants` tanpa gerbang statis) dan **10.74**
(`resource.find`/`resource.fetch` tidak mengoper predikat baris pemanggil) —
dua sisa yang tercatat saat 10.67 ditutup (`2026-10-03-011`). Keduanya berada di
kelas kegagalan yang sama, dan itu alasan keduanya dikerjakan bersama: **batas
yang hilang tanpa suara**.

## Kenapa ini bukan sekadar "menambah validasi"

Batas baris per-peran (10.67) sudah ditegakkan di store untuk baca **dan** tulis.
Yang tersisa adalah dua tempat di mana batas itu bisa **hilang tanpa jejak**, dan
keduanya punya sifat yang berlawanan dengan kegagalan biasa:

1. **`grants` dibaca secara lossy.** `grants` adalah JSON bebas pada entity
   `role`; tidak ada schema yang bisa memeriksanya. `json.Unmarshal` ke `[]Grant`
   **membuang kunci yang tidak dikenal**, jadi `row_scopes:` (alih-alih
   `row_scope:`) tetap menghasilkan permission-nya sambil menghilangkan
   batasnya. Terukur pada 10.67: dapur membaca `draft` lagi, sementara
   `formspec validate` dan `formspec check` sama-sama hijau.
2. **Script membaca lewat jalur lain.** Jalur HTTP sudah menegakkan batas, tetapi
   `resource.find()` memanggil `FindByFields` tanpa predikat dan
   `resource.fetch()` memanggil `GetByID` tanpa predikat. Pemanggil yang sama
   karena itu melihat **dua database berbeda** tergantung lapisan mana yang
   bertanya — dan tidak ada yang gagal, karena script tetap membalas 200.

Keduanya **fail-open**: yang hilang adalah batasnya, bukan permission-nya.

## Yang dikerjakan

**Satu aturan bentuk, dua konsumen.** `auth.ValidateGrantListShape` membaca
nilai `grants` **mentah** dan dipakai oleh (a) gerbang deploy — `cmd/formspec/validate_seed_grants.go`,
dipanggil dari `validate.go` **dan** `check.go` — dan (b) loader role runtime
(`roleFromRecord`). Pola yang sama dengan `spec.QualifyPermission` (10.46): dua
salinan aturan keamanan pasti melenceng, dan salinan yang melenceng adalah yang
berhenti menegakkan.

Diperiksa: `page` wajib, `actions`/`tabs` wajib, `name` per action, dan untuk
`row_scope` — field wajib, `op` di closed set, **tepat satu** sumber nilai
(`value` / `from: session` / `from: route`), kecuali operator tanpa nilai
(`null`/`notnull`/`root`). `from` **dan** `value` bersamaan ditolak: manifest
tidak akan menyatakan mana yang menang, dan separuh yang kalah adalah batasan
yang diyakini operator sedang berlaku. Resolusi page/action dan keberadaan field
di entity target tetap tugas Materializer, memakai validator
`spec.ValidateRowScopeFilters` yang sama (`pkg/spec/rowscope.go`) — diekstrak
dari validasi `row_scope` entity supaya **satu** aturan melayani keduanya.

**Cacat yang menyentuh batas baris kini MENOLAK.** `GrantProblem` membawa
`HadRowScope`, dan resolver mengembalikan error sehingga HTTP menjawab 403 —
bukan menyajikan hasil tanpa batas. Cacat yang hanya menghilangkan grant (page
tidak dikenal) tetap sekadar dilaporkan: ia sudah fail-closed dengan sendirinya,
dan memperlakukannya sebagai masalah batas baris akan mengubah salah ketik
menjadi pemadaman.

**Baca script memakai batas yang sama dengan HTTP.** `scriptRowPredicates`
(`resource/formspec.go`) meresolusi `row_scope` entity **AND** `row_scope` grant
pemanggil untuk permission `view`, dengan nilai dari identitas pemanggil (tidak
pernah dari script). Store mendapat `FindByFieldsScoped`, sehingga predikat masuk
ke query dan baris di luar batas **tidak match** — jawaban yang sama dengan
`GetByID` ber-predikat: tidak ada, bukan 403 (bukan oracle keberadaan).
Resolver yang dipakai **satu**: `appGrantScope.fn = authSvc.GrantRowScope`, holder
yang dibuat sebelum dispatcher (auth service lahir setelahnya).

Fail closed pada: batas grant yang tak terbaca, `from: session` tanpa atribut
(termasuk fallback `assignments`), `from: route` dari script (tidak ada query
string — dilewati berarti melebarkan), dan predikat yang tak bisa diekspresikan.
`SystemCaller` tidak disaring: ia sistem, bukan peran, dan itu diputuskan lapisan
dispatch — **tidak** disimpulkan dari identitas yang absen, karena pemanggil
anonim juga tanpa identitas.

## Bukti

| Gerbang / test                                 | Hasil                                                                                                                     |
| ---------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `cmd/formspec/seed_grants_validation_test.go`  | menolak `row_scopes`, menerima bentuk sah, membedakan cacat grant vs cacat batas baris                                    |
| `internal/auth/grantshape_test.go`             | 7 kasus: typo fail-open, tanpa sumber nilai, operator tak dikenal, `from`+`value`, operator tanpa nilai, cacat grant-only |
| `internal/auth/grant_scope_failclosed_test.go` | resolver menolak; grant yang cuma hilang tidak menolak                                                                    |
| `resource/row_scope_grant_e2e_test.go`         | 2 test baru: batas tak terbaca → 403 (list **dan** GET by id); field tak dikenal → 403                                    |
| `resource/script_row_scope_e2e_test.go`        | kontrol HTTP 404 dulu, lalu `find`/`fetch` mengambil keputusan yang sama; admin `*` tetap membaca semua                   |
| `go test ./...`                                | 39 paket ok                                                                                                               |
| kafe `validate` / `check`                      | 89 manifest 0 problem · 0 error 0 warning                                                                                 |

**Kalibrasi (keduanya menghasilkan ulang kebocorannya):**

- `sed 's/row_scope:/row_scopes:/g'` pada `seeds/roles.yaml` (9 kunci) →
  `formspec check` **1 error** (_"…unknown key `row_scopes` … the action is
  granted on EVERY row [row restriction]"_), `formspec validate` **1 problem**.
- Resolver: dua cabang penolakan di-`if false`-kan → `go test ./resource` gagal
  dengan _"a DRAFT order reached the kitchen"_ (10.67) dan list membalas 200
  lengkap dengan `guest_token` draft.
- `scriptRowPredicates` dibuat mengembalikan `nil` → `script_row_scope_e2e_test.go`
  gagal dengan _"resource.find() matched a row the same caller cannot GET:
  map[found:true secret:draft-secret]"_ — reproduksi 10.74, termasuk nilai yang
  bocor.

## Berkas

- Baru: `pkg/spec/rowscope.go`, `internal/auth/grantshape.go`,
  `internal/auth/grantshape_test.go`, `internal/auth/grant_scope_failclosed_test.go`,
  `cmd/formspec/validate_seed_grants.go`, `cmd/formspec/seed_grants_validation_test.go`,
  `resource/script_row_scope_e2e_test.go`.
- Diubah: `pkg/spec/entity.go` (delegasi ke validator bersama),
  `internal/auth/{grant,materialize,resolver,role,service}.go`,
  `internal/api/{handler,router,scope}.go`, `renderers/jsonb-persist/crud.go`
  (`FindByFieldsScoped`), `resource/formspec.go`, `cmd/formspec/{validate,check}.go`,
  `docs/spec/backend/01-core-basic.md` §1.7, `docs_internal/plan/todo.md` (6.2.6),
  `examples/kafe/gaps_found/TODO.md` (10.73 ✅, 10.74 ✅).

## Sisa

- `ActionGrant.Conditions` tetap inert (6.2.6c) — putuskan apakah dihidupkan
  sebagai kendala atribut **tulis** atau dihapus.
- `resource.upsert` (summary, system-only) dan `setActive` tetap system-caller.

## Tambahan — gerbang sumber atribut sesi (kafe 10.75)

Ditambahkan setelah menguji ulang risiko yang ditulis plan Fase 4 (_"validator
Fase 4 harus menangkap grant `from: session` tanpa assignment"_). Terukur
**belum**, dan ada dua cacat, keduanya pada pohon kafe sungguhan:

1. **Grant tidak diperiksa sumbernya.** `validateScopeSources` hanya berjalan
   atas manifest **Entity**. Menghapus satu-satunya `assignments` di
   `cafe-master.employee` → `validate` melaporkan 10 penolakan entity dan nol
   tentang tiga grant KDS yang kini akan membalas **403** untuk setiap `list`.
2. **`formspec check` tidak menjalankan lapisan scope.** Pada pohon yang sama ia
   melaporkan **0 error / 0 warning** sementara `validate` melaporkan **10
   problem** — dua gerbang yang berbeda pendapat tentang pohon yang sama.

Penutup: `cmd/formspec/validate_grant_scope.go` (aturan yang **sama** dengan
sisi entity: hanya bentuk implisit dinilai; `dimension` **maupun** `field` dari
`assignments` dihitung sebagai sumber; `attr` eksplisit dibiarkan karena nilai
bisa datang dari IdP lewat klaim `attrs`), dan `check.go` kini menjalankan kedua
lapisan scope.

**Bukti:** `cmd/formspec/validate_grant_scope_test.go` (8 kasus).
**Kalibrasi:** pohon dengan `assignments` dihapus → `check` **11 error**
(10 entity + 1 grant), `validate` **10 problem**; dikembalikan → kafe `check`
**0/0**, `validate` **89/0**, `go test ./...` **39 paket ok**.
