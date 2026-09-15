# Plan — S10: kosakata `widget` menjadi himpunan tertutup

**Status**: ✅ selesai (2026-09-15) — changelog `docs_internal/changelog/2026-09-15-002-widget-vocabulary-enum.md`
**TODO item**: `examples/kafe/gaps_found/TODO.md` **1.4** (prioritas §F #4)
**Menutup akar**: gap **#1** (kelas bug "salah ketik == fitur belum ada") · S10 di
`13-kelengkapan-spec-untuk-kafe.md` §A
**Sumber spec**: `docs/spec/frontend/07-component-kinds.md` §1, `docs_internal/plan/widget-strategy.md`

---

## 1. Masalah

`FormField.widget` dan `TableColumn.widget` bertipe **string bebas** di JSON
Schema (`"widget": { "type": "string" }`). Akibatnya:

- `widget: relaion-picker` (salah ketik) **lolos validasi** dan diam-diam jatuh
  ke `default: <TextInput>` di `FormFieldWidget` — tidak bisa dibedakan dari
  "widget belum ada".
- Tidak ada katalog resmi yang bisa dibaca editor → tidak ada autocomplete.
- **Dokumen dan implementasi sudah berbeda**: `07-component-kinds.md` §1
  menyebut `textinput`, `numberinput`, `dateinput`, `toggle`, `json-editor`,
  sedangkan renderer memakai `input`, `number`, `datepicker`, `switch`, `json`.
  Penulis spec yang mengikuti dokumen menulis nama yang tidak pernah dikenali.
- `ai_skills/form-layout/SKILL.md` menjanjikan `money` → `MoneyInput`, padahal
  widget itu tidak ada (`money` hari ini jatuh ke `input`).

## 2. Keputusan

**Dua himpunan tertutup, satu per permukaan** — bukan satu daftar gabungan,
supaya setiap nilai enum benar-benar _terimplementasi di permukaan itu_:

| Tipe              | Dipakai oleh                                          | Anggota                                                   |
| ----------------- | ----------------------------------------------------- | --------------------------------------------------------- |
| `FormWidget`      | `FormField.widget` (Form, Wizard step, ApprovalInbox) | 20 nama, semuanya ada `case` di `FormFieldWidget`         |
| `TableCellWidget` | `TableColumn.widget` (Table, Listing)                 | `badge`, `boolean` — keduanya ditangani `renderCellValue` |

Enum gabungan ditolak: dengan satu daftar, `widget: relation-picker` pada kolom
tabel lolos validasi lalu dirender sebagai teks mentah — persis kelas bug yang
sedang ditutup.

**Nama kanonik = nama yang benar-benar di-`switch` renderer.** Nama tipe field
(`relation`, `date`, `child`, `boolean`, …) adalah _alias_ yang masih diterima
router demi spec tree yang sudah ter-deploy, tetapi **bukan** bagian katalog
resmi: schema hanya memuat nama kanonik, dan validator memberi petunjuk arah
perbaikan bila yang ditulis adalah nama tipe ("`relation` adalah tipe field;
widget-nya `relation-picker`").

**Sumber kebenaran** = `pkg/spec/widget.go` (tipe bernama + blok `const`).
`internal/genjsonschema` sudah mengumpulkan nilai `const` dari tipe string
bernama (`enrichEnumValues`) → enum masuk JSON Schema → `formspec validate`
menolak salah ketik + editor dapat autocomplete. Tidak ada pipeline baru.

**Anti-drift** (inti S10): `internal/genjsonschema`/`pkg/spec` tidak bisa
memeriksa TS, jadi paritas ditegakkan **test di sisi renderer**
(`src/widgets/catalog.test.ts`, jsdom):

1. enum dari `schemas/formspec.schema.json` **≡** katalog runtime TS;
2. setiap nama di enum **benar-benar dirender** oleh `FormFieldWidget` (bukan
   error "widget tidak dikenal");
3. nama di luar katalog menghasilkan error yang **terlihat** (bukan `TextInput`
   senyap) — menutup `default:` yang lama.

## 3. Perubahan per file

| File                                                                                  | Perubahan                                                                                                                                                                | Effort |
| ------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------ |
| `pkg/spec/widget.go` **(baru)**                                                       | `FormWidget`/`TableCellWidget` + `const` (kanonik), `IsFormWidget`/`IsTableCellWidget`, `AllFormWidgets`/`AllTableCellWidgets`, peta alias tipe→widget untuk pesan error | small  |
| `pkg/spec/frontend.go`                                                                | `FormField.Widget FormWidget`, `TableColumn.Widget TableCellWidget`                                                                                                      | small  |
| `pkg/spec/frontend.go` (`ValidateFormSpec`)                                           | tolak `widget` tak dikenal + petunjuk alias                                                                                                                              | small  |
| `pkg/spec/` (validator baru)                                                          | `ValidateTableColumns(cols, where)` dipakai Table + Listing                                                                                                              | small  |
| `internal/manifest/loader.go`                                                         | dispatch `Table` & `Listing` → validator kolom                                                                                                                           | small  |
| `schemas/formspec.schema.json` + `schemas/kinds/{Form,Table,Listing,Wizard,...}.json` | regenerasi (`make generate-schema`)                                                                                                                                      | small  |
| `renderers/react-shadcn/src/widgets/catalog.ts` **(baru)**                            | katalog runtime + `isFormWidget`/`isTableCellWidget`                                                                                                                     | small  |
| `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx`                              | `default:` → error terlihat bila widget di luar katalog (ganti `TextInput` senyap)                                                                                       | small  |
| `renderers/react-shadcn/src/lib/renderCell.tsx`                                       | komentar katalog + `default` tetap (nilai tak dikenal tidak dirender sebagai teks mentah tanpa jejak)                                                                    | small  |
| `renderers/react-shadcn/src/widgets/catalog.test.ts` **(baru)**                       | paritas schema↔katalog↔renderer (jsdom)                                                                                                                                  | medium |
| `pkg/spec/widget_test.go` **(baru)**                                                  | pin nilai enum + `IsFormWidget`                                                                                                                                          | small  |
| `docs/spec/frontend/07-component-kinds.md` §1                                         | daftar widget = nama kanonik yang sebenarnya                                                                                                                             | small  |
| `ai_skills/form-layout/SKILL.md`                                                      | buang klaim `MoneyInput`; tabel widget = nama kanonik                                                                                                                    | small  |
| `examples/kafe/spec/**`                                                               | (tidak ada perubahan bentuk; hanya bila ada nama non-kanonik)                                                                                                            | —      |

**Di luar scope (tetap gap, dicatat sebagai TODO baru):** widget `money` dan
`time` belum ada di renderer (separuh lain gap #1). 1.4 menutup **akar**-nya —
sesudah ini `widget: money-input` gagal validasi dengan pesan jelas, bukan diam
diam diabaikan. Implementasi `MoneyInput`/`TimeInput` masuk item tersendiri.

## 4. Verifikasi

```bash
make generate-schema
formspec validate --spec examples/kafe/spec            # 0 problem
# spec uji dengan `widget: relaion-picker` → 1 problem (enum) — bukti accept
go test ./pkg/spec/ ./internal/manifest/ ./internal/genjsonschema/
cd renderers/react-shadcn && npx vitest run && npx tsc --noEmit
grep -c '"enum"' schemas/formspec.schema.json
```

_Accept:_ `widget: relaion-picker` **gagal validasi**; editor autocomplete
(enum ada di schema); paritas schema↔renderer dijaga test.
