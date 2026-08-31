# Plan: Access Management UI (User, Role, Hak Akses)

**Tanggal**: 2026-08-21
**Status**: ✅ Implemented (changelog 020 + 021 app surface)
**Scope**: C (authored forms + grants editor widget + halaman khusus) + password field

## Tujuan

UI manajemen user, role, dan hak akses yang lebih ramah, bisa diakses dari
menu admin (`/_admin`). Saat ini semua derived CRUD mentah (field JSON untuk
grants, tidak ada password field).

## Latar Belakang (temuan riset)

- Auth entities di `internal/auth/module/`: `user`, `role`, `role-assignment`,
  `app-membership`, `api-key`, `session`, `workspace` (module `formspec.core`).
- Menu admin di-generate mekanis dari entities (`useResolvedMenu` →
  `getEntitiesByModule`), bukan curated. Item User/Role/Role Assignment sudah
  muncul di sidebar admin.
- `resolveForm()` (engine/derive.ts) memakai authored form `{entity}-form`
  (generic) atau `{entity}-{mode}-form` untuk override derived form.
- Widget form tersedia: input, textarea, number, select/enum, switch/boolean,
  uuid, relation-picker, date, json, child-grid.
- Native action: `App.RegisterNative(ref, handler)`; auth service via
  `api.SetAuthService` (punya `CreateUser` yang hash password).
- Page kind mendukung `tabs` (tiap tab ref Form/Table).

## Rencana Implementasi

### Phase 1 — Backend: password hashing untuk user create/update

- Register native action `formspec.core.user.create` / `user.update` yang
  menerima `password` plaintext, hash via `auth.HashPassword`, simpan ke
  `password_hash`, lalu create/update entity user.
- Form user memakai custom action ini (bukan entity CRUD langsung).
- File: `resource/formspec.go` (register native), mungkin `internal/auth/`
  (expose helper).

### Phase 2 — Authored Form manifests (YAML)

- `internal/auth/module/master/user/forms/user-form.yaml` — sections:
  Identity (username, display_name, email), Account (active), Roles &
  Permissions (roles, permissions). Field `password` (create) via custom action.
- `internal/auth/module/master/role/forms/role-form.yaml` — name, app, module,
  description, grants (widget `grants-editor`).
- `internal/auth/module/transaction/role-assignment/forms/role-assignment-form.yaml`
  — user_id (relation-picker ke user), role_id (relation-picker ke role), app,
  active.
- Naming: `{entity}-form` agar dipakai `resolveForm` untuk semua mode.

### Phase 3 — Grants editor widget (frontend)

- Tambah widget `grants-editor` di `FormFieldWidget` router
  (`kinds/form/FormRenderer.tsx`).
- Checkbox tree: pages → tabs → actions (dari bundle pages + footprint).
- Komponen baru `widgets/GrantsEditor.tsx`.

### Phase 4 — Halaman Access Management + menu

- Author `kind: Page` `access-management` dengan tabs (Users/Roles/Assignments)
  di module `formspec.core`.
- Tambahkan ke menu admin. Karena menu admin mekanis dari entities, perlu
  modifikasi `useResolvedMenu` (frontend) atau `internal/ui/meta.go` (backend)
  untuk menyertakan authored page khusus ini.

## File Terkait

- `internal/auth/module/**/forms/*.yaml` — authored forms
- `resource/formspec.go` — register native user create/update
- `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx` — widget router
- `renderers/react-shadcn/src/widgets/GrantsEditor.tsx` — grants editor (baru)
- `renderers/react-shadcn/src/hooks/useResolvedMenu.ts` — menu admin
- `internal/ui/meta.go` — (opsional) menu admin backend

## Verifikasi

- `go build ./...` + `go test ./resource/... ./cmd/formspec/...`
- `cd renderers/react-shadcn && npx tsc --noEmit` + `vitest`
- E2E browser (dev-auth): buka `/_admin` → menu Access Management → kelola
  user (dengan password), role (grants editor), role-assignment.

## Keputusan

- Scope C: authored forms + grants editor + halaman khusus (dikonfirmasi user).
- Password field: ya, di-hash server-side (dikonfirmasi user).
- Menu admin: modifikasi `useResolvedMenu` untuk menambahkan authored page
  khusus (paling kecil dampaknya).
