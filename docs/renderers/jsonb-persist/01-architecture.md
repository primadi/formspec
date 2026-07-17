# Arsitektur jsonb-persist

**Updated:** 2026-07-16 · Status: Outline

> Outline: heading menetapkan cakupan; isi ditulis bertahap dari kode `internal/db/`.

## 1. Hybrid JSONB
Kolom inti relasional + payload JSONB. Tabel per Document mengikuti struktur
normatif berikut (dijawab dari kontrak storage-agnostic di
[`../../spec/backend/01-core-basic.md`](../../spec/backend/01-core-basic.md)
§1–§2 dan [`../../spec/backend/04-persist-backend.md`](../../spec/backend/04-persist-backend.md)):

```sql
CREATE TABLE {schema}.{module}_{plural} (
  id          uuid        PRIMARY KEY DEFAULT gen_uuid_v7(),  -- SQLite: integer PRIMARY KEY AUTOINCREMENT
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

**Penyimpangan dari kontrak:** tipe kolom `id` berbeda per driver — `uuid` di
Postgres, tapi `integer PRIMARY KEY AUTOINCREMENT` di SQLite. Kontrak
[`../../spec/backend/01-core-basic.md`](../../spec/backend/01-core-basic.md)
§2 mewajibkan PK **selalu** UUID v7, tanpa pengecualian per backend — jalur
SQLite hari ini tidak memenuhi itu. Dicatat sebagai gap di §4, bukan
dijadikan pembenaran diam-diam bahwa SQLite "boleh beda".

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
| Structural diff apply | `internal/db/migrate.go` — `PlanMigrations`/`ApplyMigrations` menerjemahkan diff Document ke DDL Postgres/SQLite |
| Query resolution | Operator filter kontrak diterjemahkan ke SQL/JSONB path — lihat [`04-query-and-keys.md`](04-query-and-keys.md) §1 |
| Tree query resolution | Operator `descendant_of`/`ancestor_of`/`child_of`/`root` diterjemahkan ke prefix-match materialized path, **tanpa recursive CTE** — [`02-schema-strategies.md`](02-schema-strategies.md) §4, [`04-query-and-keys.md`](04-query-and-keys.md) §6 |
| `ctx.next_key` | Tabel counter `forma_natural_key_counters`, alokasi di bawah lock — [`04-query-and-keys.md`](04-query-and-keys.md) §2 |
| Index generation | Generated column dari `data`, indexed — [`02-schema-strategies.md`](02-schema-strategies.md) §3 (**catatan dialek**, lihat §4 di sana) |
| Uninstall extension bersih | **Belum diimplementasikan** — DDL `ADD COLUMN ext_*` dibuat saat extension dipasang, tapi tidak ada `DROP COLUMN`/rute uninstall di kode manapun. Lihat [`02-schema-strategies.md`](02-schema-strategies.md) §2. |

## 3. Transaksi, Outbox, Audit
**Klaim awal dokumen ini (insert/update Document dan penulisan outbox
terjadi dalam satu transaksi) tidak akurat — dikoreksi di sini.** Kode nyata
hari ini: `DB.BeginTx`/`Tx` ada di interface dan diimplementasikan kedua
driver, tapi **tidak pernah dipanggil pemanggil manapun** di
`internal/db`/`internal/api`/`internal/action`. Penulisan outbox
(`OutboxStore.Enqueue`) dipanggil dari `action.DeliverEvents` **setelah**
mutasi CRUD selesai, sebagai langkah terpisah best-effort — errornya cuma
di-log, tidak membatalkan mutasi yang sudah commit. Konsekuensinya: kontrak
durabilitas "publisher durable" mensyaratkan mutasi data dan entry outbox
atomik dalam satu transaksi
([`../../spec/backend/01-core-basic.md`](../../spec/backend/01-core-basic.md)
§7) — backend ini **belum memenuhi itu** hari ini. Ini gap nyata, bukan
detail kosmetik: kalau proses mati tepat setelah commit mutasi tapi sebelum
`Enqueue` outbox berjalan, event itu hilang tanpa jejak.

## 4. Status Implementasi Hari Ini
- `internal/db.DB`/`Tx` belum jadi seam PersistBackend yang bersih — bocor
  semantik SQL (`ExecContext`, `QueryContext`, `Driver() *sql.DB`) ke
  pemanggil, dan migration engine menghasilkan `DDLResult` (teks SQL) sebagai
  representasi diff-nya, bukan diff storage-agnostic yang baru diterjemahkan
  belakangan.
- Mutasi entity dan penulisan outbox **tidak atomik** (§3) — `BeginTx` tidak
  pernah dipakai.
- PK SQLite bukan UUID v7 (§1) — penyimpangan kontrak yang belum ditutup.
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
