# Fix: derived table columns now resolve belongs_to relations automatically

**Date:** 2026-07-28

## What changed

Modified `deriveTable()` in `renderers/web/src/engine/derive.ts` so that
`belongs_to` relation fields produce dot-path column references (e.g.
`polyclinic.name`) instead of the raw foreign-key field name
(`polyclinic_id`).

## Why

When a Table manifest has no explicit `columns`, the renderer falls back to
derived columns from the entity schema. Previously, relation fields were
passed verbatim as `polyclinic_id`, causing the table to display the raw
UUID. Authored tables like the visit table worked correctly because they
explicitly declare `field: polyclinic.name`.

## Files affected

- `renderers/web/src/engine/derive.ts` — column derivation loop now
  detects `belongs_to` relations and converts them to dot-path notation
  (`{alias}.name`)

## Reference

- User reported that the derived Doctor table shows raw UUID for
  `polyclinic_id`, while the authored Visit table resolves it correctly.
- Backend `resolveRelations()` already nests related data under the alias
  (e.g. `polyclinic: {id, name, …}`), so dot-path accessor works end-to-end.
