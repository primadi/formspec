# Plan — Cardinality `options` di Entity (single vs multi): Form mengikuti Entity

**Status:** ✅ Selesai · **Tanggal:** 2026-09-25
**Referensi:** `docs/spec/backend/05-field-types.md` §1.1.1 (atribut `options`),
`docs/spec/frontend/07-component-kinds.md` §1/§1.3 (katalog widget),
`docs/spec/backend/01-core-basic.md` §2 (field & tipe)
**Prasyarat:** plan `docs_internal/plan/select-multi-tag-widget.md` +
changelog `2026-09-24-010` (atribut `Field.options` + widget `select-multi-tag`)
**Todo:** `docs_internal/plan/todo.md` 5.10.19 (selesai; 5.10.18 ikut tertutup) ·
kafe `examples/kafe/gaps_found/TODO.md` 10.33 (lanjutan)
**Changelog:** `docs_internal/changelog/2026-09-25-007-options-cardinality-entity.md`

> **✅ Selesai 2026-09-25.** Semua fase landed. Ringkasan bukti ada di changelog
> `2026-09-25-007`: 5 uji negatif (3 di `validate`, 2 di `check`), E2E browser
> kafe (chip urut deklarasi, DB `[5,1]` bertipe int, `channel='qris'` skalar),
> vitest 454, `tsc -b` bersih, kafe `validate` 85 manifest 0 problem,
> `check` 0/0. Sisa yang sengaja tidak ditutup → `todo.md` 5.10.20–5.10.24 ⏸️.

## Masalah

`Field.options` (landed 2026-09-24) mendeklarasikan **himpunan nilai yang sah +
caption**, tetapi tidak menyatakan **cardinality** — apakah field memegang satu
nilai atau banyak. Konsekuensinya:

1. **Keputusan single/multi hidup di Form, bukan Entity.** `promo.days_of_week`
   (`type: json`) hanya terbaca sebagai "pilihan hari" dari
   `promo-form.yaml` baris 44 → `widget: select-multi-tag`. Form adalah tempat
   yang salah untuk keputusan itu: "nilai apa yang sah, dan berapa banyak" adalah
   properti **data** — alasan yang sama yang menaruh `options` di Entity.
2. **`widget:` wajib ditulis tangan.** Form yang ditulis (`kind: Form`) tidak
   mendapat derivasi widget: `FormFieldWidget` memakai
   `field.widget ?? implicitWidgetForType(entityField.type)`, dan
   `implicitWidgetForType` hanya mengenal `money`/`time` — jadi field `json`
   ber-`options` jatuh ke editor JSON mentah. Itu memaksa penulis spec mengulang
   keputusan Entity di Form (dan menyimpannya di dua tempat).
