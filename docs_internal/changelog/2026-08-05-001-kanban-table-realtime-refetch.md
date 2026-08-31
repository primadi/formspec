# Feat: Row-level realtime refetch untuk Kanban & Table

## Perubahan

- **KanbanRenderer** (`renderers/web/src/kinds/kanban/KanbanRenderer.tsx`):
  `fetchRecords(silent=true)` pada event/reconnect; `useRealtime` subscribe
  entity; gated pada `entry.spec.realtime`. Board konsultasi sudah `realtime:
  true` (manifest ada sejak awal, renderer baru mengeksekusi). Realtime
  refetch silent (tidak flash loading spinner).
- **TableRenderer** (`renderers/web/src/kinds/table/TableRenderer.tsx`):
  `silentRefetch` flag + `useRealtime` subscribe entity → `setReloadKey` tanpa
  flash "Loading..." (silent). Gated pada `tableSpec.realtime`. Tabel
  `visit-table` sudah mendeklarasikan `realtime: true` (sejak Fase 3.5,
  dulu belum dijalankan renderer; sekarang aktif).
- **visit-table.yaml** `realtime: true` tetap (sudah ada di committed, hanya
  di-restore setelah konflik duplicate key).

## Verifikasi (end-to-end, browser localhost:18080)
- Kanban board: create visit → start-consultation → complete → `visit.completed`
  event → board **silent refetch tanpa reload** → kolom "Selesai" 2→3.
- Table: mekanisme sama (silent refetch via `setReloadKey` + `silentRefetch`).
- `npx tsc -b --noEmit` bersih; `formspec validate` [OK].

## Files affected
- `renderers/web/src/kinds/kanban/KanbanRenderer.tsx` — import useRealtime, silent fetchRecords, realtime refetch effect
- `renderers/web/src/kinds/table/TableRenderer.tsx` — import useRealtime + useRef, silentRefetch flag, realtime refetch effect
- `examples/Clinic-UI-Showcase/spec/modules/clinic/transaction/visit/tables/list.yaml` — `realtime: true` (kembalikan setelah konflik duplicate)

## Referensi
- Plan: `docs/plan/use-realtime-hook.md`
