# 2026-10-03-011 — Batas baris per PERAN pada grant; 10.67 & GAP-08 ditutup

**Tanggal:** 2026-10-03
**Plan:** `docs_internal/plan/grant-row-scope.md` · `docs_internal/plan/lapisan-otorisasi.md`
**Menutup:** ledger kafe **10.67** (aturan #1 ditegakkan kolom Kanban) · **GAP-08**
(penyaring cabang KDS hanya `fixed_filters` klien)
**Terkait:** `2026-10-03-006` (batas 10.67 dipetakan), `2026-10-03-008` (store = titik penegakan)

## Apa yang diubah

**Nilai literal pada `FilterSpec`.** `pkg/spec/frontend.go` mendapat `value`.
Sebelumnya `row_scope` hanya punya dua sumber nilai (`from: session|route`), dan
`switch sc.From` di `internal/api/scope.go` **jatuh diam** untuk entri tanpa
`from` — deklarasi yang tidak menyaring apa pun tetapi terlihat seperti
perlindungan. Kini `from` kosong berarti literal `value`; entri yang tidak punya
sumber nilai **maupun** literal ditolak validator entity dan gagal-closed di
runtime. `op: in`/`nin` membaca daftar berkoma, `op: between` tepat dua batas,
dan bentuk yang tidak memenuhi syarat operatornya ditolak — bukan dibiarkan
menjadi predikat kosong.

**`row_scope` per action pada grant.** `internal/auth/grant.go`:
`ActionGrant.RowScope []spec.FilterSpec`. Ia menutup celah yang tidak bisa
dijangkau `row_scope` entity: deklarasi itu **per-entity** dan memfilter
berdasarkan **siapa** (atribut sesi), sehingga filter `status` di sana akan
membutakan **kasir** terhadap draft yang sedang ia susun — dan itulah sebabnya
aturan #1 kafe tidak pernah bisa dinyatakan sebagai batas. Grant bersifat
per (role, action), yang memang granularitas yang dibutuhkan.

**Materialisasi membawa batasnya.** `Materializer.MaterializeDetailed`
mengembalikan `{Page, Action, Permission, RowScope}`; `MaterializePartial`
mendelegasi (signature lama tidak berubah). `PermissionResolver.GrantScope`
menjawab "batas baris untuk permission ini" dengan cache 30 detik
(`GrantScope` di `resolver.go`), di-invalidasi bersama cache permission.

**Penegakan di SATU tempat.** `HandlerFactory.rowPredicatesFor` menyatukan
`row_scope` entity + batas grant, dan dipakai jalur **baca** (`list`, `find`)
maupun **tulis** (`update`, `delete`). Predikatnya di-AND **di SQL** melalui
`ListParams.RowPredicates` / `GetByIDParams.RowPredicates` /
`UpdateParams.RowPredicates` / `DeleteParams.RowPredicates` — bukan digabung ke
map filter, karena map akan saling menimpa untuk field yang sama sementara AND
hanya bisa menyempitkan. `renderers/jsonb-persist/crud.go` mendapat
`filterSQL`/`predicateClauses` sebagai **satu** implementasi operator yang
dipakai filter klien, `row_scope` entity, dan batas grant sekaligus.

**Baris di luar batas = 404, bukan 403.** Predikat ikut ke dalam query, jadi
baris di luar batas **tidak cocok** — sama seperti `list` yang menyembunyikan
baris itu. 403 tetap dipakai untuk batas yang **tidak bisa diselesaikan** (salah
konfigurasi), agar kesalahan manifest berisik. Cache read-through dilewati saat
ada predikat: kuncinya `(workspace, module, entity, id)` — bukan per-pemanggil —
sehingga menyajikan pembacaan ter-scope darinya akan menyerahkan baris orang lain.

**Penegakan tulis.** `SoftDelete` diperluas dari `(ctx, workspaceID, id)` menjadi
`DeleteParams{WorkspaceID, ID, Permissions, SystemCaller, DeletedBy, RowPredicates}`
— tanpa struct param tidak ada tempat menaruh principal, dan itulah sebabnya
`delete` adalah satu-satunya jalur tulis tanpa identitas (10.45). Seluruh
pemanggil produksi dan test dimigrasikan; tiga di antaranya (`session.go` ×3,
`workspace.go`, `archive.go`) menandai dirinya `SystemCaller: true` karena
memang berjalan tanpa pengguna.

**Adopsi kafe (`roles.yaml`).** `order-page` **barista** & **dapur** mendapat
`row_scope` status `in paid,in_kitchen,ready,served` **plus** `branch_id eq from
session`; **pelayan** `in ready,served,completed`; kasir/supervisor/manajer
**tanpa** batas status (mereka harus melihat draft). Sekaligus menutup GAP-08:
penyaring cabang KDS kini ditegakkan server, bukan digabung browser.

## Kenapa

Ledger 10.67 mengukur jaraknya: `kanbans/order-board-kds.yaml` hanya
mendeklarasikan kolom `paid`/`in_kitchen`/`ready`, sehingga pembatasan itu
**tidak** ada di otorisasi — barista bisa `GET .../order/{id}` atas pesanan
`draft` dan menerima **200** beserta `guest_token` (kunci akses pesanan tamu di
permukaan publik). GAP-08 kelasnya sama pada sumbu lain: `fixed_filters` digabung
di browser, jadi `curl` cukup menghilangkannya. Keduanya adalah aturan yang
dinyatakan sebagai aturan tetapi hanya hidup di deklarasi tampilan.

## Bukti

**Kalibrasi (fitur dimatikan → test gagal):**

| Guard | Cara mematikan | Hasil |
| --- | --- | --- |
| cabang literal `row_scope` | `case ""` dilewati di `applyRowScope` | 3 test api + 1 test spec **gagal** |
| predikat di SQL | `predicateClauses` mengembalikan nol | 5 test `jsonb-persist` **gagal** |
| wiring `GrantScope` | `SetGrantScopeLookup` menyetel `nil` | test e2e kafe **gagal**: *"a DRAFT order reached the kitchen: [ORD-…01 …02 …03]"* — persis pengukuran 10.67 |
| kunci `row_scope` di seed | `row_scope:` → `row_scopes:` | 2 test seed **gagal**, termasuk *"carries NO row scope — the kitchen would read drafts again"* |

**E2E kafe in-process (spec kafe nyata, HTTP nyata):**

| Uji | Hasil |
| --- | --- |
| barista `list order` | 3 baris → **2** (draft tidak muncul) |
| barista `list?status[in]=draft` (query yang dipakai ledger) | draft **tetap tidak muncul** — klien tidak bisa melebarkan |
| barista `GET order/{draft}` | **404** — bukan 200 seperti pengukuran 10.67 |
| barista `PATCH {status: in_kitchen}` pada draft | **404**, dan status di DB **tetap `draft`** (ditolak sebelum tulis) |
| barista cabang A: order cabang B | tidak terlihat; `?branch_id[eq]=B` tetap hanya cabang A |
| pelayan `list` | hanya `ready` (draft & paid tidak terlihat) |
| **kasir** `list` | draft **+** paid terlihat — aturan per-peran tidak membutakan POS |
| role ter-scope tanpa `assignment` | **403** (fail closed, bukan "tanpa filter") |

**Test baru:** `internal/api/scope_literal_test.go` (4) · `renderers/jsonb-persist/row_predicate_test.go` (6 fungsi, 4 subtest fail-closed) · `internal/auth/grant_row_scope_test.go` (3) · `internal/api/grant_scope_test.go` (7) · `internal/auth/kafe_seed_grants_test.go` (4, memakai seed kafe yang sebenarnya) · `resource/row_scope_grant_e2e_test.go` (5) · klien `src/lib/grants.test.ts` (10).

**Suite:** `go test ./...` 39 paket `ok` · `golangci-lint` **0 issues** ·
`vitest` **626** lulus (50 berkas) · `tsc -b` bersih · `npm run lint` 0 error ·
`formspec validate --schema ../../schemas` **89 manifest, 0 problem** ·
`formspec check` **0 error, 0 warning** · `make generate-schema` dijalankan
(`FilterSpec.value` masuk schema).

## Cacat yang ikut ditemukan & diperbaiki di jalur ini

1. **`grantKey(page, tab, action)` dipanggil dengan posisi argumen salah** di
   modul klien yang baru — seorang action tanpa tab menjadi key `page` saja,
   sehingga batas barisnya menempel ke halaman, bukan ke aksi. Ditemukan oleh
   test serialisasi itu sendiri (bukan oleh review), dan itulah alasan konversi
   ini dipindah dari komponen ke modul yang punya test.
2. **Daftar predikat yang dikosongkan tidak menghapus batasan** — `scopeFor`
   jatuh kembali ke nilai tersimpan, sehingga batas baris tidak bisa dihapus dari
   UI. Kini `predicates` yang diberikan untuk sebuah key adalah otoritasnya
   (daftar kosong = hapus), bukan sekadar fallback.
3. **`permission.AutoPrefixPermission` yang sudah deprecated masih dipakai di 6
   situs** (`generator.go` ×3, `handler.go`, `check_ungated.go`,
   `materialize.go` ×2) — aturan auto-prefix yang harusnya satu sumber sudah
   bercabang menjadi dua implementasi. Semua dimigrasikan ke
   `spec.QualifyPermission`, yang penting di sini karena store dan HTTP harus
   mengkualifikasi **persis sama** atau gerbang transisinya tak pernah bisa
   dibuka (mode kegagalan 10.47). Dua fungsi mati (`assetSource`, `isModule`)
   ikut dihapus dan satu `ineffassign` dibersihkan agar `make lint` kembali 0.

## Sisa yang dicatat (bukan diklaim selesai)

- **Pemeriksaan grant saat `formspec check` belum ada** — `grants` adalah JSON
  bebas pada entity `role`; yang ada sekarang adalah test materialisasi atas seed
  kafe. Sebuah aplikasi yang menulis grant di luar seed tidak punya gerbang
  statis. → item ⏸️ di master todo.
- **Urutan evaluasi predikat tidak dijamin** untuk dua batasan pada field yang
  sama dengan operator berbeda (mis. `gt` dan `lt`); hasilnya benar karena AND
  komutatif, tetapi pesan error yang menyebut field tidak menunjukkan batasan
  mana yang menolak.
