# 2026-09-24-003 — `TableColumn.align`/`width` akhirnya diterapkan renderer

**Plan**: `docs_internal/plan/table-column-align-width.md`
**Todo**: kafe **10.26** (⏸️ → ✅ ditutup), **10.27 ⏸️** (dibuka — atribut
terakhir yang tersisa).

## Konteks

Kontrak tertulis, renderer tidak menepatinya. `TableColumn` punya `sortable`,
`width`, `align`, `link` (`pkg/spec/frontend.go`), dan
`docs/spec/frontend/06-page-kinds.md` §3 mendaftarkan keempatnya sebagai
didukung — tetapi `TableRenderer` meng-hardcode `text-left` pada `<th>` dan
`ListingRenderer` sama, jadi **dua dari empat diabaikan tanpa peringatan**:

```
$ grep -rn "col\.align\|col\.width" renderers/react-shadcn/src   → 0 hasil
```

Manifest kafe yang menyatakan niat lalu hilang diam-diam:
`order-table-pos.yaml` + `order-table-customer.yaml` (`align: right` pada Total),
`stock-level-table.yaml` (3 kolom numerik), `visit/tables/list.yaml` (6 `width`),
`journal-table.yaml` (2 `width`). Kelas yang sama dengan 10.25 — satu kosakata,
dua tempat, satu bolong.

## Yang diubah

- **`src/lib/tableColumn.ts`** (baru) — satu helper bersama untuk **kedua**
  renderer, supaya vocab-nya tidak bisa drift lagi:
  `columnAlignClass`, `columnJustifyClass`, `columnWidthStyle`.
- **`TableRenderer.tsx` / `ListingRenderer.tsx`** — `align` diterapkan ke
  `<th>` **dan** `<td>`. Mengapa keduanya wajib: `<td>` adalah _sibling_
  `<th>`, jadi `text-align` **tidak diwarisi** — memperbaiki header saja
  menghasilkan judul rata kanan di atas angka rata kiri. Header yang sortable
  membungkus labelnya di flex row, dan `text-align` tidak bisa menggeser anak
  flex, jadi ia juga butuh `justify-*`. `width` diterapkan ke `<th>` (kotak
  yang dipakai browser menentukan lebar kolom) plus `minWidth`.
- **`pkg/spec/frontend.go`** — `align` jadi **enum tertutup** (`@schema {enum}`),
  sehingga `align: rigth` ditolak validasi alih-alih diam-diam diabaikan;
  precedent: `SectionBlock.align`, `ReportColumn.format`. Schema + kind docs
  diregenerasi.
- **`docs/spec/frontend/06-page-kinds.md`** §3.1.1 (baru) — kontrak normatif
  keduanya, termasuk dua jebakan di atas. `docs/kind/ui/Table.md` dapat gotcha.

Verifikasi browser (sesi `manajer` app-scoped `kafe-pos`):
`stock-levels` → `Saldo`/`Biaya Rata-rata`/`Nilai Persediaan` `thAlign=right`
**dan** `tdAlign=right`, kolom lain tetap `left`; `orders` → `Total`
`text-right` + header `justify-end`.

Test: 10 case baru di `src/lib/tableColumn.test.tsx` — **4 dibuktikan gagal**
sebelum patch (2 sumber-paritas, 2 DOM). Suite frontend **334 lulus** (dari 324) · `tsc -p tsconfig.app.json --noEmit` bersih · `go build ./...` ok · kafe
`validate` **85 manifest, 0 problem**.

## Sisa

- **`TableColumn.link`** — atribut `TableColumn` terakhir yang tidak dikonsumsi
  (0 manifest memakainya; `grep "col\.link"` → 0 hasil). Semantik pengisian
  `:param` route Page dari record belum ditetapkan, jadi belum diimplementasikan
  → **10.27 ⏸️** + ditandai **Open** di spec §3.
- **Catatan atribusi diff schema.** `git diff schemas/formspec.schema.json`
  tampak besar (133 baris) karena generator membaca **seluruh** `pkg/spec`, dan
  working tree sudah memuat `pkg/spec/seed.go` (untracked, dari pekerjaan seed
  yang lebih dulu) — sehingga regen ikut menuliskan `Seed`/`SeedEntity`.
  Perubahan **dari sesi ini** hanya satu:
  `TableColumn.align` → `enum: ["", "left", "center", "right"]`
  (di `HEAD` masih `"left | center | right"` tanpa enum).

Referensi: `docs_internal/plan/table-column-align-width.md` ·
`docs/spec/frontend/06-page-kinds.md` §3.1.1.
