# 2026-09-16-008 — `kind: Migration`: `ddl_by` per-driver + `dml` yang dinyatakan (#35/#36)

Item `examples/kafe/gaps_found/TODO.md` **3.4**. Plan:
`docs_internal/plan/migration-dialect-and-dml.md`.

**#35 — DDL tidak portabel.** Ekspresi JSONB berbeda antar driver
(`json_extract(data,'$.x')` vs `data->>'x'`), jadi satu string `ddl` benar untuk
dev dan **salah untuk produksi** — kegagalannya baru muncul saat deploy. Kini
`ddl` tetap untuk statement portabel dan **`ddl_by`** memuat varian per driver
dengan himpunan dialek tertutup. Menulis keduanya ditolak (maksudnya jadi
ambigu), dan driver tanpa varian melewati migration itu **dengan peringatan** —
bukan menjalankan SQL driver lain.

**#36 — perbaikan data.** `CREATE UNIQUE INDEX` gagal bila tabel sudah punya
duplikat, dan duplikat itu muncul justru karena constraint-nya belum ada —
sementara perbaikannya butuh DML yang ditolak, sehingga dikerjakan manual di luar
spec tanpa jejak. Kini **`dml`** boleh dinyatakan dengan **`reason` wajib**, hanya
DML (bukan DDL), dijalankan **sebelum** DDL dalam manifest yang sama, dan
**diumumkan** saat apply serta dicetak di `migrate plan`.

**Bukti.** `TestApplyCustomMigrations_DataRepairRunsBeforeDDL` menjalankan
skenario gap-nya utuh: tabel berisi duplikat → `CREATE UNIQUE INDEX` gagal →
setelah `dml` merapikan (3 → 2 baris) constraint berhasil dan menolak duplikat
berikutnya. `TestLoadCustomMigrations_PicksDialect` (varian dipilih per driver;
`ddl` portabel berlaku di keduanya). `TestValidateMigrationSpec` (6 bentuk
ditolak: tanpa DDL, dua bentuk sekaligus, dialek tak dikenal, varian kosong,
`dml` tanpa `reason`, DDL menyusup ke `dml`). `go test ./...` hijau · kafe
`validate` 0 problem.

Normatif: `docs/spec/backend/01-core-basic.md` §4.1.

**Sisa.** `dml`+`ddl` belum dibungkus satu transaksi eksplisit (Postgres belum
diverifikasi). **Catatan cakupan:** _manifest_ `kind: Migration` kafe dihapus di
1.6 (aturan keunikannya kini deklaratif lewat `indexes[].where`), dan ternyata
**tidak ada satu pun example** di repo yang masih memakainya — jadi bukti untuk
item ini datang dari test, bukan dari aplikasi. Kind yang tidak dijalani example
mana pun bisa membusuk tanpa ketahuan; dicatat sebagai item **8.8** di ledger
kafe (satu example yang benar-benar memakai `ddl_by`/`dml`).
