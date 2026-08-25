# Fase 7 — Async Job Tracker (7.13)

**Status:** ✅ Complete · **Tanggal:** 2026-08-25
**Referensi:** `docs/spec/backend/02-core-extended.md` §13 (Async Action & Job
Tracking), `pkg/spec/entity.go` (Action), `internal/action/dispatcher.go`
(ExecuteParams), `internal/starlark/context.go` (CtxAPI)
**Todo:** `docs/plan/todo.md` §7.13

## Konteks

`call: async` fire-and-forget (7.1.4) sudah ada — tidak ada job_id, tidak ada
progres. §13 menambah **tracked async**: `call: async` + `track: true` →
langsung `202` dengan `job_id`, progres didorong di kanal `jobs`
(`progress`/`completed`/`failed`), handler melapor via `ctx.job.progress(pct,
message)`, hasil bisa dikirim via callback webhook HMAC-signed.

## Prinsip desain

- **Marker `track: true`** pada Action membedakan tracked async dari
  fire-and-forget (`call: async` saja). `call: async` + `track: true` =
  tracked job.
- Job = data di tabel `formspec_job` (JobStore, pola sama seperti outbox/saga).
- Progres/hasil didorong ke kanal `jobs` via websocket hub (per workspace).
- `ctx.job.progress(pct, message)` — reporter di-thread dari tracker lewat
  `ExecuteParams.JobID` → ScriptExecutor → CtxAPI.

## Scope

### JOB-1 — JobStore (`renderers/jsonb-persist/job.go`)

- Tabel `formspec_job`: id, tenant_id, module, entity, action, status
  (pending|running|completed|failed), progress (int), message, result (json),
  error, created_at, updated_at.
- `JobStore`: `Create`/`Update`/`Get`/`ListByWorkspace`.

### JOB-2 — Tracker (`internal/job/tracker.go`)

- `Tracker{store, hub}`:
  - `Create(ctx, ws, module, entity, action, params)` → job_id (status pending)
  - `Start(ctx, jobID)` → running
  - `Progress(ctx, jobID, pct, message)` → update + publish `progress` event
  - `Complete(ctx, jobID, result)` → completed + publish `completed` event
  - `Fail(ctx, jobID, err)` → failed + publish `failed` event
  - `Get(ctx, jobID)` → row (untuk poll_url)
- Event payload: `{job_id, progress?, message?, status?, result?}`.

### JOB-3 — `ctx.job.progress` (Starlark)

- `CtxAPI.Job *jobAPI` — `progress(pct, message)` memanggil reporter.
- `ExecuteParams.JobID` (internal/action) → `action.ScriptExecutor` →
  `starlark.ScriptExecutor.Execute` → `ctxObj.Job = NewJobAPI(reporter)`.
- Reporter nil (bukan tracked job) → `ctx.job.progress` no-op / error jelas.

### JOB-4 — Tracked async dispatch (`internal/api/handler.go`)

- `actionSpec.Call == "async" && actionSpec.Track`:
  - `tracker.Create` → job_id
  - goroutine: `Start` → `Dispatch` (dengan `execParams.JobID`) → `Complete`/
    `Fail`
  - return `202` `{job_id, status: "pending"}` + `meta.track`
    `{websocket_event: "jobs", poll_url: "/.../jobs/{job_id}"}`
- Route `GET /.../jobs/{job_id}` → status/result (polling alternatif).

### JOB-5 — Callback webhook (7.13.4)

- `Action.Callback *CallbackDecl` — `{channel: webhook, url_from: header,
  header: X-Callback-URL, sign: true, retry: {...}}`.
- Saat job selesai/gagal: kirim hasil ke URL callback (dari header request,
  disimpan di job row) — HMAC-signed (reuse webhook sign), retry durable
  (reuse outbox retry pattern).

### JOB-6 — Wiring + contoh

- `resource/formspec.go`: JobStore + Tracker, wire ke handler + script
  executor; route jobs.
- `examples/service-demo`: action `call: async` + `track: true` + script
  `ctx.job.progress`; verifikasi 202 + job_id + event jobs.

## Level of effort

| JOB | Effort |
| --- | ------ |
| 1   | medium |
| 2   | medium |
| 3   | medium |
| 4   | medium |
| 5   | large  |
| 6   | small  |

## Verifikasi end-to-end (via `formspec dev` + curl)

- `formspec dev` di `examples/service-demo` → service `report-generator`
  (`call: async` + `track: true` + `callback`).
- `POST /api/v1/demo/report-generator/generate` → `202` `{job_id, status:
  pending}` + `meta.track` (websocket_event `jobs`, poll_url).
- Script `generate_report.star` memanggil `ctx.job.progress` (progress → 100);
  `GET /api/v1/demo/report-generator/jobs/{id}` → `status: completed`,
  `result: {rows, format, generated_by}`. ✅
- `go test ./...` hijau (28 paket, termasuk `internal/job` tracker tests).

## File terdampak

- `renderers/jsonb-persist/job.go` (baru) — JobStore + tabel
- `internal/job/tracker.go` (baru) — Tracker
- `internal/job/tracker_test.go` (baru)
- `internal/starlark/context.go` — `Job *jobAPI`
- `internal/starlark/executor.go` — thread job reporter
- `internal/action/dispatcher.go` — `ExecuteParams.JobID`
- `internal/action/script.go` — thread job reporter
- `internal/api/handler.go` — tracked async dispatch + jobs route
- `pkg/spec/entity.go` — `Action.Track` + `Action.Callback`
- `resource/formspec.go` — wire JobStore + Tracker
- `examples/service-demo/` — contoh tracked async
- `docs/plan/fase7-async-job-tracker.md` (baru) — plan