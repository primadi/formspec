# Module

<!-- generated:meta -->
| | |
|---|---|
| Grup | `curation` |
| Plane | `resource` |
| Spec struct | `ModuleSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Module` adalah **bounded context** — pemilik object (Entity, Service,
instance VisualSpecKind). Satu Module = satu konteks bisnis lengkap. Struktur
Module adalah himpunan tertutup: Entity, Service, dan instance VisualSpecKind.

Deklarasikan Module **sebelum** Entity — setiap Entity wajib punya `metadata.module`
yang menunjuk ke module yang memilikinya.

**Kapan memakai Module:**
- Memecah aplikasi menjadi bounded context (mis. `arisan-master`, `arisan-field`, `arisan-report`)
- Module yang bisa di-publish ke marketplace untuk reuse

**Kapan TIDAK pakai Module:**
- Menyusun module jadi app → `kind: App` (Module tetap ada, di-mount ke App)
- Perilaku data → `kind: Entity` / `kind: Service`

**Dependensi antar module** via `depends: [{module, version?}]` — bukan `depends_on`.

**Sumber kontrak:** [`docs/spec/platform/02-workspace-app-module.md`](../spec/platform/02-workspace-app-module.md) §2.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1alpha1
kind: Module
metadata:
  name: clinic
spec:
  version: 1.0.0
  vendor: acme-corp
  depends:
    - module: formspec/core
  menu:
    - label: "Klinik"
      icon: "stethoscope"
      children:
        - { label: "Dashboard", icon: "layout-dashboard", view: clinic-dashboard }
        - { label: "Daftar Kunjungan", icon: "list", view: visits-page }
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `version` | `string` | ✅ | 1.0.0 |  |
| `vendor` | `string` | — | acme-corp |  |
| `depends` | []`Dependency` | — | formspec/core | Module dependencies — array of {module, version?} |
| `datastore` | `string` | — | default | Datastore binds the module to a named kind: Datastore for ctx.db() |
| `config` | map | — |  | module-specific configuration (02-workspace-app-module.md §2) |
| `ai_index` | `AiIndexDecl` | — |  | AI discovery metadata (ai/04-formspec-remote-mcp.md §3) |
| `menu` | []`MenuItem` | — |  | Menu is a default navigation suggestion, module-relative (no `Module` |

<!-- /generated:attributes -->

## Gotchas

- **`spec.version` WAJIB.** Dependensi pakai `depends` (array `{module, version?}`), BUKAN `depends_on`.
- **Menu module-relative** — leaf TIDAK set `module` (implied = module ini saat di-adopt). Struktur Group → Leaf, maks 3 level.
- **`datastore`** — bind module ke named `kind: Datastore` untuk `ctx.db()` (default `'default'`).
- **Setiap menu harus berkategori** (Group node di level 1). Adopt module tanpa menu → App tanpa entri navigasi untuk module itu.
- **Cross-ref:** [`docs/spec/platform/02-workspace-app-module.md`](../spec/platform/02-workspace-app-module.md) §2 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
