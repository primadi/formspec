# How to Run FormSpec

FormSpec berjalan dalam satu proses engine (`formspec dev`) + frontend (SPA React). App business logic dalam bahasa apapun (Go, PHP, Python, Ruby, Java, .NET, TypeScript, Rust) berjalan sebagai child process.

| Opsi               | Cara                               | Terminal | HMR | Cocok untuk                |
| ------------------ | ---------------------------------- | -------- | --- | -------------------------- |
| **A — `--dev-ui`** | `formspec dev` spawn Vite otomatis | **1**    | ✅  | Development paling praktis |
| **B — Manual**     | `formspec dev` + `npm run dev`     | 2        | ✅  | Development                |
| **C — Static**     | `formspec dev` + `--web-dir`       | 1        | ❌  | Demo / produksi            |

## Opsi A: Satu Terminal — `--dev-ui`

```bash
go run ./cmd/formspec/ dev \
  --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.formspec/clinic.db" \
  --addr :8080 \
  --force --dev-ui
```

`formspec dev` akan:

1. Kill engine sebelumnya (kalau ada) berkat `--force`
2. Load engine + REST API di `:8080`
3. Spawn `npm run dev` sebagai child process — Vite HMR siap di `:5173`
4. Saat `Ctrl+C`, Vite ikut dimatikan

Buka **http://localhost:5173/default/\_admin**.

## Opsi B: Dua Terminal

**Terminal 1 — Engine:**

```bash
go run ./cmd/formspec/ dev \
  --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.formspec/clinic.db" \
  --addr :8080 \
  --dev --force
```

**Terminal 2 — Frontend (Vite HMR):**

```bash
cd renderers/react-shadcn && npm run dev
```

## Opsi C: Static SPA (tanpa Vite)

```bash
cd renderers/react-shadcn && npm run build

go run ./cmd/formspec/ dev \
  --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.formspec/clinic.db" \
  --addr :8080 \
  --dev --force \
  --web-dir renderers/react-shadcn/dist
```

Satu port `:8080` untuk API + SPA. Buka **http://localhost:8080/default/\_admin**.

> Setiap edit frontend perlu `npm run build` ulang.

## Flags

| Flag               | Fungsi                                                                               | Default                    |
| ------------------ | ------------------------------------------------------------------------------------ | -------------------------- |
| `--spec`           | Path ke direktori YAML manifests                                                     | `./spec`                   |
| `--dsn`            | Database DSN (sqlite atau postgres)                                                  | `sqlite:.formspec/data.db` |
| `--addr`           | REST API listen address                                                              | `:8080`                    |
| `--dev`            | Dev mode (auth bypass, unsigned artifacts)                                           | `false`                    |
| `--state-dir`      | Local state directory                                                                | `.formspec`                |
| `--force`          | Kill previous `formspec` engine on same ports                                        | `false`                    |
| `--web-dir`        | Built SPA directory (e.g. `renderers/react-shadcn/dist`)                             | `""`                       |
| `--dev-ui`         | Spawn `npm run dev` otomatis (implikasikan `--dev`)                                  | `false`                    |
| `--runtime`        | App runtime: `auto`, `go`, `php`, `python`, `ruby`, `java`, `dotnet`, `rust`, `node` | `auto`                     |
| `--app-dir`        | App source directory (child-process runtime)                                         | `.formspec/app`            |
| `--app-entrypoint` | Entrypoint file (default tergantung runtime)                                         | auto                       |

> `--listen` dan `--app-endpoint` sudah otomatis diatur oleh `formspec dev` — tidak perlu di-set manual.

## Reset Database

```bash
rm -rf .formspec
```

Database auto-generate saat engine restart.

## Troubleshooting

| Masalah             | Solusi                                                            |
| ------------------- | ----------------------------------------------------------------- |
| Port already in use | Tambah `--force`                                                  |
| Blank page          | Hard refresh (Ctrl+F5)                                            |
| Permission denied   | `formspec dev` pilih socket/HTTP otomatis berdasarkan environment |
| Hyphen di tabel SQL | Engine otomatis `-` → `_` (fix di `internal/db/crud.go`)          |

## Prasyarat

| Tool    | Minimal | Cek              |
| ------- | ------- | ---------------- |
| Go      | 1.26+   | `go version`     |
| Node.js | 22+     | `node --version` |
| npm     | 10+     | `npm --version`  |

## 1. Engine — `formspec dev`

`formspec dev` adalah engine yang:

