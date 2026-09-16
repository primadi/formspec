# 2026-09-16-002 — Widget `qrcode` (S4 / #3), kosakata + komponen (sebagian)

Item `examples/kafe/gaps_found/TODO.md` **2.6** (gap **#3/S4**) — **sebagian**,
sengaja tidak ditandai selesai: accept-nya menuntut "dirender **& dicetak**", dan
jalur cetak belum ada.

**Keputusan.** Pemilik proyek memilih **jalur termurah: widget read-only +
dependency klien** (`qrcode.react`, MIT), bukan field type baru dan bukan Service
engine. SVG dipilih (bukan canvas) supaya tajam saat dicetak di struk thermal.

**Yang dikerjakan.** `qrcode` masuk **dua** kosakata tertutup (S10) dengan nama
yang sama, karena artinya sama di kedua permukaan — "string ini, scannable":
`FormWidget` (form: menggantikan input; nilainya ADALAH payload, tidak ada yang
bisa diketik) dan `TableCellWidget` (sel tabel/listing). Ditambah komponen
`src/widgets/QrCode.tsx` (SVG, `level: M`, teks pengganti saat kosong), cabang di
`FormFieldWidget` dan `renderCellValue`, barrel widget, dan regenerasi schema —
`$defs/FormWidget` (22 nilai) serta `$defs/TableCellWidget` (4 nilai) kini
memuat `qrcode`, sehingga `formspec validate` menolak salah ketik dan editor
memberi autocomplete.

**Bukti.** Test kosakata tertutup diperbarui dan lulus; paritas
schema↔katalog↔renderer (`catalog.test.tsx`) lulus; `go test ./...` hijau;
`vitest` 258 lulus; `tsc` bersih; enum di schema terverifikasi memuat `qrcode`.

**Sisa (dicatat di TODO 2.6).** (a) **Jalur cetak** — `Print` memakai
`resolveCellValue()` yang menjadikan nilai teks, jadi struk/kartu meja belum bisa
memuat QR; itu bagian dari **7.1**. (b) **Adopsi kafe** terhalang hal konkret: QR
yang bisa dipindai butuh **URL absolut**, sedangkan `dining-table` hanya
menyimpan kode meja — menyusun URL absolut (termasuk menyuntikkan origin saat
cetak) adalah pekerjaan pemanggil. Karena itu spec kafe **belum** memasang
`widget: qrcode`: memasangnya sekarang hanya menghasilkan QR berisi "A-01" yang
tidak menuju apa pun.
