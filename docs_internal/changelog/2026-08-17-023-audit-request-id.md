# Audit trail request_id (todo 4.7.2)

**Date**: 2026-08-17

Menambahkan `request_id` ke audit trail.

- `renderers/jsonb-persist/migrate.go`: kolom `request_id` di tabel
  `formspec_audit_log`.
- `renderers/jsonb-persist/audit.go`: `AuditRecord.RequestID`, `writeAuditLog`
  menerima requestID, `ListByEntity`/`ListByWorkspace` scan request_id.
- `renderers/jsonb-persist/crud.go`: `InsertParams.RequestID`/`UpdateParams.RequestID`
  diteruskan ke `writeAuditLog`.
- `internal/api/handler.go`: `HandleCreate`/`HandleUpdate` mengisi RequestID
  dari `requestIDFromContext(ctx)`.
- Test: `renderers/jsonb-persist/audit_test.go` (update signature).

Actor, action name, timestamp, dan before/after diff sudah ada sebelumnya.
