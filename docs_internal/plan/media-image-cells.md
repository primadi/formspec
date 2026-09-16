# Plan — Gambar produk tampil di surface non-form (#4 / 2.5)

Sumber: `examples/kafe/gaps_found/TODO.md` 2.5, gap **#4** dan **#4b** di
`02-media-dan-qr.md`.

## Masalah

Dua lapisan, dan yang lebih menjebak bukan yang pertama:

1. **#4** — upload foto sudah berfungsi (FileInput punya preview), tetapi
   **tidak ada renderer yang menampilkan gambar sebagai gambar**.
   `renderCellValue()` — dipakai bersama Table, Listing, Report, ChildTable —
   tidak punya cabang untuk `file`, jadi nilai objek key tercetak sebagai teks
   path. DetailPage menampilkan tautan unduh berikon, bukan gambar.
2. **#4b** — bentuk `allowed_types` ambigu di tiga sumber (dokumen: `[jpg]`,
   klien: `.jpg`/mime/`image/*`, server: `allowedFileType` yang sama), dan
   bentuk yang **didokumentasikan** (`[jpg]`) adalah satu-satunya yang **tidak
   cocok dengan apa pun** → upload sah ditolak. Kafe menuliskan dua bentuk
   sekaligus sebagai kompromi.

## Konstruk & aturan

| Hal                            | Keputusan                                                                                        |
| ------------------------------ | ------------------------------------------------------------------------------------------------ |
| Bentuk kanonik `allowed_types` | ekstensi tanpa titik (`jpg`, `png`, `pdf`)                                                       |
| Bentuk lain yang diterima      | `.jpg`, `image/jpeg`, `image/*` — diperlakukan sama                                              |
| Validasi                       | `formspec validate` menolak entri di luar keempat bentuk (`JPG`, `*.jpg`, `"jpg, png"`)          |
| Banyak file                    | `max_count > 1` (bukan tipe field terpisah)                                                      |
| Menampilkan gambar             | `widget: image` pada `TableColumn` — masuk kosakata tertutup `TableCellWidget` (S10)             |
| Nilai field                    | object key (string), atau array key bila `max_count > 1`                                         |
| URL                            | satu helper `fileDownloadUrl(workspace, module, entity, id, field)` (route unduh = `src` gambar) |
| Non-gambar                     | tetap tautan unduh (perilaku lama), tidak error                                                  |
| Turunan                        | field file yang `allowed_types`-nya memuat gambar otomatis `widget: image`                       |

## File

- `pkg/spec/storage.go` (baru) — `ValidateStorageSpec` + `StorageAllowsImage`.
- `pkg/spec/entity.go` — panggil validasi untuk field file; `widget.go` — `WidgetImage`.
- `internal/api/file.go` — matcher menerima ekstensi tanpa titik.
- `internal/genjsonschema` — regenerasi (enum `TableCellWidget`).
- Klien: `lib/media.ts` (baru: `allowedFileType`, `isImageFile`,
  `storageAllowsImage`, `fileDownloadUrl`), `lib/renderCell.tsx`
  (`CellRenderOpts` + cabang `image` + `cellHintsForField`), `engine/derive.ts`,
  `widgets/catalog.ts`, `widgets/FileInput.tsx` (pakai helper bersama),
  `kinds/listing`, `kinds/table`, `kinds/page/DetailPage`, `kinds/form/PickerPanel`,
  `hooks/useSurface.ts` (ekspos `workspace`).

## Bukti

`pkg/spec` 5 test · `internal/api` `TestAllowedFileType` (8 case) · klien
`lib/media.test.ts` (8 case) · parity `catalog.test.tsx` · `go test ./...` hijau ·
`vitest` 258 · `tsc` bersih · kafe `validate` 0 problem.

## Sisa (dicatat di TODO)

Tanpa verifikasi runtime di browser (butuh sesi untuk upload); kafe belum punya
kolom tabel ber-`photo`; Print belum ikut (butuh URL absolut); `transform`
thumbnail belum diverifikasi.

## Estimasi: **medium** (matcher ×2, kosakata tertutup, renderer bersama, 6 file klien)
