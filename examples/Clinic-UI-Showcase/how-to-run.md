# How to Run Forma

Forma berjalan dengan **dua proses**: backend (Go API server) dan frontend (Vite dev server atau SPA statis).

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
  --dev \
  --force
```

| Flag | Fungsi | Default |
|---|---|---|
| `--spec` | Path ke direktori YAML manifests | `./spec` |
| `--dsn` | Database DSN (sqlite atau postgres) | `sqlite:.forma/data.db` |
| `--addr` | REST API listen address | `:8080` |
| `--listen` | Sidecar ctx listener (internal) | `unix:///var/run/forma/sidecar.sock` |
| `--app-endpoint` | App endpoint (internal) | `unix:///var/run/forma/app.sock` |
| `--dev` | Dev mode (auth bypass, unsigned artifacts) | `false` |
| `--state-dir` | Local state directory | `.forma` |
| `--force` | Kill previous `forma-sidecar` on same ports (error jika port dipakai program lain) | `false` |
| `--web-dir` | Built SPA directory (e.g. `web/dist`). Serve frontend langsung tanpa Vite | `""` |
| `--dev-ui` | Spawn `npm run dev` otomatis — satu terminal full stack (implikasikan `--dev`) | `false` |
> **Catatan:** `--listen` dan `--app-endpoint` perlu HTTP URL (bukan Unix socket) jika tidak punya akses root, karena Unix socket default di `/var/run/forma/` butuh permission root.

### Contoh: Clinic UI Showcase

```bash
mkdir -p .forma
go run ./cmd/forma-sidecar/ \
  --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.forma/clinic.db" \
  --addr :8080 \
  --listen "http://127.0.0.1:9090" \
  --app-endpoint "http://127.0.0.1:9091" \
  --dev-ui \
  --force
``` 

Output yang diharapkan:
```
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

### Opsi A — Satu terminal (`--dev-ui`)

```bash
# Cukup satu command — sidecar + Vite otomatis
go run ./cmd/forma-sidecar/ \
  --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.forma/clinic.db" \
  --addr :8080 \
  --listen "http://127.0.0.1:9090" \
  --app-endpoint "http://127.0.0.1:9091" \
  --force --dev-ui
```

Buka `http://localhost:5173/default/_admin`. Vite HMR jalan di background — edit file langsung kelihatan tanpa restart.

### Opsi B — Dua terminal (manual)

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

**Terminal 2 — Frontend:**
```bash
cd web && npm install && npm run dev
```

### Opsi C — Static SPA (tanpa Vite)

```bash
cd web && npm run build
cd ..

go run ./cmd/forma-sidecar/ \
  --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.forma/clinic.db" \
  --addr :8080 \
  --listen "http://127.0.0.1:9090" \
  --app-endpoint "http://127.0.0.1:9091" \
  --dev --force \
  --web-dir web/dist
```

Satu proses di `:8080`, cocok untuk demo. Build ulang manual: `cd web && npm run build`.

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

Gunakan `--force` untuk otomatis kill previous `forma-sidecar` dan restart:

```bash
go run ./cmd/forma-sidecar/ ... --force
```

Output:
```
port 8080 is held by a previous forma-sidecar (PID 12345) — killing it...
[forma-sidecar] engine loaded: 45 routes
[forma-sidecar] REST API on :8080
```

Jika port dipakai oleh **program lain** (bukan forma-sidecar), `--force` akan error:
```
port 8080 is already in use by "nginx" (PID 12345). Use --force to kill a previous forma-sidecar, or stop the other program manually
```

Manual:
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

### Order-to-Cash

```bash
go run ./cmd/forma-sidecar/ \
  --spec examples/Order-to-Cash/spec \
  --dsn "sqlite:.forma/order-to-cash.db" \
  --addr :8080 \
  --listen "http://127.0.0.1:9090" \
  --app-endpoint "http://127.0.0.1:9091" \
  --dev
```

### Reference App

```bash
go run ./cmd/forma-sidecar/ \
  --spec examples/reference-app/spec \
  --dsn "sqlite:.forma/reference.db" \
  --addr :8080 \
  --listen "http://127.0.0.1:9090" \
  --app-endpoint "http://127.0.0.1:9091" \
  --dev
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
