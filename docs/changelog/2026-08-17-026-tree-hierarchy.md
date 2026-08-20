# Tree/hierarchy (todo 4.6.1–4.6.3)

**Date**: 2026-08-17

Mengimplementasikan tree/hierarchy di jsonb-persist.

- `renderers/jsonb-persist/ddl.go`: field relation `tree: true` → kolom
  `_tpath_{field}` (materialized path) + index.
- `renderers/jsonb-persist/crud.go`:
  - `setTreePaths` — hitung path (`""` root / `parent.child.grandchild`) saat
    insert DAN update (reparent); cycle detection (4.6.3) menolak bila path
    parent mengandung id record.
  - Filter ops tree di `List`: `descendant_of` (`LIKE 'prefix.%'`),
    `child_of` (FK query), `root` (`parent_id IS NULL`).
- Test: `renderers/jsonb-persist/tree_test.go` (materialized path, root,
  child_of, descendant_of, cycle detection).
