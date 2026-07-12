# How to Run Forma

Forma berjalan dengan backend (Go API server) dan frontend (SPA React).

| Opsi | Cara | Terminal | HMR | Cocok untuk |
|---|---|---|---|---|
| **A — `--dev-ui`** | Sidecar spawn Vite otomatis | **1** | ✅ | Development paling praktis |
| **B — Manual** | Sidecar + `npm run dev` | 2 | ✅ | Development |
| **C — Static** | Sidecar + `--web-dir` | 1 | ❌ | Demo / produksi |

## Opsi A: Satu Terminal — `--dev-ui`

```bash
go run ./cmd/forma-sidecar/ \
  --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.forma/clinic.db" \
  --addr :8080 \
  --listen "http://127.0.0.1:9090" \
  --app-endpoint "http://127.0.0.1:9091" \
  --force --dev-ui
```

Sidecar akan:
1. Kill sidecar sebelumnya (kalau ada) berkat `--force`
2. Load engine + REST API di `:8080`
3. Spawn `npm run dev` sebagai child process — Vite HMR siap di `:5173`
4. Saat `Ctrl+C`, Vite ikut dimatikan

Buka **http://localhost:5173/default/_admin**.

## Opsi B: Dua Terminal

**Terminal 1 — Backend:**
```bash
go run ./cmd/forma-sidecar/ \
  --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.forma/clinic.db" \
  --addr :8080 \
  --listen "http://127.0.0.1:9090" \
  --app-endpoint "http://127.0.0.1:9091" \
  --dev --force
```

**Terminal 2 — Frontend (Vite HMR):**
```bash
cd web && npm run dev
```

## Opsi C: Static SPA (tanpa Vite)

```bash
cd web && npm run build

go run ./cmd/forma-sidecar/ \
  --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.forma/clinic.db" \
  --addr :8080 \
  --listen "http://127.0.0.1:9090" \
  --app-endpoint "http://127.0.0.1:9091" \
  --dev --force \
  --web-dir web/dist
```

Satu port `:8080` untuk API + SPA. Buka **http://localhost:8080/default/_admin**.

> Setiap edit frontend perlu `npm run build` ulang.

## Flags

| Flag | Fungsi | Default |
|---|---|---|
| `--spec` | Path ke direktori YAML manifests | `./spec` |
| `--dsn` | Database DSN (sqlite atau postgres) | `sqlite:.forma/data.db` |
| `--addr` | REST API listen address | `:8080` |
| `--listen` | Sidecar ctx listener (internal) | `unix:///var/run/forma/sidecar.sock` |
| `--app-endpoint` | App endpoint (internal) | `unix:///var/run/forma/app.sock` |
| `--dev` | Dev mode (auth bypass, unsigned artifacts) | `false` |
| `--state-dir` | Local state directory | `.forma` |
| `--force` | Kill previous `forma-sidecar` on same ports | `false` |
| `--web-dir` | Built SPA directory (e.g. `web/dist`) | `""` |
| `--dev-ui` | Spawn `npm run dev` otomatis (implikasikan `--dev`) | `false` |

> `--listen` dan `--app-endpoint` perlu HTTP URL (bukan Unix socket) jika tidak punya akses root.

## Reset Database

```bash
rm -rf .forma
```

Database auto-generate saat sidecar restart.

## Troubleshooting

| Masalah | Solusi |
|---|---|
| Port already in use | Tambah `--force` |
| Blank page | Hard refresh (Ctrl+F5) |
| Permission denied | Pake HTTP URL: `--listen "http://127.0.0.1:9090"` |
| Hyphen di tabel SQL | Sidecar otomatis `-` → `_` (fix di `internal/db/crud.go`) |

## Prasyarat

| Tool | Minimal | Cek |
|---|---|---|
| Go | 1.26+ | `go version` |
| Node.js | 22+ | `node --version` |
| npm | 10+ | `npm --version` |

## 1. Backend — `forma-sidecar`

Backend adalah server Go yang:
- Load YAML manifest dari direktori `--spec`
- Generate tabel SQLite/Postgres sesuai entity spec
- Serve REST API di `/{workspace}/api/v1/...`
- Serve Meta API di `/{workspace}/api/v1/_meta/...`

### Command

```bash
# Dari root repository
cd /workspaces/forma

go run ./cmd/forma-sidecar/ \
  --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.forma/clinic.db" \
  --addr :8080 \
  --listen "http://127.0.0.1:9090" \
  --app-endpoint "http://127.0.0.1:9091" \
  --dev
```

### Flags

| Flag | Fungsi | Default |
|---|---|---|
| `--spec` | Path ke direktori YAML manifests | `./spec` |
| `--dsn` | Database DSN (sqlite atau postgres) | `sqlite:.forma/data.db` |
| `--addr` | REST API listen address | `:8080` |
| `--listen` | Sidecar ctx listener (internal) | `unix:///var/run/forma/sidecar.sock` |
| `--app-endpoint` | App endpoint (internal) | `unix:///var/run/forma/app.sock` |
| `--dev` | Dev mode (auth bypass, unsigned artifacts) | `false` |
| `--state-dir` | Local state directory | `.forma` |
| **`--force`** | Kill previous `forma-sidecar` on same ports; error jika port dipakai program **lain** | `false` |

