# Workflow

<!-- generated:meta -->
| | |
|---|---|
| Grup | `data` |
| Plane | `resource` |
| Spec struct | `WorkflowSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Workflow` adalah **approval multi-approver** yang di-attach ke SATU transition state-machine Entity.

**Kapan memakai Workflow:**
- Transisi butuh approval berjenjang (journal posting butuh supervisor lalu controller)
- Quorum approval (mis. `approvers: 2` dari role tertentu)
- Escalation setelah timeout

**Kapan TIDAK pakai Workflow:**
- States/transitions TIDAK dideklarasi di sini — mereka hidup di Entity (`state_machine`)
- Sekadar transisi bisnis tanpa approval → cukup state_machine + action di Entity

**Sumber kontrak:** [`docs/spec/backend/02-core-extended.md`](../spec/backend/02-core-extended.md) §2.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
kind: Workflow
metadata:
  name: journal-posting-approval
  module: gl
spec:
  entity: gl.journal-entry
  on: { transition: { from: draft, to: posted } }
  steps:
    - { roles: [gl.supervisor], approvers: 1 }
    - { roles: [gl.controller], approvers: 1,
        when: "resource.amount > 100000000" }
  on_reject: { to: rejected }
  escalation: { after: 48h, notify_roles: [gl.manager] }
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `entity` | `string` | ✅ | gl.journal-entry |  |
| `on` | `WorkflowTrigger` | ✅ |  |  |
| `steps` | []`WorkflowStep` | — |  |  |
| `on_reject` | `WorkflowReject` | — |  |  |
| `escalation` | `WorkflowEscalation` | — |  |  |

<!-- /generated:attributes -->

## Gotchas

- **JANGAN deklarasi `states:`/`transitions:` di Workflow** — itu milik Entity (`state_machine`). Workflow hanya intercept SATU `from → to` transition + approval steps.
- **Step fields**: `roles` (eligibility), `approvers` (quorum, default 1), `mode` (`all`/`any`/`sequential`), `when` (FormSpecExpr skip step), `escalation` (`after`, `notify_roles`, `reassign_roles`).
- **`on:` adalah YAML key normal** — jangan quote (`on: { transition: ... }`); hanya parser YAML 1.1 (PyYAML) yang salah baca jadi boolean.
- **Cross-ref:** [`docs/spec/backend/02-core-extended.md`](../spec/backend/02-core-extended.md) §2 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
