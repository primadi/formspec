# 2026-07-08 — Relation Spec Enhancement: on_delete + Child with UUID

**Diskusi:** Clarify child vs relation boundary. Add `on_delete` and `use_uuid`.
**File utama:** `docs/spec/02-core-basic.md`

## Perubahan

### 1. Child vs Relation — Clarified Boundary

Lifecycle ownership adalah penentu: child ikut parent, relation independen. Ditambahkan tabel perbandingan dan decision test.

### 2. Child `use_uuid: true`

Child table bisa punya UUID column untuk dirujuk oleh dokumen lain. Lifecycle tetap ikut parent (submit/cancel otomatis). Default `false`.

```yaml
child:
  storage: table
  use_uuid: true    # NEW
  sequence_field: seq
```

### 3. Relation `on_delete`

Tambah `on_delete` ke relation spec:

| Nilai | Perilaku |
|---|---|
| `restrict` (default) | Absolute block — sama dengan guard delete §4.1b |
| `cascade` | Hapus referencing document (hanya draft/lifecycle-free) |
| `set_null` | Set FK ke null (hanya jika field tidak required) |

## File yang Diubah

| File | Perubahan |
|---|---|
| `docs/spec/02-core-basic.md` | §10.2 (tabel child vs relation), §10.3 (use_uuid), §10.5 (on_delete) |
