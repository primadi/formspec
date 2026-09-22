# 4.1 — Jalur tulis summary untuk script pemelihara: `resource.upsert` (Opsi A)

**Tanggal:** 2026-09-20 · **Plan:** `docs_internal/plan/summary-maintainer-write-path.md`
· **TODO:** `examples/kafe/gaps_found/TODO.md` 4.1 · **Keputusan:** Opsi A
(pemilik proyek, 2026-09-18)

## Apa yang diubah

Entity `summary` kini punya **satu** jalur tulis yang didukung, khusus untuk
script yang dideklarasikan `maintained_by`:

- `renderers/jsonb-persist/crud.go` — `EntityStore.UpsertProjection(ctx,
  workspaceID, match, data)`: upsert by match (AND), hanya untuk summary,
  atomik (baca-lalu-tulis dalam satu transaksi), tidak diekspos ke API.
- `internal/starlark/resource.go` — builtin `resource.upsert` + `upsertFn` +
  `SetUpsertFunc`.
- `internal/starlark/executor.go` — `UpsertHandler` + `MaintainerRef`.
- `internal/action/script.go` — `SetUpsertHandler`; `MaintainerRef()` meneruskan
  `action.Impl.Ref` script yang sedang berjalan.
- `resource/formspec.go` — wiring + aturan pemanggil.
- `docs/spec/backend/02-core-extended.md` §6.1 — normatif.
- `examples/kafe/spec/.../stock_level_apply.star` — ditulis ulang memakai
  `resource.find` + `resource.upsert`.
- `examples/kafe/spec/.../stock-movement/entity.yaml` — hook `after create`.

## Kenapa

`maintained_by` menyebut script pemelihara, tetapi tidak ada cara sah untuk
menulis proyeksinya: `EntityStore.Insert/Update/SoftDelete` menolak
`CharSummary`, dan `resource.create`/`save` melewati guard yang sama. Akibatnya
valuasi inventory (4.1) tidak bisa dijalankan, dan dua summary kafe lain
(`menu-cost`, `member-point`) juga tidak bisa diisi.

Opsi A mewujudkan kontrak `maintained_by` yang sudah ada di spec: script itu
**boleh** menulis proyeksinya, dan hanya script itu. Summary tetap read-only
bagi semua orang lain.

## Aturan pemanggil

- Hanya entity `characteristic: summary` (entity lain ditolak).
- Hanya script yang **disebut `maintained_by`** entity itu (`MaintainerRef` ==
  `MaintainedBy`). Script lain → error.
- Tetap menghormati tenant isolation + `row_scope`; `invariants` (unique index)
  tetap ditegakkan.

## Dua bug yang ikut ditemukan & diperbaiki

1. `UpsertProjection` menulis kolom `is_active` yang tidak ada — `is_active`
   adalah field di dalam `data` (soft-deactivate), bukan kolom tabel. Insert
   gagal `no column named is_active`.
2. `FindByFields` mengembalikan `ErrNotFound` alih-alih `nil` saat tidak ada
   baris → `resource.find` melempar error pada pergerakan pertama (sebelum baris
   ada), sehingga pemelihara tidak pernah jalan. Kini `(nil, nil)`.

## Bukti

| Perintah | Hasil |
| --- | --- |
| `go test ./renderers/jsonb-persist/ -run 'UpsertProjection\|FindByFields_NoMatch'` | 4 PASS |
| `go test ./internal/starlark/ -run TestResourceAPI_Upsert` | 2 PASS |
| `go test ./resource/ -run TestSummaryUpsert` | 2 PASS — maintainer menulis (5+3=8), non-maintainer ditolak |
| `formspec validate` kafe | **0 problem** (69 manifest) |
| `go test ./...` | hijau |
