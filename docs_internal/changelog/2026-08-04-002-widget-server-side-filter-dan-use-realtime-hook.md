# Feat: widget query server-side filter + useRealtime hook (event-driven dashboard)

## Perubahan

1. **Widget `query` → server-side filter.** `DashboardRenderer.tsx` kini
   menerjemahkan `spec.query` (subset FormSpecExpr: `= today()`, `in [...]`,
   `!=`, `=`, `==`, compound `and`) menjadi list filters `field[op]=value`
   via `translateWidgetQuery()` + `buildListParams()`, sehingga DB pre-filter
   sebelum data lewat wire. `applySimpleQuery` (client-side) jadi fallback
   + final safety-net (idempotent). Widget showcase exclude `cancelled`:
   `visits-today` & `revenue-today` → `transaction_date = today() and
   status != 'cancelled'`; `visits-by-polyclinic` → `status != 'cancelled'`.
2. **Fix timezone `today()`.** Ditemukan saat verifikasi: wall-clock browser
   (WIB) bergeser setelah tengah malam → "Kunjungan Hari Ini" jadi 0 padahal
   data ada. `today()` kini = tanggal **server/business** (UTC, karena semua
   timestamp server RFC3339 UTC) via `serverToday()`.
3. **`useRealtime` hook.** `types/events.ts` (`RealtimeMessage`) +
   `hooks/useRealtime.ts`: koneksi WebSocket **singleton** ke
   `/{workspace}/_ui/_ws`, reconnect+backoff, filter resource/event lokal,
   `tick` naik tiap event + reconnect (non-durable). `DashboardRenderer`
   menurunkan `realtime` (dari `DashboardSpec.realtime`) ke
   `MetricWidget`/`ChartWidget`; setiap widget subscribe entity-nya dan
   refetch saat event cocok. Polling `refresh_secs` tetap backstop.
   `clinic-dashboard.yaml` → `realtime: true`.
4. **Backend auth WS (prod).** `AuthMiddleware` kini juga baca token dari
   `?token=` query param (browser tidak bisa set header pada WS handshake) +
   test `TestAuthMiddleware_TokenQueryParam`.
5. **Bug pre-existing yang ditemukan saat demo (blocking realtime):**
   - Guard `complete` pakai `!empty(diagnosis)` — `!` bukan operator
     Starlark (`not`) → konsultasi tidak pernah bisa selesai.
   - `complete.star` crash `NoneType value is not iterable` saat `treatments`
     kosong.
   - `visit.total` rule `positive` menolak 0 → visit tanpa treatment tidak
     bisa diselesaikan; diganti `[min: 0]`.

## Dampak

Dashboard Klinik membaca data live dengan filter server-side (exclude
`cancelled`), dan **real-time**: menyelesaikan konsultasi memicu event
`visit.completed` → widget refetch seketika (tanpa tunggu polling).
Terbukti end-to-end di browser: create visit #2 + complete → "Kunjungan Hari
Ini" 1→2 dan "Pendapatan Hari Ini" Rp 0→Rp 25.000 tanpa reload manual.
Node WS client menerima `{"event":"completed","resource":"clinic/visit",...}`.

## Files affected
- `renderers/web/src/kinds/dashboard/DashboardRenderer.tsx` — translate
  query → server filter, `serverToday()`, realtime wiring
- `renderers/web/src/hooks/useRealtime.ts` (baru) — realtime hook/klien
- `renderers/web/src/types/events.ts` (baru) — `RealtimeMessage`
- `examples/Clinic-UI-Showcase/spec/modules/clinic/transaction/visit/widgets/{today,revenue-today,by-polyclinic}.yaml` — query exclude cancelled
- `examples/Clinic-UI-Showcase/spec/modules/clinic/dashboards/clinic-dashboard.yaml` — `realtime: true`
- `examples/Clinic-UI-Showcase/spec/modules/clinic/transaction/visit/entity.yaml` — guard `not empty(diagnosis)`; `total` rule `[min: 0]`
- `examples/Clinic-UI-Showcase/spec/modules/clinic/transaction/visit/scripts/complete.star` — handle `treatments` kosong
- `internal/api/middleware.go` — `?token=` fallback
- `internal/api/api_test.go` — `TestAuthMiddleware_TokenQueryParam`

## Referensi
- Plan: `docs/plan/use-realtime-hook.md`, `docs/plan/fix-clinic-dashboard-summary.md`
- Verifikasi: `npx tsc -b --noEmit`, `go build ./...`, `go vet ./internal/api/`,
  `go test ./internal/api/ -count=1` (49 passed), `formspec validate`, browser
  `localhost:18080` (WS end-to-end).
