# Calendar

<!-- generated:meta -->
| | |
|---|---|
| Grup | `ui` |
| Plane | `resource` |
| Spec struct | `CalendarSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Calendar` adalah **view kalender** atas entity yang punya field tanggal/waktu — untuk penjadwalan (appointment, delivery planning).

**Kapan memakai Calendar:**
- Penjadwalan berbasis tanggal (janji temu, pengiriman)
- Resource scheduling (dokter/ruangan per lajur)

**Kapan TIDAK pakai Calendar:**
- List tanggal biasa → `kind: Table`

**Sumber kontrak:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §5.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
kind: Calendar
metadata: { name: appointment-calendar, module: clinic }
spec:
  entity: clinic.appointment
  date_field: scheduled_at
  views: [month, week, day, resource]
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `public` | `boolean` | — | true | If true (default), a route /module/calendar/<name> is auto-generated. |
| `entity` | `string` | ✅ | clinic.appointment |  |
| `date_field` | `string` | ✅ | scheduled_at |  |
| `end_field` | `string` | — |  |  |
| `title_field` | `string` | — |  |  |
| `resource_field` | `string` | — |  |  |
| `color_field` | `string` | — |  |  |
| `views` | []`string` | — |  | month, week, day, resource (default month) |
| `realtime` | `boolean` | — |  |  |

<!-- /generated:attributes -->

## Gotchas

- **Recurrence WAJIB berformat RRULE (RFC 5545)** — bukan grammar sendiri, demi interop `.ics`.
- **Expansion saat baca/render** (bukan materialized rows) — murni komputasi tampilan.
- **Drag reschedule** = action `update` ubah `date_field`; validasi server otoritas; record `submitted` immutable tak bisa di-drag.
- **Recurrence BUKAN recurring job** — itu domain module resmi `formspec/scheduler`.
- **Exception per-instance masih Open** — Calendar v1 tampilkan seluruh occurrence tanpa exception.
- **Cross-ref:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §5 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
