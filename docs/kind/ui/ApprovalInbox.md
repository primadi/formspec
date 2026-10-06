# ApprovalInbox

<!-- generated:meta -->
| | |
|---|---|
| Grup | `ui` |
| Plane | `resource` |
| Spec struct | `ApprovalInboxSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: ApprovalInbox` adalah **task-queue "persetujuan saya"** — daftar step approval (dideklarasikan pada transisi Entity) yang menunggu tindakan caller.

**Kapan memakai ApprovalInbox:**

- Menampilkan approval pending milik user saat ini, lintas entity/module dalam App

**Kapan TIDAK pakai ApprovalInbox:**

- Mesin approval sendiri → itu `state_machine.transitions[].approval` pada Entity; kind ini hanya permukaannya

**Sumber kontrak:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §11.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
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
| `filters` | [][`FilterSpec`](../../spec/frontend/06-page-kinds.md) | — |  |  |
| `search` | `boolean` | — |  |  |

<!-- /generated:attributes -->

## Gotchas

- **Zero-config** — sumber = step approval pending yang eligible untuk caller (duty/role per step), lintas entity/module, permission-filtered otomatis.
- **Sumbernya bukan entity** — barisnya di tabel framework `formspec_workflow_approval`, jadi ia dibaca lewat surface sendiri: `GET /{ws}/_ui/workflow/approvals` (keputusan: `POST …/approvals/{id}` dengan `{"decision":"approve"|"reject"}`). Ini satu-satunya kind yang tidak punya entity di belakangnya.
- **"Pemohon tak pernah menyetujui permintaannya sendiri" ditegakkan backend**, bukan disembunyikan di UI.
- **`can_decide` memisahkan terdaftar dari boleh-dijalankan.** Keanggotaan role pada step menentukan apa yang muncul; permission yang menggerbangi route transisi menentukan apa yang bisa dieksekusi. Tugas dengan `can_decide: false` tetap terlihat, tombolnya non-aktif.
- **Action inline `approve`/`reject`** = pencatatan approval bertanda tangan; `reject` mengikuti `on_reject`. Transisi baru eksekusi setelah quorum seluruh step.
- **`display_fields` hanya terisi bila caller memegang `{module}.{plural}.view`** — tugasnya tetap terlihat, nilainya tidak.
- **`realtime: true` belum dihormati** (todo 5.13.7 ⏸️): hub WS mendorong per `{module}/{entity}`, sedangkan approval bukan entity. Caller harus me-refresh.
- **Cross-ref:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §11 · [`docs/spec/backend/02-core-extended.md`](../spec/backend/02-core-extended.md) §2 · [`docs/runtimes/06-ui-rest-contract.md`](../runtimes/06-ui-rest-contract.md) §5 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
