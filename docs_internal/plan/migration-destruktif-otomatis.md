# Plan — Migrasi otomatis + gerbang perubahan destruktif (cabut `kind: Migration`)

Sumber: keputusan 2026-09-16 (opsi B) atas `kind: Migration`; menutup
`examples/kafe/gaps_found/TODO.md` 8.8 dan menggantikan
`docs_internal/plan/migration-dialect-and-dml.md` (yang menambah `ddl_by`/`dml`
ke kind yang kini dicabut).

## Masalah

`kind: Migration` adalah kind zombie: terimplementasi, **nol adopsi** di
`examples/**` (kafe menghapus ketiganya di 1.6), hanya jalan lewat
`formspec migrate apply` — bukan jalur normal (`formspec dev`/`SyncSchema`) —
dan `ValidateMigrationSpec` tidak pernah dipanggil `formspec validate`, sehingga
manifest rusak lolos validate lalu di-skip dengan `Warning:` saat apply.

Saat mencabutnya, ditemukan masalah yang lebih besar di diff otomatis:

| Fakta                                                                                                                 | Bukti                                                    |
| --------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------- |
| Diff aditif-only: hanya tabel baru, generated column baru, `CREATE INDEX` baru                                        | `renderers/jsonb-persist/migrate.go` `diffExistingTable` |
| Field dihapus / tipe berubah → **nol DDL, tanpa error, tanpa warning** (`migrate plan` cetak "No pending migrations") | idem                                                     |
| Tapi key field lama tetap di `data` → PATCH merge → `validateKnownFields` → **422 permanen pada record lama**         | `crud.go:296`, `internal/api/handler.go`                 |
| Tidak ada catatan spec ter-apply → penghapusan field tak mungkin dideteksi                                            | `formspec_schema_migrations` hanya simpan checksum       |

Jadi perilaku hari ini yang terburuk dari dua dunia: diff menyembunyikan
perubahan, runtime menghukum data lama.

## Konstruk

### 1. Klasifikasi perubahan (storage-agnostic)

| Tingkat     | Contoh                                                                                                                   | Perlakuan                          |
| ----------- | ------------------------------------------------------------------------------------------------------------------------ | ---------------------------------- |
| **aditif**  | tabel/kolom/index baru                                                                                                   | otomatis, senyap                   |
| **derived** | drop+recreate kolom turunan usang; rename via `renamed_from`; drop index                                                 | otomatis + notice                  |
| **lossy**   | strip key dari `data`; kolom child `storage: table`; unique index saat duplikat ada; type change dengan baris gagal cast | **error** kecuali dideklarasikan   |
| **never**   | `DROP TABLE` (Entity hilang dari manifest)                                                                               | selalu error, tanpa jalur otomatis |

Hanya manifest yang menyentuh storage (`Entity`) yang bergerbang; hapus manifest
`Page`/`Form`/`Table`/`Report`/`Theme` tidak berdampak DB.

### 2. Deklarasi di manifest

```yaml
fields:
  - name: old_branch_code
    type: string
    removed: true
    reason: "digantikan branch_id"
```

| Deklarasi                           | Arti                                                                          |
| ----------------------------------- | ----------------------------------------------------------------------------- |
| `removed: true` + `reason`          | tombstone: engine strip key dari `data` + drop kolom/index, mencetak hitungan |
| `accept_data_loss: true` + `reason` | consent untuk type change lossy                                               |
| `renamed_from` (sudah ada)          | rename — tidak pernah dianggap drop+add                                       |

| Aturan                                 | Alasan                                                                 |
| -------------------------------------- | ---------------------------------------------------------------------- |
| nama `removed`, **bukan** `deleted`    | `deleted_at`/`soft_delete` sudah berarti soft delete di repo ini       |
| `reason` wajib untuk tiap deklarasi    | pola yang sama dengan `dml`+`reason`: audit butuh _kenapa_             |
| tombstone tetap punya `name` + `type`  | keduanya `required` di schema; tombstone mendeskripsikan yang dibuang  |
| tombstone hidup **satu kali apply**    | setelah itu snapshot tak memuat field itu → barisnya boleh dihapus     |
| Entity/table **tidak** punya tombstone | generator schema tidak mendukung `if/then`; blast radius seluruh tabel |

