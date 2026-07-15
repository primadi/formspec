# 2026-07-08 — Update Frontend Spec: Single Characteristic, Remove Composite

**File utama:** `docs/spec/05-frontend.md`

## Perubahan

### 1. `characteristics: [transaction]` → `characteristic: transaction`

Semua referensi ke `characteristics:` (array, plural) diubah ke `characteristic:` (single value, singular) di:
- §1.7 UI Patterns — opening paragraph (2x)
- §1.7 YAML example
- Configuration Page pattern — deskripsi dan contoh YAML

### 2. `composite` references dihapus

- §1.7 tabel: "1-step (composite)" → "1-step (create-submit)"
- §1.7 YAML example: hapus `composite: true` dan `composite_calls: [create, submit]`, action name diubah ke `create-submit`
- Wizard §11: `composite_action` → `action` (field name + 5 penjelasan)

### 3. Lifecycle-free doc_status fix

Blockquote di §1.7: `doc_status remains "draft" forever technically` → `doc_status is null — lifecycle-free, no lifecycle concept`