3. **Single-select ber-caption tidak bisa dinyatakan sama sekali.**
   `enum_values` membawa nilai saja tanpa caption, jadi `1` tetap tampil `1`.
   Godoc `pkg/spec/entity.go` sudah menyebut celah ini ("a known, separately
   tracked gap") tetapi **tidak ada item todo bernomor** yang melacaknya — persis
   pola "gap sebagai prosa" yang dilarang `AGENTS.md`.

Asimetri yang membuat keluhan ini konkret: jalur **derivasi** sudah mengikuti
Entity (`engine/derive.ts` `formWidget()`: `json` + `options` →
`select-multi-tag`), sementara jalur **authored** tidak. Dua jalur, satu kosakata,
satu bolong.

## Keputusan

1. **`Field.multiple` (pointer bool) di Entity.** `true` = himpunan, `false` =
   satu nilai. Pointer karena "tidak dinyatakan" harus bisa dibedakan dari
   `false` (preseden `Precision *int`/`Scale *int`, `FormRender.Confirm *string`).
2. **Wajib bila tipe-nya container ambigu** (`json`, `string`): keduanya bisa
   memegang satu nilai **atau** daftar, jadi tipe saja tidak menentukan.
   Untuk skalar (`integer`, `date`, …) absen berarti `false`.
3. **`options` kini sah di field skalar non-enum** = single-select ber-caption
   (mis. `type: integer` + `options 1=Senin`). `enum` **tetap** hanya
   `enum_values` — tanpa overlay caption, supaya tidak ada dua sumber kebenaran
   untuk himpunan yang sudah punya CHECK constraint.
4. **Form mengikuti Entity**, ditegakkan di dua lapis:
   - **derivasi**: `formWidget()` menjadi satu sumber untuk kedua jalur, sehingga
     authored Form tidak perlu menulis `widget:` untuk field ber-`options`;
   - **gerbang**: `formspec check` menolak `widget:` yang bertentangan dengan
     cardinality Entity (dua arah).
5. **Konsumen turunan ikut disertakan**: filter `select` (menutup 5.10.18), sel
   tabel + halaman detail untuk nilai tunggal ber-caption, dan derivasi kolom
   Table/Listing/Kanban dari `options`. **Ditunda:** Wizard (tetap 5.10.17).

## Matriks tipe × cardinality (normatif)

| tipe field                                                                     | `multiple` absen         | `multiple: false`   | `multiple: true`                     |
| ------------------------------------------------------------------------------ | ------------------------ | ------------------- | ------------------------------------ |
| `json`                                                                         | ERROR bila ada `options` | satu nilai (skalar) | array → `select-multi-tag`           |
| `string`                                                                       | ERROR bila ada `options` | satu nilai (string) | comma-separated → `select-multi-tag` |
| `integer`, `decimal`, `percent`, `date`, `datetime`, `time`, `boolean`, `uuid` | single (default)         | single (eksplisit)  | ERROR (tipe menyimpan satu nilai)    |
| `enum`                                                                         | —                        | —                   | `options` ditolak → `enum_values`    |
| `text`, `richtext`, `money`, `file`, `attachment`, `relation`, `child`         | `options` ditolak        | —                   | —                                    |

Tambahan: `multiple` **tanpa** `options` → ERROR (tidak ada arti).

## Batas yang sengaja tidak ditutup

- **Tidak ada penegakan server baru.** Nilai di luar deklarasi tetap
  dipertahankan (desain `2026-09-24-010`: data lama, atau spec yang himpunannya
  menyusut, tidak boleh hilang senyap saat save). Cardinality hanya menentukan
  **bentuk** nilai, bukan memvalidasi keanggotaannya. Penegakan server dicatat
  sebagai item todo bernomor (bukan prosa).
- **Wizard** tidak memakai kosakata widget sama sekali → tetap 5.10.17 ⏸️.
- **`enum` tanpa caption** hanya tertutup sebagian (field non-enum). Godoc Go
  yang menyebut "separately tracked gap" diarahkan ke item bernomor.
- **`formspec generate`** (TS) belum menghasilkan union dari `options`.

## Nilai & bentuk

| Cardinality | Tipe     | Bentuk masuk        | Bentuk keluar                         |
| ----------- | -------- | ------------------- | ------------------------------------- |
| `true`      | `json`   | array               | array (nilai opsi apa adanya)         |
| `true`      | `string` | string koma-dipisah | string koma-dipisah                   |
| `false`     | `json`   | skalar              | skalar (tipe deklarasi dipertahankan) |
| `false`     | `string` | string              | string                                |

`value: 1` tetap angka `1` (bukan `"1"`) — mengikuti aturan `FieldOption.Value`
yang sudah ada.

## File yang dibuat/diubah

| File                                                                  | Perubahan                                                                                                                                    |
| --------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| `pkg/spec/entity.go`                                                  | `Field.Multiple`, `validateFieldOptions{Shape,Cardinality}`, `isOptionScalarType`, `isAmbiguousOptionContainer`, `WidgetCardinalityMismatch` |
| `pkg/spec/field_options_test.go`                                      | matriks cardinality + kasus yang berubah (`integer`+`options` kini sah)                                                                      |
| `pkg/spec/widget_test.go`                                             | test gerbang `WidgetCardinalityMismatch` (dua arah)                                                                                          |
| `cmd/formspec/check.go`                                               | `entityIndex.fieldDecl` + pemanggilan gerbang di `checkForms`                                                                                |
| `cmd/formspec/check_test.go`                                          | test `check` dua arah kontradiksi + satu kasus sah                                                                                           |
| `internal/genjsonschema/generator.go`                                 | verifikasi (tipe skalar; tanpa entri `sharedTypes` baru)                                                                                     |
| `renderers/react-shadcn/src/types/manifest.ts`                        | `Field.multiple?: boolean`                                                                                                                   |
| `renderers/react-shadcn/src/lib/field-options.ts`                     | `optionValueShape`, `parseSingleOptionValue`, `serializeSingleOptionValue`                                                                   |
| `renderers/react-shadcn/src/engine/derive.ts`                         | `formWidget` jadi satu sumber, `tableWidget`, `deriveKanbanColumns`                                                                          |
| `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx`              | `implicitWidgetForType` memakai derivasi bersama                                                                                             |
| `renderers/react-shadcn/src/widgets/{Select,RadioGroup,Combobox}.tsx` | opsi ber-caption + nilai skalar                                                                                                              |
| `renderers/react-shadcn/src/lib/renderCell.tsx`                       | nilai tunggal ber-`options`; `cellHintsForField` → `badge`                                                                                   |
| `renderers/react-shadcn/src/kinds/page/DetailPage.tsx`                | cabang nilai tunggal ber-`options`                                                                                                           |
| `renderers/react-shadcn/src/hooks/useSelectFilterOptions.ts`          | memakai `fieldOptions()` (menutup 5.10.18)                                                                                                   |
| `renderers/react-shadcn/src/kinds/{table,listing}/…Renderer.tsx`      | filter memakai helper bersama                                                                                                                |
| `examples/kafe/.../promo/entity.yaml`                                 | `multiple: true` pada `days_of_week`; `channel` single ber-caption                                                                           |
| `examples/kafe/.../forms/promo-form.yaml`                             | `widget: select-multi-tag` dihapus (bukti Form mengikuti Entity)                                                                             |
| `docs/spec/backend/05-field-types.md`                                 | §1.1.1 → matriks cardinality                                                                                                                 |
| `docs/spec/frontend/07-component-kinds.md`                            | §1/§1.3 → single vs multi + "Form mengikuti Entity"                                                                                          |
| `docs/renderers/shadcn-shell/03-kind-renderers.md`                    | catatan katalog + derivasi                                                                                                                   |
| `ai_skills/entity-authoring/SKILL.md`                                 | baris `options`/`multiple`                                                                                                                   |
| `docs/kind/data/Entity.md`, `schemas/`                                | hasil `make generate-kind-docs` / `make generate-schema`                                                                                     |

## Dependensi & urutan

1. Kontrak Go + schema (tanpa ini widget tidak bisa ditulis di manifest).
2. Kontrak TS + derivasi (Form mengikuti Entity) — butuh 1.
3. Gerbang `formspec check` — butuh 1 (independen dari 2).
4. Permukaan baca + filter — sejalan dengan 2.
5. Adopsi kafe (bukti) — butuh 2 + 3.
6. Docs + changelog + todo.

Effort: **large** (satu atribut kontrak menyentuh validator, schema, derivasi,
empat renderer, filter, gerbang lintas-manifest, dan adopsi contoh).

## Verifikasi

Semua dijalankan 2026-09-25; angka di bawah adalah hasil terukur.

1. `go build ./...` bersih · `gofmt -l ./cmd ./internal ./resource ./pkg` bersih ·
   `go test` **38 paket hijau** (di luar `./resource/`, lihat poin 4).
2. **Uji negatif** (dijalankan pada salinan spec kafe, bukan pernyataan):
   - `multiple: true` dihapus dari `days_of_week` → `validate`: _"`multiple` is
     required on a json field with `options`"_ (1 problem);
   - `multiple: true` pada `integer` → _"`multiple: true` is not valid on a
     \"integer\" field"_ (1 problem);
   - `options` pada `enum` → _"an enum declares its values in `enum_values`"_
     (1 problem);
   - `widget: select` pada himpunan → `check`: _"picks one value, but the Entity
     declares the field a set (`multiple: true`)"_ (1 error);
   - `widget: select-multi-tag` pada nilai tunggal → `check`: _"needs a field
     that holds a set, but the Entity declares `multiple: false`"_ (1 error).
3. `make generate-schema` → `"multiple"` muncul di `schemas/formspec.schema.json`
   (baris 1252 & 5576, `type: boolean`, `nullable: true`); `make generate-kind-docs`
   → 34 kind docs ditulis ulang (34 dokumen, `docs/kind/curation/App.md` +
   `docs/kind/ui/Form.md` tersentuh).
4. `go test ./...` → `./resource/` **flaky pra-eksisting**, bukan regresi
   perubahan ini: HEAD bersih (`dd3adc6`, `git worktree`) gagal **2/8** run,
   working tree perubahan ini gagal **1/8** — setara. Akarnya `waitForJournal`
   menunggu baris journal **ada** lalu test meng-assert `status == "posted"`
   (dua momen berbeda). Dicatat sebagai 5.10.24 ⏸️.
5. `npx tsc -b` bersih · `npx vitest run` **454 lulus** (29 file; +3 dari 451 —
   fixture derivasi `json`+`options` kini menyatakan `multiple`, plus tes
   cardinality baru) · `npm run build` hijau.
6. `formspec validate --spec examples/kafe/spec --schema schemas` →
   **85 manifest, 0 problem**; `formspec check -f spec` → **0 error, 0 warning**.
7. **E2E browser `:8099`** (`manajer`/`kafe123`, App `kafe-pos`, form promo
   **tanpa** `widget:` di YAML): multi-tag hari menawarkan 7 opsi
   (`aria-multiselectable="true"`); klik **Jumat lalu Senin** → chip tampil
   **"Senin, Jumat"** (urutan deklarasi); single-select **Kanal** menampilkan
   caption `POS`/`QRIS`/`Online`; setelah simpan, DB berisi
   `days_of_week = [5, 1]` (**int**, urutan isian) dan `channel = 'qris'`
   (skalar); halaman detail menampilkan chip `Senin`/`Jumat` + caption `QRIS`.

## Sisa → item todo bernomor (bukan prosa)

- 5.10.17 ⏸️ Wizard (tidak termasuk, sudah tercatat sebelumnya).
- 5.10.20 ⏸️ Penegakan server cardinality/keanggotaan option-set.
- 5.10.21 ⏸️ Paritas lintas-shell untuk aturan "Form mengikuti Entity".
- 5.10.22 ⏸️ `formspec generate` belum menghasilkan union dari `options`.
- 5.10.23 ⏸️ Caption untuk `enum` (hanya sebagian tertutup; godoc menunjuk item ini).
- 5.10.24 ⏸️ `TestKafe_OnPaidCreatesBalancedJournal` flaky pra-eksisting (balapan
  `waitForJournal` ↔ `status == "posted"`), ditemukan saat verifikasi ini.
