# 2026-09-25-006 — Field `help`: warisan `description` entity + tutup situs bolong

## Apa yang diubah

Pertanyaan pengguna pada `promo-form`: "di entity field ada `description`, di
form field ada `help`, apa yg ditampilkan di ui?" Jawabannya: **hanya `help`**,
dan hanya di tiga situs — sisanya hilang.

**Akar — dua kosakata untuk satu hal.** `Field.description` (Entity) dan
`FormField.help` (Form) berarti sama bagi pengguna, tetapi hanya jalur
**derivasi** yang menghubungkannya: `formField()` (`engine/derive.ts`) membaca
`field.description`; `resolveForm()` mengembalikan manifest authored apa adanya.
Jadi begitu entity punya `kind: Form` — biasanya demi urutan/section/
`visible_when`, **bukan** demi help — setiap `description` hilang, dan penulis
menyalinnya manual. Salinan manualnya masih ada di repo: `promo-form.yaml`
dan `promo/entity.yaml` memuat kalimat **identik** untuk `branch_id` dan
`priority`; `menu-item-form.yaml` untuk `prep_station` dan `is_available`.

**Perbaikan — satu resolver.** `withEntityFieldLabels()` menjadi
`withEntityFieldDefaults()`: mengisi `label` **dan** `help` yang kosong, plus
`sections[0].description` dari `metadata.description` entity. Helper baru
`entityFieldHelp()` sejajar `entityFieldLabel()`. Dipasang di **keempat** jalur
`resolveForm()`, sehingga renderer tetap tidak perlu tahu derived vs authored.

**Lubang yang ditutup.**

| Situs                                    | Sebelum                                                                                 | Sesudah                      |
| ---------------------------------------- | --------------------------------------------------------------------------------------- | ---------------------------- |
| `WizardFormStep.renderField`             | help hanya di cabang `relation` (1 dari 6)                                              | keenam cabang                |
| `WizardRenderer` `steps[].fields` inline | 0 help                                                                                  | help dirender                |
| `WizardFormStep`/`WizardRenderer`        | `getForm()` → entry mentah, lewat `resolveForm()`                                       | memanggil resolver yang sama |
| `OverlayHost` subtitle                   | `form.spec.sections[0].description` mentah → selalu `"Fill in the details for this X."` | deskripsi entity (resolver)  |

Kontradisi yang terlihat pengguna: pada wizard close-shift, satu layar
menampilkan help untuk `supervisor_id` (relation) sambil membuang help
`counted_cash` (money) di step sebelumnya.

**Anotasi & teks.** `FormField.Help` + `Field.Description` kini ber-anotasi
`@schema` (sebelumnya kolom Deskripsi kosong di schema & kind docs) —
`schemas/` dan `docs/kind/` di-regenerate. Deskripsi user-facing diganti di dua
tempat yang terbaca sebagai catatan desain
(`menu-item.prep_station`/`is_available`), sisa 81 deskripsi di `examples/kafe`
→ **todo 5.23.2 ⏸️**.

## Kenapa

Mendeklarasikan kind UI tidak boleh menurunkan kualitas informasi pada layar.
Kelas ini sudah ditutup untuk `label`/`title` (`2026-09-24-008`); ini sisi
`help`-nya, plus tiga situs yang tidak bisa ditutup warisan saja.

## File terdampak

- `renderers/react-shadcn/src/engine/derive.ts` — `withEntityFieldDefaults()`,
  `entityFieldHelp()`, `resolveForm()`
- `renderers/react-shadcn/src/engine/derive.test.ts` — +11 test
- `renderers/react-shadcn/src/kinds/wizard/WizardFormStep.tsx`,
  `kinds/wizard/WizardRenderer.tsx`
- `renderers/react-shadcn/src/shell/OverlayHost.tsx`
- `pkg/spec/entity.go`, `pkg/spec/frontend.go` — anotasi `@schema`
- `schemas/`, `docs/kind/` — regenerasi
- `examples/kafe/spec/modules/cafe-master/forms/promo-form.yaml`,
  `forms/menu-item-form.yaml`, `master/menu-item/entity.yaml` — de-redundansi
- `docs/spec/frontend/06-page-kinds.md` §2, `docs/renderers/shadcn-shell/02-derivation-engine.md` §2,
  `docs/kind/ui/Form.md`, `ai_skills/entity-authoring/SKILL.md`, `ai_skills/form-layout/SKILL.md`

## Verifikasi

- **Test:** `vitest` **444 lulus** (29 file; baseline 403, +41 — 11 di antaranya
  dari `derive.test.ts`). Dibuktikan gagal sebelum patch: menetralkan warisan
  → **3 failed | 30 passed** pada `derive.test.ts`. `tsc -b` bersih,
  `oxlint` 0 temuan pada file yang disentuh.
- **Browser** (kafe `:8099`, `manajer`, drawer promo): help field
  `["Mis. HAPPY-HOUR-20", "Menentukan field nilai mana yang dipakai.", …,
"Kosong = berlaku di semua cabang", "Dipakai memilih satu promo terbaik…"]`
  — `branch_id`/`priority` tampil **tanpa** `help:` di YAML. Subtitle drawer
  berubah `"Fill in the details for this promo."` →
  `"ATURAN promo (bukan pemakaian) — disimpan dan dievaluasi saat pemesanan"`.
- **Browser** (wizard close-shift `?step=1`): step "Hitung Uang Fisik" kini
  menampilkan **dua** help dari dua cabang berbeda —
  `["Jumlah uang fisik di laci, apa adanya.", "Wajib diisi bila ada selisih, …"]`
  (money + string); sebelumnya satu pun tidak dirender.
- `go test ./...` hijau (0 FAIL) · `formspec check -f examples/kafe/spec` →
  **0 error, 0 warning**.

Plan `docs_internal/plan/field-help-inheritance.md`; sisa → **todo 5.23.2 ⏸️**.
