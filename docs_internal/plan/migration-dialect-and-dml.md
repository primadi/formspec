# Plan — `kind: Migration`: DDL per-driver + perbaikan data yang dinyatakan (kafe TODO 3.4)

Sumber: `examples/kafe/gaps_found/TODO.md` 3.4; gap **#35** dan **#36** di
`08-ddl-index-dan-devtools.md`.

## Dua celah

**#35 — DDL tidak portabel.** `MigrationSpec` hanya punya satu string `ddl`,
pada hal ekspresi JSONB berbeda antar driver:

| Driver                | Ekspresi untuk `data.branch_id`     |
| --------------------- | ----------------------------------- |
| SQLite (dev)          | `json_extract(data, '$.branch_id')` |
| PostgreSQL (produksi) | `(data ->> 'branch_id')`            |

Jadi DDL-nya benar untuk dev dan **salah untuk produksi**, dan kegagalannya baru
muncul saat deploy. Setiap migration yang menyentuh kolom `data` — artinya semua
index/constraint pada strategi JSONB hybrid — menghadapi ini.

**#36 — hanya DDL yang diizinkan.** Konsekuensi praktisnya: `CREATE UNIQUE
INDEX` **gagal** bila tabel sudah punya duplikat, dan duplikat itu muncul justru
**karena** constraint-nya belum ada. Perbaikannya butuh `UPDATE`/`DELETE` — DML
yang ditolak — sehingga dilakukan manual di luar spec, tanpa jejak untuk audit.

## Konstruk

```yaml
kind: Migration
metadata: { name: menu-price-unique }
spec:
  reason: "dua harga untuk menu yang sama muncul selagi unique index belum ada"
  dml: ["DELETE FROM … WHERE rowid NOT IN (SELECT MIN(rowid) …)"]
  ddl_by:
    sqlite: "CREATE UNIQUE INDEX … (_menu_item_id)"
    postgres: "CREATE UNIQUE INDEX … ((data->>'menu_item_id'))"
```

| Aturan                                                           | Alasan                                                                        |
| ---------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| `ddl` (portabel) **atau** `ddl_by` (per driver) — bukan keduanya | maksud yang ambigu lebih buruk daripada pilihan yang tegas                    |
| dialek himpunan tertutup (`sqlite`, `postgres`)                  | salah ketik tidak boleh berarti "varian itu dilewati"                         |
| driver tanpa varian → migration dilewati **dengan peringatan**   | menjalankan SQL driver lain justru kegagalan yang #35 khawatirkan             |
| `dml` wajib disertai `reason`                                    | menyentuh data: audit butuh _kenapa_, bukan hanya _apa_                       |
| `dml` hanya INSERT/UPDATE/DELETE/WITH                            | perubahan skema tetap milik `ddl`; dua kanal tidak boleh bercampur            |
| `dml` **sebelum** `ddl` dalam satu manifest                      | urutan inilah yang membuat "rapikan lalu batasi" bisa dinyatakan sekali jalan |
| `dml` diumumkan saat apply + dicetak di plan                     | perubahan data tidak boleh senyap                                             |

## File

- `pkg/spec/resources.go` — `DDLByDialect`, `DML`, `Reason`,
  `DDLForDialect`, `ValidateMigrationSpec`.
- `cmd/formspec/migrate.go` — loader menerima driver, `validateDMLOnly`,
  urutan DML→DDL, plan menampilkan perbaikan data.
- `cmd/formspec/migrate_dialect_test.go` — 3 test (pemilihan dialek, validasi,
  skenario perbaikan data utuh).
- `docs/spec/backend/01-core-basic.md` §4.1 (baru).

## Bukti

`TestApplyCustomMigrations_DataRepairRunsBeforeDDL`: tabel berisi duplikat →
`CREATE UNIQUE INDEX` gagal → setelah `dml` merapikan (3 → 2 baris) constraint
berhasil dan menolak duplikat berikutnya. Plus pemilihan dialek dan 6 bentuk
validasi yang ditolak. `go test ./...` hijau · kafe `validate` 0 problem.

## Sisa

Belum dibungkus satu transaksi eksplisit (Postgres belum diverifikasi); tidak ada
adopsi baru di spec kafe karena `kind: Migration`-nya sudah tidak ada sejak 1.6.

## Estimasi: **medium**
