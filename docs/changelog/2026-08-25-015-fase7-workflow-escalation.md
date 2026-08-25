# Workflow Escalation (7.4.4) — Timeout & Reassign

**Tanggal:** 2026-08-25 · **Todo:** §7.4.4 · **Plan:** `docs/plan/fase7-workflow-escalation.md`

## Apa yang ditambahkan

Workflow engine kini mendukung **escalation** — approval tidak menggantung
selamanya karena satu orang cuti (02-core-extended.md §2):

- **Escalation state** — `WorkflowApprovalRow.EscalatedSteps` (stepIdx →
  reassign_roles) + kolom `escalated_steps` (dengan ensure-column migration
  untuk DB lama). `workflow.Approval` + `CanApprove` menerima reassign_roles
  sebagai eligible roles tambahan untuk step yang dieskalasi.
- **Escalation worker** — `internal/workflow/escalation.go`: background
  goroutine yang poll pending approvals (`ListPending`); step aktif dengan
  `escalation.after` yang sudah lewat → eskalasi: catat audit
  `workflow.escalate` (actor = system) + tandai step escalated dengan
  `reassign_roles`. Step dieskalasi maksimal sekali.
- **Fix: workflow name** — approval kini menyimpan **nama manifest workflow**
  (bukan pointer address) via `Registry.NameFor`, supaya escalation worker
  bisa resolve kembali.
- **Wire** — `App.StartBackgroundWorkers`/`Close` start/stop escalation worker.

## Verifikasi end-to-end (via `formspec dev` + curl)

- Workflow dengan `escalation.after: 10s`; create approval → tunggu → worker
  eskalasi step → audit `workflow.escalate` tercatat + `escalated_steps`
  ter-update. ✅
- `go test ./...` hijau (821 pass, termasuk unit test `internal/workflow`).

## File terdampak

- `renderers/jsonb-persist/workflow_approval.go` (`EscalatedSteps` + `ListPending`)
- `renderers/jsonb-persist/migrate.go` (kolom `escalated_steps` + ensure column)
- `internal/workflow/engine.go` (`Approval.EscalatedSteps` + `CanApprove` reassign)
- `internal/workflow/registry.go` (`NameFor`)
- `internal/workflow/escalation.go` (baru) — escalation worker
- `internal/workflow/escalation_test.go` (baru) — unit test
- `resource/formspec.go` (wire escalation worker boot + reload)
- `internal/api/handler.go` (`CanApprove` reassign roles + workflow name)
