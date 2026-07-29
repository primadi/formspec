# Fix: queue_number UNIQUE constraint violation on daily reset

## What

Fixed `queue_number` format on `visit` entity to include date components, preventing UNIQUE constraint violations when the daily-reset counter generates the same sequence number on different days.

## Why

The `queue_number` field had `reset: daily` with format `{prefix}-{seq:03d}` (producing "Q-001"). The counter resets each day, so day 2 would also generate "Q-001". However, the DDL creates a UNIQUE index on `(tenant_id, _queue_number)`, causing a `SQLITE_CONSTRAINT_UNIQUE` (2067) error on the second day's first insert.

## Files Changed

- `examples/Clinic-UI-Showcase/spec/modules/clinic/transaction/visit/entity.yaml` — format changed to `{prefix}{year}{month}{day}-{seq:03d}` (produces "Q20260728-001")
- `internal/entity/testdata/Clinic-UI-Showcase/spec/modules/clinic/transaction/visit/entity.yaml` — same format change
- `examples/Clinic-UI-Showcase/clinic_e2e_test.go` — updated regex to match new format
- `internal/entity/testdata/Clinic-UI-Showcase/clinic_e2e_test.go` — updated regex to match new format
- `renderers/jsonbpersist/counter_test.go` — updated comment referencing old format

## References

- Plan: N/A (direct bug fix)
- Spec: `docs/spec/02-core-basic.md` §2 (natural key rules)