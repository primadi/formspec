# 2026-08-25-002 — Fase 7 subset: delivery hooks + verifikasi state machine/hooks (7.8.5, 7.5, 7.8)

## Apa yang diubah

**7.8.5 before_deliver/after_deliver hooks:**

- `internal/action/hooks.go` — `SelectEventHooks` (match `h.Event`, priority order) — pasangan
  event-side dari `SelectHooks`.
- `internal/action/deliver.go` — `runBeforeDeliver` (fail → suppress delivery; ok(data) →
  enrich payload) + `runAfterDeliver` (best-effort, tidak gagalkan action) di `DeliverEvents`.
- `DeliveryDeps` + `Dispatcher` + `Hooks`; `HandlerFactory.deliveryDepsFor(module, entity)`
  me-wire dispatcher + entity hooks ke semua call site `DeliverEvents`.
- Test: `deliver_test.go` (suppress, enrich, after).

**Verifikasi & tandai item yang sudah terimplementasi:**

- **7.5.1–7.5.3** state machine engine (transition validation → `STATE_TRANSITION_ERROR`,
  Starlark inline guards, `sum_line` builtin) — `internal/entity/state_machine.go`.
- **7.8.1–7.8.4, 7.8.6, 7.8.7** hook engine (before/after/on_error + priority ordering +
  cross-module uses inheritance) — `internal/action/hooks.go`.

## File terdampak

- `internal/action/hooks.go`, `internal/action/deliver.go`, `internal/action/deliver_test.go`
- `internal/api/handler.go` (deliveryDepsFor)

## Referensi

- Plan: `docs/plan/fase7-subset.md` (WS-4, WS-5)
- Todo: `docs/plan/todo.md` §7.5, §7.8
