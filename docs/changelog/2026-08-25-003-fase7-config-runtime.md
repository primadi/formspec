# Fase 7 — Config Runtime (7.2.1, 7.2.2) + ctx.config/ctx.secrets wiring

**Tanggal:** 2026-08-25 · **Plan:** `docs/plan/fase7-config-runtime.md` · **Todo:** §7.2.1, §7.2.2

## Apa yang diubah

Config manifests (`kind: Config`) kini di-resolve ke runtime script. Sebelumnya
`ctx.config.get("key")` di script hanya diisi map kosong (CLI repl/migrate) dan
`ctx.secrets` (todo 6.8.1) didefinisikan tapi tidak pernah di-wire dari resource
layer.

- **`internal/config/registry.go`** (baru) — `Registry` yang me-resolve semua
  Config manifest menjadi dua store flat: `NonSecret()` (untuk `ctx.config`) dan
  `Secrets()` (untuk `ctx.secrets`, gated `uses.secrets`). `resolveValue` meng-koersi
  `default` ke tipe sesuai `ConfigKey.Type` (`int|string|bool|decimal|json`).
  Single-server tidak punya resolusi per-environment (Control Plane) → nilai =
  default yang dideklarasikan (spec §10).
- **`internal/starlark/executor.go`** — `SetConfigStore` (sudah ada dari batch
  sebelumnya) kini benar-benar di-wire: `Execute` set `ctxObj.Config =
  NewConfigAPI(e.ConfigStore)`.
- **`internal/action/script.go`** — passthrough `SetConfigStore`/`SetSecretsStore`
  ke engine starlark.
- **`resource/formspec.go`** — `buildConfigRegistry(specManifests.Manifests)`
  membangun `*config.Registry` dari Config manifests; diteruskan ke
  `newDispatcher` yang memanggil `scriptEx.SetConfigStore(reg.NonSecret())` +
  `scriptEx.SetSecretsStore(reg.Secrets())`. Diterapkan di **kedua** call-site
  (boot + `ReloadSpec`), jadi perubahan Config manifest berlaku tanpa restart.
- **`internal/config/registry_test.go`** (baru) — test NonSecret/Secrets
  pemisahan, koersi int dari string, dan registry kosong.

## File terdampak

- `internal/config/registry.go` (baru)
- `internal/config/registry_test.go` (baru)
- `internal/starlark/executor.go`
- `internal/action/script.go`
- `resource/formspec.go`

## Status

`go test ./...` hijau (21 paket, 0 fail). Todo 7.2.1 (Config registry) + 7.2.2
(`ctx.config.get`) ditandai ✅. 7.2.3/7.2.4 (`settings.*` + defaults) sudah selesai
sebelumnya (changelog 2026-08-24-008..014) — diverifikasi. `resource.new()` (7.14.4)
dan `<Entity>.query()` tetap di-defer (butuh scope builder query §16).
