# 2026-08-05-001-backdate-override-dan-resolve-stale.md

**Perubahan:** Wire `override_permission` di `BackdatePolicy`/`ForwardDatePolicy`,
tambah computed flag `is_stale` untuk transaksi menggantung, aksi khusus
`resolve-stale` pada entity `visit`, dan penanganan stale di UI (badge + tombol).

## Kenapa

Backdate policy (max 3 hari) tidak bisa dilewati karena `override_permission`
hanya ada di spec tapi tidak pernah di-wire di `renderers/jsonbpersist/`.
Akibatnya, menyelesaikan transaksi stale (force majeure, listrik mati, bulk
entry) terpaksa menunggu policy diperlebar — padahal policy diperlebar adalah
tuas governance massal, bukan path individual.

Model dua-tingkat yang disepakati:
- **Massal**: manajemen perlebar `backdate_policy` (governance, diaudit)
- **Individual**: path khusus via `override_permission` (disengaja, diaudit,
  emit event berbeda)

## Perubahan Framework

| File | Perubahan |
|---|---|
| `renderers/jsonbpersist/transaction_date.go` | `validateTransactionDatePolicy` terima `[]string` permissions; cek `override_permission`; helper `hasPermission` (wildcard local, decoupled dari `internal/auth`) |
| `renderers/jsonbpersist/crud.go` | `InsertParams`/`UpdateParams` + field `Permissions`; teruskan ke validasi; `evaluateComputed` inject `backdate_limit_days` ke env Starlark |
| `internal/api/handler.go` | `permissionsFromContext()`; `HandleCreate`/`HandleUpdate` teruskan `identity.Permissions`; `HandleCustomAction` inject `auth.WithPermissions` ke ctx |
| `internal/auth/auth.go` | `WithPermissions`/`PermissionsFromContext` — context propagation utk custom action path |
| `resource/forma.go` | `SaveHandler` (resource.save script) teruskan `Permissions: auth.PermissionsFromContext(ctx)` — fix: script action gagal BACKDATE_EXCEEDED karena permission tidak sampai ke store |
| `internal/starlark/evaluator.go` | builtin `today()` + `days_ago(n)` untuk formula computed |

## Perubahan Contoh (Clinic-UI-Showcase)

| File | Perubahan |
|---|---|
| `visit/entity.yaml` | `backdate_policy` (max 3 hari, override `visits.resolve-stale`), computed `is_stale` + `stale_label`, aksi `resolve-stale` + transition + event `completed-overdue` |
| `visit/scripts/resolve-stale.star` | Script aksi khusus (mirip `complete` tapi pakai path khusus) |
| `visit/kanbans/board.yaml` | `badge: stale_label` (badge "⚠ Stale" di kartu), `row_actions: resolve-stale` |
| `visit/tables/list.yaml` | Kolom `is_stale` + filter select "Stale", row_action `resolve-stale` |
| `renderers/web/.../KanbanRenderer.tsx` | Mapping ikon utk `check`/`play`/`x`/`alert-triangle` di row_actions |

## Referensi

- `docs/plan/backdate-override-resolve-stale.md` — rencana teknis
- `docs/spec/backend/01-core-basic.md` §14a — transaction_date policy
- `docs_old/changelog/2026-07-09-001-transaction-date-policy.md` — implementasi awal backdate policy
