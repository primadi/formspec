# Kanban

<!-- generated:meta -->
| | |
|---|---|
| Grup | `ui` |
| Plane | `resource` |
| Spec struct | `KanbanSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Kanban` adalah **papan kolom drag-drop** — tiap kartu satu record entity, tiap kolom satu nilai status, drag antar kolom = transisi state.

**Kapan memakai Kanban:**
- Status sebagai dimensi kerja utama (support queue, order fulfillment, triage)
- Pemindahan status = aksi utama

**Kapan TIDAK pakai Kanban:**
- Operasi utama sort/filter/edit banyak kolom → `kind: Table`

**Sumber kontrak:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §4.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
kind: Kanban
metadata: { name: support-board, module: helpdesk }
spec:
  entity: helpdesk.ticket
  status_field: status      # wajib — field state machine/enum yang jadi kolom
  realtime: true
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `public` | `boolean` | — | true | If true (default), a route /module/kanban/<name> is auto-generated. |
| `entity` | `string` | ✅ | helpdesk.ticket |  |
| `status_field` | `string` | ✅ | status |  |
| `columns` | []`KanbanColumn` | — |  |  |
| `card_template` | `KanbanCard` | — |  |  |
| `realtime` | `boolean` | — |  | default true (§12) |
| `filters` | []`FilterSpec` | — |  |  |
| `fixed_filters` | []`FilterSpec` | — |  |  |
| `search` | `boolean` | — |  |  |
| `row_actions` | []`TableAction` | — |  |  |
| `max_cards_per_column` | `integer` | — |  |  |
| `sortable` | `boolean` | — |  | enable within-column drag-to-reorder |
| `position_field` | `string` | — |  | field storing user-adjustable position (e.g. "queue_position") |

<!-- /generated:attributes -->

## Gotchas

- **`status_field` WAJIB saat ini** — zero-config derivasi kolom masih Open (tracking `docs/plan/kanban-full-implementation.md`).
- **Drag = state transition** — memanggil action `via` transisi cocok; guard state machine dievaluasi server-side (otoritas). Transisi tidak dideklarasi → tidak ada drop target; server tolak `STATE_TRANSITION_ERROR`.
- **Permission drag = permission action transisi itu**.
- **`sortable: true` butuh `position_field`** — kombinasi tanpa itu tidak valid (manifest validation wajib menolak).
- **`drag_guard` & `wip_limit` masih Open** — pengganti sementara `max_cards_per_column`.
- **No-silent-drop** — kolom ramai paginasi cursor-based, tak boleh motong kartu tanpa indikator "muat lebih".
- **Cross-ref:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §4 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
