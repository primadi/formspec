# How to Run FormSpec

FormSpec berjalan dengan **satu binary**: `formspec`. Backend (Go API server) dan
frontend (SPA React) sudah menjadi satu kesatuan — cukup satu perintah.

## Prasyarat

| Tool    | Minimal | Cek                                       |
| ------- | ------- | ----------------------------------------- |
| Go      | 1.26+   | `go version`                              |
| Node.js | 22+     | `node --version` (hanya untuk `--dev-ui`) |
| npm     | 10+     | `npm --version` (hanya untuk `--dev-ui`)  |

## Persona A — Cuma butuh UI jadi (80% developer)

```bash
# Dari root repository
cd /workspaces/formspec

go run ./cmd/formspec/ dev \
  --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.formspec/clinic.db"
```

Output yang diharapkan:

```
[formspec] engine loaded: 45 routes
[formspec] REST API on :8080
[formspec] SPA tersedia di http://localhost:8080/default/_admin
```

Buka `http://localhost:8080/default/_admin` — SPA sudah built-in.

> **Tanpa npm, tanpa Vite, tanpa build frontend.** Cukup `formspec dev`.

Verifikasi API:

```bash
curl http://localhost:8080/default/_ui/_meta/me
# → {"data":{"user_id":"developer","workspace":"default","roles":["admin"],"permissions":["*"]}}
```

> Meta API ada di `/_ui/_meta/*` (sibling dari `/api/v1/*`, bukan nested di
> dalamnya). `/api/v1/*` murni untuk REST CRUD entity yang di-generate.

### Lebih sederhana — dengan config file

Buat `formspec-sidecar.yaml` di folder project:

```yaml
spec: examples/Clinic-UI-Showcase/spec
dsn: sqlite:.formspec/clinic.db
dev: true
```

Lalu cukup:

```bash
go run ./cmd/formspec/ dev
```

## Persona B — Butuh edit frontend (20% developer)

Jika Anda perlu mengedit `renderers/react-shadcn/src/` (renderer, komponen, styles):

```bash
go run ./cmd/formspec/ dev \
  --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.formspec/clinic.db" \
  --dev-ui
```

`--dev-ui` akan:

1. Start backend API di `:8080`
2. Cari folder `renderers/react-shadcn/` (dari CWD atau module cache)
3. Spawn `npm run dev` → Vite HMR di `:5173`
4. Buka `http://localhost:5173/default/_admin`

Edit file di `renderers/react-shadcn/src/` → perubahan langsung kelihatan tanpa reload.

### Dua terminal (manual)

**Terminal 1 — Backend:**

```bash
go run ./cmd/formspec/ dev --spec examples/Clinic-UI-Showcase/spec
```

**Terminal 2 — Frontend:**

```bash
cd renderers/react-shadcn && npm install && npm run dev
```

Buka `http://localhost:5173/default/_admin` — Vite proxy API ke `:8080`.

### Akses

| URL                                                                | Keterangan                                    |
| ------------------------------------------------------------------ | --------------------------------------------- |
| `http://localhost:8080/default/_admin`                             | Admin panel — derived CRUD untuk semua entity |
| `http://localhost:8080/default/_admin/{module}/{plural}`           | List view entity                              |
| `http://localhost:8080/default/_admin/{module}/{plural}/new`       | Create form                                   |
| `http://localhost:8080/default/_admin/{module}/{plural}/{id}`      | Detail page                                   |
| `http://localhost:8080/default/_admin/{module}/{plural}/{id}/edit` | Edit form                                     |
| `http://localhost:8080/default/app`                                | App surface (override UI kinds)               |

## Production Build

SPA sudah embedded di binary. Untuk production:

```bash
# Build ulang SPA (jika ada perubahan)
cd renderers/react-shadcn && npm run build && cd ..

# Build binary
cd /workspaces/formspec
go build ./cmd/formspec/

# Jalankan
./formspec dev --spec ./spec
```

Atau gunakan `go install`:

```bash
go install github.com/primadi/formspec/cmd/formspec@latest
formspec dev --spec ./my-app/spec
```

## Reset Database

```bash
rm -rf .formspec/
```

Database akan auto-generate saat `formspec dev` dijalankan ulang.

## Flag Referensi

| Flag             | Default                    | Fungsi                                                 |
| ---------------- | -------------------------- | ------------------------------------------------------ |
| `--spec`         | `./spec`                   | Path ke direktori YAML manifests                       |
| `--dsn`          | `sqlite:.formspec/data.db` | Database DSN (sqlite atau postgres)                    |
| `--addr`         | `:8080`                    | REST API listen address                                |
| `--listen`       | `none`                     | Mode ctx listener: `none`, `local_http`, `unix_socket` |
| `--app-endpoint` | `none`                     | Mode app endpoint: `none`, `local_http`, `unix_socket` |
| `--runtime`      | auto-detect                | Runtime: `local`, `php`, `python`, `node`              |
| `--dev`          | `false`                    | Dev mode (implied oleh `--dev-ui`)                     |
| `--dev-ui`       | `false`                    | Development UI: spawn Vite HMR                         |
| `--state-dir`    | `.formspec`                | Local state directory (auto-create)                    |
| `--web-dir`      | auto-detect                | Override SPA directory                                 |

> `--listen` dan `--app-endpoint` default `none` — untuk single process tidak
> diperlukan. Gunakan `local_http` jika ada app process (PHP/Python/Node).

## Troubleshooting

### Port already in use

`formspec dev` otomatis kill previous instance (`--force` implied saat `--dev`).
Jika port dipakai program lain:

```bash
lsof -i :8080
kill <PID>
```

### Blank page di browser

1. Buka DevTools (F12) → Console, cek error
2. Hard refresh (Ctrl+F5)
3. Verifikasi API: `curl http://localhost:8080/default/api/v1/_meta/me`
4. Pastikan `renderers/react-shadcn/dist/` ada (untuk embedded SPA) atau akses via `:5173` (dev-ui)

### Permission denied (Unix socket)

Tidak relevan — default `listen: none`. Jika Anda perlu app process,
gunakan `--listen local_http` (TCP, tanpa permission khusus).

## Contoh Lain

### Billing

```bash
go run ./cmd/formspec/ dev --spec verticals/billing/spec
```

### Reference App

```bash
go run ./cmd/formspec/ dev --spec examples/reference-app/spec
```

## Arsitektur Singkat

```
Browser ─── formspec dev (:8080)
                │
        ┌───────┴───────┐
        │  Entity Engine│
        │  REST API     │
        │  Meta API     │
        │  SPA (embed)  │
        │  SQLite/      │
        │  Postgres     │
        └───────────────┘
```

- **Single binary**: `formspec` — backend + frontend dalam satu proses
- **SPA built-in**: via `//go:embed renderers/react-shadcn/dist/*`
- **Vite HMR**: opsional (`--dev-ui`) untuk development frontend
