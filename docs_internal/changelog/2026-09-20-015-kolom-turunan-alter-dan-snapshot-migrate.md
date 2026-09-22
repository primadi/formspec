# 3.11 + 3.10 — Kolom turunan hasil ALTER & snapshot yang memblokir migrate

**Tanggal:** 2026-09-20 · **TODO:** `examples/kafe/gaps_found/TODO.md` 3.11 & 3.10 ·
**Master todo:** 15.7 · **Plan:** `docs_internal/plan/kafe-sisa-gap.md`

## Masalah

Dua bug migrasi ditemukan saat walkthrough kafe (9.4), keduanya di DB dev yang
sudah ada:

1. **3.11 — aturan bisnis lolos.** Dua shift `open` untuk (cabang, kasir) yang
   sama **diterima** (201), padahal partial unique index
   `(branch_id, cashier_id) WHERE status='open'` ada di DB. `sqlite_master`:
   `_cashier_id text` — kolom **biasa** hasil ALTER, tidak pernah terisi (NULL),
   dan NULL tidak pernah bentrok di unique index.
2. **3.10 — migrate terblokir.** `formspec migrate apply` menolak seluruh run:
   18× `[lossy] field_removed is_active` (tidak bisa dideklarasikan — field itu
   bukan milik manifest) + 8× `[never] table_removed formspec_core_*`.

## Akar

**3.11:** SQLite menolak `ALTER TABLE ADD COLUMN … GENERATED ALWAYS … STORED`,
jadi jalur alter menambahkan kolom polos — padahal jalur INSERT hanya menulis
`(id, tenant_id, version, data)` dan mengandalkan kolom menghitung dirinya
sendiri. Kolom polos = NULL selamanya = index tanpa gigi.

**3.10:** dua akar, bukan "snapshot lama salah" seperti yang dicurigai ledger:
(a) `spec.ValidateEntitySpec` bukan hanya validator — ia **meng-inject** field
milik engine (`is_active` dari `soft_deactivate`). Server mendaftarkan entity
lewat jalur yang memanggilnya; `formspec migrate`/`diff` hanya memanggil
`RawSpecToEntitySpec` → bentuk CLI tanpa `is_active` vs snapshot server dengan
`is_active` → selisihnya muncul sebagai "field dihapus". (b) Server
mendaftarkan `formspec.core.*` saat runtime; CLI yang hanya memuat spec tree
pengguna melaporkan tabelnya sebagai `table_removed` — perubahan yang
mustahil dideklarasikan.

## Perbaikan (`renderers/jsonb-persist`, `internal/manifest`, `cmd/formspec`)

