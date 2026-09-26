# Wizard step memakai kosakata widget yang sama dengan Form (todo 5.10.17)

## Apa yang diubah

`WizardFormStep` merender **setiap** input dengan tangan (`import { Input }`) dan
tidak pernah memanggil `FormFieldWidget`, jadi **seluruh katalog widget
diabaikan di dalam wizard**. Bukti pada manifest nyata:

```yaml
# examples/kafe/spec/modules/cafe-order/wizards/close-shift-wizard.yaml
- field: counted_cash # entity: cafe-order/shift, type: money
- field: supervisor_id
  widget: relation-picker
```

`counted_cash` adalah `type: money`. Di luar wizard ia merender `MoneyInput`
(currency + amount, numpad di layar sentuh); **di dalam wizard ia merender
`<Input type="number">` polos** — mata uangnya tidak terlihat dan nilai yang salah
tidak bisa ditangkap. Tidak ada yang gagal: nama widget-nya sah, `validate` hijau,
dan widget-nya sekadar tidak pernah dibaca.

**Perbaikannya bukan menyalin switch-nya** — itu berarti dua implementasi katalog
yang harus disinkronkan (kelas bug yang sama dengan 5.11.7/5.14.6). Sebagai
gantinya, cabang default dan cabang date/numeric mendelegasikan ke
`FormFieldWidget` yang sudah ada, dengan satu set `WIZARD_NATIVE_WIDGETS` untuk
widget yang memang dirender wizard sendiri.

**Tiga cabang sengaja tetap ditulis tangan**, karena membawa perilaku yang tidak
dimiliki Form:

| Cabang   | Kenapa bukan router generik                                                     |
| -------- | ------------------------------------------------------------------------------- |
| relation | opsi di-fetch dengan penyaringan `depends_on` (mis. dokter difilter poliklinik) |
| enum     | `<select>` polos di dalam step                                                  |
| boolean  | checkbox dengan label terikat `id`-nya                                          |

Konsekuensinya: widget yang ditambahkan ke katalog **otomatis bekerja di wizard**
tanpa edit kedua di sini.

Sekalian: `WizardFormStep` kini memakai helper `wrap(field, label, help, control)`
seperti `SearchSelect` — sebelumnya tiap cabang menulis `<label>` + `{help}`
sendiri, dan itulah bagaimana help tadinya hanya ada di satu dari enam cabang
(todo 5.23.1). Baca `help` dan `{help}` kini satu situs (`wrap`) plus satu untuk
boolean.

## Kenapa

Item 5.10.17 sudah mencatat akarnya persis (`grep 'FormFieldWidget' wizard/*.tsx`
→ 0 hasil) dan menyerahkan satu keputusan: "apakah wizard memakai
`FormFieldWidget` langsung". Jawabannya **ya untuk cabang generik, tidak untuk
tiga cabang berperilaku khusus** — dan itulah yang membuat perbaikan ini tidak
menghapus `depends_on`.

## File terdampak

- `renderers/react-shadcn/src/kinds/wizard/WizardFormStep.tsx` — `FormFieldWidget`
  untuk cabang generik/date/numeric, `WIZARD_NATIVE_WIDGETS`, helper `wrap`
- `renderers/react-shadcn/src/kinds/wizard/wizard-widget-vocabulary.test.ts` — **baru** (4 test)

## Bukti

- `npx vitest run` → **503 lulus** / 36 file (baseline sesi ini 499 setelah
  5.23.3; +4 dari file ini). `npx tsc -b` bersih.
- **Dibuktikan gagal:** dua guard `WIZARD_NATIVE_WIDGETS.has(field.widget)`
  dinetralkan menjadi `if (true)` → test `honours 'widget:' outside the native
set` gagal (`Tests 1 failed | 3 passed`), hijau sesudah dikembalikan.
- Test juga mem-pin hal yang **tidak** boleh hilang: import `FormFieldWidget`,
  keberadaan cabang `depends_on`, `relation?.resource`, dan `enum_values`.

## Rujukan

Todo **5.10.17** (tertutup), sisa dari **5.10.15/5.10.19** · katalog widget
`renderers/react-shadcn/src/widgets/catalog.ts` (S10) ·
`docs_internal/changelog/2026-09-24-010` (widget `select-multi-tag` yang
menyingkap "siapa saja yang membaca `widget:`").
