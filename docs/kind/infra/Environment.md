# Environment

<!-- generated:meta -->
| | |
|---|---|
| Grup | `infra` |
| Plane | `control` |
| Spec struct | `EnvironmentSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Environment` adalah **target deployment** (dev, staging, production).

**Kapan memakai Environment:**
- Mendeklarasikan target deployment + konfigurasinya

**Kapan TIDAK pakai Environment:**
- Menyusun data → `kind: Entity`; aturan governance → `kind: Policy`

> ⚠️ **Control Plane kind** — dikelola oleh Platform Operator, bukan app developer.

**Sumber kontrak:** [`docs/spec/platform/04-control-plane.md`](../spec/platform/04-control-plane.md).

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
kind: Environment
metadata:
  name: production
spec:
  # dikelola Platform Operator
  plane: resource
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `mode` | enum (dev · prod) | ✅ | prod | Environment mode |
| `tier` | enum (standalone · cloud · enterprise) | ✅ | cloud | Deployment tier |
| `resource_pool` | enum (shared · exclusive) | ✅ | shared | Resource pool strategy |
| `resource_planes` | []`EnvironmentPlane` | — |  | ResourcePlanes lists the resource plane endpoints for this environment. |
| `key_ref` | `string` | ✅ | kms://prod-signing | KeyRef is the location of the platform signing key (e.g. kms://prod-signing). |
| `policy` | `string` | ✅ |  | Policy names the kind: Policy that applies to this environment. |

<!-- /generated:attributes -->

## Gotchas

- **Control Plane kind — TIDAK boleh membaca data bisnis atau eksekusi handler bisnis.**
- **Dikelola Platform Operator** — app developer tidak menulis ini langsung.
- **Cross-ref:** [`docs/spec/platform/04-control-plane.md`](../spec/platform/04-control-plane.md) · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
