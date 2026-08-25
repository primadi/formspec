# 2026-08-25-021 — Fase 7: Async Job Tracker (7.13)

## Apa yang diubah

Implementasi **async job tracking** (kontrak `docs/spec/backend/02-core-extended.md`
§13) — tracked async action (`call: async` + `track: true`) langsung `202`
dengan `job_id`, progres didorong di kanal `jobs`, handler melapor via
`ctx.job.progress`, hasil bisa dikirim via callback webhook HMAC-signed.

**Marker `track: true`** pada `Action` membedakan tracked async dari
fire-and-forget (`call: async` saja, 7.1.4). `Action.Callback` (`CallbackDecl`)
mendeklarasikan callback webhook (channel/url_from/header/sign/retry).

**JobStore** (`renderers/jsonb-persist/job.go`) — tabel `formspec_job`
(tenant_id, module, entity, action, status pending|running|completed|failed,
progress, message, result, error, callback_url, created_at, updated_at);
`Create`/`Update`/`Get`/`ListByWorkspace`.

**Tracker** (`internal/job/tracker.go`) — `Create`/`Start`/`Progress`/
`Complete`/`Fail`/`Get`; publish event `progress`/`completed`/`failed` ke kanal
`jobs` via websocket hub (per workspace); `deliverCallback` (7.13.4) kirim
hasil ke URL callback (dari header request) HMAC-SHA256 (`X-FormSpec-Signature`)
dengan bounded retry.

**`ctx.job.progress(pct, message)`** — `CtxAPI.Job` (`jobAPI`); reporter
di-thread dari tracker lewat `ExecuteParams.JobID` → `action.ScriptExecutor`
→ `starlark.ScriptExecutor` → `ctxObj.Job`. Non-tracked job → error jelas
("not inside a tracked async job").

**Dispatch** (`internal/api/handler.go`) — `dispatchTrackedAsync`: create job →
goroutine (Start → Dispatch dengan JobID → Complete/Fail) → `202` `{job_id,
status: pending}` + `meta.track` (websocket_event `jobs`, poll_url). Route
`GET /api/v1/{module}/{service}/jobs/{job_id}` (polling alternatif) di
`GenerateServiceRoutes` (hanya untuk service dengan tracked action).

## Kenapa

Fire-and-forget (7.1.4) tidak punya job_id/progres/hasil; §13 menambah tracked
async untuk kerja yang hasilnya dinanti (report, batch, integrasi) — 202
seketika + progres realtime + hasil via websocket/polling/callback.

## File terdampak

- `renderers/jsonb-persist/job.go` (baru) — JobStore + tabel `formspec_job`
- `renderers/jsonb-persist/migrate.go` — DDL `formspec_job`
- `internal/job/tracker.go` (baru) — Tracker
- `internal/job/tracker_test.go` (baru) — unit test
- `internal/starlark/context.go` — `CtxAPI.Job` (`jobAPI`) + `Attr`/`AttrNames`
- `internal/starlark/executor.go` — thread job reporter
- `internal/action/dispatcher.go` — `ExecuteParams.JobID`
- `internal/action/script.go` — `SetJobProgressReporter` + thread
- `internal/api/handler.go` — `dispatchTrackedAsync` + `HandleJobStatus` +
  `TrackMeta`
- `internal/api/generator.go` — jobs polling route
- `internal/api/router.go` — `SetJobTracker` + `job-status` handler
- `pkg/spec/entity.go` — `Action.Track` + `Action.Callback` + `CallbackDecl`
- `resource/formspec.go` — wire JobStore + Tracker (boot + reload)
- `examples/service-demo/` — service `report-generator` + script
  `generate_report.star`
- `docs/plan/fase7-async-job-tracker.md` (baru) — plan

## Referensi

- Todo: `docs/plan/todo.md` §7.13
- Plan: `docs/plan/fase7-async-job-tracker.md`
- Spec: `docs/spec/backend/02-core-extended.md` §13