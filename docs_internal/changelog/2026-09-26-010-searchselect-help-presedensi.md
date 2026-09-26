# SearchSelect quick-create: `help` mengikuti presedensi bersama (todo 5.23.3)

## Apa yang diubah

`SearchSelect` (`renderers/react-shadcn/src/kinds/wizard/SearchSelect.tsx`)
adalah **satu-satunya** situs baca-Form yang tidak memakai presedensi `help`.
Ia hanya memanggil `entityFieldLabel`, jadi dialog quick-create di wizard adalah
tempat **satu-satunya** di aplikasi di mana `description` sebuah Entity tidak
pernah sampai ke pengguna: label diwarisi, help tidak.

Kini ia memakai presedensi yang sama dengan `withEntityFieldDefaults` dan
`06-page-kinds.md` §2:

```
help: (authored)  →  Entity field `description`  →  tidak dirender
```

**Kelima cabang `renderField` (date, enum, boolean, numeric, default)
dikonsolidasikan lewat satu helper lokal `wrap(control)`.** Sebelumnya tiap
cabang menulis `<label>` + kontrolnya sendiri, jadi menambahkan help berarti
mengedit lima tempat dan satu yang terlewat tidak terlihat — persis bagaimana
kelima cabang itu sama-sama tidak punya help sejak awal. Setelah `wrap`, help
dirender **satu kali** untuk semua cabang.

Item 5.23.3 menyatakan "putuskan apakah SearchSelect memang perlu help — ia layar
pemilihan, bukan input — lalu samakan atau dokumentasikan alasannya". Jawabannya
menyempit begitu kodenya dibaca: **`SearchSelect` memang layar pemilihan, tetapi
ia juga punya dialog quick-create berisi input sungguhan** (`renderField`,
`handleCreate` → `apiPost`). Itu bagian yang tidak punya help, dan bagian itulah
yang diperbaiki; layar pencariannya sendiri tidak berubah.

## Kenapa

Melanjutkan fase 5.23 (field `help`) yang menutup 5.23.1. Tanpa ini, warisan
`description → help` yang baru dibangun punya lubang yang tidak terlihat: field
yang sama menampilkan help di form utama dan tidak menampilkannya di dialog
wizard yang membuat record dengan field itu.

## File terdampak

- `renderers/react-shadcn/src/kinds/wizard/SearchSelect.tsx` — `entityFieldHelp`
  - helper `wrap(control)`, 5 cabang dikonsolidasikan
- `renderers/react-shadcn/src/kinds/wizard/search-select-help.test.tsx` — **baru** (10 test)

## Bukti

- `npx vitest run` → **499 lulus** / 35 file (baseline 473 / 31; +10 test ini,
  sisanya dari batch lain di sesi yang sama). `npx tsc -b` bersih.
- **Dibuktikan gagal:** satu cabang dikembalikan ke bentuk lamanya
  (`<label>` + kontrol manual tanpa `wrap`) → `routes EVERY field branch through
'wrap'` gagal (`Expected 5, Received 4`), hijau sesudah dikembalikan.
  Catatan kalibrasi: percobaan pertama memakai ambang "label ≤ 2" dan **tidak**
  menangkap regresi itu (cabang boolean membangun labelnya lintas baris sehingga
  regex label meleset) — ambangnya diganti menjadi menghitung `return wrap(`,
  yang memang membedakan cabang yang dikonsolidasikan dari yang tidak.
- Test lain menegaskan `entityFieldHelp` sendiri: description diwarisi, relasi
  cocok lewat sufiks `_id`, dan **`undefined` bukan `""`** saat tidak ada apa pun
  — pembedaan itu yang membuat "diwarisi" dan "sengaja dikosongkan" tidak
  tertukar.

## Rujukan

Todo **5.23.3** (tertutup), melengkapi **5.23.1** (`docs_internal/changelog/2026-09-25-006`)
· presedensi normatif di `docs/spec/frontend/06-page-kinds.md` §2.
