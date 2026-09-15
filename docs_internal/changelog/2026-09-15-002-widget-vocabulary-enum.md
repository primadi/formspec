# 2026-09-15-002 — S10: kosakata `widget` menjadi himpunan tertutup

Item `examples/kafe/gaps_found/TODO.md` 1.4 (prioritas §F #4) — menutup akar gap
**#1**. Plan: `docs_internal/plan/widget-vocabulary-enum.md`.

**Masalahnya.** `FormField.widget` dan `TableColumn.widget` bertipe string bebas
di JSON Schema (`{"type": "string"}`). Akibatnya `widget: relaion-picker` lolos
validasi dan **diam-diam dirender sebagai `<TextInput>`** — tidak bisa dibedakan
dari "widget belum ada". Tidak ada katalog resmi, jadi tidak ada autocomplete.
Lebih buruk: dokumen dan implementasi sudah berbeda — `07-component-kinds.md` §1
menyebut `textinput`, `numberinput`, `dateinput`, `toggle`, `json-editor`,
sementara renderer memakai `input`, `number`, `datepicker`, `switch`, `json`.
Nama dokumen itu **tidak pernah ada** di renderer.

**Dua himpunan tertutup, satu per permukaan.** Bukan satu daftar gabungan:
widget form pada kolom tabel diabaikan `renderCellValue` dan nilainya tercetak
mentah — daftar gabungan akan meloloskan persis kelas bug itu.

| Tipe              | Dipakai                                   | Anggota            |
| ----------------- | ----------------------------------------- | ------------------ |
| `FormWidget`      | `FormField.widget` (Form, langkah Wizard) | 20 nama            |
| `TableCellWidget` | `TableColumn.widget` (Table, Listing)     | `badge`, `boolean` |

**Yang diubah.**

- `pkg/spec/widget.go` (baru) — dua tipe bernama + blok `const` (sumber
  kebenaran), `IsFormWidget`/`IsTableCellWidget`, daftar tertutup untuk pesan
  error, `ValidateFormWidget`/`ValidateTableCellWidget`/`ValidateTableColumns`/
  `ValidateFormSections`, dan peta petunjuk alias tipe→widget.
- `pkg/spec/frontend.go` — `FormField.Widget FormWidget`,
  `TableColumn.Widget TableCellWidget`; `ValidateFormSpec` memvalidasi widget
  setiap section.
- `internal/manifest/loader.go` — dispatch `Table` dan `Listing` →
  `ValidateTableColumns`.
- `schemas/**` — regenerasi; enum muncul sebagai `$defs/FormWidget` /
  `$defs/TableCellWidget` dan kedua field `$ref` ke sana. Itulah yang membuat
  `formspec validate` menolak salah ketik **dan** editor memberi autocomplete.
  Tidak ada pipeline baru: `internal/genjsonschema.enrichEnumValues` sudah
  mengumpulkan nilai `const` tipe string bernama.
- `renderers/react-shadcn/src/widgets/catalog.ts` (baru) — katalog runtime
  (cermin) + `isFormWidget`/`isTableCellWidget` + peta alias legacy.
- `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx` — `widget:` eksplisit
  di luar katalog kini merender `UnknownWidget` (`role="alert"`, menyebut nama
  yang diizinkan) alih-alih `TextInput` senyap. Turunan tipe field yang **belum**
  punya widget (`money`, `time`) tetap `input` seperti sebelumnya — itu gap #1
  yang kini menjadi item 2.14, bukan sesuatu yang ditutupi. `FormFieldWidget`
  di-export untuk dipakai test paritas.
- `renderers/react-shadcn/src/widgets/catalog.test.tsx` (baru, jsdom) — **gerbang
  anti-drift**: enum schema ≡ katalog runtime; setiap nama katalog punya `case`
  di `FormFieldWidget` (scan label `case`); setiap nama tabel punya cabang di
  `renderCellValue`; `derive.formWidget()` tidak pernah mengembalikan nama di
  luar katalog. Gerbang ini diuji bisa gagal (menambah `money-input` ke katalog →
  3 test merah).
- `pkg/spec/widget_test.go` (baru) — pin anggota himpunan + bentuk pesan error
  (typo, alias tipe, widget form pada kolom tabel, lokasi field/kolom).
- Dokumen: `docs/spec/frontend/07-component-kinds.md` §1 ditulis ulang memakai
  nama yang sebenarnya + aturan yang mengikat; `ai_skills/form-layout/SKILL.md`
  berhenti menjanjikan `MoneyInput` (diganti `money → input (belum ada widget)`).

**Bukti accept** (`formspec validate --schema schemas`, spec uji 3 manifest):

| Yang ditulis                      | Hasil                                                                                                                                                     |
| --------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `widget: select` (kanonik)        | **0 problem**                                                                                                                                             |
| `widget: relaion-picker`          | **1 problem** — schema (`/spec/sections/0/fields/1/widget: validation failed`) + engine (`unknown widget "relaion-picker" (allowed: input, textarea, …)`) |
| `widget: relation`                | gagal + `"relation" is a field type; the widget is "relation-picker"`                                                                                     |
| `widget: select` pada kolom tabel | gagal + `"select" is a form widget, not a table cell widget`                                                                                              |
| `widget: money-input`             | gagal (akar #1 tertutup)                                                                                                                                  |

**Regresi:** kafe **0 problem**, `examples/cafe` **0 problem**,
`cmd/formspec-registry/app-spec` **0 problem**; `Clinic-UI-Showcase` /
`crc-management` / `reference-app` **jumlah problem identik sebelum-sesudah**
(10 / 1 / 32 — drift schema lama pada spec itu, diverifikasi dengan schema
pra-perubahan dari `git archive HEAD schemas`). `go test ./...` **35 paket ok**;
`vitest` **221 lulus** (207 + 14); `tsc --noEmit` bersih.

**Catatan (tetap terbuka, dicatat bukan ditutupi).** Widget `money` dan `time`
belum ada — `money-input` sekarang _gagal validasi_ dengan pesan jelas, dan
implementasinya menjadi TODO **2.14**. Enam properti lain yang masih string bebas
(`ReportParam.type`, `ReportColumn.format/.aggregate`, `EventDeliveryDecl.channel`,
`PrintOutput.format`, `WorkflowStep.mode`) tetap di **8.5**, di luar 1.4.