- Load YAML manifest dari direktori `--spec`
- Generate tabel SQLite/Postgres sesuai entity spec
- Serve REST API di `/{workspace}/api/v1/...`
- Serve Meta API di `/{workspace}/api/v1/_meta/...`
- Auto-detect runtime dari `--app-dir` dan spawn app child process
- Enforce permission, tenant isolation, ctx.\* primitives

### Command

```bash
# Dari root repository
cd /workspaces/formspec

go run ./cmd/formspec/ dev \
  --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.formspec/clinic.db" \
  --addr :8080 \
  --dev
```

### App Child Process

App business logic dalam bahasa apapun berjalan sebagai **child process** dari `formspec dev`. Engine dan app berkomunikasi via Unix socket:

```
┌───────────────────────────────────────────┐
│ formspec dev (engine)                         │
│  • Entity engine, state machine           │
│  • Permission enforcement                 │
│  • Tenant isolation                       │
│  • REST API + Admin panel                 │
│  • ctx.* primitives                       │
│              │                            │
│              ▼ Unix socket                │
│  app child process (via lib-formspec-*)      │
│  • Business logic only                    │
│  • Go / PHP / Python / Ruby / Java        │
│    .NET / TypeScript / Rust               │
└───────────────────────────────────────────┘
```

### Auto-detect Runtime

`formspec dev` mendeteksi runtime dari file di `--app-dir`:

| File                                  | Runtime              |
| ------------------------------------- | -------------------- |
| `go.mod`                              | Go                   |
| `Cargo.toml`                          | Rust                 |
| `package.json`                        | Node.js / TypeScript |
| `composer.json`                       | PHP                  |
| `pyproject.toml` / `requirements.txt` | Python               |
| `Gemfile`                             | Ruby                 |
| `pom.xml` / `build.gradle`            | Java                 |
| `*.csproj` / `*.sln`                  | .NET                 |

Override manual dengan `--runtime <name>` atau `runtime:` di `formspec-app.yaml`.

### Flags

| Flag               | Fungsi                                        | Default                    |
| ------------------ | --------------------------------------------- | -------------------------- |
| `--spec`           | Path ke direktori YAML manifests              | `./spec`                   |
| `--dsn`            | Database DSN (sqlite atau postgres)           | `sqlite:.formspec/data.db` |
| `--addr`           | REST API listen address                       | `:8080`                    |
| `--dev`            | Dev mode (auth bypass, unsigned artifacts)    | `false`                    |
| `--state-dir`      | Local state directory                         | `.formspec`                |
| **`--force`**      | Kill previous `formspec` engine on same ports | `false`                    |
| `--web-dir`        | Built SPA directory                           | `""`                       |
| `--dev-ui`         | Auto-spawn Vite dev server                    | `false`                    |
| `--runtime`        | Override runtime auto-detect                  | `auto`                     |
| `--app-dir`        | App source directory                          | `.formspec/app`            |
| `--app-entrypoint` | Entrypoint file                               | auto                       |

> `--listen` dan `--app-endpoint` sudah diatur internal — tidak perlu di-set manual. `--app-endpoint-url` untuk override jika perlu endpoint spesifik.

### --force Flag

`--force` otomatis membunuh proses `formspec` sebelumnya yang masih menempel di port yang sama, lalu restart yang baru. Jika port dipakai program **lain**, akan muncul error:

```bash
port 8080 is already in use by "nginx" (PID 12345).
Use --force to kill a previous formspec instance, or stop the other program manually
```

### Contoh: Clinic UI Showcase

```bash
mkdir -p .formspec
go run ./cmd/formspec/ dev \
  --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.formspec/clinic.db" \
  --addr :8080 \
  --dev \
  --force
```

Output:

```
port 8080 is held by a previous formspec instance (PID 12345) — killing it...
[formspec] engine loaded: 45 routes
[formspec] ctx listener on unix:///tmp/formspec/sidecar.sock
[formspec] REST API on :8080
```

Verifikasi:

```bash
curl http://localhost:8080/default/api/v1/_meta/me
# → {"data":{"user_id":"developer","workspace":"default","permissions":["*"]}}
```

## 2. Frontend — Vite Dev Server

Frontend adalah SPA React 19 yang membaca Meta API secara runtime.

### Command

```bash
cd renderers/react-shadcn

# Install dependencies (pertama kali)
npm install

# Jalankan dev server
npm run dev
```

### Vite Proxy

