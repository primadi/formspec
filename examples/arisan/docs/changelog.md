# Changelog — Aplikasi Arisan

Semua perubahan dicatat dengan format `YYYY-MM-DD`. Versi aplikasi mengikuti
`spec/apps/arisan.yaml` → `spec.version` (saat ini `1.0.0`).

## [1.0.0] — 2026-08-03

### Added
- **17 manifest** tervalidasi (`formspec validate` → 17 OK, 0 problems):
  - `kind: App` — `spec/apps/arisan.yaml` (root `/app/arisan`, menu 3 modul)
  - 3 `kind: Module` — `arisan-master`, `arisan-field`, `arisan-report`
  - 7 `kind: Entity`:
    - `arisan-master`: `arisan-group`, `member`, `group-member`
    - `arisan-field`: `bank-mutation`, `contribution`, `arisan-period`, `draw`
  - 1 `kind: Dashboard` — `arisan-summary-dashboard` (4 widget)
  - 4 `kind: Widget` — `active-groups`, `open-periods`, `paid-count`, `pending-count`
  - 1 `kind: Report` — `payment-recap-report`
- **2 script Starlark** (custom action):
  - `contribution/scripts/validate.star` — validasi iuran vs mutasi bank
  - `arisan-period/scripts/run-lottery.star` — undian & tutup periode
- **State machine** pada 4 entity: `arisan-group` (active→completed),
  `contribution` (pending→validated/rejected), `arisan-period` (open→closed),
  `draw` (drawn→paid_out)
- **Event async** → channel `audit_log`: `validated`, `rejected`, `drawn`

### Changed
- Semua entity memakai `lifecycle: plain_crud` + `actions: [{name: submit,
  disabled: true}]` agar lifecycle-free (relasi ke record draft ditolak engine).
- `contribution` expose dibatasi `[list, find, create]` — status hanya diubah
  lewat aksi.

### Fixed
- **Bug engine (SQLite deadlock)**: aksi custom `resource.fetch()` pada entity
  berelasi deadlock di SQLite. Dipatch lokal di module cache
  (`resolveRelations` → `txReadDB(ctx, s.db)`) + rebuild `formspec.exe`.
  Detail: [`engine-sqlite-deadlock.md`](./engine-sqlite-deadlock.md).
- Flowchart Mermaid `run-lottery` — label node dengan `(...)` diperbaiki jadi
  `:` (validasi Mermaid).

### Verified (smoke test end-to-end)
- CRUD semua entity berfungsi (SQLite).
- Aksi `validate`: HTTP 200 7 ms — contribution `pending→validated` +
  `matched_mutation_id`, mutation `unmatched→matched` + `matched_contribution_id`.
- Aksi `run-lottery`: HTTP 200 7 ms — draw dibuat, period `open→closed`.
- Guard: `run-lottery` ulang pada periode tertutup → HTTP 422
  `INVALID_TRANSITION`.
- UI aplikasi `/default/app/arisan` — menu Master/Transaksi/Laporan render.

### Known Limitations
- Widget metric menampilkan `--` (evaluasi query widget belum diimplementasikan
  engine build ini).
- Query builder `<Entity>.query()` dan primitif `ctx.db`/`ctx.cache`/`ctx.lock`/
  `ctx.queue` adalah stub.
- Patch engine SQLite ada di module cache (tidak ter-commit; hilang bila
  `go clean -modcache`).

### Added (docs)
- Folder `docs/` lengkap: README, overview, architecture, domain-model,
  automation, permissions, development, changelog, engine-sqlite-deadlock.
