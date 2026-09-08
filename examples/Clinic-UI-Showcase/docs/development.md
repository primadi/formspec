# Development — Clinic UI Showcase

Panduan singkat untuk bekerja dengan example ini. Panduan menjalankan yang
lengkap (Persona A/B, dua terminal, akses) ada di
[`how-to-run.md`](../how-to-run.md).

## Menjalankan

```bash
# Dari root repository — SPA built-in, tanpa npm
go run ./cmd/formspec/ dev \
  --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.formspec/clinic.db"

# Buka http://localhost:8080/default/_admin
```

Dengan config file (dari folder example):

```bash
cd examples/Clinic-UI-Showcase
go run ../../cmd/formspec/ dev          # pakai formspec-app.yaml
```

`formspec-app.yaml` juga mengaktifkan **Node.js sidecar** (`app/`) untuk
demo polyglot handler `otc-sale.sell` dan memuat theme dari
`../../ui-theme/*`.

### Dengan hot-reload frontend

Jika mengedit `renderers/react-shadcn/src/`:

```bash
go run ./cmd/formspec/ dev \
  --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.formspec/clinic.db" \
  --dev-ui     # Vite HMR di :5173
```

## Validasi Manifest

```bash
# Dari root repository
go run ./cmd/formspec/ validate --spec examples/Clinic-UI-Showcase/spec
```

Semua manifest harus lulus validasi schema (`schemas/`) dan validasi silang
(entity/field/action/route refs) sebelum di-commit.

## Testing

E2E test example ini (backend Go, entity + aksi + state machine):

```bash
go test ./examples/Clinic-UI-Showcase/...
```

| Test                                          | Cakupan                                             |
| --------------------------------------------- | --------------------------------------------------- |
| `clinic_e2e_test.go`                          | Alur klinik: kunjungan, antrian, kasir, rekam medis |
| `pharmacy_otc_sale_e2e_test.go`               | Penjualan OTC: guard stok, sell/cancel              |
| `pharmacy_prescription_scenarios_e2e_test.go` | Skenario resep: peracikan, dispense, cancel         |

Test frontend renderer (jika mengubah renderer):

```bash
cd renderers/react-shadcn && npm test   # vitest
```

## Reset Database

```bash
rm -f examples/Clinic-UI-Showcase/.formspec/clinic.db
```

> Path DSN SQLite relative di-anchor ke lokasi spec (project root), bukan
> working directory — dijalankan dari root repo maupun dari folder example,
> db-nya selalu `examples/Clinic-UI-Showcase/.formspec/clinic.db`.

## Verifikasi Meta API

App ini juga menjadi fixture end-to-end Meta API:

```bash
curl http://localhost:8080/default/_ui/_meta/ui     # bundle UI (ETag/304)
curl http://localhost:8080/default/_ui/_meta/me     # identity + permissions
```

## Tips Mengedit Spec

- **Tambah entity baru** — ikuti pola `spec/modules/<module>/<tier>/<entity>/entity.yaml`;
  kalau sengaja tanpa manifest UI, ia otomatis derived (D17).
- **Edit UI manifest** — setiap kind punya folder sendiri (`forms/`,
  `tables/`, `pages/`, dst.); cek matriks coverage di
  [`../README.md`](../README.md) agar fitur yang di-exercise tetap terwakili.
- **Script Starlark** — taruh di folder entity (`scripts/*.star`), refer via
  `impl: { type: script_ref, ref: <name> }`.
- **Permission** — selalu `required_permission` di aksi custom, jangan
  hardcode nama role.
- **Hot-reload** — `formspec dev` me-watch folder spec; perubahan YAML
  terlihat tanpa restart.

## Catatan

- Fokus example ini adalah **manifest UI**, bukan business logic — sebagian
  primitif `ctx.*` di script masih stub.
- Entity `daily-visit-summary` belum di-recompute otomatis (projection
  engine belum ada); widget dashboard membaca `clinic/visit` live.
