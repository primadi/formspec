# 2026-09-01-003 — Render context Phase 3: reaktivitas (source entity + realtime)

## Apa

Phase 3 dari plan `docs_internal/plan/render-context-standard.md`:
reaktivitas untuk `source: entity` — context auto-update saat record
berubah (dashboard live-update).

- `pkg/spec/frontend.go` + `manifest.ts`: flag `Realtime`/`realtime` di
  `ContextDecl`.
- `renderers/react-shadcn/src/hooks/useRenderContext.ts`: untuk entity decl
  dengan `realtime: true`, subscribe via `subscribeRealtime(entity)` dan
  re-resolve entry saat event/reconnect (realtime non-durable → refetch
  wajib, sesuai kontrak `useRealtime`). Cleanup unsubscribe.
- `registry/spec/modules/portal/pages/profile.yaml`: demo `module_status`
  (entity `registry.module`, `realtime: true`, fallback "—").

## Kenapa

Melengkapi render-context standard: context tidak hanya statis — entity
context bisa hidup (auto-refresh) memakai infrastruktur WebSocket yang
sudah ada (`useRealtime`/`subscribeRealtime`), tanpa custom code.

## Verifikasi

- `go build ./...` bersih; `go test ./pkg/spec` 43 pass.
- `tsc --noEmit` bersih; `vitest run` 166 pass.
- `formspec validate` — 13 manifest, 0 problem.
- Browser: profile page render normal dengan realtime entity context
  (fallback untuk user tanpa grant).

## Catatan

Demo live-update penuh butuh user ber-permission + producer event (mis.
update record module). Mekanisme subscribe+refetch ter-wire dan teruji
via type-check + test suite.

## File terdampak

- `pkg/spec/frontend.go`, `renderers/react-shadcn/src/types/manifest.ts`
- `renderers/react-shadcn/src/hooks/useRenderContext.ts`
- `registry/spec/modules/portal/pages/profile.yaml`
- `schemas/`, `docs/kind/` (regenerated)
- `docs_internal/plan/render-context-standard.md` (Phase 3 ✅)

## Status

Render-context standard COMPLETE: Phase 1 (user slot + interpolasi) ·
Phase 2 (spec.context, closed source set lengkap) · Phase 3 (reaktivitas).
