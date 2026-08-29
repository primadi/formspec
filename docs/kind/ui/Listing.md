# Listing

<!-- generated:meta -->
| | |
|---|---|
| Grup | `ui` |
| Plane | `resource` |
| Spec struct | `ListingSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Listing` adalah **katalog publik** (e-commerce, movie search) — pasangan alami App `access: public` (biasanya `app_renderer: no-nav`).

**Kapan memakai Listing:**

- Halaman publik yang bisa diakses tanpa auth-wrap (katalog, search)

**Kapan TIDAK pakai Listing:**

- Operasi tulis terautentikasi → `kind: Table`

**Sumber kontrak:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §10.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
kind: Listing
metadata: { name: product-catalog, module: shop }
spec:
  entity: shop.product
  columns:
    - { field: name, label: "Nama" }
    - { field: price, label: "Harga", format: currency }
  search: true
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `entity` | `string` | ✅ | shop.product |  |
| `columns` | []`TableColumn` | — |  |  |
| `filters` | []`FilterSpec` | — |  |  |
| `search` | `boolean` | — |  |  |

<!-- /generated:attributes -->

## Gotchas

- **Struktural mirip `table-list`** tapi tanpa asumsi Auth-wrap dari App renderer-nya.
- **Tanpa `row_actions`/`bulk_actions`** yang menyiratkan operasi tulis terautentikasi.
- **Renderer**: `src/kinds/listing/ListingRenderer.tsx` (read-only: search +
  filter dari manifest, tanpa create/row/bulk action; klik baris → detail
  entity). Route di `{basePath}/listing/{name}`.
- **Cross-ref:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §10 · [`docs/spec/frontend/05-app-kinds.md`](../spec/frontend/05-app-kinds.md) §4 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
