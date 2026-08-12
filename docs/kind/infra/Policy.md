# Policy

<!-- generated:meta -->
| | |
|---|---|
| Grup | `infra` |
| Plane | `control` |
| Spec struct | `PolicySpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Policy` adalah **aturan governance** (security, compliance, resource limits).

**Kapan memakai Policy:**
- Governance rules yang ditegakkan platform
- Compliance/security/resource limits

**Kapan TIDAK pakai Policy:**
- Permission action → `required_permission` di Entity/Service (bukan Policy)

> ⚠️ **Control Plane kind** — dikelola oleh Platform Operator.

**Sumber kontrak:** [`docs/spec/platform/04-control-plane.md`](../spec/platform/04-control-plane.md).

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1alpha1
kind: Policy
metadata:
  name: data-residency
spec:
  # dikelola Platform Operator
  plane: control
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `require_signing` | `boolean` | ✅ |  |  |
| `require_staging_first` | `boolean` | ✅ |  |  |
| `require_approval` | []`PolicyApproval` | — |  |  |
| `blocked` | []`string` | — |  | Blocked is the policy floor — rules that cannot be configured away. |
| `rego` | `string` | — |  | Rego is the escape hatch — full OPA policy body. |

<!-- /generated:attributes -->

## Gotchas

- **Control Plane kind** — tidak boleh dieksekusi handler bisnis; permission tetap `resource + action` di resource plane.
- **Jangan dipakai untuk hardcode role** — permission = resource + action, never hardcoded role names dalam YAML.
- **Cross-ref:** [`docs/spec/platform/04-control-plane.md`](../spec/platform/04-control-plane.md) · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
