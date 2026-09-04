# 2026-09-03-001 — Fase 5 Auth Redesign: OAuth Multi-Provider

## Apa

Implementasi OAuth multi-provider (auth redesign Fase 5) — OIDC + OAuth2
generic + preset Google/Microsoft/GitHub, deklaratif via Config.

### Package `internal/auth/oauth` (baru)

- `Provider` interface: `Name()`, `AuthorizeURL(state, redirectURL)`,
  `Exchange(ctx, code)` → normalized `UserInfo{ID, Email, Name}`.
- `New(cfg)` — apply preset (google/microsoft OIDC, github OAuth2), OIDC
  discovery (`{issuer}/.well-known/openid-configuration`, cache), fallback
  explicit endpoints.
- `fetchUserInfo` — GET userinfo endpoint, normalize `sub/id/email/name`.

### Spec (`pkg/spec/resources.go`)

- `Settings.Auth *AuthSettings` → `Providers map[string]*OAuthProviderSettings`
  (`type` oidc|oauth2, `client_id`, `client_secret`, `scopes`, `issuer`,
  `authorize_url`, `token_url`, `userinfo_url`).
- `ResolveSettings` overlay `Auth`.

### Auth service (`internal/auth/service.go`)

- `SetOAuthProviders` / `OAuthProvider` / `OAuthProviders`.
- `OAuthLogin(ctx, ws, provider, code)` — exchange code → find-or-create user
  by email (username derived dari email, unique suffix), status per policy
  (open → active, approval → pending), issue token pair.
- `GetByEmail` di user store (email `index: true` di entity).

### API (`internal/api/oauth_handler.go` baru)

- `GET /{ws}/_ui/auth/oauth/{provider}/authorize` — CSRF state (in-memory,
  TTL 10m, single-use) → redirect provider.
- `GET /{ws}/_ui/auth/oauth/{provider}/callback` — validate state → exchange
  → redirect SPA callback dengan token di URL fragment (tidak ke server).
- Meta bundle `oauth_providers` (nama provider untuk tombol login).

### Frontend

- `LoginScreen.tsx` — tombol "Sign in with <Provider>" per provider (dari
  `bundle.oauth_providers`).
- `OAuthCallback.tsx` (baru) — baca token dari fragment, boot session,
  redirect ke surface root. Route `/{ws}/_admin/oauth/callback`.

## Kenapa

User poin 5: auth saat ini hanya password; harus ditambah OAuth multi-provider
(Google, Microsoft, GitHub) yang bisa dikustom sesuai kebutuhan.

## Verifikasi

- Config test (GitHub preset, dummy creds): meta `oauth_providers:["github"]`;
  authorize → 302 ke GitHub dengan redirect_uri workspace-aware
  (`/default/...`); callback invalid state → 400.
- Browser: login screen menampilkan tombol "Sign in with Github".
- Test `TestService_OAuthLogin_*` (mock provider): new user (username derived
  - active), existing user (by email, no duplicate), unknown provider →
    ErrInvalidCredentials, approval policy → pending (login blocked).
- `go test ./internal/auth ./internal/api ./pkg/spec ./resource`: 295 passed.

## File terdampak

- `internal/auth/oauth/oauth.go` (baru)
- `internal/auth/{service,user}.go`, `internal/auth/oauth_login_test.go` (baru)
- `internal/auth/module/master/user/entity.yaml` (email index)
- `internal/api/{oauth_handler.go (baru),router.go,meta.go}`
- `internal/ui/meta.go`
- `pkg/spec/resources.go`
- `resource/formspec.go` (wire providers boot + hot-reload)
- `renderers/react-shadcn/src/{shell/LoginScreen.tsx,shell/OAuthCallback.tsx (baru),shell/index.ts,App.tsx,types/manifest.ts}`
- `registry/web/dist/` (sync build)

## Referensi

- Plan: `/memories/session/plan.md` (Fase 5)
- Todo: 5.2.16
- Dokumentasi: `docs/guides/authentication.md` (panduan konfigurasi auth:
  setup, registration policy, page gating, OAuth multi-provider + sandbox
  testing)
