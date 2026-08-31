# Feat: Type-ahead search pada ThemedSelect (enum/select field)

## Masalah

Field `enum` (mis. `status` di `cafe-master/table`) dirender oleh
`ThemedSelect` — dropdown custom berbasis `<button>`, bukan native `<select>`.
Dropdown custom ini hanya mendukung klik + navigasi panah, **tidak ada
type-to-search**. User mengetik "O" (mis. ingin lompat ke "Occupied") tidak
melakukan apa-apa, tidak seperti native `<select>` yang punya type-ahead.

## Perbaikan

Tambahkan type-ahead search (gaya native `<select>`) di komponen bersama
`renderers/react-shadcn/src/components/ui/select.tsx`:

- Ketik huruf/angka → buffer type-ahead diakumulasi (case-insensitive) dan
  `focusedIndex` melompat ke opsi pertama yang labelnya diawali buffer
  (mulai dari posisi fokus saat ini, wrap-around).
- Bekerja baik saat dropdown terbuka maupun tertutup (jika tertutup, otomatis
  terbuka dulu).
- Buffer di-reset setelah jeda 600ms atau saat dropdown ditutup/opsi dipilih.
- Karena perubahan di komponen bersama, semua permukaan yang memakai
  `ThemedSelect` ikut dapat fitur ini: form enum, filter tabel/kanban/listing,
  wizard, login, dan ChildTable.

## File terdampak

- `renderers/react-shadcn/src/components/ui/select.tsx` — type-ahead search

## Referensi

- `examples/cafe/spec/modules/cafe-master/master/table/entity.yaml` — field
  `status` enum yang memicu permintaan ini
