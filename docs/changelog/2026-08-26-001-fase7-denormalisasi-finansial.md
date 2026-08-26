# 2026-08-26-001 — Fase 7: Denormalisasi Finansial (7.10)

## Apa yang diubah

Implementasi **denormalisasi field finansial** (kontrak
`docs/spec/backend/02-core-extended.md` §1.1) — field `characteristic: master`
yang memengaruhi perhitungan finansial pada Entity transaksi **disalin
(snapshot)** ke transaksi saat `create`/`submit`, bukan live-join.

**Spec** (`pkg/spec/entity.go`) — `RelationDecl.Snapshot []SnapshotField`
(`{from, as}`, default `as` = `from`) pada field relation `belongs_to`.
Contoh:

```yaml
fields:
  - name: customer_id
    type: relation
    relation:
      type: belongs_to
      resource: customer
      snapshot:
        - { from: tier, as: customer_tier_at_transaction }
        - { from: discount_rate, as: discount_rate_at_transaction }
```

**Store** (`renderers/jsonb-persist/crud.go`) — `applyFinancialSnapshot(ctx,
txdb, workspaceID, data)`: untuk setiap relation `belongs_to` dengan
`snapshot:`, resolve record master (query target table via txdb — transaksi
yang sama, tidak deadlock pada SQLite single-connection) dan salin field
deklarasi ke `data[as]`. Dipanggil di `Insert` (setelah
`ValidateRelationTargets`) dan `Submit` (re-snapshot sebelum UPDATE data +
doc_status).

**Schema** — `SnapshotField` ditambah ke `sharedTypes` generator schema;
schema di-regenerate (sekalian `CallbackDecl` dari 7.13 yang sebelumnya tidak
masuk `$defs` — bug generator: tipe baru harus didaftarkan di `sharedTypes`).

## Kenapa

Kalau nilai Master berubah kemudian (mis. customer naik tier), transaksi lama
yang sudah dibuat **tidak boleh** ikut berubah retroaktif. Snapshot terjadi
setiap transaksi (create/submit), berbeda dari master snapshot saat archiving.

## File terdampak

- `pkg/spec/entity.go` — `RelationDecl.Snapshot` + `SnapshotField`
- `renderers/jsonb-persist/crud.go` — `applyFinancialSnapshot` (Insert + Submit)
- `renderers/jsonb-persist/snapshot_test.go` (baru) — unit test
- `internal/genjsonschema/generator.go` — `SnapshotField` (+ `CallbackDecl`) di sharedTypes
- `schemas/formspec.schema.json` — regenerate
- `docs/plan/fase7-denormalisasi-finansial.md` (baru) — plan

## Referensi

- Todo: `docs/plan/todo.md` §7.10
- Plan: `docs/plan/fase7-denormalisasi-finansial.md`
- Spec: `docs/spec/backend/02-core-extended.md` §1.1