# Fase 7 — Service Runtime (7.1.1–7.1.4)

**Tanggal:** 2026-08-25 · **Plan:** `docs/plan/fase7-service-runtime.md` · **Todo:** §7.1

## Apa yang diubah

`kind: Service` (komputasi stateless murni) kini punya runtime penuh.
Sebelumnya `ServiceSpec` sudah di-load oleh manifest loader tapi tidak ada
registry, dispatch, atau exposure via API.

- **`internal/service/registry.go`** (baru) — `Registry` memetakan
  `{module}.{name}` → `ServiceSpec`; `Add`/`Get`/`GetAction`/`List`.
- **`resource/formspec.go`** — `buildServiceRegistry(specManifests.Manifests)`
  membangun registry dari kind: Service manifests; di-wire ke `newDispatcher`
  dan router (boot + reload). `resource.call("module.service", "action", ...)`
  dari script kini resolve Service dulu, fallback ke entity.
  `invokeServiceAction` dispatch action Service via dispatcher yang sama
  (impl native/script/script_ref/compiled/sidecar + permission/uses uniform).
- **`internal/api/generator.go`** — `GenerateServiceRoutes` menghasilkan route
  `POST /api/v1/{module}/{service}/{action}` untuk action ber-impl;
  permission default `{module}.{service}.{action}`.
- **`internal/api/handler.go`** — `HandleServiceAction` (parse params, validasi,
  kondisi, dispatch; `call: async` → 202 Accepted fire-and-forget).
- **`internal/api/router.go`** — `SetServiceRegistry` + `case "service"` di
  `registerRoute`.
- **Test**: `internal/service/registry_test.go` (Add/Get/GetAction/List) +
  `internal/api/service_test.go` (HTTP dispatch native + `call: async` 202).

## File terdampak

- `internal/service/registry.go` (baru) + `registry_test.go` (baru)
- `internal/api/service_test.go` (baru)
- `internal/api/generator.go`, `internal/api/handler.go`, `internal/api/router.go`
- `resource/formspec.go`

## Status

`go test ./...` hijau (0 fail). Todo 7.1.1 (registry), 7.1.2 (impl.native),
7.1.3 (impl.script/script_ref/compiled/sidecar — via dispatcher seragam),
7.1.4 (`call: async` fire-and-forget) ditandai ✅. Catatan: `impl.native`
"scan `impl/**/*.go`" otomatis belum ada — native handler tetap didaftarkan
eksplisit via `RegisterNative` (sama seperti entity action); auto-scan adalah
enhancement.
