# 2026-09-24-005 — Lima gap mekanis: format enum, conditional hook, kind docs, state dir, backup storage

**Plan**: tidak ada plan terpisah per item (semua perbaikan kecil/terarah);
bukti di bawah. Permintaan: "perbaiki gaps found" — scope disepakati dengan
pengguna: **hanya gap mekanis yang tidak butuh keputusan kontrak**.

Lima item ditutup, dua gap baru ditemukan & dicatat (4.8.6, 4.8.7).

## 10.30 — `TableColumn.format` belum himpunan tertutup (kafe)

Kosakata `format` hidup hanya di komentar (`// currency | date | relative |
...`) dan di rantai `if` renderer, jadi `format: currncy` **lolos validasi** dan
sel diam-diam mencetak nilai mentah. `ReportColumn` sudah punya himpunan
tertutup sejak S16 dengan alasan yang persis sama.

Ditambahkan `TableCellFormat` (closed set) di `pkg/spec/widget.go` +
`ValidateTableCellFormat`, dipanggil dari `ValidateTableColumns` yang sudah ada.
Himpunannya **sengaja bukan** `ReportFormat`: laporan tak punya
`relative`/`number`, sel tabel tak punya `datetime` (kolom `datetime`
menderivasi `relative`). Menyamakannya memaksa satu permukaan menerima nama
yang tak bisa ia render; karena itu `datetime` pada kolom tabel ditolak dengan
petunjuk "is a report format, not a table cell format".

**Terukur:** `format: currncy` → `unknown cell format "currncy" (allowed:
currency, number, date, relative, percent)`; `format: datetime` → pesan +
petunjuk. Diff schema terverifikasi **hanya** menambah `TableCellFormat` +
`$ref`-nya.

**Drift yang ikut ditemukan & diperbaiki:** tabel §3.1.2 yang saya tulis sesi
sebelumnya mencantumkan `datetime` sebagai format kolom tabel — padahal
`renderCellValue` tidak punya cabang itu. Barisnya dihapus, dan tabelnya kini
menyatakan himpunan tertutup.

## 5.19.1 — `useSelectFilterOptions` dipanggil kondisional (main todo)

`TableRenderer.FilterControl` memanggil hook itu **di dalam `case "select":`**
sebuah `switch (filter.type)`, sehingga jumlah hook berubah bila tipe filter
berubah antar-render. Perbaikan: hook diangkat ke atas `switch`, dan syarat
"hanya untuk select" dipindah **ke dalam hook** (`isSelect`) — kalau tidak,
pemanggilan yang kini selalu jalan akan memicu fetch relasi untuk filter
`date`/`text`. Sekaligus diperbaiki: `exhaustive-deps` pada
`tableSpec.fixed_filters` (manifest scope immutable yang berubah harus
memicu refetch) dan pada `entity.module`.

**Terukur:** `oxlint src/kinds/table/TableRenderer.tsx` → **hanya 0 temuan**
(sebelumnya 1 **error** `rules-of-hooks` + 1 warning). Filter `Cabang`
(`type: select` pada field relasi) di `/kafe/app/pos/cafe-stock/stock-levels`
tetap memuat opsinya: **Kafe Dago, Kafe Senayan** — jadi perilakunya tidak
berubah.

## 10.15 — `docs/kind/` belum punya halaman `Seed` (kafe)

`kind: Seed` ada di `genjsonschema.KindMapping()` tetapi tidak di
`kindGroups`, jadi generator melewatinya dan kontrak `$ref`/`$asset`/reconcile
hanya hidup di godoc + CLI docs.

Ditambahkan ke grup **`data`** (bukan `curation`): `docs/kind/README.md` sudah
menghitung `data/` = **11** sejak awal, dan Seed memang deklarasi data domain,
bukan struktur workspace. Halaman `docs/kind/data/Seed.md` ditulis lengkap
(Kapan Memakai / Contoh / Gotchas — 0 TODO tersisa).

**Stale yang ikut diperbaiki:** tabel taksonomi di
`docs/spec/platform/03-kind-system.md` §1 masih menulis "33 kind" dengan
Curation=2 dan tanpa `Seed`/`Workspace`, padahal kenyataannya 34 halaman dan
Curation=3. Tabelnya + §4 (plane) + rincian per grup diselaraskan dengan
`KindMapping()`.

**Terukur:** total halaman kind **34** (dari 33), `data/` 11, `0 TODO` di
halaman Seed, regenerate idempotent (narasi tidak tersentuh).

## 10.17 — `formspec dev` memakai state dir CWD-relatif (kafe)

