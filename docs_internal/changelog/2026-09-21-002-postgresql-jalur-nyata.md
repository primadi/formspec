# 15.8 — PostgreSQL dijalankan nyata: 9 bug jalur PG (dan 1 bug SQLite diam-diam)

**Tanggal:** 2026-09-21 · **TODO:** master todo 15.8 ·
**Setup:** PG 17.11, instance user-sendiri (`initdb -U vscode`, port 55432, trust)

## Konteks

Jalur PostgreSQL selama ini hanya pernah dikompilasi, tidak pernah dijalankan
(dev hanya SQLite). Item 15.8 mengharuskan verifikasi di DB nyata. Setup:
instal `postgresql` via apt (sudo tersedia untuk apt; `pg_ctlcluster` milik
root tidak bisa dikendalikan) → buat instance sendiri di `/tmp/pgdata-kafe`
→ `createdb kafe` → `formspec migrate apply --spec spec --dsn postgres://…`.

## 9 bug yang ditemukan (semuanya diperbaiki)

| # | Bug | Gejala | Fix |
| --- | --- | --- | --- |
| 1 | DEFAULT tabel sistem = nama tipe (`DEFAULT timestamptz`) | PG: error keras. SQLite: quirk diterima — **kolom sistem di DB SQLite yang ada menyimpan literal `"text"`** sebagai timestamp (terbukti di kafe.db) | `currentTimestampFn` per dialect |
| 2 | `gen_uuid_v7()` tidak ada di PG ≤17 | `function gen_uuid_v7() does not exist` | `gen_random_uuid()` (PG 13+) |
| 3 | `existingColumns` schema kosong → cek kolom selalu "tidak ada" | ALTER escalated_steps gagal "already exists" | `COALESCE(NULLIF($1,''), current_schema())` |
| 4 | Schema kategori tidak pernah dibuat | `schema "operational" does not exist` | `CREATE SCHEMA IF NOT EXISTS` di EnsureSystemTables |
| 5 | CHECK enum memakai `json_extract` unconditional | `function json_extract(jsonb, unknown) does not exist` | `payloadExpr(driver, …)` |
| 6 | pgx stdlib tidak mendukung placeholder `?` | `syntax error at or near ","` pada statement parameterized pertama | rewriter `?`→`$n` (string-safe, stabil) di `PostgresDB`+`pgTx`; operator jsonb `?` → `jsonb_exists(...)` |
| 7 | Ekspresi kolom generated bertipe text untuk kolom timestamptz/numeric; cast text→date/time tidak immutable | dua error berbeda dari PG | cast non-text inline; date/time lewat fungsi IMMUTABLE `formspec_to_timestamptz`/`formspec_to_date` |
| 8 | `tenant_id`/`created_by`/`updated_by` bertipe `uuid` di PG, app menulis `kafe`/`anonymous` | setiap INSERT gagal "invalid input syntax for type uuid" | tiga kolom `text` di kedua dialect |
| 9 | Inline partial UNIQUE bukan sintaks PG | `syntax error at or near WHERE` | partial lewat `CREATE UNIQUE INDEX … WHERE` |
| 10 | Normalisasi indexdef PG memotong `::` sebelum tanda kutip | index enum-predicate dilaporkan drift selamanya | strip token cast yang diketahui + buang parens; test `TestIndexShapeOf_PostgreSQLIndexDef` |

Bug 1 adalah temuan paling berbahaya kelasnya: **diam-diam salah di semua DB
SQLite yang pernah dibuat** — `formspec_schema_snapshot.updated_at`,
`formspec_schema_migrations.applied_at`, dst. selama ini menyimpan kata
`"text"`, bukan waktu.

## Bukti E2E (PG 17, spec kafe)

| Langkah | Hasil |
| --- | --- |
| fresh DB → `migrate apply` | `Applied 24 migration(s)` |
| INSERT `tenant_id='kafe'`, `created_by='anonymous'` | berhasil (dulu: `invalid input syntax for type uuid`) |
| `migrate plan` | `No pending migrations` (konvergen) |
| field_added (indexed) | `Applied 1`, kolom `_probe_code` + index ada |
| field dihapus tanpa deklarasi | **ditolak**: `[lossy] field_removed … (1 row(s) affected)` — preflight `jsonb_exists` menghitung baris |
| dideklarasikan `removed: true` + reason | `Applied 1`; payload ter-strip (`data - 'x'`); kolom + index hilang (`DROP COLUMN` + `DROP INDEX`) |
| re-plan akhir | `No pending migrations` — PG dan kedua DB SQLite tetap konvergen |

## Sisa (dicatat, bukan disembunyikan)

- **`ddl_by: postgres` — TERVERIFIKASI (2026-09-21, sisa terakhir 15.8 tuntas):**
  `raw_ddl` dengan `ddl_by` dua dialect dideklarasikan di spec copy kafe →
  `[additive] raw_ddl_added` → apply → index PG ada dengan definisi benar
  (`upper((data ->> 'code'::text))`); varian SQLite tidak pernah jalan di PG
  (`count(*)=0`) dan sebaliknya; jalur SQLite → varian sqlite-nya dibuat;
  semantik **forward-only** terverifikasi: deklarasi dihapus →
  `[derived] raw_ddl_removed … the DDL stays applied (forward-only)` → index
  tetap ada, konvergen.
- DB SQLite yang sudah ada masih menyimpan `"text"` di kolom timestamp sistem
  (perbaikan butuh rebuild tabel; nilai tidak dibaca logika mana pun).
- Runtime penuh `formspec dev` di PG belum diuji; query runtime sudah
  driver-aware dan rewriter menjangkau semuanya, tapi boot lengkap (auth,
  registry, SPA) di PG adalah verifikasi tersendiri.
