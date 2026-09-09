# 2026-09-09-004 — Confirm dialog global (create/update/delete) + per-form override

## Apa

Fitur confirm dialog untuk operasi mutasi, dengan resolusi berlapis
(plan confirm-dialogs.md):

```
Form override  >  App default  >  off (backward compatible)
```

Semantik pointer: nil = inherit, "" = explicitly off (opt-out), non-empty =
pesan dialog.

## Implementasi

### Backend

- `pkg/spec/resources.go` — `AppConfirm{Create,Update,Delete *string}` +
  `AppSpec.Confirm` (App-wide default).
- `pkg/spec/frontend.go` — `FormConfirm{...}` + `FormSpec.Confirm`
  (per-form override; create → mode=create, update → mode=edit).
- `internal/ui/meta.go` — `AppContext.Confirm`, `AppSummary.Confirm`
  (`ConfirmConfig`), `resolveConfirm`, wiring `BuildBundle`.
- `internal/api/meta.go` — pass `resolved.Spec.Confirm` ke AppContext.
- `make generate-schema` dijalankan (schemas/ ter-update).

### Frontend

- `types/manifest.ts` — `AppSummary.confirm` + `FormSpec.confirm` /
  `FormConfirm`.
- `FormRenderer.tsx` — intercept submit: resolusi verb (edit→update,
  create→create) form > app > off; jika ada pesan → `ConfirmDialog`
  (data form disimpan di `pendingConfirm`, submit lanjut setelah konfirmasi).
  Validasi zod sudah lulus sebelum dialog muncul.
- `TableRenderer.tsx` — fallback confirm delete: table action >
  entity action `ui.confirm` > App `confirm.delete`.

## File terdampak

- `pkg/spec/resources.go`, `pkg/spec/frontend.go`
- `internal/ui/meta.go`, `internal/api/meta.go`
- `schemas/` (regenerated)
- `renderers/react-shadcn/src/types/manifest.ts`
- `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx`
- `renderers/react-shadcn/src/kinds/table/TableRenderer.tsx`
- `internal/auth/module/master/user/forms/{create,edit}.yaml` (enable)
