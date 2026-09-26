# Cardinality `options` di Entity — Form mengikuti Entity

**Tanggal:** 2026-09-25 · **Seq:** 007

## Apa

`Field.options` (landed `2026-09-24-010`) mendeklarasikan **himpunan nilai yang
sah + caption**, tetapi tidak menyatakan **cardinality** — berapa banyak dari
nilai itu yang dipegang field. Konsekuensinya terlihat langsung di kafe:
`promo.days_of_week` (`type: json`) hanya terbaca sebagai "pilihan hari" dari
`promo-form.yaml` baris 44 → `widget: select-multi-tag`. Keputusan single/multi
hidup di **Form**, padahal "nilai apa yang sah, dan berapa banyak" adalah
properti **data**.

Akarnya asimetri dua jalur: jalur **derivasi** sudah mengikuti Entity
(`engine/derive.ts` `formWidget()`: `json` + `options` → `select-multi-tag`),
sementara jalur **authored** tidak — `FormRenderer` memakai
`field.widget ?? implicitWidgetForType(type)`, dan `implicitWidgetForType` hanya
mengenal `money`/`time`, jadi field `json` ber-`options` jatuh ke editor JSON
mentah kecuali spec mengulang keputusan Entity.

Ditutup dengan **satu atribut kontrak** + penegakan dua lapis:

1. **`Field.multiple` (pointer bool) di Entity.** `json`/`string` bisa memegang
   satu nilai **atau** daftar, jadi `multiple` **wajib** bila `options` dinyatakan
   di keduanya; pada skalar absen berarti `false`. Pointer karena "tidak
   dinyatakan" harus bisa dibedakan dari `false` (preseden `Precision *int`).
2. **`options` kini sah di field skalar non-enum** = single-select ber-caption
   (`type: integer` + `options 1=Senin`) — celah yang selama ini disebut godoc
   sebagai "known, separately tracked gap" tanpa item todo bernomor. `enum`
   tetap hanya `enum_values` (sudah punya CHECK constraint di DB; caption-nya
   jalur terpisah supaya tidak ada dua sumber kebenaran).
3. **Form mengikuti Entity** di dua lapis: `deriveFormWidget` menjadi satu sumber
   untuk kedua jalur (authored Form berhenti menentukan widget sendiri), dan
   `formspec check` menolak widget yang bertentangan **dua arah**.
4. **Konsumen turunan ikut disertakan** — filter `select` (menutup 5.10.18), sel
   tabel + halaman detail untuk nilai tunggal ber-caption, dan derivasi kolom
   Table/Listing/Kanban dari `options`.

**Tanpa penegakan server baru**: nilai di luar deklarasi tetap dipertahankan
(desain 10.33 — data lama / spec yang menyusut tidak boleh hilang senyap).
Cardinality menentukan **bentuk** nilai, bukan memvalidasi keanggotaannya.

## Kenapa bukan sekadar mengubah manifest

Menghapus `widget: select-multi-tag` dari `promo-form.yaml` saja akan
mengembalikan field ke editor JSON mentah — perbaikannya justru **bergantung**
pada perubahan renderer. Yang dibutuhkan adalah tempat deklarasi cardinality
(Entity) + renderer yang membacanya di kedua jalur + gerbang yang menolak
kontradiksi saat spec ditulis.

## File

**Backend/spec:** `pkg/spec/entity.go` (`Field.Multiple`,
`validateFieldOptionsShape`/`Cardinality`, `isOptionScalarType`,
`isAmbiguousOptionContainer`, `FieldIsMultiple`, `WidgetCardinalityMismatch`,
`cardinalityWord`), `pkg/spec/field_options_test.go` (matriks cardinality + uji
gerbang), `pkg/spec/widget.go` (gofmt: align blok const yang tertinggal dari
sesi sebelumnya), `cmd/formspec/check.go` (`entityIndex.fieldDecl` + gerbang di
`checkForms`), `cmd/formspec/check_test.go` (kontradiksi dua arah + kasus sah).
Schema + kind docs regenerasi: `schemas/formspec.schema.json`,
`docs/kind/curation/App.md`, `docs/kind/ui/Form.md`.

**Frontend:** `types/manifest.ts` (`Field.multiple`), `lib/field-options.ts`
(`isMultiValue` re-export, `optionValueShape`, `parseSingleOptionValue`,
`serializeSingleOptionValue`), `engine/derive.ts` (`deriveFormWidget` satu
sumber, `isMultiValue`, `tableWidget` single→badge, `deriveKanbanColumns` baca
`options`), `kinds/form/FormRenderer.tsx` (`implicitWidgetForType` → derivasi
bersama; tiga picker membaca `fieldOptions`), `widgets/Select.tsx`
(`SelectChoice`/`normalizeChoice` + nilai skalar), `widgets/RadioGroup.tsx`,
`widgets/Combobox.tsx`, `widgets/SelectMultiTag.tsx` (`optionValueShape`),
`lib/renderCell.tsx` (nilai tunggal + badge hint), `kinds/page/DetailPage.tsx`,
`hooks/useSelectFilterOptions.ts` (**menutup 5.10.18**),
`kinds/{table,listing}/…Renderer.tsx` (filter/batch picker ber-caption).

