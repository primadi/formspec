# Feat: opsi select filter dari master data + opsi "All" generik (show_all / all_label)

## Perubahan

Dropdown filter `select` di Kanban sebelumnya men-derivasi opsi dari **records
board** yang sudah ter-scope tanggal — saat board kosong (mis. tidak ada
kunjungan hari ini), dropdown poliklinik jadi kosong meski `Poli Umum` ada di
master data. Sekarang opsi select diambil dari **definisi field entity**
(relation → master data entity terkait; enum → `enum_values`), sama seperti
perilaku Table.

Ditambah kontrak generik opsi "All" (clear) pada `FilterSpec`:

```yaml
filters:
  - field: polyclinic_id
    type: select
    all_label: "(ALL)"   # caption opsi All — default "(ALL)"
    show_all: false      # opsional — sembunyikan opsi All; default true
```

### Renderer
- **`hooks/useSelectFilterOptions.ts`** (baru) — derivasi opsi select bersama
  (relation fetch id+label_field / enum_values), dipakai Table & Kanban.
- **`KanbanRenderer.tsx`** — `KanbanFilterControl` sub-component (select/date/
  text); opsi select dari master data + opsi All; `getColumnRecords` kini
  mencocokkan **raw field value** (id uuid untuk `_id`), selaras dengan filter
  server — bukan lagi label relation (bug lama: id vs name tidak pernah match).
- **`TableRenderer.tsx`** — `FilterControl` select memakai hook bersama;
  label All `"(ALL)"` (default), dihormati `show_all`/`all_label`.

### Schema & Docs
- `FilterSpec` + `show_all` (`*bool`, default true) + `all_label` (default
  `"(ALL)"`) di `pkg/spec/frontend.go` & `types/manifest.ts`; schema JSON
  di-regenerate.
- `docs/spec/frontend/06-page-kinds.md` §3.3 + baris `show_all`/`all_label`.
- Test `lib/filters.test.ts` +3 kasus (`shouldShowAll`/`allLabel`).

## File Terkena

- `renderers/web/src/hooks/useSelectFilterOptions.ts` (baru)
- `renderers/web/src/kinds/kanban/KanbanRenderer.tsx`
- `renderers/web/src/kinds/table/TableRenderer.tsx`
- `renderers/web/src/lib/filters.ts`, `.../lib/filters.test.ts`
- `renderers/web/src/types/manifest.ts`, `pkg/spec/frontend.go`, `schemas/*`
- `docs/spec/frontend/06-page-kinds.md`

Referensi: `docs/plan/kanban-filter-tanggal-filter-generik.md`, todo §5.5.9.
