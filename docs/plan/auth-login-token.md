# Plan: Fase 6.1 — Login & Token (Auth)

**Status**: ✅ Complete (2026-08-19)
**Referensi spec**: `docs/spec/backend/01-core-basic.md` §8 (auth), §15 (auth required by default);
`docs/spec/platform/02-workspace-app-module.md` §3 (auth per-App, strategy terbuka);
`docs/spec/frontend/04-spec-resolution-api.md` §1 (`/_meta/me`).

## Tujuan

Menghidupkan alur login + issuance token JWT (access + refresh) di single-server,
melengkapi fondasi yang sudah ada (`internal/auth` sudah punya `Identity`,
`JWTValidator` untuk _validate_, `DevValidator`, `AuthMiddleware`, `RequirePermission`,
`/_meta/me`).

## Item todo yang dicakup

- **6.1.1** Login endpoint — `POST /{ws}/api/v1/auth/login`, verifikasi kredensial
  (password hash), issuance JWT (access + refresh).
- **6.1.2** Token claims — `sub`, `workspace`, `roles`, `permissions`, `exp`, `iat`.
- **6.1.3** Token refresh — rotate (invalidate old, issue new).

## Desain

> **Keputusan (2026-08-19):** User store, session store, dan seluruh resource
> auth **di-backing oleh Entity FormSpec** di namespace bawaan `formspec.core`
> (`platform/02` §9) — bukan store custom. `user` dan `session` didaftarkan
> sebagai Entity built-in saat startup; `api-key`/`role`/`role-assignment`/
> `app-membership` menyusul di 6.3/6.4. Ini menghindari duplikasi model data dan
> memberi CRUD/audit/permission gratis dari engine.

### 0. Strategi merge spec auth ↔ spec app

**Keputusan final (2026-08-19):** Auth service mengharapkan **kontrak minimal per
peran** (mis. `user` wajib `username` + `password_hash`), bukan entity spesifik.
Default = implicit merge dari `formspec.core` (common use case). Full override =
user sediakan entity sendiri yang memenuhi kontrak. **User override menang.**

Lapisan resolusi peran logis (`user`, `session`, …) → referensi entity:

1. **Override eksplisit** — `auth_config_ref` (App manifest) / config Go memetakan
   peran → entity ref (`{module}/{name}`).
2. **`external/`** — module external yang dikustomisasi user (di-commit ke git),
   di-load loader; entity di sini **menang** atas default `formspec.core`.
3. **`formspec.core`** — default bawaan (internal, tanpa route).

Alur DX: `formspec generate-auth` meng-clone auth module bawaan ke `vendors/`
(readonly) → user pindahkan ke `external/` → modifikasi → auth service pakai
versi kustom.

**Folder `external/`** (konsep baru, selaras `vendors/` tanpa titik): module
external yang dikustomisasi user, **wajib di-commit** ke git — berbeda dari
`vendors/` (readonly, tidak di-commit). Didokumentasikan di
`docs/spec/platform/08-project-layout.md` + todo.

`formspec.core` tetap namespace reserved (app tidak bisa deklarasi), tapi
`external/` bisa menyediakan pengganti per peran.

### 1. Core entities (`internal/auth/core.go`)

- `RegisterCoreEntities(reg *entity.Registry) error` — mendaftarkan Entity
  built-in `formspec.core` via `RegisterArtifactManifest` (programatik, bukan
  filesystem), dipanggil di `New()` sebelum `SyncSchema` agar tabel dibuat.
- **`user`** (characteristic: master, internal — tanpa `expose`):
  - `username` (string, unique, index, required) — lookup login
  - `password_hash` (string, **masked**) — bcrypt hash, tak pernah bocor
  - `display_name` (string), `email` (string)
  - `roles` (json), `permissions` (json)
  - `active` (boolean, default true)
- **`session`** (characteristic: transaction, internal):
  - `user_id` (string, index), `refresh_jti` (string, unique, index)
  - `expires_at` (datetime), `ip` (string), `user_agent` (string)

### 2. User store (`internal/auth/user.go`)

- `User` struct: `ID`, `Username`, `PasswordHash`, `WorkspaceID`, `Roles`,
  `Permissions`, `Active`.
- `UserStore` interface: `GetByUsername(ctx, workspaceID, username) (*User, error)`.
- `EntityUserStore` — membaca `formspec.core.user` via `reg.GetEntityStore`
  → `FindByField(ctx, ws, "username", username)` (panggilan store langsung,
  tanpa permission — auth service adalah kode framework tepercaya).

### 2. Password hashing (`internal/auth/password.go`)

- `HashPassword(plain string) (string, error)` — bcrypt (cost default).
- `VerifyPassword(hash, plain string) bool` — bcrypt compare.

### 3. Token issuer (`internal/auth/token.go`)

- `TokenIssuer` struct: `secret`, `issuer`, `audience`, `accessTTL`, `refreshTTL`.
- `IssueAccessToken(user *User) (string, error)` — claims: `sub`, `ws`, `roles`,
  `perms`, `type=access`, `iat`, `exp`, `iss`, `aud`.
- `IssueRefreshToken(user *User) (string, error)` — claims: `sub`, `ws`, `jti`,
  `type=refresh`, `iat`, `exp`.
- `ParseRefreshToken(token string) (*RefreshClaims, error)` — validasi signature +
  `type == refresh`.
