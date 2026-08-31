# 2026-08-31-004 — Embed SPA ke formspec-registry (fallback --web-dir)

## Apa

`formspec-registry` kini menyajikan SPA renderer dari **embed** ketika
`--web-dir` tidak diberikan:

- Package baru `registry/web` (`registry/web/embed.go`): `//go:embed all:dist`
  - `DistFS()` yang mengembalikan `fs.FS` rooted di dist (index.html di root).
- `cmd/formspec-registry/main.go`: jika `--web-dir` kosong → `cfg.WebFS =
web.DistFS()`; jika diisi → `cfg.WebDir` (prioritas tetap di flag).
- `Makefile`: target baru `build-registry` (sync
  `renderers/react-shadcn/dist` → `registry/web/dist` lalu `go build`),
  ditambahkan ke `build`. Pola sama dengan `build-formspec` →
  `cmd/formspec/dist`.

## Kenapa

Sebelumnya, registry tanpa `--web-dir` tidak me-mount SPA sama sekali —
`GET /{ws}` (mis. `/default`) jatuh ke workspace 404 handler
(`internal/api/router.go`) dan mengembalikan JSON
`{"code":"NOT_FOUND","message":"endpoint not found"}`. Dengan embed, binary
registry jadi single-file deployment penuh: spec (`registry/embed.go`) +
SPA (`registry/web`) tanpa flag tambahan.

## File terdampak

- `registry/web/embed.go` (baru)
- `registry/web/dist/` (baru — hasil sync, di-commit mengikuti konvensi
  `cmd/formspec/dist`)
- `cmd/formspec-registry/main.go`
- `Makefile`

## Verifikasi

`go build` + smoke test: `go run ./cmd/formspec-registry --addr :18099` →
`GET /default` = 200 dengan index.html SPA.

## Referensi

- todo 13.5.6 (native registry binary), 13.3.3 (native handlers)
- `docs_internal/plan/` — infra-registry fase D