> **Catatan:** `--listen` dan `--app-endpoint` perlu HTTP URL (bukan Unix socket) jika tidak punya akses root, karena Unix socket default di `/var/run/forma/` butuh permission root.

### --force Flag

`--force` otomatis membunuh proses `forma-sidecar` sebelumnya yang masih menempel di port yang sama, lalu restart yang baru. Jika port dipakai program **lain**, akan muncul error:

```bash
port 8080 is already in use by "nginx" (PID 12345).
Use --force to kill a previous forma-sidecar, or stop the other program manually
```

### Contoh: Clinic UI Showcase

```bash
mkdir -p .forma
go run ./cmd/forma-sidecar/ \
  --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.forma/clinic.db" \
  --addr :8080 \
  --listen "http://127.0.0.1:9090" \
  --app-endpoint "http://127.0.0.1:9091" \
  --dev \
  --force
```

Output:
```
port 8080 is held by a previous forma-sidecar (PID 12345) — killing it...
[forma-sidecar] engine loaded: 45 routes
[forma-sidecar] ctx listener on http://127.0.0.1:9090
[forma-sidecar] REST API on :8080
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
cd web

# Install dependencies (pertama kali)
npm install

# Jalankan dev server
npm run dev
```

### Vite Proxy

Vite perlu proxy API calls ke backend. Konfigurasi ada di `web/vite.config.ts`:

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

| URL | Keterangan |
|---|---|
| `http://localhost:5173/default/_admin` | Admin panel — derived CRUD untuk semua entity |
| `http://localhost:5173/default/_admin/{module}/{plural}` | List view entity |
| `http://localhost:5173/default/_admin/{module}/{plural}/new` | Create form |
| `http://localhost:5173/default/_admin/{module}/{plural}/{id}` | Detail page |
| `http://localhost:5173/default/_admin/{module}/{plural}/{id}/edit` | Edit form |
| `http://localhost:5173/default/app` | App surface (override UI kinds) |

## 3. Production Build

Build SPA statis, lalu backend serve semuanya:

```bash
cd web && npm run build
cd ..

go run ./cmd/forma-sidecar/ \
  --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.forma/clinic.db" \
  --addr :8080 \
  --listen "http://127.0.0.1:9090" \
  --app-endpoint "http://127.0.0.1:9091" \
  --dev
```

> **Note:** `--web-dir` flag belum diimplementasikan di `forma-sidecar`. Untuk production, gunakan HTTP server (nginx, caddy) untuk serve `web/dist/` dan proxy `/{workspace}/api/` ke `localhost:8080`.

## 4. Reset Database

```bash
# Hapus file SQLite
rm .forma/clinic.db

# Atau hapus seluruh state
rm -rf .forma
```

Database akan auto-generate saat sidecar restart.

## 5. Troublehshooting

### Port already in use

Gunakan `--force`:
```bash
go run ./cmd/forma-sidecar/ ... --force
```

Atau manual:
```bash
# Cek proses yang pakai port
lsof -i :8080
lsof -i :9090
lsof -i :5173

# Kill
kill <PID>
# Atau
kill -9 <PID>
```

### Permission denied (Unix socket)

Gunakan HTTP URL untuk `--listen` dan `--app-endpoint`:
```bash
go run ./cmd/forma-sidecar/ ... \
  --listen "http://127.0.0.1:9090" \
  --app-endpoint "http://127.0.0.1:9091"
```

### Blank page di browser

1. Buka Chrome DevTools (F12) → Console, cek error
2. Hard refresh (Ctrl+F5) — browser cache mungkin stale
3. Pastikan Vite dan backend sama-sama running
4. Cek proxy: akses langsung `http://localhost:8080/default/api/v1/_meta/me`

### Table name with hyphens

Backend otomatis mengganti `-` dengan `_` di nama tabel SQL. Jika masih error, pastikan backend versi terbaru (fix di `internal/db/crud.go`).

## 6. Contoh Lain

### Billing (formerly Order-to-Cash)

```bash
go run ./cmd/forma-sidecar/ \
  --spec verticals/billing/spec \
  --dsn "sqlite:.forma/billing.db" \
  --addr :8080 \
  --listen "http://127.0.0.1:9090" \
  --app-endpoint "http://127.0.0.1:9091" \
  --dev \
  --force
```

### Reference App

```bash
go run ./cmd/forma-sidecar/ \
  --spec examples/reference-app/spec \
  --dsn "sqlite:.forma/reference.db" \
  --addr :8080 \
  --listen "http://127.0.0.1:9090" \
  --app-endpoint "http://127.0.0.1:9091" \
  --dev \
  --force
```

## 7. Arsitektur Singkat

```
Browser ─── Vite (:5173) ─── proxy /default/api/v1/ → forma-sidecar (:8080)
                                                              │
                                                    ┌─────────┴──────────┐
                                                    │  Entity Engine     │
                                                    │  REST API          │
                                                    │  Meta API          │
                                                    │  SQLite/Postgres   │
                                                    └────────────────────┘
```

- **Vite** dev server: HMR, hot reload, proxy API ke backend
- **forma-sidecar**: Go binary — entity engine, CRUD, permissions, Starlark
- **SPA**: React 19 + shadcn/ui — runtime reader Meta API, manifest-driven renderer
