# Theme

<!-- generated:meta -->
| | |
|---|---|
| Grup | `ui` |
| Plane | `resource` |
| Spec struct | `ThemeSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Theme` adalah **look & feel** — CSS variables dan styling configuration untuk UI.

**Kapan memakai Theme:**
- Restyle tampilan pustaka komponen dasar tanpa mengubah semantik layout
- Per-workspace branding (warna, font)

**Kapan TIDAK pakai Theme:**
- Menambah widget baru → daftarkan `VisualSpecKind` tier component, bukan Theme
- Mengubah perilaku/visibilitas → itu spec/permission, bukan styling

**Sumber kontrak:** [`docs/spec/frontend/05-app-kinds.md`](../spec/frontend/05-app-kinds.md) §5.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1alpha1
kind: Theme
metadata:
  name: ocean-blue
  module: core
spec:
  # CSS variables — lihat ui-theme/ untuk contoh penuh
  tokens:
    --color-primary: "#2563eb"
    --radius: "0.5rem"
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `public` | `boolean` | — |  | If true (default), the theme is active and published in the bundle. |
| `tokens` | map | — |  |  |
| `stylesheet` | `string` | — |  |  |
| `widgets` | map | — |  | base widget → asset skin |

<!-- /generated:attributes -->

## Gotchas

- **Theme tidak pernah mengubah semantik layout** atau melewati visibilitas berbasis permission.
- **Himpunan dasar widget CLOSED** — widget baru via `VisualSpecKind` baru (tier component), bukan memperluas daftar ad-hoc.
- **Cross-ref:** [`docs/spec/frontend/05-app-kinds.md`](../spec/frontend/05-app-kinds.md) §5 · [`docs/spec/frontend/07-component-kinds.md`](../spec/frontend/07-component-kinds.md) §1 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
