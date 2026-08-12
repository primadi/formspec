# NotificationCenter

<!-- generated:meta -->
| | |
|---|---|
| Grup | `ui` |
| Plane | `resource` |
| Spec struct | `NotificationCenterSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: NotificationCenter` adalah **permukaan in-app notifikasi** untuk user saat ini — sumbernya module resmi `formspec/notify`.

**Kapan memakai NotificationCenter:**
- Feed notifikasi in-app dengan badge unread

**Kapan TIDAK pakai NotificationCenter:**
- Template pesan & channel provider (email/push/in-app) → itu hidup di `formspec/notify`, bukan di kontrak ini

**Sumber kontrak:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §12.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1alpha1
kind: NotificationCenter
metadata: { name: notifications, module: core }
spec:
  realtime: true
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `realtime` | `boolean` | — |  |  |

<!-- /generated:attributes -->

## Gotchas

- **Zero-config** — daftar notifikasi caller (terurut terbaru), badge unread, aksi `mark-read` (per item + mark-all).
- **`realtime: true` default** — item baru masuk in-place, badge unread naik.
- **Klik notifikasi → navigasi deep-link** entity/Page yang dirujuk (bila ada).
- **Notifikasi per-user & tenant/workspace-scoped** seperti semua data.
- **Cross-ref:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §12 · [`docs/spec/backend/02-core-extended.md`](../spec/backend/02-core-extended.md) §3 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