**Contoh/docs:** `examples/kafe/.../promo/entity.yaml` (`multiple: true` pada
`days_of_week` + field baru `channel` single ber-caption),
`.../forms/promo-form.yaml` (`widget: select-multi-tag` **dihapus** — bukti Form
mengikuti Entity), `docs/spec/backend/05-field-types.md` §1.1.1 (matriks +
bentuk nilai), `docs/spec/frontend/07-component-kinds.md` §1.3/§1.4,
`docs/renderers/shadcn-shell/03-kind-renderers.md`,
`ai_skills/entity-authoring/SKILL.md`, plan
`docs_internal/plan/options-cardinality-entity.md`.

## Bukti

- `go build ./...` · `go test ./...` **38 paket hijau** + `./resource/` flaky
  pra-eksisting (lihat di atas; 7/8 run hijau, kegagalan pada HEAD bersih juga) ·
  `gofmt -l` bersih.
- `golangci-lint`: 4 issue `unused`, semuanya di file yang **tidak disentuh**
  (`cmd/formspec/seed_asset.go`, `internal/ui/meta.go`,
  `resource/ctxresolver.go`, `resource/storage_reload_e2e_test.go`) — pre-existing.
- `npx vitest run` **454 lulus** (29 file) · `npx tsc -b` bersih ·
  `npm run build` hijau.
- `formspec validate --spec examples/kafe/spec --schema schemas` → **85 manifest,
  0 problem**; `formspec check -f spec` → **0 error, 0 warning**.
- **Uji negatif (dibuktikan pada salinan spec kafe, bukan pernyataan):**
  - hapus `multiple: true` dari `days_of_week` → engine: _"`multiple` is
    required on a json field with `options`"_ (1 problem);
  - `multiple: true` pada `integer` → _"`multiple: true` is not valid on a
    \"integer\" field"_ (1 problem);
  - `options` pada `enum` → _"`options` is not valid on a \"enum\" field —
    an enum declares its values in `enum_values`"_ (1 problem);
  - `widget: select` pada himpunan → check: _"`widget: select` picks one value,
    but the Entity declares the field a set (`multiple: true`)"_ (1 error);
  - `widget: select-multi-tag` pada nilai tunggal → check: _"needs a field that
    holds a set, but the Entity declares `multiple: false`"_ (1 error).
- **E2E browser `:8099`** (`manajer`/`kafe123`, App `kafe-pos`): form promo
  **tanpa** `widget:` di YAML menampilkan chip hari (7 opsi, `aria-multiselectable`
  benar) dan single-select **Kanal** ber-caption `POS`/`QRIS`/`Online`. Klik
  **Jumat lalu Senin** → chip tampil **"Senin, Jumat"** (urutan deklarasi).
  Tersimpan di DB: `days_of_week = [5, 1]` **int** (urutan isian — tampilan
  tidak menulis ulang data) dan `channel = 'qris'` (skalar string, bukan caption).
  Halaman detail menampilkan chip `Senin`/`Jumat` + caption `QRIS`.

## Temuan sampingan: `go test ./resource/` flaky (pra-eksisting, bukan regresi)

Verifikasi awal `go test ./...` terlihat hijau, lalu run berikutnya gagal di
`TestKafe_OnPaidCreatesBalancedJournal` — jadi penyebabnya diselidiki, bukan
diabaikan. Diukur dengan `git worktree` pada HEAD bersih (`dd3adc6`):
`go test -count=1 ./resource/` gagal **2 dari 8** run dengan test yang sama;
working tree perubahan ini gagal **1 dari 8** — laju setara, jadi **bukan akibat
perubahan ini**.

Akarnya terbaca di kode, bukan diduga: `waitForJournal`
(`resource/o2c_e2e_test.go:304`) hanya menunggu `countJournalEntries() > 0` —
barisnya **ada** — sedangkan test lalu meng-assert `status == "posted"`. Baris
lahir saat `journalize.star` meng-insert; `posted` di-set handler setelahnya, dan
outbox worker polling pada interval. Di antara dua momen itu assertion melihat
`"draft"`. Pesan persisnya: `journal status = "draft", want posted (the handler
posts it itself)`.

Dicatat sebagai **5.10.24 ⏸️** (fix: tunggu status, bukan keberadaan baris).
Atribusi lama yang menyebut item kafe 10.7 (`gl/config/gl.yaml`) **dikoreksi** —
`git diff HEAD -- examples/kafe/spec/modules/gl/` kosong.

## Sisa

Tidak ada sisa tersembunyi: lima residual yang sengaja tidak ditutup tercatat
sebagai item `[⏸️]` bernomor di `docs_internal/plan/todo.md` — **5.10.20 ⏸️**
(penegakan server), **5.10.21 ⏸️** (paritas lintas-shell), **5.10.22 ⏸️**
(`generate` belum emit union `options`), **5.10.23 ⏸️** (caption `enum`, godoc
`validateFieldOptionsShape` menunjuk item ini), **5.10.24 ⏸️** (test flaky
pra-eksisting di atas) — plus **5.10.17 ⏸️** (Wizard) yang tetap terbuka.
Kafe `examples/kafe/gaps_found/TODO.md` 10.33 diperbarui menunjuk semuanya.