`StateDirFromDSN` hanya memahami DSN SQLite; untuk `postgres://…` ia
mengembalikan `.formspec` mentah yang lalu di-resolve terhadap **CWD** — jadi
fallback storage filesystem dan `dev-jwt-secret` berpindah mengikuti direktori
pemanggilan. Ini kelas yang sama dengan yang sudah diperbaiki
`dsn-spec-anchored.md` untuk file DB itu sendiri.

Ditambahkan `StateDirFor(dsn, specPath)`: meng-anchor ke project root, dan
membiarkan path absolut apa adanya (DSN SQLite sudah di-anchor `resolveDSN`,
jadi anchoring kedua akan menggandakan root). Dipakai di `resource/formspec.go`
(3 tempat), `cmd/formspec/dev.go`, dan `cmd/formspec/seed.go` (logika salinan
di seed diganti helper bersama).

**Terukur (A/B dua binary, CWD `/tmp/ab/run-here`, spec `/tmp/ab/proj/spec`):**

| Binary                   | Output                                                                 |
| ------------------------ | ---------------------------------------------------------------------- |
| lama (`StateDirFromDSN`) | `.formspec/dev-jwt-secret` — **di CWD** (`/tmp/ab/run-here/.formspec`) |
| baru (`StateDirFor`)     | `/tmp/ab/proj/.formspec/dev-jwt-secret` — **di project root**          |

Test `resource/statedir_test.go`: 2 case (anchor + tidak menggandakan absolut).

## 10.16 — `formspec backup` hanya menyertakan storage FILESYSTEM (kafe)

`backup create` menambahkan `storage/` dengan membaca path
`{StateDirFromDSN(dsn)}/storage` yang **hardcoded** — bukan lewat storage
service. Begitu sebuah `kind: Datastore` menyajikan `storage` dengan driver
garage/minio/s3 (jalur prod), seluruh objek `file` tidak masuk backup, tanpa
peringatan.

Dua cacat, bukan satu:

1. **Sisi baca:** diganti `ResolveStorage` dari datastore registry yang sama
   dengan server (helper `backupStorageService`, sejajar dengan jalur seed).
   Kunci objek di-**enumerasi dari record** yang ikut ter-backup, karena
   kontrak `Storage` hanya Upload/Download/Stat — tidak ada cara portable untuk
   melisting; dan ini cukup, sebab field `file` menyimpan kunci kanoniknya.
2. **Sisi restore hilang sama sekali:** entri `storage/*` disaring oleh cek
   ekstensi `.jsonl`, jadi restore menulis record yang field `file`-nya
   menunjuk objek yang tak pernah ditulis. Sekarang objek di-upload **verbatim**
   (kunci identik dengan yang dirujuk record — remap akan memutusnya), dengan
   tally `storage_objects` di manifest dan laporan.

**Terukur:** test `TestRestoreUploadsStorageObjects` **gagal sebelum patch**
("expected 1 object restored, got 0") dan lulus sesudah; byte objek
diverifikasi identik dan kuncinya sama dengan referensi record.

## Gap baru yang ditemukan (tidak diperbaiki — butuh keputusan)

- **4.8.7 `backup`/`restore` menulis ke workspace `"demo"` hardcoded.**
  **Terukur:** `formspec backup create --full --spec examples/kafe/spec --dsn
sqlite:.formspec/kafe.db` → **`27 table(s), 0 record(s)`**, padahal
  `cafe_master_menu_items` berisi `tenant_id=kafe` **9 baris**. Jadi backup
  aplikasi nyata menghasilkan arsip **kosong tanpa error** — kelas gagal senyap
  yang sama dengan 10.16, dan lebih berbahaya karena tampak berhasil.
  `formspec seed` sudah punya `--workspace` sebagai preseden. Effort: medium
  (butuh keputusan: workspace aktif / semua workspace / flag eksplisit).
- **4.8.6 `backup create --incremental`** belum ada definisi "sejak backup
  terakhir". Effort: large.

## Verifikasi

- `go build ./...` ok · `go test ./...` **39 paket ok** · `gofmt` bersih.
- Frontend: `vitest` **346 lulus** · `tsc -p tsconfig.app.json --noEmit` bersih
  · `oxlint src` **exit 0** (warning sisa semuanya pre-existing, terverifikasi
  dengan membandingkan file sebelum/sesudah).
- `make generate-schema` + `make generate-kind-docs` idempoten.
- kafe `validate`: 85 manifest, 0 problem (tak berubah).
- Browser (sesi `manajer` app-scoped): `orders` 21 baris, `stock-levels` filter
  relasi memuat opsi, **0 page error**.

Referensi: kafe `examples/kafe/gaps_found/TODO.md` (10.15, 10.16, 10.17, 10.30)
· `docs_internal/plan/todo.md` (4.8.1–4.8.7, 5.18.6, 5.19.1).
