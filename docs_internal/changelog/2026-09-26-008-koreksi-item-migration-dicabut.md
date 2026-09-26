# Todo 4.2.6 tidak berlaku lagi: `kind: Migration` sudah dicabut (koreksi 4.2.4/4.2.5/4.2.6)

## Apa yang ditemukan

Saat mengerjakan item 4.2.6 ("`dml` + `ddl` dalam satu manifest belum dibungkus satu
transaksi eksplisit"), ternyata **subjek item itu sudah tidak ada** — dan bukan
baru-baru ini:

| Bukti                                                                       | Hasil                                                                                       |
| --------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| `grep -rn '"dml"\|yaml:"dml' --include='*.go' .`                            | **0 hasil**                                                                                 |
| `grep -rn 'DDLByDialect\|DMLBy' --include='*.go' .`                         | **0 hasil**                                                                                 |
| `KnownKinds` (`internal/manifest/loader.go:309`)                            | tidak memuat `"Migration"`                                                                  |
| `formspec migrate` verb (`cmd/formspec/migrate.go:51`)                      | hanya `plan\|apply` (tidak ada `data`)                                                      |
| `docs/kind/`                                                                | tidak ada halaman `Migration`                                                               |
| `schemas/kinds/Migration.schema.json`                                       | tidak ada                                                                                   |
| **Terukur:** manifest `kind: Migration` pada spec uji → `formspec validate` | `unknown kind "Migration" for spec version v1` + `read Migration.schema.json: no such file` |

`git log` menunjuk penyebabnya: **commit `ceaaf2a` (2026-09-17)** mencabut
`MigrationSpec`, `DataMigrationSpec`, `ValidateMigrationSpec`, `MigrationDialects`,
entri loader, schema, kind doc, dan verb `formspec migrate data` — didokumentasikan
sebagai changelog **`2026-09-16-012`** ("Migrasi otomatis + gerbang perubahan
destruktif (cabut `kind: Migration`)").

**Jadi 4.2.6 difile pada `2026-09-16-008` dan subjeknya dicabut di `2026-09-16-012`
— pada hari yang sama, urutan NNN menunjukkan pencabutan lebih dulu.** Item itu
tidak pernah punya kesempatan untuk benar.

## Koreksi yang lebih penting: 4.2.4 dan 4.2.5 juga salah

Ketiganya masih `[x]`/`[⏸️]` dengan teks yang mengklaim kind-kind itu ada:

- **4.2.4** mengklaim "`formspec migrate plan|apply` kini load `kind: Migration`
  manifests, `validateDDLOnly` menolak DML" — tidak lagi benar.
- **4.2.5** mengklaim "`kind: DataMigration` (`version`/`run`/`rollback`) +
  `formspec migrate data <name> run|rollback`" — tidak lagi benar.
- **4.2.6** meminta `dml`+`ddl` dibungkus satu transaksi — subjeknya tidak ada.

Ini kelas misinformasi yang sama dengan yang item 5.22.8 tangani: pembaca melihat
`[x]` lalu berhenti, padahal fitur yang disebut sudah tidak ada. Ketiganya
dikoreksi di tempat, masing-masing menyebut penggantinya.

## Penggantinya (agar tidak dikira celah baru)

- **DDL di luar bahasa spec** → `Entity.spec.persist.raw_ddl`
  (`pkg/spec/entity.go:2219`, divalidasi `ValidateRawDDL`): DDL-only, `reason`
  wajib, `ddl` **atau** `ddl_by` per-dialek, **forward-only**. Ia ikut jalur sync
  normal: `alter.go` membangun langkah 1–6 (termasuk raw_ddl) menjadi **satu
  string DDL** yang dieksekusi di dalam **satu tx per entity** (`applyPlans`) —
  sehingga masalah atomisitas yang 4.2.6 khawatirkan tidak punya bentuk lagi.
- **Perbaikan data** → **sengaja tidak punya permukaan spec.** `formspec migrate`
  menolak perubahan yang butuh perbaikan data **dengan hitungan**
  (`RefuseUndeclared`, `renderers/jsonb-persist/diff.go:563`, menyertakan
  `Remedy`); operator merapikan sekali lewat `formspec repl -f <script>`
  (`cmd/formspec/repl.go:56`), lalu apply diulang.

## Kenapa tidak sekadar menghapus itemnya

Karena "item salah" dan "pekerjaan belum selesai" adalah dua hal berbeda, dan
hanya yang pertama berlaku di sini. Item dibiarkan **terlihat** (bukan dihapus)
supaya orang yang mencarinya menemukan jawabannya, lengkap dengan perintah yang
membuktikannya — kalau hanya dihapus, satu-satunya cara tahu adalah mengulang
seluruh penelusuran ini.

## File terdampak

- `docs_internal/plan/todo.md` — 4.2.4, 4.2.5, 4.2.6 dikoreksi

## Bukti

Semua baris di tabel bukti di atas, masing-masing bisa dijalankan ulang. Tidak ada
kode yang diubah, jadi tidak ada test baru.

## Rujukan

Changelog **`2026-09-16-012`** (pencabutan) · `2026-09-16-008` (yang memfile 4.2.6) ·
`docs/spec/backend/01-core-basic.md` §4.3 (`persist.raw_ddl`) ·
`docs_internal/plan/close-open-items-2026-09-26.md`.
