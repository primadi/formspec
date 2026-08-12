# Migration

<!-- generated:meta -->
| | |
|---|---|
| Grup | `data` |
| Plane | `resource` |
| Spec struct | `MigrationSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Migration` adalah **structural change** untuk DDL yang tidak dicakup field Entity.

**Kapan memakai Migration:**
- Index custom, function, trigger, extension, materialized view
- DDL yang melebihi apa yang diekspresikan field definitions

**Kapan TIDAK pakai Migration:**
- Perubahan struktur biasa (tambah field, ganti type) → framework hitung structural diff otomatis
- Data backfill → data migration (script ber-versi), bukan structural diff
- **DML ditolak saat runtime** — Migration murni DDL

**Sumber kontrak:** [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §4.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1alpha1
kind: Migration
metadata:
  name: add-invoice-index
  module: billing
spec:
  up: "CREATE INDEX idx_invoice_date ON invoice(transaction_date)"
  down: "DROP INDEX idx_invoice_date"
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `ddl` | `string` | ✅ | CREATE INDEX idx_invoice_date ON invoice(transaction_date) |  |
| `module` | `string` | — | billing |  |

<!-- /generated:attributes -->

## Gotchas

- **Tiga jenis migrasi**: structural (otomatis dari diff Entity, tidak pernah ditulis tangan) · custom DDL (`kind: Migration`) · data migration (script ber-versi).
- **DML ditolak saat runtime** — hanya DDL.
- **Structural diff dihasilkan framework** dari perubahan spec; PersistBackend menerjemahkan ke storage-nya (bukan "framework generate SQL").
- **Cross-ref:** [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §4 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
