# 2026-09-16-012 — Migrasi otomatis + gerbang perubahan destruktif (cabut `kind: Migration`)

**Apa yang diubah.** `kind: Migration` dan `kind: DataMigration` dicabut seluruhnya
(beserta `MigrationSpec`, `DataMigrationSpec`, `ValidateMigrationSpec`,
`MigrationDialects`, loader entry, schema, kind doc, dan verb `formspec migrate
data`). Penggantinya bukan kind baru: migrasi struktural sepenuhnya otomatis dari
diff Entity, dan **setiap perubahan dinilai** — `additive` (tabel/kolom/index
baru) dan `derived` (kolom turunan dibangun ulang, rename via `renamed_from`,
index dihapus) berlaku sendiri; `lossy` (field dihapus, type change pada kolom
turunan yang nilainya gagal cast, unique index sementara duplikat ada) **ditolak**
sampai manifest menyatakannya; `never` (tabel dihapus) selalu ditolak. Deklarasi
ditaruh di field-nya: `removed: true` + `reason` (tombstone) dan
`accept_data_loss: true` + `reason`. DDL di luar bahasa spec pindah ke
`Entity.spec.persist.raw_ddl` (`ddl` atau `ddl_by`, DDL-only, `reason` wajib,
forward-only) dan ikut jalur sync normal. Perbaikan data tidak punya permukaan
spec: `formspec migrate` menolak dengan **hitungan** (jumlah baris/grup duplikat),
operator merapikannya sekali lewat `formspec repl -f <script>` (flag baru), lalu
apply diulang.

**Kenapa diubah.** Diff lama aditif-only: field dihapus / tipe berubah → nol DDL,
tanpa error, tanpa warning, sementara key field lama tetap ada di `data` sehingga
setiap PATCH pada record lama gagal 422 (`ErrUnknownField`) — diff menyembunyikan,
runtime menghukum. Penghapusan field juga mustahil dideteksi karena
`formspec_schema_migrations` hanya menyimpan checksum. Jadi kebijakan destruktif
bukan tambahan: ia prasyarat agar migrasi otomatis boleh dipercaya.

**File terdampak.** `pkg/spec/entity.go` (`Field.Removed`/`AcceptDataLoss`/
`Reason`, `PersistSpec.RawDDL`, `RawDDLDecl`, `ValidateRawDDL`, `ValidateDDLOnly`),
`pkg/spec/resources.go`, `pkg/spec/spec.go`, `internal/manifest/loader.go`,
`internal/genjsonschema/{kinds,generator}.go` (`RawDDLDecl` wajib ada di daftar
`sharedTypes`, atau setiap Entity gagal compile schema-nya),
`internal/genkinddocs/markdown.go`, `cmd/formspec/{migrate,repl}.go`,
`renderers/jsonb-persist/{diff,alter,snapshot,migrate_plan}.go` (baru) +
`migrate.go`/`crud.go`; dokumen: `docs/spec/backend/01-core-basic.md` §4 (ditulis
ulang), `04-persist-backend.md`, `docs/spec/platform/03-kind-system.md` (11 → 10
kind data), `docs/cli-tools/02-formspec-cli.md`, `docs/reference/glossary.md`,
`docs/renderers/jsonb-persist/03-migration-engine.md`, `ai_skills/*` + salinan
vendored, `.github/skills/formspec-backend/SKILL.md`, example kafe; schema
diregenerasi (`schemas/`, 34 kind).

**Bukti.** `TestMigrateRefusesUndeclaredRemoval` (YAML → ditolak → deklarasi →
nilai ter-strip → tombstone dihapus → bersih), `TestMigrateRefusesDroppedEntity`
(tabel tidak pernah di-drop; setelah drop manual, snapshot dibersihkan),
`TestMigrate_UniqueIndexBlockedByDuplicates` (ditolak dengan hitungan → repair
sekali → apply), `TestMigrate_RawDDLRunsOnce`, `TestMigrate_BootstrapAdoptsBaseline`,
`TestDiffShapes_*`, `TestChange_MeasurePromotesToLossy`, `TestValidateRawDDL`,
`TestValidateEntitySpec_DestructiveDeclarations`, `TestValidateDDLOnly`.
`go test ./...` hijau. Plan: `docs_internal/plan/migration-destruktif-otomatis.md`.

**Catatan/konsekuensi yang dicatat, bukan disembunyikan.** (1) Menghapus `dml`
menghilangkan jejak audit in-spec untuk perbaikan data — jejaknya kini di riwayat
shell/ops. (2) `TestMigrationRunner_ChecksumChange` diperbarui: perubahan yang
tidak butuh DDL (field payload-only) sekarang tetap dicatat sebagai migrasi,
karena kontrak entity-nya berubah dan itulah yang membuat `formspec diff`
bermakna. (3) Gap baru ditemukan saat menulis test dan **belum diperbaiki**:
kolom turunan yang ditambahkan setelah tabel dibuat di SQLite adalah kolom biasa
yang tidak terisi (modernc tidak bisa `ADD COLUMN ... GENERATED ALWAYS`), jadi
index atasnya tidak menegakkan apa pun sampai baris ditulis ulang — preflight
tidak terpengaruh (ia menghitung payload), tapi index-nya belum bisa dipercaya.
