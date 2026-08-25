# Fase 7 — Workflow Engine (7.4)

**Status:** ✅ Core Complete (7.4.1–7.4.3, 7.4.5) · **Tanggal:** 2026-08-25
**Referensi:** `docs/spec/backend/02-core-extended.md` §2 (Workflow),
`pkg/spec/resources.go` (WorkflowSpec)
**Todo:** `docs/plan/todo.md` §7.4

## Konteks

`kind: Workflow` menempelkan approval berbasis role ke transisi state machine
**tanpa mengubah Entity** — pola yang sama dengan Subscription. Transisi yang
di-intercept baru eksekusi setelah seluruh step yang berlaku mencapai
quorum-nya. Approval adalah pernyataan bertanda tangan yang tercatat di audit.

`WorkflowSpec` sudah ada di `pkg/spec` (Entity + On + Steps + OnReject +
Escalation) tapi tidak ada runtime — tidak ada registry, tidak ada intercept
transisi, tidak ada tracking approval.

## Scope

### WF-1 — Workflow registry (7.4.1) ✅

- `internal/workflow/registry.go` — `Registry` memetakan `{module}.{name}` →
  `WorkflowSpec`; `Add`/`Get`/`List`/`ForTransition`.
- `buildWorkflowRegistry` di `resource/formspec.go` (boot + reload).
- Index by `{entity}.{from}.{to}` → workflow yang meng-intercept transisi itu;
  re-registration (hot reload) menghapus index lama.

### WF-2 — Approval state machine (7.4.1, 7.4.2, 7.4.3) ✅

- `internal/workflow/engine.go` — `Engine` mengevaluasi workflow untuk sebuah
  transisi.
- `RequiresApproval(entity, from, to)` — apakah transisi di-intercept workflow.
- `ApplicableSteps(workflow, resourceData)` — tentukan step mana yang berlaku
  (evaluasi `when` FormSpecExpr); step yang tidak berlaku di-skip.
- Multi-approver modes: `all` (semua yang berhak wajib), `any` (kuorum N dari
  kumpulan), `sequential` (berurutan sesuai urutan roles) — `StepMode`/`Quorum`.
- `CanApprove(workflow, stepIdx, userID, userRoles, requesterID, resourceData)`
  — cek eligibilitas approver + mode + `when`.
- **`FieldMap`** (internal/starlark/fieldmap.go) — tipe Starlark baru yang
  mendukung dot-notation `resource.amount` (bukan cuma `resource["amount"]`),
  dipakai di `EvaluateGuard` untuk `when`/guard expressions.

### WF-3 — Wire ke state machine transition (7.4.1) ✅

- `HandleCustomAction` (internal/api/handler.go) — setelah `CanTransition`
  lolos, cek apakah transisi di-intercept workflow. Jika ya, transisi TIDAK
  langsung eksekusi — masuk ke approval flow.
- `handleWorkflowApproval` — call pertama (tanpa pending approval) membuat
  approval request dan return `202 approval_required`; call berikutnya dengan
  `{"decision": "approve"}` / `{"decision": "reject"}` memproses keputusan.
- `executeWorkflowTransition` — update state record setelah semua step
  disetujui.
- Persistensi: tabel `formspec_workflow_approval` + `WorkflowApprovalStore`
  (renderers/jsonb-persist/workflow_approval.go).

### WF-4 — Requester can't approve own request (7.4.5) ✅

- `CanApprove` menolak jika `userID == requesterID` (created_by record).

### WF-5 — Audit trail (7.4.6) ⏸️ deferred

- Setiap approval/rejection/escalation tercatat di audit trail bisnis — belum
  diimplementasikan (butuh wiring ke event log / audit_log channel).

## Level of effort

| WF  | Effort |
| --- | ------ |
| 1   | small  |
| 2   | medium |
| 3   | medium |
| 4   | small  |
| 5   | small  |

## Verifikasi end-to-end (via `formspec dev` + curl)

- Entity `product` (state machine `active → discontinued` via `discontinue`);
  Workflow `demo.product-discontinue-approval` meng-intercept transisi.
- `POST .../discontinue` → `202 approval_required` (status record tetap
  `active`). ✅
- Approve oleh requester (admin) → `403 WORKFLOW_DENIED`. ✅
- Approve oleh manager (role `demo.manager` + permission) → `200
transition_completed`, status record → `discontinued`. ✅
- `go test ./...` hijau (809 pass, termasuk unit test `internal/workflow`).

## File terdampak

- `internal/workflow/registry.go` (baru) — registry
- `internal/workflow/engine.go` (baru) — evaluasi workflow + approval
- `internal/workflow/registry_test.go` (baru) — unit test
- `internal/starlark/fieldmap.go` (baru) — FieldMap (dot-notation resource)
- `internal/starlark/guard.go` — `resource`/`data` pakai FieldMap
- `internal/starlark/evaluator.go` — `toStarlark` handle `*FieldMap`
- `renderers/jsonb-persist/workflow_approval.go` (baru) — approval store
- `renderers/jsonb-persist/migrate.go` — tabel `formspec_workflow_approval`
- `internal/api/handler.go` — intercept transisi + approval flow
- `internal/api/router.go` — `SetWorkflowRegistry`/`SetWorkflowApprovalStore`
- `resource/formspec.go` — `buildWorkflowRegistry` + wiring (boot + reload)
- `internal/manifest/loader.go` — `RawSpecToWorkflowSpec`
- `examples/service-demo/` — contoh workflow (entity + workflow + script)
