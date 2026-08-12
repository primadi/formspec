# PersistBackend

<!-- generated:meta -->
| | |
|---|---|
| Grup | `infra` |
| Plane | `resource` |
| Spec struct | `PersistBackendSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: PersistBackend` adalah **implementasi seam penyimpanan** — setara dengan Shell di sisi visual.

**Kapan memakai PersistBackend:**
- Implementasi penyimpanan (JSONB on Postgres/SQLite)
- Memenuhi kontrak `04-persist-backend.md` (transaksi, natural key, outbox, index generation)

**Kapan TIDAK pakai PersistBackend:**
- Koneksi database bernama → `kind: Datastore`
- Menyusun data bisnis → `kind: Entity`

**Sumber kontrak:** [`docs/spec/backend/04-persist-backend.md`](../spec/backend/04-persist-backend.md).

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1alpha1
kind: PersistBackend
metadata:
  name: jsonb-persist
  module: formspec/rendering
spec:
  implements: formspec/storage.entity-persist
  trust_tier: official
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `implements` | `string` | ✅ | formspec/storage.entity-persist |  |
| `trust_tier` | enum (official · verified · community) | ✅ | official |  |

<!-- /generated:attributes -->

## Gotchas

- **Spes storage-agnostic** — contoh SQL hidup di dokumentasi renderer (`docs/renderers/jsonb-persist/`), bukan di spec.
- **Kontrak wajib**: transaksi atomik, `ctx.next_key` (gap-free/atomik), tabel outbox + worker, index generation, type-aware sort/filter.
- **Cross-ref:** [`docs/spec/backend/04-persist-backend.md`](../spec/backend/04-persist-backend.md) · [`docs/spec/platform/06-datastore.md`](../spec/platform/06-datastore.md) · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
