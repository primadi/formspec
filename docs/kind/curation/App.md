# App

<!-- generated:meta -->
| | |
|---|---|
| Grup | `curation` |
| Plane | `resource` |
| Spec struct | `AppSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: App` adalah **kurasi** — keranjang module yang di-mount jadi satu
aplikasi user-facing. App **tidak memiliki** object; Module yang memiliki.
Module yang sama bisa di-mount oleh banyak App di workspace yang sama.

Deklarasikan App **paling akhir** setelah Module + Entity selesai — App hanya
menyusun apa yang sudah ada.

**Kapan TIDAK pakai App:**
- Membangun satu bounded context → itu `kind: Module`
- Mengubah perilaku data → `kind: Entity` / `kind: Service`

**Menu dimiliki App.** Menu = "apa yang bisa dicapai via navigasi" — App yang
berbeda boleh mengekspos subset module yang berbeda. Analogi: **Module =
katalog, App.menu = daftar belanja dari katalog.**

**Sumber kontrak:** [`docs/spec/platform/02-workspace-app-module.md`](../spec/platform/02-workspace-app-module.md) §4.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
kind: App
metadata:
  name: klinik-internal
  module: core
spec:
  version: 1.0.0
  vendor: acme-corp
  modules: [clinic, pharmacy]
  root_url: /app/klinik
  menu:
    # Adopt: splice module's default menu suggestion
    - type: module
      module: clinic
    # Group with mixed children from different modules
    - label: "Farmasi"
      icon: "pill"
      children:
        - { label: "Antrian Resep", view: pharmacy-queue, module: pharmacy }
        - { label: "Semua Resep", route: /pharmacy/prescriptions, module: pharmacy }
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `version` | `string` | ✅ | 1.0.0 |  |
| `vendor` | `string` | ✅ | acme-corp |  |
| `root_url` | `string` | ✅ | /app/klinik |  |
| `modules` | []`string` | — | [clinic, pharmacy] |  |
| `app_renderer` | `string` | — | shadcn-shell |  |
| `theme_ref` | `string` | — | ocean-blue |  |
| `auth_config_ref` | `string` | — |  | per-App auth strategy config |
| `menu` | []`MenuItem` | — |  |  |
| `publishes` | []`AppInterface` | — |  | cross-app interfaces offered |
| `consumes` | []`AppConsume` | — |  | cross-app interfaces needed → grant request |

<!-- /generated:attributes -->

## Gotchas

- **`version`, `vendor`, `root_url` WAJIB.** `root_url` harus mulai `/app/` dan unik di workspace.
- **Menu = struktur pohon Group → Leaf** (1–2 level, maks 3). Jangan leaf gundul di level top — renderer menyelipkannya ke group terakhir (merusak navigasi).
- **`type: module` adopt node hanya di level 1** — splicing `Module.spec.menu` di posisi itu. Adopt node TIDAK boleh punya `label`/`icon`/`view`/`route`/`children`.
- **Leaf node butuh `module` + persis satu dari `view`/`route`.** `view` resolve kind visual terdaftar (Page, Dashboard, Widget, Report, Wizard, Kanban, Timeline, Print — bukan Form/Table); `route` adalah escape hatch.
- **Module tanpa `spec.menu` → adopt node kosong** → module tanpa entri navigasi. Kalau SEMUA module tanpa menu, sidebar + redirect kosong.
- **`view` resolve ALL visual kinds termasuk Form/Table** — masing-masing dapat auto-Page wrapper `/<module>/form/<name>` / `/<module>/table/<name>` (kecuali `public: false`). Tidak perlu `route` escape hatch lagi untuk Form/Table.
- **Cross-ref:** [`docs/spec/platform/02-workspace-app-module.md`](../spec/platform/02-workspace-app-module.md) §4 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
