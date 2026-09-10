# 2026-09-10-002 — Pindah `formspec-app.yaml` ke `registry/` + script dev registry

## Apa yang diubah

`formspec-app.yaml` di root repo pindah ke `registry/formspec-app.yaml`
(git mv). File itu memang khusus untuk menjalankan FormSpec Module Registry
(`spec: registry/spec`, `dsn: sqlite:.formspec/registry.db`), bukan config
umum root repo. Root kini bersih: config project hanya ada di folder
project masing-masing (`examples/*/formspec-app.yaml`, registry/, dll).

## Perubahan terkait

- `cmd/formspec-registry/main.go` — flag baru `--config` untuk membaca
  config file format `formspec-app.yaml` (subset: `spec`, `dsn`, `addr`,
  `jwt-secret`, `web-dir`). Tanpa `--config`, auto-discover
  `formspec-app.yaml` di CWD (legacy `formspec-sidecar.yaml` ikut
  didukung). Flag CLI selalu menang atas config file.
- `scripts/run-registry.sh` (baru, executable) — menjalankan
  `go run ./cmd/formspec-registry --config registry/formspec-app.yaml`
  dari root repo; opsi CLI bisa diteruskan (mis. `--addr :8081`).
- `Makefile` — target baru `make registry-dev` yang memanggil script itu.

## Kenapa

Sebelumnya binary `formspec-registry` tidak membaca config file sama
sekali (semua via flag), sehingga yaml root hanya terpakai oleh
`formspec dev` dari root. Kini config mengikuti programnya: satu tempat
(`registry/`) berisi spec, web, dan config dev registry.

## File terkena dampak

- `formspec-app.yaml` → `registry/formspec-app.yaml` (pindah + update komentar)
- `cmd/formspec-registry/main.go`
- `scripts/run-registry.sh` (baru)
- `Makefile`

## Referensi

- `docs/spec/platform/08-project-layout.md` §1.1 — status config file
- `docs/registry/05-self-hosting.md` — dev vs prod run registry
