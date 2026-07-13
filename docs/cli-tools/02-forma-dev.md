# `forma dev` — Development Server

**Version:** 1.0
**Status:** Draft

> `forma dev` adalah satu-satunya perintah untuk menjalankan Forma development
> server. Backend (Go entity engine) dan frontend (SPA React) berjalan dalam
> satu proses. Tidak perlu Vite, npm, atau build frontend terpisah — cukup
> `forma dev`.

---

## 1. Filosofi

Forma mengenal **dua persona** developer:

| Persona | Kebutuhan | Command |
|---|---|---|
| **A** (80%) | UI jadi, tidak edit frontend | `forma dev` |
| **B** (20%) | Edit renderer/komponen React | `forma dev --dev-ui` |

Persona A cukup satu perintah — SPA sudah embedded di binary (`//go:embed`).
Persona B mendapat Vite HMR untuk hot-reload frontend.

---

## 2. Quick Start

### Persona A — SPA built-in

```bash
forma dev --spec ./my-app/spec
```

- Backend API di `:8080`
- SPA tersedia di `http://localhost:8080/default/_admin`
- Tidak perlu npm, Vite, atau build frontend

### Persona B — Vite HMR

```bash
forma dev --spec ./my-app/spec --dev-ui
```

- Backend API di `:8080`
- Vite HMR di `:5173`
- Edit `web/src/` → perubahan langsung kelihatan
- `--dev-ui` implied `--dev` + `--force`

### Dengan config file

Buat `forma-sidecar.yaml` di folder project:

```yaml
spec: ./my-app/spec
dsn: sqlite:.forma/data.db
dev-ui: true
```

Lalu cukup:

```bash
forma dev
```

---

## 3. Flag Reference

| Flag | Default | Deskripsi |
|---|---|---|
| `--spec` | `./spec` | Path direktori YAML manifests |
| `--dsn` | `sqlite:.forma/data.db` | Database DSN |
| `--addr` | `:8080` | REST API listen address |
| `--listen` | `none` | Mode ctx listener (lihat §5) |
| `--app-endpoint` | `none` | Mode app endpoint (lihat §5) |
| `--runtime` | auto-detect | Runtime app process |
| `--dev` | `false` | Dev mode (auth bypass) |
| `--dev-ui` | `false` | Dev mode + Vite HMR (implied `--dev`) |
| `--state-dir` | `.forma` | State directory (auto-create) |
| `--web-dir` | auto-detect | Override SPA directory |
| `--workspace-id` | `default` | Workspace/tenant ID |

---

## 4. Runtime Auto-Detect

`forma dev` mendeteksi runtime dari project files di CWD:

| File | Runtime | Keterangan |
|---|---|---|
| `go.mod` | `local` | Go — gunakan `go run .` untuk server sendiri |
| `composer.json` | `php` | Sidecar spawn `app.php` |
| `package.json` | `node` | Sidecar spawn `app.js` |
| `pyproject.toml` / `requirements.txt` | `python` | Sidecar spawn `app.py` |
| `*.csproj` / `*.sln` | `local` | .NET SDK belum tersedia |
| (none) | `local` | API-only, tanpa app process |

Override dengan `--runtime` eksplisit:

```bash
forma dev --runtime php    # paksa PHP, meski tidak terdeteksi
forma dev --runtime local  # paksa single-process
```

---

## 5. Mode listen & app-endpoint

`--listen` dan `--app-endpoint` memiliki 3 mode:

| Mode | Arti | Kapan Digunakan |
|---|---|---|
| `none` | **Default.** Tidak ada ctx listener atau app endpoint | Single process, tanpa app process |
| `local_http` | TCP localhost (`:9090` / `:9091`) | Dev dengan app process (PHP/Python/Node) |
| `unix_socket` | Unix socket (`/tmp/forma/...`) | Production di K8s pod |

Backward compatibility: `--listen "http://127.0.0.1:9090"` auto-detect sebagai
`local_http`. `--listen "unix:///tmp/forma/sidecar.sock"` auto-detect
sebagai `unix_socket`.

### Example dengan app process

```bash
forma dev --listen local_http --app-endpoint local_http --runtime php
```

---

## 6. SPA Serving Priority

`forma dev` mencari SPA dengan prioritas:

1. **`--web-dir` eksplisit** — serve dari folder yang ditentukan
2. **`//go:embed`** — SPA embedded di binary (release build)
3. **Auto-detect** — cari `web/dist/index.html`, `./dist/index.html`, `./index.html`
4. **Tidak ditemukan** — API-only, warning "SPA not found"

---

## 7. Config File (`forma-sidecar.yaml`)

Jika `forma dev` dijalankan tanpa flag, ia mencari `./forma-sidecar.yaml`
atau `./forma-sidecar.yml` untuk konfigurasi.

Format:

```yaml
spec: ./spec
dsn: sqlite:.forma/data.db
addr: :8080
listen: none            # none | local_http | unix_socket
app-endpoint: none       # none | local_http | unix_socket
listen-url: ""           # custom URL, override listen
app-endpoint-url: ""
workspace-id: default
runtime: auto            # auto | local | php | python | node
state-dir: .forma
dev: false
force: false
web-dir: ""
dev-ui: false
```

Prioritas (low → high): Default code → Config file → CLI flags.

---

## 8. Contoh Lengkap

### Go developer — embed Forma

```go
import forma "github.com/primadi/forma/resource"

func main() {
    app, _ := forma.New(forma.Config{
        SpecPath: "./spec",
        DSN:      "sqlite:data.db",
    })
    app.ListenAndServe()
}
```

Jalankan dengan `go run .` — tidak perlu `forma dev`.

### Go developer — quick prototyping

```bash
go run github.com/primadi/forma/cmd/forma@latest dev --spec ./spec
```

### PHP developer

```bash
# Download binary
wget .../forma-linux-amd64.tar.gz
./forma dev
# Auto-detect composer.json → spawn PHP
```

### Frontend specialist

```bash
git clone ... forma
cd forma/web && npm install && npm run dev
# Terminal 2:
cd .. && go run ./cmd/forma/ dev --spec ./my-app/spec
```

---

## 9. Arsitektur

```
┌─────────────────────────────────────────────────┐
│                 forma dev                       │
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
