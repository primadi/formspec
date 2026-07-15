# 2026-07-08 — Hapus `doc_name` dari Reserved Fields

**File:** `pkg/spec/entity.go`, `docs/spec/02-core-basic.md`

## Ringkasan

`doc_name` dihapus dari reserved field list. Tidak perlu kolom khusus untuk display title — framework bisa auto-compute dari field manapun via naming rule engine (deferred).

## Perubahan

- `ReservedFieldNames`: `doc_name` dihapus — tersisa `owner`, `created_at`, `modified`, `doc_status`, `amends`, `amended_by`, `version`
- §4.1a: baris `doc_name` dihapus dari tabel
- §10: reserved field names list di-update
- Conformance: reserved field names list di-update
