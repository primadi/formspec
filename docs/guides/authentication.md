# Authentication & User Management

Panduan konfigurasi autentikasi FormSpec: model user, setup pertama kali,
policy registrasi, gate akses per-page, dan external auth (OAuth multi-provider).

> Status: Draft — mengikuti implementasi di `internal/auth/`, `internal/api/`,
> `resource/formspec.go` (auth redesign Fase 1–5).

---

## 1. Model User

FormSpec menggunakan **satu jenis user** di level workspace — disimpan sebagai
entity `formspec.core.user` (JSONB), scoped per workspace (tenant).

Field user:

| Field           | Tipe    | Keterangan                                                                         |
| --------------- | ------- | ---------------------------------------------------------------------------------- |
| `username`      | string  | Unique per workspace, 3–32 chars (`[a-zA-Z0-9._-]`)                                |
| `password_hash` | string  | bcrypt — plaintext tidak pernah disimpan                                           |
| `display_name`  | string  | Nama tampilan                                                                      |
| `email`         | string  | Dipakai untuk OAuth login (find-or-create by email)                                |
| `roles`         | json    | Daftar nama role, mis. `["admin"]`                                                 |
| `permissions`   | json    | Permission langsung, mis. `["*"]`                                                  |
| `active`        | boolean | `false` = blokir login                                                             |
| `status`        | string  | `active` \| `pending` \| `disabled` — `pending` tidak bisa login sampai di-approve |

Effective permission = permission langsung + materialisasi grant dari role
yang dipegang (dengan cache per-session).

---

## 2. Setup Pertama Kali (Bootstrap)

### Dev mode (`formspec dev`)

Otomatis — tidak perlu setup:

- Tanpa `--dev-auth`: bypass auth penuh (identity sintetis `developer`, `*`).
- Dengan `--dev-auth` (default di `formspec-registry`): user `admin`/`admin`
  di-seed otomatis + 4 owner roles (`workspace-owner`, `app-owner`,
  `module-owner`, `cloud-owner`).

### Production / self-hosting

Jalankan pertama kali dengan **database kosong** → `_admin` otomatis
mengarahkan ke **setup wizard** (`/{ws}/_admin/setup`). Buat admin pertama di
situ — admin mendapat `roles: ["admin"]`, `permissions: ["*"]` + owner roles
di-seed.

Deteksi: workspace tanpa satu pun user → meta bundle
`"setup_required": true` → SPA redirect. Setup adalah one-time: setelah ada
user, endpoint setup menolak (409).

Manual check:

```bash
curl http://localhost:8080/{ws}/_ui/setup
# {"data":{"setup_required":true},...}
```

---

## 3. Registration Policy (Sign-up)

Deklarasikan di `kind: Config` manifest, level **workspace**:

```yaml
apiVersion: formspec.dev/v1
kind: Config
metadata:
  name: app-config
  module: portal
spec:
  settings:
    registration:
      policy: open # open | approval | closed
      default_role: user # dipakai saat policy: open
```

| Policy           | Efek saat sign-up                       | Login                               |
| ---------------- | --------------------------------------- | ----------------------------------- |
| `open` (default) | User aktif + `default_role` (opsional)  | Langsung bisa                       |
| `approval`       | User `pending`                          | Diblokir (401) sampai admin approve |
| `closed`         | **Ditolak** — 403 `REGISTRATION_CLOSED` | — (admin buat user lewat `_admin`)  |

**Approve user pending** (admin, butuh permission `formspec.core.users.update`):

```bash
curl -X POST http://localhost:8080/{ws}/_ui/auth/approve \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username": "pendaftar", "roles": ["user"]}'
```

Perubahan policy berlaku **tanpa restart** (spec hot-reload).

---

## 4. Gate Akses Halaman (App Access + Per-Page)

Dua lapis gate, keduanya deklaratif:

### Level App — default surface

```yaml
kind: App
spec:
  access: public # public (default private) — surface boot anonim
```

### Level Page — override per-page

```yaml
kind: Page
spec:
  route: /landing
  public: true # anonim walau App-nya private
```

| Surface App       | Tanpa `public:`   | `public: true` | `public: false`   |
| ----------------- | ----------------- | -------------- | ----------------- |
| `access: public`  | Anonim            | Anonim         | **Butuh session** |
| `access: private` | **Butuh session** | Anonim         | Butuh session     |

Plus gate `permissions` pada page (list of permission strings — caller harus
punya minimal satu; difilter server-side di meta bundle).

Surface admin (`/{ws}/_admin`) digate permission biner `_admin.access`.

---

## 5. External Auth — OAuth Multi-Provider

