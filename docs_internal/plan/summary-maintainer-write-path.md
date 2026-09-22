# Plan — 4.1: Jalur tulis summary untuk script pemelihara (Opsi A)

**Sumber:** `examples/kafe/gaps_found/TODO.md` 4.1 (BLOCKED) · keputusan pemilik
proyek 2026-09-18: **Opsi A** — jalur tulis internal khusus pemelihara.
**Referensi spec:** `docs/spec/backend/02-core-extended.md` §6 (summary diisi
eksklusif lewat event durable) + §6.1 (`maintained_by` + `invariants`).

## Masalah

Entity `summary` tidak punya jalur tulis yang didukung:
- `EntityStore.Insert/Update/SoftDelete` menolak `CharSummary`
  (`renderers/jsonb-persist/crud.go:465,795,1000`).
- `maintained_by` menyebut script pemelihara, tetapi script itu tidak bisa
  menulis proyeksinya (`resource.create`/`save` melewati guard yang sama).
- `internal/summary` hanya **merencanakan** rebuild; tidak ada jalur tulis.

Akibatnya valuasi inventory (4.1) tidak bisa dijalankan, dan dua summary kafe
lain (`menu-cost`, `member-point`) juga tidak bisa diisi.

## Keputusan: Opsi A

Primitif Starlark baru **`resource.upsert(entity, match, data)`** yang:

1. **Hanya** boleh menargetkan entity `characteristic: summary` (entity lain
   ditolak — mereka punya action pipeline).
2. **Hanya** boleh dipanggil dari script yang **disebut `maintained_by`** entity
   itu. Script lain → error. Ini yang menjaga summary tetap read-only bagi
   semua orang kecuali pemeliharanya.
3. Melewati guard `CharSummary` lewat method store baru **`UpsertProjection`**
   yang tidak diekspos ke API.
4. Tetap menghormati tenant isolation + `row_scope`, dan tetap menegakkan
   `invariants` (unique index).

## File yang akan dibuat/diubah

| File | Perubahan |
| --- | --- |
| `renderers/jsonb-persist/crud.go` | `EntityStore.UpsertProjection(ctx, workspaceID, match, data)` — upsert by match, hanya untuk summary |
| `internal/starlark/resource.go` | builtin `resource.upsert` + `upsertFn` + `SetUpsertFunc` |
| `internal/starlark/executor.go` | `UpsertHandler` + `MaintainerRef` (ref script yang sedang jalan) |
| `internal/action/script.go` | `SetUpsertHandler`; teruskan `action.Impl.Ref` sebagai maintainer ref |
| `resource/formspec.go` | wiring: verifikasi `ref == entity.MaintainedBy` sebelum upsert |
| `docs/spec/backend/02-core-extended.md` §6.1 | normatif: `resource.upsert` + aturan pemanggil |
| `examples/kafe/spec/.../stock_level_apply.star` | pakai `resource.upsert` (bukan raw SQL) |
| `examples/kafe/spec/.../stock-movement/entity.yaml` | hook `after create` → panggil pemelihara |
| test | `TestUpsertProjection_*`, `TestResourceAPI_Upsert_*` |

## Dependensi

- 4.3 (`resource.find`) ✅ — pola handler yang sama.
- 4.5 (`ctx.db` scope-aware) ✅ — `UpsertProjection` memakai `txReadDB`/`runTx`.
- 4.6 (`unit.factors`) ✅ — konversi untuk ledakan resep.

## Effort

**Large** — menyentuh store, Starlark, action, wiring, spec kafe, docs, test.

## Accept

- `stock-movement` create → `stock-level` ter-upsert dengan rata-rata bergerak
  benar (masuk mengubah avg; keluar tidak).
- Script **bukan** pemelihara yang memanggil `resource.upsert` → **error**.
- `resource.upsert` ke entity **bukan** summary → **error**.
- `formspec validate` kafe **0 problem**; `go test ./...` hijau.
