# Arsitektur jsonb-persist

**Updated:** 2026-07-20 · Status: Outline

> Outline: heading menetapkan cakupan; isi ditulis bertahap dari kode `renderers/jsonbpersist/`.

## 1. Hybrid JSONB
Kolom inti relasional + payload JSONB. Tabel per Document mengikuti struktur
normatif berikut (dijawab dari kontrak storage-agnostic di
[`../../spec/backend/01-core-basic.md`](../../spec/backend/01-core-basic.md)
§1–§2 dan [`../../spec/backend/04-persist-backend.md`](../../spec/backend/04-persist-backend.md)):

```sql
CREATE TABLE {schema}.{module}_{plural} (
  id          uuid        PRIMARY KEY,               -- app-generated UUID v7, both drivers (SQLite column type: text)
  tenant_id   uuid        NOT NULL,
  version     integer     NOT NULL DEFAULT 1,      -- optimistic concurrency
  doc_status  text,                                 -- NULL = lifecycle-free
                                                    -- 'draft' | 'submitted' | 'cancelled'
  amends      uuid,                                 -- UUID original yang di-cancel oleh amend
  amended_by  uuid,                                 -- UUID versi baru dari amend
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,                          -- kolom ini absen kalau persist.soft_delete: false
  created_by  uuid, updated_by uuid,
  data        jsonb       NOT NULL DEFAULT '{}'
);
```

**Kontrak §2 terpenuhi:** PK adalah UUID v7 di kedua driver, digenerate di
app layer (`NewUUIDv7` di `renderers/jsonbpersist/tx.go`) dan disertakan
eksplisit pada tiap `INSERT` — bukan `DEFAULT` khusus driver. Sebelumnya
SQLite memakai `integer PRIMARY KEY AUTOINCREMENT`; itu sudah ditutup, tidak
lagi jadi penyimpangan per-backend.

Alasan desain: field bisnis (di `data`) berubah bentuknya jauh lebih sering
daripada kolom struktural (`id`, `tenant_id`, `version`, lifecycle,
audit timestamp) — memisahkan keduanya membuat structural diff (§2, migration
engine [`03-migration-engine.md`](03-migration-engine.md)) hanya perlu
menyentuh DDL saat kolom struktural berubah, bukan setiap kali field bisnis
ditambah. Trade-off: field di `data` tidak ter-index secara native — field
yang butuh index/query cepat diangkat jadi *generated column* (§ Index
Generation, [`02-schema-strategies.md`](02-schema-strategies.md) §3).

## 2. Pemenuhan Kontrak PersistBackend
Pemetaan tiap kemampuan wajib
([`../../spec/backend/04-persist-backend.md`](../../spec/backend/04-persist-backend.md)
§2) ke mekanisme konkret backend ini:

| Kemampuan kontrak | Mekanisme jsonb-persist |
|---|---|
| Structural diff apply | `renderers/jsonbpersist/migrate.go` — `PlanMigrations`/`ApplyMigrations` menerjemahkan diff Document ke DDL Postgres/SQLite |
| Query resolution | Operator filter kontrak diterjemahkan ke SQL/JSONB path — lihat [`04-query-and-keys.md`](04-query-and-keys.md) §1 |
| Tree query resolution | Operator `descendant_of`/`ancestor_of`/`child_of`/`root` diterjemahkan ke prefix-match materialized path, **tanpa recursive CTE** — [`02-schema-strategies.md`](02-schema-strategies.md) §4, [`04-query-and-keys.md`](04-query-and-keys.md) §6 |
| `ctx.next_key` | Tabel counter `forma_natural_key_counters`, alokasi di bawah lock — [`04-query-and-keys.md`](04-query-and-keys.md) §2 |
| Index generation | Generated column dari `data`, indexed — [`02-schema-strategies.md`](02-schema-strategies.md) §3 (**catatan dialek**, lihat §4 di sana) |
| Uninstall extension bersih | **Belum diimplementasikan** — DDL `ADD COLUMN ext_*` dibuat saat extension dipasang, tapi tidak ada `DROP COLUMN`/rute uninstall di kode manapun. Lihat [`02-schema-strategies.md`](02-schema-strategies.md) §2. |

## 3. Transaksi, Outbox, Audit
`DB.BeginTx`/`Tx` sekarang benar-benar dipakai lewat `InTx` (helper di
`renderers/jsonbpersist/tx.go`): `EntityStore.Insert`/`Update`/`SoftDelete`
membungkus seluruh isi mutasinya — UPSERT counter natural key, `INSERT`/
`UPDATE` baris utama, sinkronisasi child table, audit log, dan (untuk
Insert/Update) enqueue outbox — dalam satu transaksi, commit semua atau
rollback semua. Ini menutup dua gap yang sebelumnya dicatat di sini:

1. **Counter natural key tidak lagi bisa "terpakai" tanpa insert** — UPSERT
   counter (`NaturalKeyCounter.NextSequence`) jalan di koneksi transaksi yang
   sama dengan `INSERT` baris; kalau insert gagal (validasi, guard,
   constraint), rollback membatalkan increment counter juga. Lihat
   [`04-query-and-keys.md`](04-query-and-keys.md) §2.
