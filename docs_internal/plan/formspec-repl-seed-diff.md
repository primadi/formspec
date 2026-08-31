# Plan: `formspec repl`, `formspec seed`, `formspec diff`

**Status**: Draft
**Date**: 2026-08-17
**Todo refs**: 3.6.2 (`repl`), 3.6.3 (`seed`), 3.4.1 (`diff`)
**Spec refs**: `docs/cli-tools/02-formspec-cli.md` §4 (`repl`), §6 (`seed`, `diff`)

## Scope

Tiga verb CLI yang selama ini stub (`not implemented yet` di `cmd/formspec/main.go`).
Semuanya beroperasi terhadap manifest lokal + engine yang sudah ada — tidak
membutuhkan Control Plane (yang di-defer).

## 1. `formspec repl [--environment <env>]` (3.6.2)

Console Starlark interaktif dengan akses `ctx.*` penuh. Menggunakan
`go.starlark.net` REPL loop terhadap globals yang persisten per sesi.

- Bangun `formspec.New(Config{SpecPath, DSN})` untuk dapat database + registry.
- Tambah getter `App.Database()` (ekspos `a.database`) agar command bisa
  membangun `ctxPrimitiveResolver` yang sama dengan `newDispatcher`.
- Predeclare `ctx` (CtxAPI dengan datastore resolver + `ctx.config`),
  `resource` (ResourceAPI kosong), `ok`/`fail` helper.
- Loop: baca baris, `starlark.ExecREPLChunk` / `starlark.ExecFile` terhadap
  globals persisten; cetak hasil ekspresi.
- `--environment` diterima (scope policy Control Plane di-defer; hanya
  diteruskan ke `ctx.environment` bila tersedia — di sini dicatat sebagai
  no-op dengan warning).

## 2. `formspec seed [--module <m>] [--spec <path>] [--dsn <dsn>]` (3.6.3)

Jalankan seeder YAML untuk data dev/testing. Format seed (baru, didokumentasikan
di sini karena `formspec/seed` official module belum ada):

```yaml
apiVersion: formspec.dev/v1
kind: Seed
metadata:
  name: demo-data
  module: billing
spec:
  entities:
    - entity: customer
      records:
        - { code: C-001, name: "PT Maju" }
```

- Load semua `kind: Seed` dari spec tree (via `manifest.NewLoader`).
- Untuk tiap record: `EntityStore.Insert` (natural key + field defaults).
- `--module` membatasi ke module tertentu.
- Idempotent: skip bila natural key sudah ada (warning), bukan error.

## 3. `formspec diff -f <path>` (3.4.1)

Bandingkan spec lokal dengan state yang sudah ter-deploy. Dalam scope
single-server (tanpa Control Plane), "deployed" = state yang sudah ada di
database (schema) vs manifest lokal. Ini adalah interpretasi dry-run yang
aman: tidak mengubah apapun.

- Load entity manifests lokal (`loadEntityMigrations`).
- `db.NewMigrationRunner.PlanMigrations` → cetak DDL yang akan berjalan
  (field ditambah/dihapus/tipe berubah) — sama seperti `migrate plan`.
- Output ringkas per entity: `+ field`, `- field`, `~ field type`.
- Exit 0 bila tidak ada perbedaan; exit 1 bila ada (agar bisa dipakai di CI).

## Files

- `cmd/formspec/repl.go` (baru)
- `cmd/formspec/seed.go` (baru)
- `cmd/formspec/diff.go` (baru)
- `cmd/formspec/main.go` (dispatch + usage)
- `resource/formspec.go` (tambah `App.Database()`)
- `pkg/spec/seed.go` (baru — `SeedSpec` struct) — atau inline di seed.go
- Test: `cmd/formspec/repl_test.go`, `seed_test.go`, `diff_test.go`

## Verification

- `rtk go test ./...`
- `make build`
- Manual smoke: `formspec repl`, `formspec seed`, `formspec diff` terhadap
  `examples/cafe/`.
