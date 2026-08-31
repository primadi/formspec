# Access Management UI (User, Role, Hak Akses)

**Tanggal**: 2026-08-21 · **Sequence**: 020
**Plan**: `docs/plan/access-management-ui.md`

## Apa yang diubah

UI manajemen user, role, dan hak akses yang lebih ramah, bisa diakses dari
menu admin (`/_admin`). Sebelumnya semua derived CRUD mentah (grants sebagai
JSON, tanpa password field).

### Backend

- `internal/auth/module/master/user/entity.yaml` — tambah field `password`
  (masked) + hooks `before create/update` → native `formspec.core.user.hash-password`.
- `resource/auth_native.go` (baru) — handler `hashUserPassword`: hash
  `password` → `password_hash` (bcrypt), hapus plaintext sebelum insert.
- `resource/formspec.go` — register native handler; `uiReg.LoadEmbedded(auth.ModuleFS())`
  di `New()` dan `ReloadSpec()` agar forms/pages/tables auth termuat.
- `internal/ui/registry.go` — `resolveEntityRef` split di **titik terakhir**
  agar module bernama titik (`formspec.core`) resolve benar.

### Authored manifests (auth module)

- Forms: `user-create`, `user-edit` (dengan password), `role-form` (grants
  editor), `role-assignment-form` (relation pickers).
- Tables: `user-table`, `role-table`, `role-assignment-table`.
- Page: `access-management` (tabs Users/Roles/Assignments).

### Frontend

- `widgets/GrantsEditor.tsx` (baru) — checkbox tree page → tab → action untuk
  field `grants`; `components/ui/checkbox.tsx` (baru).
- `kinds/form/FormRenderer.tsx` — case widget `grants-editor`.
- `engine/entityRef.ts` — `resolveEntityRef` split di titik terakhir.
- `shell/OverlayHost.tsx`, `kinds/table/TableRenderer.tsx`,
  `kinds/listing/ListingRenderer.tsx`, `kinds/page/DetailPage.tsx` — pakai
  `resolveEntityRef` (dukung module bertitik).
- `hooks/useResolvedMenu.ts` — item menu "Access Management" di admin.

## Kenapa

User ingin form manajemen user/role/hak akses yang ramah dan bisa diakses dari
menu. Module `formspec.core` (bertitik) sebelumnya memecah konvensi
`module.entity` — diperbaiki dengan resolve dari titik terakhir.

## Verifikasi

- `go test ./...` + `npx vitest run` (96 tests) + `tsc --noEmit` lulus.
- E2E browser (dev-auth): menu admin → Access Management → tabs Users/Roles/
  Assignments; form Role dengan grants editor (checkbox page→tab→action);
  form User dengan password (di-hash); create user via API → login sukses.
