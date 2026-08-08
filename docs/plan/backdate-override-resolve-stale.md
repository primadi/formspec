# Plan: Special Path untuk Transaksi Stale (Backdate Override + resolve-stale)

**Tanggal:** 2026-08-05
**Status:** ✅ Selesai
**Referensi spec:** `docs/spec/backend/01-core-basic.md` §14a (transaction_date policy), `docs_old/spec/11-reference.md` (transaction_date), `docs/spec/platform/06-datastore.md` §2 (closed primitives / scheduler pattern)

## Latar Belakang

Diskusi desain menyimpulkan model dua-tingkat untuk transaksi yang melewati
`backdate_policy`:

| Skenario | Tuas | Path |
|---|---|---|
| Stale massal (force majeure, bulk entry) | Manajemen perlebar `backdate_policy` | Normal |
| Stale individual (satu transaksi menggantung) | Kondisi khusus + aksi khusus | Khusus (disengaja, diaudit) |

Temuan kode: `validateTransactionDatePolicy` di `renderers/jsonbpersist/`
dijalankan di **setiap** Insert/Update yang membawa `transaction_date`, dan
`override_permission` **dideklarasikan di spec tapi tidak pernah di-wire**
(hanya ada di komentar). Akibatnya, mengedit record stale (form update yang
membawa `transaction_date`) terpaksa menunggu policy diperlebar — tidak ada
path khusus.

## Tujuan

1. **Wire `override_permission`** — staf berwenang bisa menyentuh record stale
   (mis. edit form yang membawa `transaction_date`) tanpa memperlebar policy.
2. **Derived flag `is_stale`** — computed field yang menandai visit menggantung
   (badge + filter + report), mengikuti `backdate_policy.max_days_back`.
3. **Aksi khusus `resolve-stale`** — path penyelesaian transaksi menggantung
   yang disengaja: permission-gated, audited, emit event berbeda.

## File yang Diubah

| File | Perubahan | Effort |
|---|---|---|
| `renderers/jsonbpersist/transaction_date.go` | `validateTransactionDatePolicy` terima `permissions`; cek `override_permission`; helper `hasPermission` | small |
| `renderers/jsonbpersist/crud.go` | `InsertParams`/`UpdateParams` + field `Permissions`; teruskan ke validasi; `evaluateComputed` inject `backdate_limit_days` | small |
| `internal/api/handler.go` | `HandleCreate`/`HandleUpdate` teruskan `identity.Permissions` | small |
| `internal/starlark/evaluator.go` | builtin `today()` + `days_ago(n)` untuk computed field | small |
| `examples/Clinic-UI-Showcase/.../visit/entity.yaml` | `backdate_policy` (override), computed `is_stale`, aksi `resolve-stale` + transition + event `completed-overdue` | small |
| `examples/Clinic-UI-Showcase/.../visit/scripts/resolve-stale.star` | script aksi khusus | small |

## Dependensi

- Wiring `override_permission` (framework) mendahului aksi `resolve-stale`
  (contoh) — aksi memakai permission yang sama.
- Builtin `today()`/`days_ago()` mendahului computed `is_stale`.

## Keputusan Teknis

- **Override check di renderer** via `[]string` permissions + matcher wildcard
  lokal (bukan import `internal/auth`) — menjaga renderer tetap decoupled.
- **`resolve-stale` = transisi paralel** `in_consultation → completed` dengan
  `via: resolve-stale` (state machine butuh transition eksplisit), guard
  `not empty(diagnosis)` sama seperti `complete`.
- **Event `completed-overdue`** (async) — hilir (payment, revenue) tahu ini
  pengecualian, bukan complete normal.
- **`is_stale` formula** memakai `days_ago(backdate_limit_days)` — limit
  di-inject dari policy, jadi mengikuti perubahan config (3 ↔ 4 hari).

## Level of Effort

Total: **medium** (6 file, semua small, satu alur dependensi).