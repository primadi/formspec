# Query Builder — aggregate + window functions (todo 4.5.1–4.5.6)

**Date**: 2026-08-17

Mengimplementasikan aggregate + window query di jsonb-persist.

- `renderers/jsonb-persist/crud.go`:
  - `EntityStore.Aggregate(ctx, AggregateParams)` — `sum`/`count`/`avg`/`min`/`max`,
    `GroupBy []string`, `Having []FilterOp` (post-aggregation, ke ekspresi
    agregat), `Filters map[string]FilterOp`, `DateTrunc *DateTruncDecl` (4.5.4 —
    PG `date_trunc`, SQLite `strftime`; unit day/week/month/quarter/year).
  - `EntityStore.Window(ctx, WindowParams)` — `running_total`/`rank`/`row_number`,
    `PartitionBy`/`OrderBy` (4.5.5).
  - `derefAny` helper untuk unwrap hasil scan.
- Test: `renderers/jsonb-persist/aggregate_test.go` (sum/count/avg/min/max,
  group_by, having, date_trunc, running_total, rank, unknown function/unit).
- 4.5.6 (`include()` batched) diverifikasi sudah terimplementasi via
  `resolveRelations` (batch-fetch per relation field, bukan per record).
