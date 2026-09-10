# `formspec dev` — Development Server

**Version:** 1.0
**Status:** Draft

> `formspec dev` adalah satu-satunya perintah untuk menjalankan FormSpec development
> server. Backend (Go entity engine) dan frontend (SPA React) berjalan dalam
> satu proses. Tidak perlu Vite, npm, atau build frontend terpisah — cukup
> `formspec dev`.

---

## 1. Filosofi

FormSpec mengenal **dua persona** developer:

| Persona     | Kebutuhan                    | Command                 |
| ----------- | ---------------------------- | ----------------------- |
| **A** (80%) | UI jadi, tidak edit frontend | `formspec dev`          |
| **B** (20%) | Edit renderer/komponen React | `formspec dev --dev-ui` |

Persona A cukup satu perintah — SPA sudah embedded di binary (`//go:embed`).
Persona B mendapat Vite HMR untuk hot-reload frontend.

---

## 2. Quick Start

### Persona A — SPA built-in

```bash
formspec dev --spec ./my-app/spec
```

- Backend API di `:8080`
- SPA tersedia di `http://localhost:8080/default/_admin`
- Tidak perlu npm, Vite, atau build frontend

### Persona B — Vite HMR

```bash
formspec dev --spec ./my-app/spec --dev-ui
```

- Backend API di `:8080`
- Vite HMR di `:5173`
- Edit `renderers/react-shadcn/src/` → perubahan langsung kelihatan
- `--dev-ui` implied `--dev` + `--force`

### Dengan config file

Buat `formspec-app.yaml` di folder project:

```yaml
spec: ./my-app/spec
dsn: sqlite:.formspec/data.db
dev-ui: true
```

Lalu cukup:

```bash
formspec dev
```

---

## 3. Flag Reference

| Flag             | Default                    | Deskripsi                                                                                                                                                  |
| ---------------- | -------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `--spec`         | `./spec`                   | Path direktori YAML manifests                                                                                                                              |
| `--dsn`          | `sqlite:.formspec/data.db` | Database DSN — path SQLite **relative di-anchor ke project root dari `--spec`** (lihat catatan di bawah); absolute dipakai apa adanya                      |
| `--addr`         | `:8080`                    | REST API listen address                                                                                                                                    |
| `--listen`       | `none`                     | Mode ctx listener (lihat §5)                                                                                                                               |
| `--app-endpoint` | `none`                     | Mode app endpoint (lihat §5)                                                                                                                               |
| `--runtime`      | auto-detect                | Runtime app process                                                                                                                                        |
| `--dev`          | `false`                    | Dev mode (hot-reload, Vite proxy, dsb.) — auth tetap JWT asli, seragam dengan prod                                                                         |
| `--dev-ui`       | `false`                    | Dev mode + Vite HMR (implied `--dev`)                                                                                                                      |
| `--jwt-secret`   | auto                       | HMAC secret untuk JWT signing. Kosong di dev → auto-generate + persist ke `.formspec/dev-jwt-secret` (sesi bertahan antar restart); kosong di prod → error |
| `--state-dir`    | `.formspec`                | State directory (auto-create) — default ikut lokasi db hasil anchor DSN                                                                                    |
| `--web-dir`      | auto-detect                | Override SPA directory                                                                                                                                     |
| `--workspace-id` | `default`                  | Workspace/tenant ID                                                                                                                                        |

### Resolusi path DSN (relative di-anchor ke lokasi spec)

Path SQLite pada `--dsn` yang **relative** di-anchor ke project root yang
di-derive dari lokasi `--spec` (konvensi: spec tinggal di `<root>/spec`),
bukan ke working directory. Dengan begitu file database **statis** —
dijalankan dari mana pun, db yang sama:

```bash
# Dari root repo ATAU dari dalam folder example — hasilnya sama:
# <project>/examples/Clinic-UI-Showcase/.formspec/clinic.db
formspec dev --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.formspec/clinic.db"
```

- DSN **absolute** dipakai apa adanya: `sqlite:///abs/path/clinic.db`
- DSN **postgres** tidak diubah
- Query param SQLite (`?_pragma=…`) dipertahankan
- `--state-dir` default mengikuti folder db hasil anchor (set `--state-dir`
  eksplisit untuk meng-override)

---

## 4. Runtime Auto-Detect

`formspec dev` mendeteksi runtime dari project files di CWD:

