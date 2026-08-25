# Fase 7 — Satukan dua implementasi state machine guard (7.5.4)

**Tanggal:** 2026-08-25 · **Todo:** §7.5.4

## Apa yang diubah

Sebelumnya ada **dua implementasi evaluasi guard state machine** yang
terduplikasi dan tidak identik:

1. `internal/entity/state_machine.go` — `StateMachineEngine.evaluateGuard`
   (dipakai `HandleCustomAction` via `CanTransition`), yang menyuntik helper
   `sum_line_*` / `item_count` / `line_count`.
2. `renderers/jsonb-persist/crud.go` — `EntityStore.validateStateTransition`
   (dipakai saat `Update`), yang **tidak** menyuntik helper `sum_line_*` /
   `item_count` / `line_count` — jadi guard GL-style (`sum_line('debit') ==
   sum_line('credit')`) tidak berfungsi saat transisi lewat Update.

Keduanya juga membangun env dengan cara berbeda (satu pakai `resourceData`
saja, satu pakai `combined` old+new).

**Fix:** ekstrak evaluasi guard ke satu helper bersama
`internal/starlark.EvaluateGuard` (`internal/starlark/guard.go`) yang
menyuntik helper `sum_line_*`/`item_count`/`line_count` secara konsisten.
Kedua call-site kini memanggil helper yang sama:

- `state_machine.go` `evaluateGuard` → delegasi ke `starlark.EvaluateGuard`.
- `crud.go` `validateStateTransition` → delegasi ke `starlark.EvaluateGuard`
  dengan `combined` (old+new) sebagai resource data, sehingga guard melihat
  nilai state baru — perilaku lama dipertahankan, tapi kini dapat helper yang
  sama.

Catatan: `HandleCustomAction` **sudah** memanggil `StateMachineEngine.CanTransition`
sebelumnya (bagian "tidak dipanggil" dari todo sudah terpenuhi); yang tersisa
adalah duplikasi evaluasi guard, yang kini disatukan.

## File terdampak

- `internal/starlark/guard.go` (baru) — `EvaluateGuard` + `computeSums` shared.
- `internal/entity/state_machine.go` — `evaluateGuard` delegasi ke shared helper.
- `renderers/jsonb-persist/crud.go` — `validateStateTransition` delegasi ke
  shared helper.

## Status

`go test ./...` hijau (0 fail). Todo 7.5.4 ditandai ✅.
