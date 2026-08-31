# 2026-08-24-028 — Calendar Kind End-to-End (5.6.1–5.6.6)

## Apa yang diubah

Implementasi `kind: Calendar` end-to-end (06-page-kinds.md §5) — view kalender atas entity
dengan field tanggal/datetime.

**Backend:**

- `pkg/spec/spec.go` — `KindCalendar` + `IsValidKind`.
- `internal/manifest/loader.go` — `KnownKinds` + `Calendar`.
- `internal/ui/registry.go` — `Calendars` map + register case + `ResolveViewRoute`.
- `internal/ui/meta.go` — `Bundle.Calendars` + build (entity-gated).
- `internal/ui/validate.go` — validasi date_field/end_field/title_field/resource_field/
  color_field terhadap entity.
- `internal/ui/ui_test.go` — `TestCalendarApprovalNotificationKinds`.

**Frontend:**

- `types/manifest.ts` — `CalendarSpec` + `MetaBundle.calendars`.
- `shell/router.tsx` — route `/calendar/{name}`.
- `kinds/calendar/CalendarRenderer.tsx` — views month/week/day/resource; event dari
  `date_field` + `end_field`, title dari `title_field`/`label_field`; klik event → detail;
  klik slot kosong → Form create date pre-filled (`prefill.{date_field}`); drag reschedule →
  PATCH date_field (+ end_field proporsional), submitted rows ditolak; **RRULE (RFC 5545)**
  via library `rrule` (npm) — expand render-time, bukan materialized; resource view satu
  lajur per `resource_field` + warna dari `color_field`.

  5.6.7 (RRULE exception per-instance) tetap deferred per spec.

## File terdampak

- `pkg/spec/spec.go`, `internal/manifest/loader.go`, `internal/ui/{registry,meta,validate}.go`
- `renderers/react-shadcn/src/kinds/calendar/CalendarRenderer.tsx`
- `renderers/react-shadcn/src/types/manifest.ts`, `shell/router.tsx`
- `renderers/react-shadcn/package.json` (+ `rrule`)

## Referensi

- Plan: `docs/plan/fase5-completion.md` (WS-D)
- Todo: `docs/plan/todo.md` §5.6.1–5.6.6
