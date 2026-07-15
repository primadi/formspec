# Fix: Field Label Fallback Menggunakan Description

## Perubahan

`fieldLabel()` di `web/src/engine/derive.ts` sebelumnya memprioritaskan
`description` sebagai label field ketika `title` tidak diset. Ini menyebabkan
kolom tabel dan caption form menampilkan teks deskripsi yang panjang (contoh:
*"Opsional — pembeli walk-in belum tentu pasien terdaftar"*) daripada
humanized `name` (*"Patient Id"*).

## Perbaikan

1. **`pkg/spec/entity.go`** — tambah field `Title` ke struct `Field`
2. **`web/src/types/manifest.ts`** — tambah properti `title?: string` ke interface `Field`
3. **`web/src/engine/derive.ts`** — ubah `fieldLabel()`: `title` → humanized `name`;
   `description` tetap dipakai sebagai `help` di form (sudah benar)

## Prioritas Label (hasil)

1. `field.title` (eksplisit dari YAML)
2. Humanized `field.name` (contoh: `patient_id` → `Patient Id`)
3. `field.description` → hanya sebagai tooltip/help text

## File Terkena Dampak

- `pkg/spec/entity.go` (+1 line)
- `web/src/types/manifest.ts` (+1 line)
- `web/src/engine/derive.ts` (modified logic)

## Referensi

- `docs/spec/05-frontend.md` — derived field labels
- Plan: `docs/plan/fix-field-label-fallback.md`
