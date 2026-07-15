# Relation Picker Widget — Default Lookup untuk Field Bertipe Relation

**Date:** 2026-07-12
**Plan:** docs/plan/relation-picker-plan.md

## Perubahan

Field relation (`patient_id`, `polyclinic_id`, `doctor_id`, dsb.) sebelumnya
dirender sebagai `TextInput` biasa karena `FormFieldWidget` di
`web/src/kinds/form/FormRenderer.tsx` tidak memiliki handler untuk
`entityField.type === "relation"`.

### Yang diubah

| File | Aksi |
|---|---|
| `web/src/widgets/RelationPicker.tsx` | **Baru** — searchable combobox untuk belongs_to relation |
| `web/src/widgets/index.ts` | **Edit** — tambah export RelationPicker |
| `web/src/kinds/form/FormRenderer.tsx` | **Edit** — tambah `case "relation"` di FormFieldWidget + Zod builder |

### Cara kerja RelationPicker

1. Menerima `entityField` (dari entity spec) + `currentModule`
2. Me-resolve entity target dari `entityField.relation.resource` (support
   format `"patient"` same-module dan `"clinic/visit"` cross-module)
3. Input dengan debounce 300ms → `GET /{module}/{plural}?search=...`
4. Dropdown hasil pencarian dengan label dari `label_field` entity target
5. `onChange(item.id)` → form value terisi dengan UUID record terpilih
6. Readonly mode: tampilkan label dari record (fetch by ID)
7. Clear button untuk menghapus seleksi

### Dampak

- Semua form yang memiliki field `type: relation` (derived maupun authored)
  otomatis mendapat widget lookup tanpa perlu deklarasi eksplisit di YAML.
- Tidak ada perubahan pada YAML manifest — prinsip "One Definition, Many Protocols".