- `NewTokenIssuer(secret, issuer, audience string, accessTTL, refreshTTL time.Duration)`.

### 4. Session store untuk refresh rotation (`internal/auth/session.go`)

- `SessionStore` interface: `Create(ctx, s Session) error`, `Get(ctx, jti) (*Session, bool)`,
  `Delete(ctx, jti) error`, `DeleteForUser(ctx, userID) error`.
- `EntitySessionStore` — membaca/menulis `formspec.core.session` via
  `reg.GetEntityStore` (Insert untuk Create, FindByField `refresh_jti` untuk
  Get, SoftDelete untuk Delete).
- `Session`: `JTI`, `UserID`, `WorkspaceID`, `ExpiresAt`, `CreatedAt`.

### 5. Auth service (`internal/auth/service.go`)

- `Service` struct: `roles *RoleResolver`, `issuer *TokenIssuer`.
- `RoleResolver` — memetakan peran logis (`user`, `session`, …) → entity ref
  (`{module}/{name}`). Urutan: override eksplisit (config) → `external/` →
  default `formspec.core`.
- `Login(ctx, workspaceID, username, password string) (*TokenPair, error)` —
  resolve role `user` → EntityStore → `FindByField("username")`; verifikasi
  aktif + bcrypt; issue access + refresh; simpan session (jti).
- `Refresh(ctx, refreshToken string) (*TokenPair, error)` — validasi refresh token,
  cek session masih ada & belum expired, rotate (hapus jti lama, issue baru).
- `TokenPair`: `AccessToken`, `RefreshToken`, `ExpiresIn` (detik).

### 6. HTTP handlers (`internal/api/auth_handler.go`)

- `HandleLogin` — `POST /{ws}/api/v1/auth/login`; body `{username, password}`;
  sukses → `200 {data: {access_token, refresh_token, expires_in}, meta}`;
  gagal kredensial → `401 UNAUTHORIZED`.
- `HandleRefresh` — `POST /{ws}/api/v1/auth/refresh`; body `{refresh_token}`;
  sukses → `200 {data: {access_token, refresh_token, expires_in}}`;
  invalid/expired → `401 UNAUTHORIZED`.
- Keduanya **public** (tanpa `RequirePermission`), dipasang di router sebelum
  `AuthMiddleware` menolak (AuthMiddleware sudah mengizinkan token kosong → anonymous).

### 7. Wiring

- `RouterBuilder` dapat `SetAuthService(svc *auth.Service)`.
- Di `BuildHTTP`, daftarkan route login/refresh di bawah `/{ws}/api/v1/auth/...`
  (public, tanpa auth).
- `resource/formspec.go` `configureAuth` / `New` — konstruksi `auth.Service` dari
  `Config` (secret, issuer, TTL) + seed user default (dev) bila tidak ada.

### 8. `external/` loader (`internal/manifest/loader.go`)

- Loader scan `external/` (selain `spec/`) — module external yang dikustomisasi
  user. Entity di sini **menang** atas `formspec.core` default saat resolver
  membangun role → entity ref.
- `vendors/` tetap dibaca (readonly) — `generate-auth` menyalin auth module ke
  sana dulu; user yang ingin memodifikasi memindahkannya ke `external/`.

### 9. `formspec generate-auth` (`cmd/formspec/generate_auth.go`)

- `formspec generate-auth` — meng-clone auth module bawaan (user + session specs)
  ke `vendors/auth/` (readonly); flag `--external` menyalin langsung ke
  `external/auth/` (siap di-modifikasi, wajib di-commit).

## File yang diubah/dibuat

| File                            | Aksi                                                           |
| ------------------------------- | -------------------------------------------------------------- |
| `internal/auth/core.go`         | baru — `RegisterCoreEntities` (user + session)                 |
| `internal/auth/role.go`         | baru — `RoleResolver` (peran → entity ref)                     |
| `internal/auth/entitystore.go`  | baru — `EntityUserStore` + `EntitySessionStore`                |
| `internal/auth/user.go`         | ubah — hapus `MemoryUserStore`, tambah `EntityUserStore`       |
| `internal/auth/password.go`     | baru                                                           |
| `internal/auth/token.go`        | baru                                                           |
| `internal/auth/session.go`      | ubah — hapus `MemorySessionStore`, tambah `EntitySessionStore` |
| `internal/auth/service.go`      | baru                                                           |
| `internal/auth/service_test.go` | baru                                                           |
| `internal/api/auth_handler.go`  | baru                                                           |
| `internal/api/router.go`        | ubah — route auth + `SetAuthService`                           |
| `internal/entity/registry.go`   | ubah — `SpecInfo.Internal`, tolak `formspec.core` dari app     |
| `internal/api/generator.go`     | ubah — skip entity internal                                    |
| `internal/manifest/loader.go`   | ubah — scan `external/` + `vendors/`                           |
| `cmd/formspec/generate_auth.go` | baru — `formspec generate-auth`                                |
| `resource/formspec.go`          | ubah — register core entities + wire `auth.Service`            |
| `go.mod`                        | tambah `golang.org/x/crypto` (bcrypt)                          |

## Level of effort

- **Large** — ~12 file baru/ubah + wiring + test.

## Verifikasi

- `go test ./internal/auth/... ./internal/api/... ./internal/entity/... ./internal/manifest/...`
- `go build ./...`
- `go test ./...` (regresi)
