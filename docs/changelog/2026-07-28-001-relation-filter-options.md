# Relation-based Filter Options for Select Filters

## Changes
- **`renderers/web/src/kinds/table/TableRenderer.tsx`** — `FilterControl` component
  - Added relation-based option fetching for `select` type filters on relation fields
  - When a filter field is a relation type (e.g., `polyclinic_id`), the component now
    fetches options from the related entity's API endpoint using `getClient()`
  - Uses the related entity's `label_field` (defaults to `"name"`) for display labels
  - Falls back to showing only "All" if the fetch fails silently

## Problem
Filter dropdown for `polyclinic_id` (a relation field, not enum) only showed "All"
because `FilterControl` only derived options from `fieldDef?.enum_values`, which is
empty for relation fields.

## Resolution
When the field type is `relation`, find the related entity in `metaBundle`, call its
API with `per_page=500`, and populate the select options with `{ value: id, label: label_field }`.
