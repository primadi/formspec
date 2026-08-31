# Fase 7 — Denormalisasi Finansial (7.10)

**Status:** ✅ Complete · **Tanggal:** 2026-08-26
**Referensi:** `docs/spec/backend/02-core-extended.md` §1.1 (Denormalisasi
Field Finansial), `pkg/spec/entity.go` (RelationDecl),
`renderers/jsonb-persist/crud.go` (EntityStore)
**Todo:** `docs/plan/todo.md` §7.10

## Konteks

Field `characteristic: master` yang memengaruhi perhitungan finansial pada
Entity transaksi (harga, diskon, tier, tarif pajak) **wajib disalin
(snapshot)** ke Entity transaksi saat `create`/`submit` — **tidak boleh**
dibaca ulang lewat live-join. Kalau nilai Master berubah kemudian, transaksi
lama **tidak boleh** ikut berubah retroaktif.

## Prinsip desain

- Deklaratif: blok `snapshot:` pada field relation `belongs_to` —
  `{from: <master field>, as: <target field>}` (default `as` = `from`).
- Snapshot terjadi di `create` (Insert) dan `submit` (Submit) — nilai master
  "as-of" saat transaksi dibuat/disubmit, bukan live-join.
- Baca master lewat transaksi yang sama (txdb) — tidak deadlock pada SQLite
  single-connection.

## Scope

### SNAP-1 — Spec (`pkg/spec/entity.go`)

- `RelationDecl.Snapshot []SnapshotField` — deklarasi snapshot per relation.
- `SnapshotField{From, As}` — master field → target field.

### SNAP-2 — Store (`renderers/jsonb-persist/crud.go`)

- `applyFinancialSnapshot(ctx, txdb, workspaceID, data)` — untuk setiap
  relation `belongs_to` dengan `snapshot:`, resolve record master (query
  target table via txdb) dan salin field deklarasi ke `data[as]`.
- Dipanggil di `Insert` (setelah `ValidateRelationTargets`) dan `Submit`
  (re-snapshot sebelum UPDATE data + doc_status).

### SNAP-3 — Schema + test

- `SnapshotField` ditambah ke `sharedTypes` generator schema; schema
  di-regenerate.
- `renderers/jsonb-persist/snapshot_test.go` — `TestFinancialSnapshot_OnCreate`
  (snapshot di create) + `TestFinancialSnapshot_OldTransactionUnaffected`
  (master berubah → transaksi lama tidak ikut berubah).

## Level of effort

| SNAP | Effort |
| ---- | ------ |
| 1    | small  |
| 2    | medium |
| 3    | small  |

## Verifikasi

- `go test ./renderers/jsonb-persist/ -run TestFinancialSnapshot` hijau.
- `go test ./...` hijau (28 paket).

## File terdampak

- `pkg/spec/entity.go` — `RelationDecl.Snapshot` + `SnapshotField`
- `renderers/jsonb-persist/crud.go` — `applyFinancialSnapshot` (Insert + Submit)
- `renderers/jsonb-persist/snapshot_test.go` (baru) — unit test
- `internal/genjsonschema/generator.go` — `SnapshotField` di sharedTypes
- `schemas/formspec.schema.json` — regenerate
- `docs/plan/fase7-denormalisasi-finansial.md` (baru) — plan
