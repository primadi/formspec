# 4.3 — `resource.find()`: API find-by-field untuk Starlark (#31)

**Tanggal:** 2026-09-18 · **Plan:** `docs_internal/plan/fase-4-kafe-stok-hpp.md`
(4.3) · **TODO:** `examples/kafe/gaps_found/TODO.md` 4.3

## Apa yang diubah

Builtin Starlark baru `resource.find(entity, {field: value, ...})` yang mencari
satu record lewat **lapisan entity** dan mengembalikan resource atau `None`.

Rantai implementasi:

- `internal/starlark/resource.go` — `builtinFind` + `findFn` + `SetFindFunc`.
- `internal/starlark/executor.go` — `FindHandler`.
- `internal/action/script.go` — `SetFindHandler`.
- `renderers/jsonb-persist/crud.go` — `EntityStore.FindByFields` (multi-field
  AND, scope-aware via `txReadDB`); `FindByField` diperluas ke `value any`.
- `resource/formspec.go` — wiring + `checkCrossModuleUses`.

## Kenapa

Sebelum ini tidak ada API "find by field value": `resource.fetch(entity, id)`
butuh ID, bukan filter, sehingga guard keunikan terpaksa memakai
`ctx.db().query()` dengan SQL mentah — melanggar konvensi proyek "never raw SQL"
dan membuat script harus tahu nama tabel/kolom fisik. `resource.find` menutupnya:
pencariannya lewat lapisan entity, jadi tenant isolation & `row_scope` berlaku.

## File yang terkena dampak

- `internal/starlark/resource.go`, `internal/starlark/executor.go`,
  `internal/action/script.go`, `renderers/jsonb-persist/crud.go`,
  `resource/formspec.go` — implementasi.
- `internal/starlark/resource_test.go` — `TestResourceAPI_Find_ReturnsMatchAndNone`.
- `examples/kafe/spec/modules/cafe-master/scripts/guard_menu_item_price_unique.star`,
  `examples/kafe/spec/modules/cafe-order/scripts/guard_shift_open_unique.star` —
  ditulis ulang tanpa SQL.
- `examples/kafe/spec/modules/cafe-master/master/menu-item-price/entity.yaml`,
  `examples/kafe/spec/modules/cafe-order/transaction/shift/entity.yaml` —
  `uses: {primitives: [db]}` dihapus (tidak lagi menyentuh `ctx.db()`).
- `docs/spec/platform/02-workspace-app-module.md` §9.3 — dokumentasi.

## Bukti

| Perintah | Hasil |
| --- | --- |
| `go test ./internal/starlark/ -run TestResourceAPI_Find` | PASS |
| `formspec validate` kafe | **0 problem** (69 manifest) |
| `go test ./...` | hijau |

## Sisa

GAP-32 (item 4.4) tetap: guard masih SELECT-then-act; yang menutupnya adalah
UNIQUE index (database), bukan guard.
