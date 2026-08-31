# Fase 6 Dogfooding — API Keys, App-Membership, Workspace (Fase B)

**Tanggal**: 2026-08-20 · **Sequence**: 004
**Plan**: `docs/plan/fase6-dogfooding-auth-module.md` (Fase B)

## Apa yang diubah

Melanjutkan Fase 6 dogfooding: menambah entity auth baru ke bundled module
(`internal/auth/module/`) + middleware API key untuk permukaan external.

### Fase B — selesai

- **B1** Entity `formspec.core.api-key` (UIExposed) — `name`, `key_hash`
  (masked, indexed), `key_prefix`, `scope` (workspace|app), `app`,
  `permissions`, `expires_at`, `revoked_at`, `active`. `internal/auth/apikey.go`:
  `ApiKeyStore` (Create → return plaintext sekali, GetByKey via hash, List
  masked, Revoke, expiry/revoke check) + `ApiKey.Identity()` (service account
  `apikey:<id>`). Todo 6.4.1, 6.4.2.
- **B2** Middleware `X-FormSpec-Key` di `internal/api/middleware.go` —
  `SetApiKeyStore` global; `AuthMiddleware` resolve key hanya di surface
  external (`/api/v1/`), **tidak** di `/_ui/`. Todo 6.4.3.
- **B3** Entity `formspec.core.app-membership` (UIExposed) — `user_id`, `app`,
  `attributes` (mis. kode cabang), `active`. Todo 6.3.2.
- **B4** Entity `formspec.core.workspace` — identitas tenant (name, slug,
  owner_user_id, settings). Batas resource `formspec.core` dicatat: `job`/
  `audit-log`/`setting` milik sistem lain (7.13/4.7/7.2). Todo 6.3.5 (sebagian).

## Kenapa

Melengkapi data model auth sebagai modul FormSpec (dogfooding) dan menghidupkan
akses non-interaktif (service-to-service) ke permukaan external — fondasi untuk
Fase C (permission model) dan Fase E (middleware pipeline).

## File yang terkena dampak

- `internal/auth/module/master/api-key/entity.yaml` (baru)
- `internal/auth/module/transaction/app-membership/entity.yaml` (baru)
- `internal/auth/module/master/workspace/entity.yaml` (baru)
- `internal/auth/apikey.go` + `apikey_test.go` (baru)
- `internal/auth/core.go` — konstanta `RoleApiKey`
- `internal/api/middleware.go` — `SetApiKeyStore` + deteksi `X-FormSpec-Key`
- `internal/api/apikey_middleware_test.go` (baru)
- `resource/formspec.go` — wire `ApiKeyStore`
- `cmd/formspec/generate_auth_test.go` — update jumlah manifest (8)

## Verifikasi

- `go build ./...` + `go test ./...` hijau.
- Test `ApiKeyStore` (create/resolve/list-masked/revoke/expiry/identity) hijau.
- Test middleware (external surface resolve, UI surface tolak, invalid key 401) hijau.
- Boot `verticals/billing/spec`: meta bundle berisi `api-key`, `app-membership`,
  `role`, `role-assignment`, `user` (UIExposed); `session`/`workspace` tersembunyi.
  Invalid `X-FormSpec-Key` → 401 di surface external; tanpa auth → 404 (deny-by-default).
