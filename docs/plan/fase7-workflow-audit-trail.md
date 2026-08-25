# Fase 7 — Workflow Audit Trail (7.4.6)

**Status:** ✅ Complete · **Tanggal:** 2026-08-25
**Referensi:** `docs/spec/backend/02-core-extended.md` §2 (Workflow),
`docs/spec/backend/01-core-basic.md` §11 (Audit trail bisnis)
**Todo:** `docs/plan/todo.md` §7.4.6

## Konteks

Workflow engine core (7.4.1–7.4.3, 7.4.5) sudah selesai. 7.4.6 melengkapi:
**setiap approval/rejection/escalation adalah pernyataan bertanda tangan yang
tercatat di audit trail bisnis** — siapa, kapan, keputusan apa.

## Scope

### AUD-1 — Expose audit writer (7.4.6) ✅

- `renderers/jsonb-persist/audit.go` — tambah exported `WriteAuditLog` yang
  membungkus `writeAuditLog` (unexported, dipakai CRUD layer). Action
  `workflow.approve` / `workflow.reject` / `workflow.transition`.

### AUD-2 — Record approval decisions (7.4.6) ✅

- `internal/api/handler.go` — `handleWorkflowApproval` mencatat setiap
  keputusan ke audit trail:
  - approve: action `workflow.approve`, actor = approver, changes =
    `{workflow, step, from, to, decision: "approve"}`.
  - reject: action `workflow.reject`, actor = approver, changes =
    `{workflow, step, from, to, decision: "reject"}`.
  - transition_completed: action `workflow.transition`, actor = system,
    changes = `{to}`.
- `recordWorkflowAudit` helper (best-effort — gagal tulis audit tidak
  menggagalkan keputusan workflow).
- Wire `SetAuditWriter` di `resource/formspec.go` (boot + reload) via
  `db.WriteAuditLog`.

## Level of effort

| AUD | Effort |
| --- | ------ |
| 1   | small  |
| 2   | small  |

## Verifikasi end-to-end (via `formspec dev` + curl)

- Product discontinue → approve oleh manager → audit trail berisi
  `workflow.approve` (actor = manager-1) + `workflow.transition` (actor =
  system). ✅
- `go test ./...` hijau (816 pass).

## File terdampak

- `renderers/jsonb-persist/audit.go` — `WriteAuditLog` exported
- `internal/api/handler.go` — record approval/rejection ke audit trail
- `internal/api/router.go` — `SetAuditWriter`
- `resource/formspec.go` — wire audit writer (boot + reload)
