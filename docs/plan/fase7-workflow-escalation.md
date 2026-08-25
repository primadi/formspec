# Fase 7 — Workflow Escalation (7.4.4)

**Status:** ✅ Complete · **Tanggal:** 2026-08-25
**Referensi:** `docs/spec/backend/02-core-extended.md` §2 (Workflow — Timeout &
eskalasi)
**Todo:** `docs/plan/todo.md` §7.4.4

## Konteks

Workflow engine (7.4.1–7.4.3, 7.4.5, 7.4.6) sudah selesai. 7.4.4 melengkapi
**escalation**: `escalation.after` menandai durasi diam sebelum step
dieskalasi; `notify_roles` diberi tahu, dan `reassign_roles` (opsional)
memindahkan hak persetujuan ke role lain setelah durasi itu lewat — sehingga
approval tidak menggantung selamanya karena satu orang cuti. Eskalasi wajib
tercatat di audit trail bisnis.

## Scope

### ESC-1 — Escalation state di approval (7.4.4) ✅

- `WorkflowApprovalRow` ditambah `EscalatedSteps map[int][]string` — step yang
  sudah dieskalasi + role reassignment-nya (kolom `escalated_steps` + ensure
  column migration).
- `workflow.Approval` ditambah field yang sama; `CanApprove` menerima role
  reassignment sebagai eligible roles tambahan untuk step yang dieskalasi.
- Approval menyimpan **nama manifest workflow** (bukan pointer address) via
  `Registry.NameFor` — supaya escalation worker bisa resolve kembali.

### ESC-2 — Escalation worker (7.4.4) ✅

- `internal/workflow/escalation.go` — `EscalationWorker` background goroutine
  yang poll pending approvals secara periodik (`ListPending`).
- Untuk setiap pending approval: cek step aktif punya `escalation.after`; jika
  durasi sudah lewat sejak step aktif (updated_at), eskalasi:
  - catat audit `workflow.escalate` (actor = system, changes = `{workflow,
step, reassign_roles}`).
  - tandai step sebagai escalated + simpan reassign_roles.
- Step dieskalasi maksimal sekali (tracked di `EscalatedSteps`).
- `Start`/`Stop` lifecycle; di-start dari `App.StartBackgroundWorkers`.

### ESC-3 — Wire (7.4.4) ✅

- `resource/formspec.go` — buat `EscalationWorker` + start di
  `StartBackgroundWorkers`; stop di `Close`.
- `App` struct — field `escalationWorker`.

## Level of effort

| ESC | Effort |
| --- | ------ |
| 1   | small  |
| 2   | medium |
| 3   | small  |

## Verifikasi end-to-end (via `formspec dev` + curl)

- Workflow dengan `escalation.after: 10s`; create approval → tunggu → worker
  eskalasi step → audit `workflow.escalate` tercatat + `escalated_steps`
  ter-update. ✅
- `go test ./...` hijau (821 pass, termasuk unit test `internal/workflow`).

## File terdampak

- `renderers/jsonb-persist/workflow_approval.go` — `EscalatedSteps` + `ListPending`
- `renderers/jsonb-persist/migrate.go` — kolom `escalated_steps` + ensure column
- `internal/workflow/engine.go` — `Approval.EscalatedSteps` + `CanApprove` reassign
- `internal/workflow/registry.go` — `NameFor`
- `internal/workflow/escalation.go` (baru) — escalation worker
- `internal/workflow/escalation_test.go` (baru) — unit test
- `resource/formspec.go` — wire escalation worker (boot + reload)
- `internal/api/handler.go` — `CanApprove` reassign roles + workflow name
