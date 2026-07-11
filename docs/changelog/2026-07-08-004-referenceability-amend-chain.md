# 2026-07-08 — Referenceability Rule + Amend Chain

**Diskusi:** Hanya dokumen submitted/null yang bisa direferensi. Amend diperluas.
**File utama:** `docs/spec/02-core-basic.md`

## Perubahan

### 1. Referenceability Rule (NEW)

Hanya dokumen dengan `doc_status = null` (lifecycle-free) atau `submitted` yang bisa menjadi target field `relation`. `draft` dan `cancelled` ditolak di runtime. Error code: `FORMA.REF.TARGET_NOT_SUBMITTED`.

### 2. Amend Guard Diperluas

```diff
- amend: "doc_status == cancelled"
+ amend: "doc_status == submitted OR doc_status == cancelled"
+   → if submitted: atomic cancel original + set amended_by
+   → creates new linked Document (with amends link), start as draft
```

Satu klik, bukan dua (cancel dulu baru amend).

### 3. Amend Version Chain

- `amends` (reserved field pada dokumen baru) — UUID original yang di-cancel
- `amended_by` (reserved field pada original) — UUID versi baru
- `version` column increment per amend cycle

### 4. `amend-submit` sebagai 8th Reserved Action

Auto-derived jika `amend` + `submit` keduanya enabled. Single-click amend + immediate re-approve.

### 5. Reserved Fields Table

Tambah `amends` dan `amended_by` ke §4.1a.

## File yang Diubah

| File | Perubahan |
|---|---|
| `docs/spec/02-core-basic.md` | Amend guard, referenceability rule, amend-submit, reserved fields, conformance |
