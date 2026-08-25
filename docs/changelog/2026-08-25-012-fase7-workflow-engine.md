# Workflow Engine Core (7.4.1–7.4.3, 7.4.5)

**Tanggal:** 2026-08-25 · **Todo:** §7.4.1–7.4.3, §7.4.5 · **Plan:** `docs/plan/fase7-workflow-engine.md`

## Apa yang ditambahkan

`kind: Workflow` kini punya runtime approval — role-based approval menempel ke
transisi state machine **tanpa mengubah Entity** (02-core-extended.md §2):

- **Registry (7.4.1)** — `internal/workflow/registry.go` memetakan
  `{module}.{name}` → `WorkflowSpec`; index by `{entity}.{from}.{to}`;
  `buildWorkflowRegistry` di `resource/formspec.go` (boot + reload).
- **Approval state machine (7.4.1)** — `HandleCustomAction` intercept transisi
  yang di-workflow; call pertama membuat pending approval (tabel
  `formspec_workflow_approval` + `WorkflowApprovalStore`) dan return
  `202 approval_required`; call berikutnya dengan `{"decision": "approve"}`
  / `{"decision": "reject"}` memproses keputusan; `executeWorkflowTransition`
  update state record setelah semua step disetujui.
- **Multi-approver modes (7.4.2)** — `all` (semua yang berhak wajib), `any`
  (kuorum N dari kumpulan), `sequential` (berurutan) — `StepMode`/`Quorum`.
- **`when` condition (7.4.3)** — `ApplicableSteps` evaluasi `when` FormSpecExpr;
  step yang tidak berlaku di-skip. **`FieldMap`** (internal/starlark/fieldmap.go)
  — tipe Starlark baru yang mendukung dot-notation `resource.amount` (bukan
  cuma `resource["amount"]`), dipakai di `EvaluateGuard` untuk `when`/guard.
- **Requester exclusion (7.4.5)** — `CanApprove` menolak jika `userID ==
  requesterID` (created_by record).

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

- `internal/workflow/registry.go`, `engine.go`, `registry_test.go` (baru)
- `internal/starlark/fieldmap.go` (baru) — FieldMap (dot-notation resource)
- `internal/starlark/guard.go`, `evaluator.go` — FieldMap wiring
- `renderers/jsonb-persist/workflow_approval.go` (baru) — approval store
- `renderers/jsonb-persist/migrate.go` — tabel `formspec_workflow_approval`
- `internal/api/handler.go`, `router.go` — intercept transisi + wiring
- `resource/formspec.go` — `buildWorkflowRegistry` + wiring (boot + reload)
- `internal/manifest/loader.go` — `RawSpecToWorkflowSpec`
- `examples/service-demo/` — contoh workflow (entity + workflow + script)