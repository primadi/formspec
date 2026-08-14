# Timeline

<!-- generated:meta -->
| | |
|---|---|
| Grup | `ui` |
| Plane | `resource` |
| Spec struct | `TimelineSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Timeline` adalah **feed kronologis vertikal**, dikelompokkan per tanggal — untuk audit trail append-only, activity log, rekam medis (ditulis sekali, tidak pernah diubah).

**Kapan memakai Timeline:**
- Urutan waktu sebagai narasi utama (rekam medis, audit log, activity feed) — *cerita read-only*

**Kapan TIDAK pakai Timeline:**
- User perlu sort/filter/operate baris → `kind: Table` (*permukaan operasional*)

**Sumber kontrak:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §9.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
kind: Timeline
metadata:
  name: patient-medical-history
  module: clinic
spec:
  entity: clinic.medical_record
  bind_param: patient_id
  bind_value: ":patient_id"
  display:
    title_field: visit_date
    subtitle_field: doctor.name
    content_field: diagnosis_and_notes
  group_by: date                          # date | month | year | none
  sort: desc
  page_size: 20
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `public` | `boolean` | — | true | If true (default), a route /module/timeline/<name> is auto-generated. |
| `entity` | `string` | ✅ | clinic.medical_record |  |
| `event_field` | `string` | — |  |  |
| `date_field` | `string` | — |  |  |
| `bind_param` | `string` | — |  |  |
| `bind_value` | `string` | — |  |  |
| `display` | `TimelineDisplay` | — |  |  |
| `group_by` | `string` | — |  | date \| month \| year \| none |
| `sort` | `string` | — |  | asc \| desc (default desc) |
| `page_size` | `integer` | — |  |  |
| `empty_state` | `string` | — |  |  |

<!-- /generated:attributes -->

## Gotchas

- **Renderer tidak menampilkan tombol create/edit/delete** untuk entity Timeline — entity SEBAIKNYA disable `update`/`delete`, kind ini sendiri yang jadi guard.
- **Infinite scroll cursor-based pakai `created_at`**; realtime subscribe `created`.
- **Cross-ref:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §9 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