Login via provider eksternal (Google, Microsoft, GitHub, atau OIDC/OAuth2
lain). Deklaratif — tidak ada kode provider yang perlu ditulis.

### 5.1 Mengaktifkan

Tambahkan `settings.auth.providers` di `kind: Config` manifest:

```yaml
apiVersion: formspec.dev/v1
kind: Config
metadata:
  name: app-config
  module: portal
spec:
  settings:
    auth:
      providers:
        google:
          client_id: "<dari Google Cloud Console>"
          client_secret: "<dari Google Cloud Console>"
        github:
          client_id: "<dari GitHub OAuth App>"
          client_secret: "<dari GitHub OAuth App>"
        # provider custom (OIDC apa pun, mis. Keycloak internal):
        keycloak:
          type: oidc
          issuer: https://keycloak.internal/realms/formspec
          client_id: formspec
          client_secret: "..."
```

Setelah apply/hot-reload:

- Meta bundle menyertakan `"oauth_providers": ["google", "github", "keycloak"]`
- Login screen otomatis merender tombol **"Sign in with \<Provider\>"** per provider
- Endpoint aktif:
  - `GET /{ws}/_ui/auth/oauth/{provider}/authorize` — mulai flow
  - `GET /{ws}/_ui/auth/oauth/{provider}/callback` — callback provider

### 5.2 Memilih Provider

| Kebutuhan                                   | Pilihan                   | Konfigurasi minimum                          |
| ------------------------------------------- | ------------------------- | -------------------------------------------- |
| Login dengan akun Google                    | `google` (preset OIDC)    | `client_id`, `client_secret`                 |
| Login dengan akun Microsoft (Entra ID)      | `microsoft` (preset OIDC) | `client_id`, `client_secret`                 |
| Login dengan akun GitHub                    | `github` (preset OAuth2)  | `client_id`, `client_secret`                 |
| OIDC lain (Keycloak, Auth0, Okta, Dex, ...) | `type: oidc`              | `issuer` + `client_id`, `client_secret`      |
| OAuth2 non-OIDC                             | `type: oauth2`            | `authorize_url`, `token_url`, `userinfo_url` |

Presets (`google`, `microsoft`, `github`) mengisi endpoint URL otomatis —
hanya credentials yang perlu diisi. Provider `oidc` men-resolve endpoint via
discovery dari `issuer`. Provider `oauth2` deklarasikan endpoint eksplisit.

Scopes default: `openid email profile` (email wajib — dipakai untuk
linking akun). Override dengan `scopes: [...]`.

### 5.3 Perilaku Login

1. Klik tombol → redirect ke provider (`authorize`)
2. User autentikasi di provider → callback ke FormSpec
3. Backend validasi CSRF state (single-use, TTL 10 menit) → exchange code →
   fetch userinfo
4. **Find-or-create by email:**
   - Email sudah terdaftar → login sebagai user itu
   - Email baru → user dibuat (username derived dari email, unik otomatis),
     status mengikuti registration policy:
     - `open` → aktif, langsung dapat token
     - `approval` → `pending` (login diblokir sampai di-approve)
5. Token pair dikirim ke SPA via **URL fragment** (`#token=...`) — tidak pernah
   ke server/log

Catatan: user yang di-link tetap bisa login password (dua jalur ke satu akun).

---

## 6. Testing — Provider Sandbox

Rekomendasi untuk menguji tanpa membayar / tanpa domain publik:

| Provider                         | Cara test                                                                                                                                       | Biaya           |
| -------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- | --------------- |
| **Keycloak** (rekomendasi utama) | Jalankan lokal via Docker — OIDC penuh, buat realm + client sendiri, redirect URI `http://localhost:8080/{ws}/_ui/auth/oauth/keycloak/callback` | Gratis, offline |
| **GitHub**                       | Buat OAuth App (Settings → Developer settings → OAuth Apps) — callback URL boleh `http://localhost:8080/...`                                    | Gratis          |
| **Google**                       | Cloud Console → OAuth consent screen mode **Testing** + test users; client type Web                                                             | Gratis          |
| **Microsoft**                    | Microsoft 365 Developer Program (tenant E5 developer) atau app registration free-tier di Entra ID                                               | Gratis          |
| **Auth0**                        | Free tier (dev tenant), OIDC-compliant                                                                                                          | Gratis (limits) |
| **Dex**                          | Static binary, konfigurasi YAML — sering dipakai untuk test OIDC                                                                                | Gratis, offline |

### Contoh: Keycloak lokal (Docker)

Keycloak adalah pilihan test terbaik karena **mendukung relative redirect
URI** — cocok dengan redirect_uri yang dikirim FormSpec saat ini.

