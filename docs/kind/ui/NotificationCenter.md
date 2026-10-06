# NotificationCenter

<!-- generated:meta -->
| | |
|---|---|
| Grup | `ui` |
| Plane | `resource` |
| Spec struct | `NotificationCenterSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: NotificationCenter` adalah **permukaan in-app notifikasi** untuk user saat ini — sumbernya entity `formspec.core.notification` dari module resmi `formspec/notify`, yang diisi channel delivery `notification`.

**Kapan memakai NotificationCenter:**
- Feed notifikasi in-app dengan badge unread

**Kapan TIDAK pakai NotificationCenter:**
- Template pesan & channel provider (email/push/in-app) → itu hidup di `formspec/notify`, bukan di kontrak ini; untuk kanal luar, tambahkan `handler:` pada entry `notification`

**Sumber kontrak:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §12.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
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

- **Sumbernya `formspec.core.notification`** — sebuah Entity di module resmi
  `formspec/notify`, bukan tabel tersembunyi. Karena itu ia dapat `row_scope`
  (`recipient_id` = `user_id` sesi), jadi "hanya pemiliknya yang melihat"
  ditegakkan server, dan pemanggil tanpa identitas **fail-closed** (bukan "lihat
  semua").
- **Yang mengisinya adalah channel delivery `notification`** — jadi sebuah event
  bisa menghasilkan notifikasi in-app **tanpa kode apa pun**:
  `deliver: [{channel: notification, notification: {recipient: customer_id,
  title: "..."}}]`. Renderer ini mencari ref entity konvensional
  (`formspec.core.notification` / `formspec.core.notify`).
- **Zero-config** — daftar notifikasi caller (terurut terbaru), badge unread, aksi
  `read` (per item).
- **`realtime: true`** — item baru masuk in-place lewat hub WS per
  `{module}/{entity}`, jadi entity ini (bukan entity notifikasi) sudah terhubung.
- **Notifikasi per-user & tenant/workspace-scoped** seperti semua data.
- **Cross-ref:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §12 · [`docs/spec/backend/02-core-extended.md`](../spec/backend/02-core-extended.md) §3 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