Jalur resmi hapus tabel: `formspec backup` → drop manual → hapus manifest.

### 3. Snapshot spec ter-apply

Tabel sistem `formspec_schema_snapshot` (per entity: field name/type/derived,
indexes, checksum `raw_ddl`), ditulis dalam transaksi yang sama dengan record
migrasi. Ini yang membuat penghapusan field **mungkin** dideteksi, dan
memungkinkan diff terstruktur `DiffSpecs(snapshot, spec) → []Change` —
storage-agnostic, memajukan gap arsitektural `04-persist-backend.md` §8.

Bootstrap: DB yang belum punya snapshot mengadopsi spec saat ini sebagai baseline
(log eksplisit) supaya upgrade tidak memunculkan error retroaktif.

### 4. `raw_ddl` — escape hatch, di dalam Entity

```yaml
kind: Entity
spec:
  persist:
    raw_ddl:
      - name: menu-price-unique
        reason: "satu harga per menu per cabang"
        ddl_by:
          sqlite: "CREATE UNIQUE INDEX … (json_extract(data, '$.menu_item_id'))"
          postgres: "CREATE UNIQUE INDEX … ((data->>'menu_item_id'))"
```

`ddl` portabel **atau** `ddl_by` (himpunan tertutup sqlite/postgres) — menjaga
penutupan gap #35. DDL-only divalidasi di `formspec validate`, dijalankan di
jalur sync normal dengan checksum tercatat (skip bila tak berubah), dan
**forward-only**: menghapus deklarasi tidak menjatuhkan apa pun.

### 5. Penghapusan data & backfill

`dml` dan `kind: DataMigration` dicabut. Perbaikan data (mis. duplikat sebelum
unique index — gap #36) ditolak preflight **dengan hitungan**, lalu dijalankan
operator sekali lewat `formspec repl -f <script.star>` (flag baru memakai
`replEval` yang sudah ada).

## File

- `pkg/spec/entity.go` — `Field.Removed`, `Field.AcceptDataLoss`, `Field.Reason`,
  `PersistSpec.RawDDL`, `RawDDLDecl`, validasinya.
- `pkg/spec/resources.go` — cabut `MigrationSpec`, `DataMigrationSpec`,
  `ValidateMigrationSpec`, `MigrationDialects`, `statementStartsWithDML`,
  `firstWord`; tambah `ValidateRawDDL`.
- `pkg/spec/spec.go`, `internal/manifest/loader.go` — cabut `KindMigration`.
- `renderers/jsonb-persist/diff.go` (baru) — `ChangeClass`, `Change`, `DiffSpecs`.
- `renderers/jsonb-persist/snapshot.go` (baru) — tabel + baca/tulis snapshot.
- `renderers/jsonb-persist/migrate.go` — plan/apply memakai diff berklasifikasi,
  gerbang lossy, bootstrap snapshot.
- `cmd/formspec/migrate.go` — cabut custom migration + `migrate data`;
  `cmd/formspec/repl.go` — flag `-f`; `cmd/formspec/validate.go` — hygiene deklarasi.
- `internal/genjsonschema/kinds.go`, `internal/genkinddocs/markdown.go` — cabut
  entri `Migration`; `schemas/kinds/Migration.schema.json` dihapus.

## Bukti

Test: klasifikasi diff, preflight counts, gerbang undeclared (error) vs declared
(strip + drop + hitungan), siklus hidup tombstone (error → apply → hapus tombstone
→ bersih), DROP TABLE selalu error + prune snapshot, `renamed_from` bukan
drop+add, `raw_ddl` DDL-only + idempotent, reproduksi bug `ErrUnknownField`.
`go test ./...` hijau · kafe `validate` 0 problem · `make generate-schema` +
`make generate-kind-docs` bersih.

## Estimasi: **large**
