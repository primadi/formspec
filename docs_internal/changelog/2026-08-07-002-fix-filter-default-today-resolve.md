# Fix: filter `default: today` tidak ter-resolve → date input tampil mm/dd/yyyy

## Perubahan

`resolveFilterValue()` di `renderers/web/src/lib/filters.ts` hanya menerjemahkan
`today()` (dengan parens), padahal manifest memakai `default: today` (tanpa
parens). Akibatnya nilai filter = string literal `"today"`:

- **UI:** `<input type="date">` menerima value invalid `"today"` → tampil
  placeholder `mm/dd/yyyy`, bukan tanggal aktual.
- **Filter server:** request menjadi `transaction_date[eq]=today` → backend
  tidak me-resolve `today` → tidak ada record yang cocok.

## Perbaikan

- `resolveFilterValue()` kini menerima **`today`** maupun **`today()`** →
  tanggal server (`serverToday()`, UTC) — konvensi sama dengan widget query.
- Test unit baru `renderers/web/src/lib/filters.test.ts` (9 kasus: resolve,
  fixed_filters, user filters).
- Docstring Go `FilterSpec.Default` + spec `06-page-kinds.md` §3.3 disinkronkan
  (`today` / `today()`); schema JSON di-regenerate.

## File Terkena

- `renderers/web/src/lib/filters.ts`, `renderers/web/src/lib/filters.test.ts` (baru)
- `pkg/spec/frontend.go`, `schemas/*`
- `docs/spec/frontend/06-page-kinds.md`

Referensi: `docs/plan/kanban-filter-tanggal-filter-generik.md`, todo §5.5.9.