Vite perlu proxy API calls ke backend. Konfigurasi ada di `renderers/react-shadcn/vite.config.ts`:

```typescript
server: {
  proxy: {
    '/default/api/v1': {
      target: 'http://localhost:8080',
      changeOrigin: true,
    },
  },
},
```

Jika backend di port berbeda, sesuaikan `target`.

### Akses

| URL                                                                | Keterangan                                    |
| ------------------------------------------------------------------ | --------------------------------------------- |
| `http://localhost:5173/default/_admin`                             | Admin panel — derived CRUD untuk semua entity |
| `http://localhost:5173/default/_admin/{module}/{plural}`           | List view entity                              |
| `http://localhost:5173/default/_admin/{module}/{plural}/new`       | Create form                                   |
| `http://localhost:5173/default/_admin/{module}/{plural}/{id}`      | Detail page                                   |
| `http://localhost:5173/default/_admin/{module}/{plural}/{id}/edit` | Edit form                                     |
| `http://localhost:5173/default/app`                                | App surface (override UI kinds)               |

## 3. Production Build

Untuk production, gunakan `formspec serve` (tanpa `--dev`):

```bash
# Build SPA
cd renderers/react-shadcn && npm run build
cd ..

# Jalankan engine dalam mode production
go run ./cmd/formspec/ serve \
  --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.formspec/clinic.db" \
  --addr :8080 \
  --web-dir renderers/react-shadcn/dist
```

`formspec serve` mengaktifkan:

- **JWT auth** — semua request API perlu token valid
- **Strict `uses` enforcement** — action hanya bisa akses resource yang di-declare
- **Production logging** — structured, tanpa debug output
- **No auto-reload** — app child process dijalankan sekali, tidak di-watch

Untuk deployment skala besar, gunakan nginx/caddy sebagai reverse proxy:

- Serve `renderers/react-shadcn/dist/` untuk static assets
- Proxy `/{workspace}/api/` ke `localhost:8080`
- Rate limiting, SSL termination, CDN

## 4. Reset Database

```bash
# Hapus file SQLite
rm .formspec/clinic.db

# Atau hapus seluruh state
rm -rf .formspec
```

Database akan auto-generate saat engine restart.

## 5. Troublehshooting

### Port already in use

Gunakan `--force`:

```bash
go run ./cmd/formspec/ dev ... --force
```

Atau manual:

```bash
# Cek proses yang pakai port
lsof -i :8080
lsof -i :5173

# Kill
kill <PID>
# Atau
kill -9 <PID>
```

### Blank page di browser

1. Buka Chrome DevTools (F12) → Console, cek error
2. Hard refresh (Ctrl+F5) — browser cache mungkin stale
3. Pastikan Vite dan engine sama-sama running
4. Cek proxy: akses langsung `http://localhost:8080/default/api/v1/_meta/me`

### Table name with hyphens

Engine otomatis mengganti `-` dengan `_` di nama tabel SQL. Jika masih error, pastikan engine versi terbaru (fix di `internal/db/crud.go`).

## 6. Contoh Lain

### Billing (formerly Order-to-Cash)

```bash
go run ./cmd/formspec/ dev \
  --spec verticals/billing/spec \
  --dsn "sqlite:.formspec/billing.db" \
  --addr :8080 \
  --dev \
  --force
```

### Reference App

```bash
go run ./cmd/formspec/ dev \
  --spec examples/reference-app/spec \
  --dsn "sqlite:.formspec/reference.db" \
  --addr :8080 \
  --dev \
  --force
```

## 7. Arsitektur Singkat

```
Browser ─── Vite (:5173) ─── proxy /default/api/v1/ → formspec dev (:8080)
                                                              │
                                                    ┌─────────┴──────────┐
                                                    │  Entity Engine     │
                                                    │  REST API          │
                                                    │  Meta API          │
                                                    │  ctx.* primitives  │
                                                    │  App child process │
                                                    │  SQLite/Postgres   │
                                                    └────────────────────┘
```

- **Vite** dev server: HMR, hot reload, proxy API ke engine
- **`formspec dev`**: Engine — entity engine, CRUD, permissions, Starlark, ctx.\* primitives, spawn app child process
- **App child process**: Business logic dalam bahasa apapun (Go/PHP/Python/Ruby/Java/.NET/TypeScript/Rust) via `lib-formspec-*` SDK
- **SPA**: React 19 + shadcn/ui — runtime reader Meta API, manifest-driven renderer
