# Migration Engine

**Updated:** 2026-09-16 · Status: Draft

> Kontrak normatif: [`../../spec/backend/01-core-basic.md`](../../spec/backend/01-core-basic.md) §4
> (klasifikasi perubahan, deklarasi destruktif, `persist.raw_ddl`).

## 1. Bentuk Diff → SQL

`PlanDetailed` (di `renderers/jsonb-persist/migrate.go`) membandingkan
**bentuk schema ter-apply** dengan bentuk yang diminta manifest, bukan dua teks
SQL:

- `DesiredSnapshot` (`diff.go`) meringkas manifest menjadi bentuk storage:
  field + tipe + apakah punya kolom turunan, index beserta checksum definisinya,
  dan checksum tiap `persist.raw_ddl`.
- Bentuk ter-apply dibaca dari tabel sistem **`formspec_schema_snapshot`**
  (`snapshot.go`), yang ditulis dalam transaksi yang sama dengan record migrasi
  (§5). Checksum saja tidak cukup: dari checksum tidak bisa dibedakan "field
  dihapus" dan "field belum pernah ada", sehingga gerbangnya tidak akan punya
  dasar apa pun.
- `DiffShapes` menilai setiap perbedaan dan mengembalikan `[]Change` dengan
  `ChangeClass` (`additive` / `derived` / `lossy` / `never`).

`buildChangeDDL` (`alter.go`) menerjemahkan perubahan yang **boleh** terjadi
menjadi pernyataan, dengan urutan yang bukan detail implementasi:

1. index yang hilang/berubah — atau yang menaungi kolom yang akan dibangun ulang
   — di-drop lebih dulu (SQLite menolak men-drop kolom yang masih dipakai index,
   dan index dengan predikat atas kolom yang belum ada tidak bisa dibuat);
2. field yang dihapus: nilainya dibuang dari `data`, lalu kolom turunannya
   di-drop;
3. field yang tipenya berubah: kolom turunan dibangun ulang;
4. diff aditif (kolom turunan dan index yang belum ada);
5. index yang di-drop tadi dibuat kembali — sekarang semua kolomnya sudah ada;
6. `raw_ddl` yang baru atau berubah.

Field rename **wajib** dideklarasikan lewat `renamed_from` pada field — tanpa
itu, diff membacanya sebagai drop+add (dan drop yang tidak dideklarasikan
ditolak).

## 2. Gerbang & Preflight

`RefuseUndeclared` menolak seluruh run **sebelum** pernyataan pertama dijalankan,
jika ada perubahan `lossy` yang tidak dinyatakan manifest atau `never`
(penghapusan tabel) — apa pun environment-nya. Apply setengah jalan yang berhenti
setelah sebagian perubahan ter-commit meninggalkan database yang tidak dijelaskan
manifest mana pun, jadi penolakan terjadi di depan.

Preflight (`measureChanges`) mengubah klaim menjadi angka: jumlah baris yang
kehilangan nilai, jumlah grup duplikat yang menghalangi unique index, jumlah
baris yang gagal cast. Untuk tipe yang tidak bisa diverifikasi otomatis,
perubahannya **ditolak** sampai `accept_data_loss` dinyatakan — bukan dianggap
aman.

## 3. Snapshot: baseline & bootstrap

- **Baseline**: snapshot ditulis bersama record migrasi, berisi checksum yang
  sama (`entityChecksum`), yang mencakup bentuk schema **dan** deklarasi —
  menambah tombstone mengubah pekerjaannya (pembersihan payload) tanpa mengubah
  DDL, jadi checksum atas DDL saja akan menyebutnya "tidak berubah" dan melewati
  pembersihannya selamanya.
- **Bootstrap**: database yang belum punya snapshot mengadopsi manifest saat ini
  sebagai baseline (dan hanya merekonsiliasi bagian aditif). Tidak ada bentuk
  sebelumnya untuk "berubah darinya", jadi menolak di sini berarti memblokir
  deployment karena perubahan yang tak seorang pun bisa lihat — atau nyatakan.
- **Lupa snapshot**: Entity yang tidak lagi dinyatakan manifest dan tabelnya
  sudah tidak ada → entri snapshot dibuang saat apply berikutnya (`ForgetOnly`),
  supaya penolakan tidak berulang selamanya tanpa cara memenuhinya. `formspec
diff` tetap murni dry-run — pembuangannya terjadi saat apply.

## 4. Operasi yang Didukung

- **Field ditambah** — SQLite: kolom biasa (modernc tidak bisa `ADD COLUMN`
  dengan `GENERATED ALWAYS`, jadi kolom turunan di SQLite berupa kolom biasa);
  PostgreSQL: generated column.
- **Field dihapus** — hanya dengan deklarasi `removed: true` + `reason`: nilai
  dibuang dari `data`, kolom turunan dan index-nya di-drop. Runtime-nya: field
  tidak lagi masuk `s.fields`, dan key-nya dibersihkan pada baca **dan** tulis,
  sehingga record lama tetap bisa di-PATCH.
- **Field berubah tipe** — kolom turunan di-drop dan dibangun ulang; nilai mentah
  di `data` tidak diubah. Baris yang nilainya gagal cast adalah `lossy`, jadi
  butuh `accept_data_loss` + `reason`.
- **Index** — di-drop/dibuat mengikuti definisi manifest. Menghapus unique index
  diumumkan (penegakan aturan hilang), dan unique index baru dihitung
  duplikatnya lebih dulu.
- **Relasi** — foreign key logis diverifikasi di level aplikasi (bukan FK
  constraint SQL literal, karena target relation kadang lintas kategori/schema
  yang sengaja tidak boleh di-join langsung, lihat
  [`01-architecture.md`](01-architecture.md) §1 soal isolasi kategori).
  **Gap implementasi:** resolusi nama tabel target (`ValidateRelationTargets`)
  memakai module milik entity itu sendiri + pluralisasi naif (tambah `s`) —
  belum benar-benar resolve module/plural asli entity target. Relasi lintas
  module atau entity ber-plural tidak beraturan bisa diam-diam lolos dari
  guard referenceability (§1.2 core-basic) alih-alih ditolak.
- **Data repair & backfill** — di luar engine: operator menjalankannya sekali
  lewat `formspec repl -f`.

## 5. Keamanan Migrasi

Migrasi structural per Entity dieksekusi dalam satu transaksi, bersama record
migrasi dan snapshot-nya — gagal di tengah berarti rollback penuh, tidak ada DDL
setengah-jalan yang ter-commit dan tidak ada snapshot yang mengklaim bentuk yang
belum pernah ada. Data existing di `data` tidak pernah ditulis ulang oleh migrasi
structural, kecuali pembuangan nilai yang dinyatakan eksplisit lewat
`removed: true`.

## 6. Status Implementasi Hari Ini

Diff masih berujung pada `DDLResult` (teks SQL), belum diformulasikan ulang
sebagai objek diff storage-agnostic yang lantas diterjemahkan tiap backend —
lihat [`01-architecture.md`](01-architecture.md) §4 dan
[`../../spec/backend/04-persist-backend.md`](../../spec/backend/04-persist-backend.md)
§8. Yang sudah storage-agnostic adalah **klasifikasinya** (`ChangeClass`,
`DiffShapes` di `diff.go`): penilaian bahaya sebuah perubahan tidak bergantung
pada SQL yang kebetulan dihasilkan satu backend.
