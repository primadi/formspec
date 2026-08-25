# Fase 7 — Service Runtime (7.1)

**Status:** ✅ WS-1..WS-4 selesai (registry, dispatch, API exposure, call:async) · **Tanggal:** 2026-08-25
**Referensi:** `docs/spec/backend/01-core-basic.md` §1.1 (Service = stateless),
`docs/spec/backend/02-core-extended.md` §5 (Service action `call: async`),
`docs/kind/data/Service.md`
**Todo:** `docs/plan/todo.md` §7.1

## Konteks

`kind: Service` adalah komputasi stateless murni — tidak ada state yang
dipersist, tidak ada `characteristic`/`doc_status`/lifecycle guard. Saat ini
`ServiceSpec` (`pkg/spec/resources.go`) sudah ada dan di-load oleh manifest
loader (`RawSpecToServiceSpec`), tapi **tidak ada runtime**: tidak ada registry
yang memetakan `{module}.{name}` → ServiceSpec, tidak ada cara dispatch action
Service, dan tidak ada exposure via API.

## Scope

### WS-1 — Service registry (7.1.1)

- `internal/service/registry.go` — `Registry` baru:
  - `Add(module, name, *spec.ServiceSpec)` — daftarkan Service manifest.
  - `Get(module, name) (*spec.ServiceSpec, bool)` — lookup.
  - `GetAction(module, name, actionName) (*spec.Action, bool)` — lookup action.
  - `List() []ServiceInfo` — ringkasan untuk meta/CLI.
- Wire ke `resource/formspec.go`: bangun registry dari `specManifests` (kind:
  Service), teruskan ke `newDispatcher` dan router.

### WS-2 — Dispatch Service action (7.1.2, 7.1.3)

- `invokeServiceAction(ctx, svcReg, disp, workspaceID, module, service, actionName, params)`:
  - Lookup action spec dari service registry.
  - Dispatch via `disp.Dispatch` (sama seperti entity action) — impl
    `native`/`script`/`script_ref`/`compiled`/`sidecar` semua sudah didukung
    oleh dispatcher.
  - Permission enforcement: `required_permission` di-check sebelum dispatch
    (sama seperti entity custom action).
- `resource.call("module.service", "action", params)` dari script → resolve
  service dulu, fallback ke entity.

### WS-3 — API exposure (7.1.2/7.1.3)

- Route `POST /api/v1/{module}/{service}/{action}` (dan `/_ui/entity/...`?)
  untuk Service action yang di-expose. Service tidak punya `id`/`plural` —
  path langsung ke action.
- Permission: `required_permission` (default `{module}.{service}.{action}`).

### WS-4 — `call: async` fire-and-forget (7.1.4)

- Action `call: async` → dispatch di goroutine terpisah, return `202 Accepted`
  tanpa body/job_id (beda dari async job §13 yang return `job_id`).

## Level of effort

| WS  | Effort |
| --- | ------ |
| 1   | small  |
| 2   | medium |
| 3   | medium |
| 4   | small  |