| File                                  | Runtime  | Keterangan                                   |
| ------------------------------------- | -------- | -------------------------------------------- |
| `go.mod`                              | `local`  | Go — gunakan `go run .` untuk server sendiri |
| `composer.json`                       | `php`    | Sidecar spawn `app.php`                      |
| `package.json`                        | `node`   | Sidecar spawn `app.js`                       |
| `pyproject.toml` / `requirements.txt` | `python` | Sidecar spawn `app.py`                       |
| `*.csproj` / `*.sln`                  | `local`  | .NET SDK belum tersedia                      |
| (none)                                | `local`  | API-only, tanpa app process                  |

Override dengan `--runtime` eksplisit:

```bash
formspec dev --runtime php    # paksa PHP, meski tidak terdeteksi
formspec dev --runtime local  # paksa single-process
```

---

## 5. Mode listen & app-endpoint

`--listen` dan `--app-endpoint` memiliki 3 mode:

| Mode          | Arti                                                  | Kapan Digunakan                          |
| ------------- | ----------------------------------------------------- | ---------------------------------------- |
| `none`        | **Default.** Tidak ada ctx listener atau app endpoint | Single process, tanpa app process        |
| `local_http`  | TCP localhost (`:9090` / `:9091`)                     | Dev dengan app process (PHP/Python/Node) |
| `unix_socket` | Unix socket (`/tmp/formspec/...`)                     | Production di K8s pod                    |

Backward compatibility: `--listen "http://127.0.0.1:9090"` auto-detect sebagai
`local_http`. `--listen "unix:///tmp/formspec/sidecar.sock"` auto-detect
sebagai `unix_socket`.

### Example dengan app process

```bash
formspec dev --listen local_http --app-endpoint local_http --runtime php
```

---

## 6. SPA Serving Priority

`formspec dev` mencari SPA dengan prioritas:

1. **`--web-dir` eksplisit** — serve dari folder yang ditentukan
2. **`//go:embed`** — SPA embedded di binary (release build)
3. **Auto-detect** — cari `renderers/react-shadcn/dist/index.html`, `./dist/index.html`, `./index.html`
4. **Tidak ditemukan** — API-only, warning "SPA not found"

---

## 7. Config File (`formspec-app.yaml`)

Jika `formspec dev` dijalankan tanpa flag, ia mencari `./formspec-app.yaml`
(atau `./formspec-sidecar.yaml` untuk backward compatibility).

Format:

```yaml
spec: ./spec
dsn: sqlite:.formspec/data.db
addr: :8080
listen: none # none | local_http | unix_socket
app-endpoint: none # none | local_http | unix_socket
listen-url: "" # custom URL, override listen
app-endpoint-url: ""
workspace-id: default
runtime: auto # auto | local | php | python | node
state-dir: .formspec
dev: false
dev-ui: false
jwt-secret: "" # kosong di dev → auto-generate + persist ke .formspec/dev-jwt-secret
force: false
web-dir: ""
```

Prioritas (low → high): Default code → Config file → CLI flags.

---

## 8. Contoh Lengkap

### Go developer — embed FormSpec

```go
import formspec "github.com/primadi/formspec/resource"

func main() {
    app, _ := formspec.New(formspec.Config{
        SpecPath: "./spec",
        DSN:      "sqlite:data.db",
    })
    app.ListenAndServe()
}
```

Jalankan dengan `go run .` — tidak perlu `formspec dev`.

### Go developer — quick prototyping

```bash
go run github.com/primadi/formspec/cmd/formspec@latest dev --spec ./spec
```

### PHP developer

```bash
# Binary prebuilt — tanpa Go. Lihat docs/guides/install.md untuk semua platform.
curl -fsSL https://formspec.dev/install.sh | sh
formspec dev
# Auto-detect composer.json → spawn PHP
```

### Frontend specialist

```bash
git clone ... formspec
cd formspec/web && npm install && npm run dev
# Terminal 2:
cd .. && go run ./cmd/formspec/ dev --spec ./my-app/spec
```

---

## 9. Arsitektur

```
┌─────────────────────────────────────────────────┐
│                 formspec dev                       │
│  ┌──────────────┐  ┌──────────────────────────┐  │
│  │ REST API     │  │ Ctx Listener             │  │
│  │ (:8080)      │  │ (opsional, default none) │  │
│  │              │  │                          │  │
│  │ Entity engine│  │ ctx.db, ctx.cache,       │  │
│  │ CRUD, Meta   │  │ ctx.lock, dll.           │  │
│  │ SPA (embed)  │  └──────────┬───────────────┘  │
│  └──────────────┘             │                   │
└───────────────────────────────┼───────────────────┘
                                │
                    ┌───────────▼───────────┐
                    │   App Process         │
                    │   (opsional)          │
                    │   PHP/Python/Node     │
                    └───────────────────────┘
```
