# Table Renderer: Implementasi Filters, Row Action Icons, dan Bulk Actions

**Date:** 2026-07-27
**Plan ref:** - (langsung implementasi dari issue)

## Changes

`TableRenderer.tsx` — tiga fitur Table kind yang belum diimplementasikan di renderer:

### 1. Row Action Icons (fix)
`ActionIcon` sebelumnya hanya mengenali action name (`view`/`edit`/`delete`) untuk mapping ke lucide-react icons. Sekarang menerima `icon` prop dari manifest (e.g. `eye`, `play`, `check`, `x`, `download`) dan melakukan mapping ke komponen icon yang sesuai. Backward compatible — fallback ke action name jika icon tidak disediakan.

### 2. Filters (new)
- Menambahkan `filterValues` state dan `FilterBar` component
- Support 3 tipe filter: `select` (dropdown dari `enum_values` entity), `date_range` (native date input), `text` (text input)
- Filter values dikirim sebagai query params via `ListParams.filters`
- Tombol "Reset" untuk membersihkan semua filter
- Perubahan filter otomatis mereset page ke 1

### 3. Bulk Actions (new)
- Menambahkan `rowSelection` state via TanStack Table's built-in row selection
- Checkbox column (`__select`) muncul otomatis jika `bulk_actions` didefinisikan
- `BulkActionsBar` muncul ketika satu atau lebih row dipilih
- Menampilkan jumlah selected + action buttons + tombol Clear

## Files affected
- `renderers/web/src/kinds/table/TableRenderer.tsx` — major changes

## Testing
- Backend meta API mengembalikan data yang benar (4 filters, 4 row_actions, 1 bulk_action)
- Frontend render semua komponen dengan benar (verified via browser)
- TypeScript compilation clean (`npx tsc --noEmit`)