```bash
docker run -p 8081:8080 -e KEYCLOAK_ADMIN=admin \
  -e KEYCLOAK_ADMIN_PASSWORD=admin \
  quay.io/keycloak/keycloak start-dev
```

1. Buka `http://localhost:8081` → buat realm `formspec`
2. Clients → Create client: `formspec`, **Valid redirect URIs**:
   `/{ws}/_ui/auth/oauth/keycloak/callback` (relative URI didukung Keycloak)
3. Credentials tab → copy client secret
4. Config manifest:

```yaml
settings:
  auth:
    providers:
      keycloak:
        type: oidc
        issuer: http://localhost:8081/realms/formspec
        client_id: formspec
        client_secret: "<secret>"
```

### Contoh: GitHub OAuth App

1. GitHub → Settings → Developer settings → OAuth Apps → **New OAuth App**
2. Homepage URL: `http://localhost:8080`, Authorization callback URL:
   `http://localhost:8080/default/_ui/auth/oauth/github/callback`
3. Copy Client ID + generate Client Secret
4. Config manifest: `github: { client_id: ..., client_secret: ... }`

### Keterbatasan redirect_uri (penting)

`redirect_uri` yang dikirim FormSpec saat ini berupa **path relatif**
(`/{ws}/_ui/auth/oauth/{provider}/callback`) — workspace-aware, cocok untuk
Keycloak dan deployment di balik reverse proxy yang me-resolve-nya.

Provider cloud umumnya **menuntut absolute URL**:

| Provider                         | Relative redirect_uri               |
| -------------------------------- | ----------------------------------- |
| Keycloak, Dex                    | ✅ didukung                         |
| Google, Microsoft, GitHub, Auth0 | ❌ absolute (perlu public base URL) |

Solusi sementara untuk provider absolute-only: deploy di balik reverse proxy
dan daftarkan callback absolute di provider — flow callback tetap berakhir di
FormSpec yang sama. Konfigurasi public base URL eksplisit adalah enhancement
berikutnya.

### Yang perlu diperhatikan saat test

- **Callback URL harus exact-match** dengan yang terdaftar di provider —
  format: `/{ws}/_ui/auth/oauth/{provider}/callback` (relative) atau absolute
  sesuai kebutuhan provider (lihat keterbatasan di atas)
- **Client secret** di Config manifest adalah secret — jangan commit nilai asli
  (gunakan external/ override atau environment-specific config di control plane)
- Error OAuth diarahkan ke login dengan fragment `#oauth=error` — cek console
  server log untuk detail kegagalan exchange
- User OAuth baru mengikuti **registration policy**: policy `approval` → user
  pending dan login diblokir sampai di-approve admin

---

## 7. Endpoint Ringkas

| Endpoint                             | Method   | Auth                         | Fungsi                                            |
| ------------------------------------ | -------- | ---------------------------- | ------------------------------------------------- |
| `/{ws}/_ui/auth/login`               | POST     | Public                       | Login username/password → token pair              |
| `/{ws}/_ui/auth/refresh`             | POST     | Public                       | Rotasi refresh token                              |
| `/{ws}/_ui/auth/register`            | POST     | Public                       | Sign-up (mengikuti registration policy)           |
| `/{ws}/_ui/auth/approve`             | POST     | `formspec.core.users.update` | Approve user pending                              |
| `/{ws}/_ui/auth/oauth/{p}/authorize` | GET      | Public                       | Mulai flow OAuth                                  |
| `/{ws}/_ui/auth/oauth/{p}/callback`  | GET      | Public                       | Callback provider                                 |
| `/{ws}/_ui/setup`                    | GET/POST | Public (one-time)            | Bootstrap admin pertama                           |
| `/{ws}/_ui/_meta/ui`                 | GET      | Permission-filtered          | UI bundle (+ `setup_required`, `oauth_providers`) |

## 8. Implementasi Referensi

| Komponen                                                                      | Lokasi                              |
| ----------------------------------------------------------------------------- | ----------------------------------- |
| OAuth package (Provider, presets, discovery)                                  | `internal/auth/oauth/`              |
| Auth service (`Login`, `Register`, `ApproveUser`, `GrantRoles`, `OAuthLogin`) | `internal/auth/service.go`          |
| Handlers (login/register/approve/setup/oauth)                                 | `internal/api/*_handler.go`         |
| Settings spec (`registration`, `auth.providers`)                              | `pkg/spec/resources.go`             |
| Wiring (boot + hot-reload)                                                    | `resource/formspec.go`              |
| Frontend (LoginScreen, SetupScreen, OAuthCallback, guards)                    | `renderers/react-shadcn/src/shell/` |
