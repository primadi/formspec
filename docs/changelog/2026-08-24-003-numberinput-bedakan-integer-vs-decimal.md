# 2026-08-24-003 — NumberInput: Bedakan integer vs decimal via prop eksplisit

## Apa yang diubah

User bertanya: bagaimana `NumberInput` membedakan field number integer vs
decimal? Jawaban: sebelumnya **tidak bisa andal** — ia menebak dari `step`
(`step === 1 || !step ? parseInt : parseFloat`), padahal caller mengirim `step`
tidak konsisten: `FormRenderer` mengirim `step={undefined}` untuk decimal, yang
oleh logika itu dibaca sebagai integer → **decimal ter-truncate ke integer**.

### Frontend (`renderers/react-shadcn`)

- `widgets/NumberInput.tsx` — prop baru **`integer?: boolean`** sebagai sinyal
  eksplisit (bukan inferensi dari `step`):
  - `integer=true` → `parseInt`, blokir `.`/`,`/`e`/`E` di `onKeyDown`,
    `step` default `1`.
  - `integer=false` (default) → `parseFloat`, izinkan pecahan, `step` default
    `"any"` (atau `1/10^precision` bila `precision` diisi — prop yang tadinya
    tidak terpakai kini dipakai).
- `kinds/form/FormRenderer.tsx` — `case "number"` kini mengirim
  `integer={entityField.type === "integer"}` dan tidak lagi mengirim `step`
  (diturunkan otomatis). Alur: `derive.formWidget()` memetakan `integer`/
  `decimal` → widget `"number"`, jadi `entityField.type` di case itu andal
  membedakan keduanya.

## Verifikasi

- `tsc -b` bersih; `vitest` 103 pass.
- Integer: ketik `1.5` → `.` diblokir (hasil `15`), paste `1.5` → `1`.
- Decimal: `parseFloat`, pecahan dipertahankan.

## Referensi

- Lanjutan: `docs/changelog/2026-08-24-002-fix-integer-input-menerima-pecahan.md`
- Catatan: `WizardFormStep`/`SearchSelect` masih memakai `Input` mentah dengan
  `step` sebagai pembeda — kandidat refactor memakai `NumberInput` (deferred).
