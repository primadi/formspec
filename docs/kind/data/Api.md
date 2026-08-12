# Api

<!-- generated:meta -->
| | |
|---|---|
| Grup | `data` |
| Plane | `resource` |
| Spec struct | `ApiSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Api` adalah **override permukaan API external** untuk Entity yang sudah exposed.

**Kapan memakai Api:**
- Ubah `base_path` / `version` permukaan REST external
- Nonaktifkan REST (`rest.disable`) sambil tetap biarkan surface lain (gRPC/ws)
- Konfigurasi detail exposure per protocol

**Kapan TIDAK pakai Api:**
- Mengontrol apakah Entity bisa diakses external → `spec.expose` di Entity (§8.4)
- Permukaan UI (`/_ui/entity/`) — `kind: Api` TIDAK berlaku untuk UI surface

**Sumber kontrak:** [`docs/spec/backend/02-core-extended.md`](../spec/backend/02-core-extended.md) §12.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1alpha1
kind: Api
metadata:
  name: invoice-api
  module: billing
spec:
  entity: billing.invoice
  rest:
    base_path: /invoices
    version: v2
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `rest` | `ApiRESTConfig` | — |  |  |
| `grpc` | `ApiGRPCConfig` | — |  |  |

<!-- /generated:attributes -->

## Gotchas

- **`spec.expose` dulu, `kind: Api` kedua** — Api hanya meng-override CARA permukaan external dipublikasikan (base_path, version, disable), tidak membuka/menutup akses.
- **Tidak berlaku untuk permukaan UI** — UI selalu tersedia, gated permission.
- **Cross-ref:** [`docs/spec/backend/02-core-extended.md`](../spec/backend/02-core-extended.md) §12 · [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §8.4 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