0. **CREATE dan ALTER berbeda tentang "field mana yang punya kolom".** Field
   `relation` dengan `index: true` tapi tanpa `foreign_key` eksplisit tidak pernah
   dapat kolom turunan di jalur CREATE TABLE (jalur relation `continue` sebelum
   menangani `index:`), padahal `derivedColumnFields` (sumber tunggal aturan)
   menandainya. Akibatnya DB segar tampak "drift" setiap kali re-plan: snapshot
   bilang field itu derived, tabel tidak punya kolomnya. Perbaikan: jalur CREATE
   kini membuat kolom + index untuk relation ber-`index:`/`unique:` tanpa
   `foreign_key` (tipe text = id referensi), selaras dengan 3.1 ("field relation
   dapat kolom turunan"). Dampak pada DB lama: 5 `index_added` aditif → apply →
   konvergen.
1. **`addDerivedColumnSQL`** memakai `GENERATED ALWAYS AS (…) VIRTUAL` di
   SQLite (PostgreSQL tetap STORED) — kolom dihitung saat baca, benar untuk
   baris lama maupun baru.
2. **`generatedColumnExpr`** (`ddl.go`) — satu sumber ekspresi untuk jalur
   CREATE TABLE dan ALTER, jadi keduanya tidak mungkin berbeda.
3. **`generatedColumns`** — introspeksi kolom yang benar-benar generated
   (SQLite `pragma_table_xinfo.hidden IN (2,3)`; PostgreSQL
   `information_schema.columns.is_generated='ALWAYS'`).
4. **`diffExistingTable`** membangun ulang kolom stale: DROP index dependen
   dulu (`indexTouchesColumns`), DROP COLUMN, ADD COLUMN generated, CREATE
   index kembali. Rekonseilasi storage kini satu jalur untuk semua kasus
   (snapshot-diff, checksum-sama, bootstrap) — menggantikan `driftedIndexes`
   yang hanya melihat index; kind perubahan baru `storage_drift`.
5. **`manifest.EntitySpecFromRaw`** — parse + validate (normalisasi) dalam satu
   panggilan; dipakai jalur migrate. Komentarnya menyatakan kontraknya.
6. **`MigrationRunner.IgnoreModules("formspec.core")`** — snapshot modul
   framework tidak dilaporkan sebagai entity dihapus; dipanggil `formspec
   migrate` dan `formspec diff`.

## Bukti

| Perintah / aksi | Hasil |
| --- | --- |
| `go test ./renderers/jsonb-persist/ -run Altered\|Stale\|IgnoresFramework` | PASS (3 test pengunci baru) |
| `go test ./...` | hijau (semua paket) |
| `make lint` | 0 issues |
| `vitest run` (react-shadcn) | 288 lulus · `tsc` bersih |
| kafe `validate --schema ../../schemas` | 69 manifest, 0 problem |
| `migrate apply` DB segar | `Applied 24 migration(s)` → re-plan `No pending migrations` → `diff` `No differences` |
| `migrate plan` (DB kafe lama) | 8× `storage_drift` + 5× `index_added` — nol `field_removed is_active`, nol `table_removed formspec_core_*` |
| `migrate apply` (DB kafe lama) | `Applied 8` + `Applied 5 migration(s)` |
| `migrate plan` ulang | `No pending migrations` · `formspec diff` → `No differences` |
| INSERT shift `open` kedua (cabang, kasir sama) | **REJECTED** `UNIQUE constraint failed: …_branch_id, …_cashier_id` |
| INSERT shift kasir lain / shift `closed` | diterima (partial index benar) |

## Sisa

- PostgreSQL jalur baru (`is_generated`, `DROP INDEX <schema>.<name>`) belum
  pernah dijalankan di DB nyata — tetap tercatat di master todo 15.8.
- Skenario 9.4 UI browser (keranjang QR, report tampilan) tetap tercatat di 9.4;
  skenario 6 (void approval) ⬜, 8 (jurnal GL) ⛔ terhalang 6.3; 6.3/6.4 tetap
  deferred (Control Plane / keputusan desain).

## Lanjutan run yang sama (2026-09-20, verifikasi walkthrough)

Walkthrough 9.4 dijalankan penuh pada data dev yang dikurasi ulang. Hasil:

- **Skenario 1–5, 7, 9 ✅** — rantai QR anonim penuh (katalog → table-session →
  order `ORD-2026-00005`), bayar tunai (`change 7500 IDR`), KDS queue,
  kanban barista (`paid → in_kitchen → ready → served → completed`), shift +
  kas (`difference` computed benar), stok/HPP (moving average `60 IDR`),
  struk thermal ESC/POS nyata (init/bold/cut, 348 byte).
- **Sisa proyeksi `stock-level` dituntaskan** — `stock_value`,
  `last_movement_at`, `is_below_min` kini diisi `stock_level_apply.star`;
  akses `cafe-stock.ingredient` dideklarasikan lewat `hooks[].uses.resources`
  (honesty check menolaknya tanpa itu). Bukti: `value 54300` = 905×60,
  `is_below_min True` saat qty 905 < min 1000.
- **Temuan baru:** `expected_cash` TIDAK punya mekanisme compute di spec
  (deskripsi menyebut "compute" tapi tidak ada `computed:`/script) — dicatat
  di ledger 9.4 sebagai keputusan produk, bukan gap engine.
- **Temuan DX:** error compile script pada hook `after` **senyap** bagi
  pemanggil (RuntimeLogger default = no-op, respons tetap 201) — penemuan
  butuh menanam log sementara di `RunAfterPhase`. Kelas yang sama dengan
  3.12 (money/float); dicatat, bukan diperbaiki di run ini karena perubahan
  logging menuntut keputusan UX operator.
