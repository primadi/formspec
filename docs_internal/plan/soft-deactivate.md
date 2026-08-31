# Plan: Soft-deactivation pattern (4.10.2)

**Status**: Draft
**Date**: 2026-08-17
**Todo refs**: 4.10.2
**Spec refs**: `02-core-extended.md` §19, `05-field-types.md`

## Scope

`soft_deactivate: {enabled: true}` pada Entity master menambah:

1. Field `is_active` (boolean, default true) — di-inject bila tidak dideklarasikan.
2. Action `deactivate` (set is_active=false) dan `reactivate` (set is_active=true).

## Files

- `pkg/spec/entity.go` — inject `is_active` field di `ValidateEntitySpec`.
- `renderers/jsonb-persist/crud.go` — `Deactivate`/`Reactivate`/`setActive`.
- `internal/api/descriptor.go` — `deactivate`/`reactivate` di StandardRESTActions.
- `internal/api/generator.go` — gate route pada soft_deactivate; UI surface include.
- `internal/api/handler.go` — `HandleDeactivate`/`HandleReactivate`/`handleSetActive`.
- `internal/api/router.go` — dispatch handler.
- `internal/entity/registry.go` — register permission deactivate/reactivate.
- Test: `resource/soft_deactivate_e2e_test.go`.

## Verification

- `rtk go test ./...`
- `make build`

## Notes

- Dropdown filter `is_active: true` untuk transaksi baru = concern frontend (Fase 5).
- `4.10.1` (soft_delete) sudah terimplementasi sebelumnya — diverifikasi, ditandai ✅.
