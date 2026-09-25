# Plan — kolom relasi tampil UUID & angka tanpa pemisah ribuan

Sumber: laporan runtime pengguna di
`/kafe/app/pos/cafe-stock/stock-levels`:

1. "mengapa cabang dan bahan masih uuid?"
2. "mengapa saldo tidak ada pemisah ribuan?"

## Masalah 1 — kolom relasi menampilkan kunci, bukan nama

`stock-level-table.yaml` mendeklarasikan `field: branch_id` dan
`field: ingredient_id`. API mengembalikan **dua-duanya** — skalar FK **dan**
objek relasi yang sudah di-resolve:

```json
"branch_id": "01a0bf3d-…", "branch": { "id": "…", "name": "Kafe Senayan", … }
```

tapi tidak ada satu pun jalur render yang memakai objek itu:

- `derive.ts` menulis ulang kolom turunan jadi `branch.name` (dot-path), jadi
  tabel **turunan** benar;
- `DetailPage.tsx` punya resolusi sendiri (blok ~40 baris) untuk halaman detail;
- `TableRenderer`/`ListingRenderer` hanya `String(value)` → **UUID**.

Jadi tabel **tertulis** yang menyebut FK-nya langsung menampilkan UUID —
padahal datanya sudah ada di baris yang sama. Terdampak juga
`order-table-pos.yaml` + `order-table-customer.yaml` (`dining_table_id`) dan
`visit/tables/list.yaml` (`polyclinic_id` pada filter).

Catatan kontrak yang ditemukan: `sort=branch.name` **422 unknown field**
(validasi `internal/api/handler.go` hanya menerima nama field entity), sehingga
konvensi dot-path `derive.ts`/clinic **tidak bisa disortir server-side** —
sementara `sort=branch_id` bekerja (200). Karena itu perbaikan diletakkan di
**renderer**, bukan dengan memaksa manifest pindah ke dot-path.

## Masalah 2 — angka tanpa pemisah ribuan

`quantity_on_hand` bertipe `decimal` (`scale: 3`) dan kolomnya tidak
mendeklarasikan `format`. Kosakata `renderCellValue` hanya punya
`currency`/`date`/`relative`/`percent` — **tidak ada** cabang angka — sehingga
jatuh ke `String(value)` → `20000`.

`formatter.number()` sudah ada dan sudah benar, tetapi hanya dipakai widget
Dashboard dan `NumberInput` (readonly); tidak ada satu pun kolom tabel/laporan
yang bisa memintanya. Jadi ini bukan bug formatir, melainkan **kosakata yang
kurang**: tidak ada cara menyatakan "kolom ini angka".

**Jebakan yang harus dihindari:** `formatter.number()` memakai
`settings.decimal_scale` (2), sedangkan field boleh punya `scale` sendiri
(`quantity_on_hand` = 3, `weight_per_unit` = 3). Memakai skala global untuk
field ber-`scale: 3` akan **membulatkan digit yang tersimpan** (1,234 → "1,23")
— lie, bukan sekadar tampilan. Jadi skala field harus menang.

## Konstruk

| Hal             | Keputusan                                                                                                                                                                                                                                                            |
| --------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Tampilan relasi | Helper bersama `lib/relation.ts`: sel relasi menampilkan **label record terkait** (`label_field` entity tujuan, fallback `name`), dibaca dari objek alias yang sudah dikirim API. Alias mengikuti aturan server: `patient_id` → `patient`, selain itu nama resource. |
| Dot-path        | `branch.name` tetap bekerja lewat `getNestedValue` yang sudah ada — kedua ejaan benar, tidak ada yang perlu diubah di manifest.                                                                                                                                      |
| Format angka    | `format: number` ditambahkan ke kosakata sel, **opt-in** — tidak diturunkan dari tipe (`decimal`/`integer` sering bukan kuantitas: `line_number`, kode; prinsip "komponen tidak pernah menebak").                                                                    |
| Skala           | `formatter.number(value, scale?)` — skala field menang atas `settings.decimal_scale`. `format: number` memakai `field.scale`.                                                                                                                                        |
| Aplikasi        | `stock-level-table.yaml`: `format: number` pada Saldo.                                                                                                                                                                                                               |

## File

- `src/lib/relation.ts` (baru) — `relationDisplay(record, fieldName, entity, findEntity)`.
- `src/lib/format.ts` — `number(value, scale?)`.
- `src/lib/renderCell.tsx` — cabang `format === "number"` + `opts.scale`.
- `src/kinds/table/TableRenderer.tsx`, `src/kinds/listing/ListingRenderer.tsx` —
  pakai helper relasi + kirim `scale` field.
- `examples/kafe/spec/modules/cafe-stock/tables/stock-level-table.yaml`.
- Docs: `docs/spec/frontend/06-page-kinds.md` (§3.1.1 lanjutan: kosakata
  `format` + aturan sel relasi), `docs/kind/ui/Table.md`.
- Test: `src/lib/relation.test.ts`, tambahan `src/lib/format.test.ts`,
  DOM di `TableRenderer`/`ListingRenderer`.

## Verifikasi

- Browser: Cabang → "Kafe Senayan", Bahan → "Beras Putih", Saldo → "20.000".
- Unit: helper relasi (FK `_id`, alias non-`_id`, objek hilang → jangan
  menampilkan UUID?), skala menang atas global.
- `vitest` penuh · `tsc` · `go build ./...` · `kafe validate`.

## Sisa yang sudah terlihat (jadi item todo)

- **Sortir kolom relasi menyortir UUID** — `sortable: true` pada `branch_id`
  mengirim `sort=branch_id`; server tidak bisa menyortir lewat nama relasi
  (dot-path 422), jadi "urutkan menurut Cabang" mengurutkan UUID acak.
- **`TableColumn.format` belum himpunan tertutup** — beda dari
  `ReportColumn.format` (enum). Salah ketik lolos dan mencetak nilai mentah.
  (Sejalan dengan 10.27/5.18.3 soal `link`.)

## Estimasi: **medium**
