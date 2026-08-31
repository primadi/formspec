# 2026-08-10-003-cleanup-backend-01-core-basic

**Lapis**: Spec (`docs/spec/backend/01-core-basic.md`)
**Referensi**: `docs/plan/ai-skills-and-spec-cleanup.md` §B.1

## Perubahan

Cleanup dokumen Core Basic spec — menambah 3 section baru dan memperbaiki artifact:

- **Tambah Daftar Isi** di awal dokumen — navigasi 10 section + deskripsi singkat
- **Tambah §1.5 Entity Spec Reference** — tabel atribut lengkap Entity dalam satu tempat (version, characteristic, plural, display_field, lifecycle, soft_deactivate, fields, state_machine, actions, expose, persist, auth, events, indexes)
- **Tambah §1.6 `doc_status` vs `state_machine`** — penjelasan dua lapis state yang berjalan paralel: doc_status (framework-managed) dan custom state_machine (developer-defined). Lengkap dengan contoh YAML yang mendemonstrasikan keduanya berjalan bersamaan.
- **Fix artifact**: karakter Mandarin `找不到` di §1.1 diganti dengan "tidak menemukan"

## File terdampak

- `docs/spec/backend/01-core-basic.md` — 4 perubahan di atas
