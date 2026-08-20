# `formspec backup` / `formspec restore` / `formspec logs` (todo 3.7.1–3.7.4)

**Date**: 2026-08-17
**Plan**: `docs/plan/formspec-repl-seed-diff.md`

Mengimplementasikan verb data-lifecycle CLI (sebelumnya stub).

- `cmd/formspec/backup.go` (baru): `backup create --full` (tar open-format:
  `manifest.json` + `<module>_<entity>.jsonl` per entity), `backup inspect`
  (baca manifest), `restore --from --conflict skip|overwrite --dry-run`
  (idempotent via natural key; overwrite = update record yang ada).
- `cmd/formspec/logs.go` (baru): `logs` baca event log
  (`formspec_event_log`, channel audit_log) dengan filter
  workspace/module/entity + output pretty|json.
- `cmd/formspec/main.go`: dispatch + usage.
- Test: `backup_test.go` (create→inspect→restore round-trip, idempotent,
  dry-run), `logs_test.go` (event log round-trip + filter).

Gap yang dicatat (bukan blocker): `--incremental`/`--filter` backup, file
storage (ctx.storage) ikut backup (4.8.1), `--map-resource`/`remap` restore,
dan filter logs `--action/--level/--since/--until/--request-id/--follow`
(full 12-field request logging = Fase 8.2).
