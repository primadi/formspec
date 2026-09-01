# 2026-09-01-002 — Render context: source api via UI-surface service routes

## Apa

Menyelesaikan `source: api` pada render-context standard (plan
`render-context-standard.md` Phase 2) — sebelumnya deferred karena service
actions hanya di-route di surface `/api/v1` (deny-by-default).

- `internal/api/generator.go`: `GenerateUIServiceRoutes` — route
  `/_ui/service/{module}/{service}/{action}` (POST) untuk semua service
  action ber-impl; permission default `{module}.{service}.{action}`.
- `internal/api/router.go`: merge `uiSvcRoutes` di `BuildRoutes`; grup
  `/_ui/service` di `BuildHTTP`; `registerRouteWithPattern` kini handle
  `Handler: "service"` (sebelumnya hanya auto/custom/prepare → route tak
  pernah ter-register, 404).
- `renderers/react-shadcn/src/hooks/useRenderContext.ts`: `source: api`
  memanggil `client.post("../service/{module}/{service}/{action}")`
  (prefix `/{ws}/_ui/entity` menormalisasi ke `/_ui/service/...`) dengan
  permission ceiling `can(call)`; params dari `decl.params`.
- `registry/spec/modules/portal/pages/profile.yaml`: demo `api` source
  (`registry.signature-verify.verify`) — untuk user tanpa grant fallback.

## Kenapa

Melengkapi closed source set context (`session | entity | api | const |
expr`). UI-surface service route dibutuhkan karena `/api/v1` deny-by-default
untuk programmatic client; session-authenticated caller di UI surface kini
bisa invoke service untuk context render.

## Verifikasi

- `go build ./...` bersih; `go test ./internal/api` 120 pass.
- `tsc --noEmit` bersih; `vitest run` 166 pass.
- `formspec validate` — 13 manifest, 0 problem.
- curl: testuser (tanpa permission) → 403; service tak ada → 404; admin
  (permission `*`) → handler tervalidasi (VALIDATION_ERROR params kosong).
- Browser: profile page render normal dengan `api` source fallback.

## File terdampak

- `internal/api/generator.go`, `internal/api/router.go`
- `renderers/react-shadcn/src/hooks/useRenderContext.ts`
- `registry/spec/modules/portal/pages/profile.yaml`
- `docs_internal/plan/render-context-standard.md` (api source ✅)

## Lanjutan

Phase 3 (plan): reaktivitas (`source: entity` + realtime).
