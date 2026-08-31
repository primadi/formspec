# Fase 7 — Config Runtime (7.2) + Script Runtime Contract (7.14.4)

**Status:** ✅ WS-1 selesai (Config registry + ctx.config/ctx.secrets wiring) · WS-2 (`resource.new()`) & WS-3 deferred · **Tanggal:** 2026-08-25
**Referensi:** `docs/spec/backend/01-core-basic.md` §10 (Config & Global Settings),
`docs/spec/backend/06-script-runtime.md` §2/§4/§6 (script runtime contract)
**Todo:** `docs/plan/todo.md` §7.2, §7.14.4

## Konteks

- Config manifests (`kind: Config`) sudah di-load ke `configs map[string]*spec.ConfigSpec`
  di `resource/formspec.go`, tapi hanya dipakai untuk (a) resolusi auth strategy dan
  (b) resolusi global settings `settings:`. **`ctx.config.get("key")` di script TIDAK
  di-wire** ke Config manifests — `ctx.config` hanya diisi map kosong di CLI repl/migrate.
- `ctx.secrets` (todo 6.8.1) juga belum di-wire dari resource layer (`SetSecretsStore`
  didefinisikan tapi tidak pernah dipanggil).
- Global settings namespace `settings.*` (7.2.3) + defaults (7.2.4) SUDAH selesai via
  `Settings`/`DefaultSettings`/`ResolveSettings` di `pkg/spec` (changelog 2026-08-24-008..014).
- Script runtime contract (7.14.4): `resource.field`/`set`/`save`/`call`/`fetch`/`create`,
  `ok()`/`fail()` sudah ada. Gap: `resource.new()` (handle baru entity yang sama, spec §2)
  belum ada; `<Entity>.query()` (builder, spec §3 / 02-core-extended §16) belum ada.

## Scope batch ini

### WS-1 — Config registry + wiring `ctx.config` (7.2.1, 7.2.2)

1. **`internal/config/registry.go`** — `Registry` baru:
   - `Add(name, *spec.ConfigSpec)` — daftarkan Config manifest.
   - `NonSecret() map[string]any` — resolve semua key non-secret → flat map untuk `ctx.config`.
   - `Secrets() map[string]string` — resolve semua key `secret: true` → flat map untuk `ctx.secrets`.
   - `resolveValue(ConfigKey)` — koersi `Default` ke tipe sesuai `ConfigKey.Type`
     (`int|string|bool|decimal|json`). Single-server tidak punya resolusi per-environment
     (Control Plane) → nilai = default yang dideklarasikan (spec §10: "spec wajib menetapkan
     default standar").
2. **`internal/starlark/executor.go`** — tambah `ConfigStore map[string]any` +
   `SetConfigStore`; di `Execute`, set `ctxObj.Config = NewConfigAPI(e.ConfigStore)`.
3. **`resource/formspec.go`** — bangun `*config.Registry` dari `specManifests` (Config
   manifests), teruskan ke `newDispatcher`, dan di dalamnya panggil
   `scriptEx.SetConfigStore(reg.NonSecret())` + `scriptEx.SetSecretsStore(reg.Secrets())`.
   Terapkan di kedua call-site (boot + reload).

### WS-2 — `resource.new()` (7.14.4)

- Tambah `resource.new()` di `internal/starlark/resource.go`: membuat handle record baru
  untuk entity yang SAMA (belum tersimpan), isi via `set(...)` lalu `save()`.
  `save()` pada handle baru → insert (bukan update). Perlu `createFn` untuk entity yang sama.

### WS-3 — Verifikasi & tandai

- 7.2.3 (`settings.*` namespace) & 7.2.4 (defaults) — sudah selesai, verifikasi + tandai ✅.
- 7.14.4 — tandai ✅ untuk bagian yang sudah ada + `resource.new()`; `<Entity>.query()`
  di-defer (butuh scope builder query §16 yang besar).

## Level of effort

| WS  | Effort |
| --- | ------ |
| 1   | medium |
| 2   | small  |
| 3   | small  |
