# 2026-09-11-001 — Stamp versi di `make build` + usage "not implemented" basi

## Apa

1. `make build-formspec` kini meng-stamp versi via `-ldflags "-X
main.version=$(VERSION)"` (nilai dari `git describe --tags --always
--dirty`). Sebelumnya build lokal selalu mencetak `formspec dev`.
2. Daftar "Not yet implemented" di `usage()` CLI diperbarui — `module`,
   `sign`, dan `workspace` sudah lama terimplementasi tetapi masih tercantum.
3. `docs/guides/releasing.md` §Rollback diperjelas: tag pada release yang
   masih **draft** boleh dihapus dan dipakai ulang (`gh release delete
--cleanup-tag` + `git tag -d`), sedangkan tag published tidak boleh.

## Kenapa

`formspec version` di binary lokal menampilkan `dev` sehingga terkesan command
belum implemented; sebenarnya `version` sudah ada sejak lama (binary di
`bin/` hanya stale). Klarifikasi draft-tag dicatat karena muncul di sesi
review release v0.0.5.

## File terdampak

- `Makefile` (build-formspec)
- `cmd/formspec/main.go` (usage)
- `docs/guides/releasing.md`
