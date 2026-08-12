# Renderer

<!-- generated:meta -->
| | |
|---|---|
| Grup | `infra` |
| Plane | `resource` |
| Spec struct | `RendererSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Renderer` adalah **implementasi konkret sebuah VisualSpecKind** untuk shell/stack tertentu.

**Kapan memakai Renderer:**
- Implementasi visual renderer (React/shadcn, Flutter, dst.) untuk satu `VisualSpecKind`
- Mendistribusikan shell baru lewat marketplace

**Kapan TIDAK pakai Renderer:**
- Menyusun UI aplikasi → UI kinds (Page/Form/Table/dst.)
- Menyimpan data → `kind: PersistBackend`

**Sumber kontrak:** [`docs/spec/frontend/03-renderer-kind.md`](../spec/frontend/03-renderer-kind.md).

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1alpha1
kind: Renderer
metadata:
  name: shadcn-shell
  module: formspec/rendering
spec:
  implements: formspec/visual.form-page
  stack_family: react-shadcn
  trust_tier: official
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `implements` | `string` | ✅ | formspec/visual.form-page |  |
| `stack_family` | `string` | ✅ | react-shadcn |  |
| `trust_tier` | enum (official · verified · community) | ✅ | official |  |

<!-- /generated:attributes -->

## Gotchas

- **`implements`, `stack_family`, `trust_tier` wajib** — ini seam antara VisualSpecKind (kontrak) dan renderer (implementasi).
- **Trust tier**: `official` | `verified` | `community` — distribusi lewat marketplace.
- **Cross-ref:** [`docs/spec/frontend/03-renderer-kind.md`](../spec/frontend/03-renderer-kind.md) · [`docs/spec/frontend/02-visual-spec-kind.md`](../spec/frontend/02-visual-spec-kind.md) · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
