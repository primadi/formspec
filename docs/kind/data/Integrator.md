# Integrator

<!-- generated:meta -->
| | |
|---|---|
| Grup | `data` |
| Plane | `resource` |
| Spec struct | `IntegratorSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Integrator` adalah **jembatan reaktif lintas-module** — subscribe event + call action resource lain (reactive bridge).

**Kapan memakai Integrator:**
- Reaksi otomatis lintas bounded context (mis. sales → inventory, sales → GL)
- Alur request/response antar module yang butuh kompensasi (`compensate`)

**Kapan TIDAK pakai Integrator:**
- Reaksi fire-and-forget ke event → `kind: Subscription`
- Reaksi sesama module → event `on_*` di Entity

**Sumber kontrak:** [`docs/spec/backend/02-core-extended.md`](../spec/backend/02-core-extended.md) §5.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
kind: Integrator
metadata:
  name: sales-to-gl
  module: sales
spec:
  listen:
    resource: sales.order
    event: on_submit
  call:
    resource: gl.journal-entry
    action: create
  compensate:
    resource: gl.journal-entry
    action: cancel
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `listen` | `IntegratorListen` | ✅ |  |  |
| `call` | `IntegratorCall` | ✅ |  |  |
| `compensate` | `string` | — | gl.journal-entry.cancel |  |

<!-- /generated:attributes -->

## Gotchas

- **`compensate` penting** — alur lintas module yang gagal di tengah harus punya jalur kompensasi, jangan tinggalkan state konsisten.
- **Bedanya dgn Subscription**: Integrator = bridge reaktif (listen + call + compensate); Subscription = reaksi event sederhana.
- **Cross-ref:** [`docs/spec/backend/02-core-extended.md`](../spec/backend/02-core-extended.md) §5 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
