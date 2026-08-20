# `formspec diff` — compare local vs deployed (todo 3.4.1)

**Date**: 2026-08-17
**Plan**: `docs/plan/formspec-repl-seed-diff.md`

Mengimplementasikan `formspec diff -f <path>` (sebelumnya stub). Dalam scope
single-server (tanpa Control Plane), "deployed" = schema yang sudah
ter-materialisasi di database; verb ini memakai `MigrationRunner.PlanMigrations`
sebagai diff structural dry-run (field add/remove/type-change) tanpa mengubah
apapun.

- `cmd/formspec/diff.go` (baru): reuse `loadEntityMigrations`, cetak DDL
  pending per entity; exit 0 bila tidak ada perbedaan, exit 1 bila ada (bisa
  jadi gate CI).
- `cmd/formspec/main.go`: dispatch + usage.
- `cmd/formspec/diff_test.go`: laporan perbedaan pada DB fresh, nol perbedaan
  setelah apply.

Interpretasi "deployed" (schema DB vs manifest lokal) didokumentasikan di plan
file — menunggu Control Plane untuk diff artifact-to-artifact yang sebenarnya.
