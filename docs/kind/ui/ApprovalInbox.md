# ApprovalInbox

<!-- generated:meta -->
| | |
|---|---|
| Grup | `ui` |
| Plane | `resource` |
| Spec struct | `ApprovalInboxSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: ApprovalInbox` adalah **task-queue "persetujuan saya"** — daftar step approval Workflow yang menunggu tindakan caller.

**Kapan memakai ApprovalInbox:**
- Menampilkan approval pending milik user saat ini, lintas entity/module dalam App

**Kapan TIDAK pakai ApprovalInbox:**
- Mesin approval sendiri → itu backend Workflow; kind ini hanya permukaannya

**Sumber kontrak:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §11.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1alpha1
kind: ApprovalInbox
metadata: { name: my-approvals, module: core }
spec:
  realtime: true
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `realtime` | `boolean` | — |  |  |
| `filters` | []`FilterSpec` | — |  |  |
| `search` | `boolean` | — |  |  |

<!-- /generated:attributes -->

## Gotchas

- **Zero-config** — sumber = step Workflow pending yang eligible untuk caller (role per step), lintas entity/module, permission-filtered otomatis.
- **"Pemohon tak pernah menyetujui permintaannya sendiri" ditegakkan backend**, bukan disembunyikan di UI.
- **Action inline `approve`/`reject`** = pencatatan approval bertanda tangan; `reject` mengikuti `on_reject`. Transisi baru eksekusi setelah quorum seluruh step.
- **`realtime: true` default** (badge count).
- **Cross-ref:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §11 · [`docs/spec/backend/02-core-extended.md`](../spec/backend/02-core-extended.md) §2 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
