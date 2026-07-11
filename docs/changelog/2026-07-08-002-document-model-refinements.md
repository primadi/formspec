# 2026-07-08 — Perbaikan Document Model: Karakteristik, Lifecycle-Free, create-submit

**Referensi:** Diskusi dengan user di sesi review spec
**File utama:** `docs/spec/02-core-basic.md`

## Perubahan

### 1. Characteristics → Mutually Exclusive (bukan "combinable hints")

Sebelumnya `characteristics: []` adalah array opsional yang "combinable." Sekarang `characteristic:` adalah single value, mutually exclusive:

- `master` — stable, MAY have lifecycle atau tidak
- `transaction` — append-heavy, REQUIRES `transaction_date`
- `reference` — read-only seed, owned by App Owner
- `summary` — system-managed projection, lifecycle bypassed

`forma apply` REJECT jika lebih dari satu nilai. Lifecycle behavior independen dari characteristic — diatur oleh `submit: disabled`.

### 2. Lifecycle-Free → `doc_status = null` (bukan "draft forever")

Jika `submit`, `cancel`, `amend` semuanya disabled → dokumen **lifecycle-free**:
- `doc_status = null` (nullable column)
- Semua lifecycle guards bypassed
- `update`/`delete` tanpa guard `doc_status == draft`
- Guard `delete` (referential integrity) tetap jalan

### 3. Transitive Gating

Lifecycle actions punya dependency chain: `submit ← cancel ← amend`. Framework auto-disable dependent actions + emit warning.

### 4. `create-submit` sebagai 7th Reserved Action

Auto-derived jika `create` + `submit` keduanya enabled. Composite action bawaan: create + submit dalam satu transaksi atomik. Developer boleh tambah `conditions`, tidak boleh lemahkan guard.

## File yang Diubah

| File | Perubahan |
|---|---|
| `docs/spec/02-core-basic.md` | Taxonomy: `characteristics: []` → `characteristic:` single. §4.1b: transitive gating + create-submit. §4.1d: "Forever-Draft" → "Lifecycle-Free", `doc_status = null`. §11.1: create-submit di reserved actions table. §14a: `characteristic: transaction`. §19: normative table → nullable `doc_status`. Conformance: 7 reserved actions + NULL lifecycle. |
| `docs/spec/error-glossary.yaml` | Tambah `FORMA.DOC.CREATE_SUBMIT_NOT_AVAILABLE`. Fix `characteristics: [transaction]` → `characteristic: transaction`. |
