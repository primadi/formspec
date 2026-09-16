# 2026-09-16-001 — Gambar tampil di cell/listing/detail + kanonik `allowed_types` (#4/#4b)

Item `examples/kafe/gaps_found/TODO.md` **2.5** (gap **#4** dan **#4b**). Plan:
`docs_internal/plan/media-image-cells.md`.

**#4b dulu, karena ia akar dari "upload sah tapi ditolak".** Bentuk
`allowed_types` yang **didokumentasikan** — ekstensi tanpa titik, `[jpg, png]` —
adalah satu-satunya bentuk yang **tidak cocok dengan apa pun**: matcher klien
maupun server hanya mengenal `.jpg`, MIME persis, dan wildcard `image/*`. Jadi
spec yang benar menghasilkan "File type not allowed". Bentuk kanonik kini
ditegaskan (ekstensi tanpa titik), keempat ejaan diperlakukan sama oleh matcher
yang **kini sepasang** (`internal/api/file.go` dan `src/lib/media.ts`, dengan
tabel test di kedua sisi), dan `formspec validate` menolak entri di luar keempat
bentuk itu. `menu-item.photo` tidak lagi menuliskan dua bentuk sekaligus. Cara
menyatakan banyak file juga dinyatakan eksplisit: `max_count > 1`, bukan tipe
field terpisah (dokumen lama menyebut `file_list` yang tidak ada).

**#4: gambar akhirnya dirender sebagai gambar.** `renderCellValue` mendapat
cabang `widget: image` — nilai `file` adalah object key, dan route unduh entity
menjadi `src`-nya, sehingga kolom tabel tidak butuh widget preview tersendiri.
`image` masuk kosakata tertutup `TableCellWidget` (S10) sehingga salah ketik
tetap ditolak validator dan paritas schema↔katalog↔renderer dijaga
`catalog.test.tsx`. Renderer menurunkan `image` untuk field file yang
`allowed_types`-nya memuat gambar, jadi manifest tidak wajib menuliskannya; file
non-gambar tetap tautan unduh seperti sebelumnya. Table/Listing meneruskan URL
lewat `CellRenderOpts.imageUrl`; DetailPage merender `<img>` untuk nilai gambar;
satu helper `fileDownloadUrl` menggantikan URL yang sebelumnya disusun sendiri
oleh `PickerPanel` dan `FileInput`.

**Bukti.** `pkg/spec` 5 test (bentuk diterima/ditolak, `StorageAllowsImage`,
entity-level), `internal/api` `TestAllowedFileType` (8 case, termasuk regresi
bentuk kanonik), klien `lib/media.test.ts` (8 case) + parity widget.
`go test ./...` hijau, `vitest` 258 lulus (dari 250), `tsc` bersih, kafe
`validate` 0 problem.

**Sisa yang dicatat:** belum ada verifikasi runtime gambar di browser (butuh
sesi untuk upload; kafe belum punya kolom tabel ber-`photo`), Print belum ikut
(butuh URL absolut), dan `transform` thumbnail belum diverifikasi.
