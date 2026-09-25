# Plan — Widget `select-multi-tag` (pilih dari daftar, tampil sebagai tag)

**Status:** Selesai · **Tanggal:** 2026-09-24
**Referensi:** `docs/spec/frontend/07-component-kinds.md` §1 (katalog widget tertutup),
`docs/spec/backend/05-field-types.md` §1.1 (`json`/`string`), `docs/spec/frontend/06-page-kinds.md` §2 (Form)
**Todo:** `docs_internal/plan/todo.md` 5.10.15 · kafe `examples/kafe/gaps_found/TODO.md` 10.33
**Changelog:** `docs_internal/changelog/2026-09-24-010`

> **Selesai 2026-09-24.** Semua langkah 1–5 landed. Bukti dan angka ada di
> changelog; sisa yang sengaja tidak ditutup tercatat sebagai
> `todo.md` 5.10.17 ⏸️ (Wizard tidak memakai kosakata widget) dan 5.10.18 ⏸️
> (filter `select` belum membaca `options`).

## Masalah

Field `promo.days_of_week` (`type: json`, nilai `[1,2,3,4,5]`) dirender `JsonInput`:
penulis spec harus mengetik JSON mentah, dan `1..7` tidak pernah terlihat sebagai
hari. Sudah ada `widget: tags` — tetapi `tags` menerima **ketikan bebas**, jadi
nilai di luar `1..7` bisa dibuat (`[9]` tetap tersimpan). Yang dibutuhkan: tag
yang **hanya** bisa diisi dari himpunan pilihan yang dideklarasikan.

Belum ada tempat untuk mendeklarasikan himpunan itu: `enum_values` hanya
`[]string` **tanpa caption**, jadi `1` tampil sebagai `1`, bukan `Senin`.

## Keputusan

1. **Atribut baru `Field.options`** (`[]FieldOption{value, label}`) di Entity —
   himpunan tertutup + caption untuk field yang nilainya daftar. Ditaruh di
   **Entity**, bukan Form, karena "nilai apa yang sah" adalah properti **data**
   (sama seperti `enum_values`), bukan properti satu form; dengan begitu
   filter/tabel/detail bisa memakai daftar yang sama.
   - `value` — nilai tersimpan, bentuknya **dipertahankan** (`value: 1` tetap
     angka 1, bukan `"1"`), supaya array `json` tetap array angka.
   - `label` — caption; opsional, default = nilai itu sendiri di-humanise.
2. **Widget baru `select-multi-tag`** — tag input yang sumber nilainya
   deklarasi (`options`, fallback `enum_values`), bukan ketikan.
   - Opsi yang sudah dipilih **tidak ditawarkan lagi** (tidak ada duplikat).
   - Chip diurutkan menurut **urutan deklarasi** (untuk himpunan terurut seperti
     hari, `Senin..Jumat` terbaca alami), bukan urutan klik.
   - Nilai tersimpan yang **tidak ada di daftar** (data lama / opsi diubah)
     tetap ditampilkan sebagai chip bertanda — **tidak** dibuang diam-diam saat
     save. Ini kelas kegagalan senyap yang sama dengan S10/5.18.x.
   - Nilai non-daftar (mis. object di field `json`) → **error terlihat**, bukan
     degradasi ke `[]` yang menyembunyikan data.
3. **Derivasi**: field `json` yang mendeklarasikan `options` → `select-multi-tag`.
   Tanpa `options` tetap `json` (tidak ada spec lama yang berubah perilaku).
4. **Detail Page**: field `json` ber-`options` dirender chip berlabel, bukan
   `<pre>` JSON mentah.

## Kontrak nilai

| Tipe field | Bentuk masuk             | Bentuk keluar                                                               |
| ---------- | ------------------------ | --------------------------------------------------------------------------- |
| `json`     | array                    | array (nilai opsi apa adanya; key tak dikenal dipertahankan sebagai string) |
| `string`   | string (comma-separated) | string (comma-separated)                                                    |

Mengikuti kontrak `TagsInput` (value-type-aware): tipe nilai tidak berubah
karena widget ini.

## File yang dibuat/diubah