2. **Mutasi + outbox atomik untuk jalur create/update** — `internal/api`'s
   `HandleCreate`/`HandleUpdate` me-resolve emission event yang `durable`
   *sebelum* memanggil `Insert`/`Update`, lalu mengirimkannya lewat
   `InsertParams`/`UpdateParams.PendingEvents`; `EntityStore` meng-enqueue-nya
   ke `forma_outbox` di transaksi yang sama dengan baris entity
   (`enqueueOutbox` di `outbox.go`, dipanggil dengan DB yang terikat
   transaksi, bukan `s.db` polos). `action.DeliverEvents` yang dipanggil
   sesudahnya menerima flag `outboxAlreadyEnqueued=true` sehingga tidak
   meng-enqueue ulang — perannya di jalur ini murni pengiriman best-effort
   (push websocket langsung, tulis event log non-durable).

**Custom action (Starlark/native/sidecar) kini juga atomik** lewat
`TxScope` (`renderers/jsonbpersist/txscope.go`). `HandleCustomAction`
membuka satu `TxScope` per eksekusi action, membungkusnya ke `ctx`
(`db.WithTxScope`); setiap `EntityStore.Insert`/`Update`/`SoftDelete`/
`UpdateFields`/`IncrementField`/`DecrementField` yang dipanggil dalam satu
eksekusi itu — lewat `resource.save()`/`resource.create()` Starlark,
handler native, atau callback sidecar (`/ctx/entity/{op}`, dikorelasikan
lewat header `X-Forma-Scope-Id`, lihat
[`../runtimes/04-forma-sidecar.md`](../runtimes/04-forma-sidecar.md) §4.3a)
— ikut transaksi yang sama alih-alih commit sendiri-sendiri. Enqueue outbox
untuk event durable juga naik ke transaksi yang sama (`EnqueueOutboxTx`),
baru `scope.Commit()` di akhir; error di titik mana pun → `scope.Rollback()`
membatalkan semua mutasi dalam eksekusi itu, bukan cuma mutasi terakhir.

Kontrak kedua yang ditegakkan `TxScope`: transaksi boleh mencakup
**banyak Module selama semuanya berbagi satu Datastore fisik yang sama**
(hari ini selalu begitu — belum ada Datastore per-Module, Fase 2.9) —
`TxScope.join` membandingkan **identitas store** (`db.DB` yang mendasari),
bukan nama Module. Baru kalau dua Module benar-benar terikat ke Datastore
berbeda (nanti, saat Fase 2.9 ada), mutasi kedua akan gagal dengan
`ErrCrossStoreTx` — mencegah dua transaksi SQL dipaksa berbagi satu
koneksi yang secara fisik tidak mungkin, bukan melarang orkestrasi
lintas-Module itu sendiri (yang tetap sah selama satu Datastore).

**Gap yang tersisa, dicatat eksplisit, bukan disembunyikan:**
`action.RunAfterPhase` masih tidak mengembalikan error (fire-and-forget,
tidak berubah) — mutasi di after-hook yang gagal di situ tidak memicu
rollback scope, sama seperti perilaku sebelum `TxScope` ada. SDK
`lib-forma-*` (PHP/Python/TypeScript/dll di `sdk/`) belum ada yang mengirim
`X-Forma-Scope-Id` — sampai diperbarui, callback `ctx.entity.*` dari app
process sidecar tetap commit independen (bukan regresi, gap follow-up
terpisah).

## 4. Status Implementasi Hari Ini
- `renderers/jsonbpersist.DB`/`Tx` belum jadi seam PersistBackend yang
  bersih — bocor semantik SQL (`ExecContext`, `QueryContext`,
  `Driver() *sql.DB`) ke pemanggil, dan migration engine menghasilkan
  `DDLResult` (teks SQL) sebagai representasi diff-nya, bukan diff
  storage-agnostic yang baru diterjemahkan belakangan.
- Mutasi entity + counter natural key + outbox **atomik untuk create/update
  DAN custom action** (§3) — `BeginTx` dipakai lewat `InTx` (jalur mandiri)
  atau `TxScope` (jalur request-scoped, lintas beberapa panggilan store
  dalam satu eksekusi action). Sisi SDK sidecar untuk header
  `X-Forma-Scope-Id` belum diperbarui (gap tercatat di §3).
- PK UUID v7 di kedua driver (§1) — penyimpangan SQLite sebelumnya sudah
  ditutup.
- Uninstall extension bersih (§2 tabel) belum ada implementasinya sama
  sekali, bukan cuma belum lengkap.
- `ExtensionStore` (baca/tulis kolom `ext_*` saat request) ada di kode tapi
  **tidak pernah dipanggil** dari `EntityStore`/HTTP handler manapun — kolom
  extension dibuat migrasi tapi belum bisa diisi/dibaca lewat jalur apa pun
  hari ini. Lihat [`02-schema-strategies.md`](02-schema-strategies.md) §2.

Dicatat sebagai gap arsitektural — lihat
[`../../spec/backend/04-persist-backend.md`](../../spec/backend/04-persist-backend.md)
§8 dan [`../../architecture/08-repo-structure.md`](../../architecture/08-repo-structure.md)
§4.
