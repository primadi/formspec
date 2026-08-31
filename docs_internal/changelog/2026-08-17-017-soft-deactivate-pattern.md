# Soft-deactivation pattern (todo 4.10.2)

**Date**: 2026-08-17
**Plan**: `docs/plan/soft-deactivate.md`

Mengimplementasikan pola soft-deactivation (1.4.10 / 4.10.2,
`02-core-extended.md` §19): `soft_deactivate: {enabled: true}` pada Entity
master kini menambah field `is_active` (boolean, default true) + action
`deactivate`/`reactivate`.

- `pkg/spec/entity.go`: `ValidateEntitySpec` inject field `is_active` bila
  belum dideklarasikan.
- `renderers/jsonb-persist/crud.go`: `Deactivate`/`Reactivate`/`setActive`
  (update `is_active` di data JSONB, CAS via version).
- `internal/api/descriptor.go`: `deactivate`/`reactivate` di
  `StandardRESTActions` (POST `/{id}/deactivate`, `/{id}/reactivate`).
- `internal/api/generator.go`: gate route hanya bila soft_deactivate enabled;
  UI surface include deactivate/reactivate.
- `internal/api/handler.go`: `HandleDeactivate`/`HandleReactivate`/
  `handleSetActive` (before/after hooks + NotifyMutation).
- `internal/api/router.go`: dispatch handler.
- `internal/entity/registry.go`: register permission
  `{module}.{plural}.deactivate`/`.reactivate`.
- Test: `resource/soft_deactivate_e2e_test.go` (create → is_active true →
  deactivate → false → reactivate → true).

Catatan: dropdown filter `is_active: true` untuk transaksi baru adalah concern
frontend (Fase 5), bukan backend.
