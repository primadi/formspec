# `formspec seed` — run YAML seeders (todo 3.6.3)

**Date**: 2026-08-17
**Plan**: `docs/plan/formspec-repl-seed-diff.md`

Mengimplementasikan `formspec seed` (sebelumnya stub). Karena `formspec/seed`
official module belum ada, verb ini mendefinisikan format seed deklaratif
minimal (`kind: Seed`) dan menyisipkan record lewat `EntityStore` yang sama
dengan engine — natural-key generation, field defaults, dan validasi semua
berlaku.

- `cmd/formspec/seed.go` (baru): `SeedSpec`/`SeedEntity` struct, load semua
  `kind: Seed` dari spec tree, insert per record, idempotent (skip bila
  natural key sudah ada), filter `--module`.
- `cmd/formspec/main.go`: dispatch + usage.
- `cmd/formspec/seed_test.go`: insert + skip idempotent, filter module.

Format seed didokumentasikan di plan file (belum ada di `docs/cli-tools/` —
ditandai sebagai format baru sampai `formspec/seed` official module landing).