| File                                                       | Perubahan                                                       |
| ---------------------------------------------------------- | --------------------------------------------------------------- |
| `pkg/spec/entity.go`                                       | `FieldOption`, `Field.Options`, validasi (duplikat/tipe/kosong) |
| `pkg/spec/field_options_test.go`                           | test validasi                                                   |
| `pkg/spec/widget.go`                                       | `WidgetSelectMultiTag = "select-multi-tag"` + daftar tertutup   |
| `pkg/spec/widget_test.go`                                  | pin anggota himpunan                                            |
| `renderers/react-shadcn/src/types/manifest.ts`             | `FieldOption`, `Field.options`                                  |
| `renderers/react-shadcn/src/lib/field-options.ts`          | resolusi opsi + parse/serialize nilai                           |
| `renderers/react-shadcn/src/widgets/SelectMultiTag.tsx`    | komponen widget                                                 |
| `renderers/react-shadcn/src/widgets/{index.ts,catalog.ts}` | barrel + katalog runtime                                        |
| `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx`   | `case "select-multi-tag"`                                       |
| `renderers/react-shadcn/src/engine/derive.ts`              | `json` + `options` → `select-multi-tag`                         |
| `renderers/react-shadcn/src/kinds/page/DetailPage.tsx`     | chip berlabel untuk `json` ber-`options`                        |
| `examples/kafe/.../promo/entity.yaml`                      | `options` 1..7 + label hari                                     |
| `examples/kafe/.../forms/promo-form.yaml`                  | `widget: select-multi-tag`                                      |
| `docs/spec/frontend/07-component-kinds.md`                 | himpunan tertutup + §1.3                                        |
| `docs/spec/backend/05-field-types.md`                      | atribut `options`                                               |
| `docs/renderers/shadcn-shell/03-kind-renderers.md`         | daftar widget (25)                                              |
| `schemas/` (+`dist/`)                                      | hasil `make generate-schema` / `publish-schemas.sh --stage`     |
| `docs/kind/**`                                             | hasil `make generate-kind-docs`                                 |

## Dependensi & urutan

1. Go spec (`pkg/spec`) → schema. Tanpa ini widget tak bisa ditulis di manifest.
2. Katalog + komponen TS → router.
3. Derivation & DetailPage.
4. Kafe spec.
5. Docs + changelog + todo.

Effort: **medium** (kontrak field baru + widget + paritas schema/katalog/renderer).

## Verifikasi

1. `go test ./pkg/spec/` — validasi opsi + himpunan tertutup.
2. `cd renderers/react-shadcn && npx vitest run` — `SelectMultiTag.test.tsx`,
   `field-options.test.ts`, `catalog.test.tsx` (paritas), `derive.test.ts`.
3. `npx tsc -b` dan `npx oxlint` di `renderers/react-shadcn`.
4. `make generate-schema` + `formspec validate` di `examples/kafe` → 0 problem.
5. E2E browser `:8099` — buka
   `/kafe/app/pos/cafe-master/promos?action=create&form=promo-form&mode=drawer`:
   pilih `Senin`, buka dropdown → `Senin` **tidak ada lagi**; simpan; buka ulang
   → tersimpan `[1]` (angka, bukan `"1"`).

## Sisa yang mungkin terbuka (kandidat `⏸️`)

- ✅ **Filter `select` di Table belum memakai `label` dari `options`** —
  terkonfirmasi saat implementasi dan dicatat sebagai **5.10.18 ⏸️**; bukan
  bagian dari pekerjaan ini karena menyentuh `useSelectFilterOptions`, jalur
  yang sudah punya dua cabang (enum/relasi) sendiri.
- ✅ **Sel tabel ber-`options`** — dikerjakan di sini: satu resolver bersama
  (`resolveColumnCell` di `lib/renderCell.tsx`) melayani Table, Listing, dan
  ChildTable sekaligus, sehingga tidak ada dua jalur yang bisa berbeda.
- **Wizard step tidak memakai kosakata widget sama sekali** — ditemukan saat
  menelusuri pembaca `widget:`; kelas yang sama (widget katalog diabaikan), tapi
  batasnya sudah ada sebelum perubahan ini. Dicatat sebagai **5.10.17 ⏸️**.
