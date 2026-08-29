# App

<!-- generated:meta -->

|             |            |
| ----------- | ---------- |
| Grup        | `curation` |
| Plane       | `resource` |
| Spec struct | `AppSpec`  |

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
  title: "Klinik Internal"
  modules: [clinic, pharmacy]
  root_url: /app/klinik
  app_renderer: sidebar-nav # sidebar-nav | topnav | no-nav
  access: private # private (default) | public
  menu:
    # Adopt: splice module's default menu suggestion
    - type: module
      module: clinic
    # Group with mixed children from different modules
    - label: "Farmasi"
      icon: "pill"
      children:
        - { label: "Antrian Resep", view: pharmacy-queue, module: pharmacy }
        - {
            label: "Semua Resep",
            route: /pharmacy/prescriptions,
            module: pharmacy,
          }
```

Contoh `chrome` — katalog publik `no-nav` yang opt-in nav link + auth
controls (pola portal registry; lihat `05-app-kinds.md` §5):

```yaml
spec:
  app_renderer: no-nav
  access: public
  chrome:
    nav: menu # default no-nav = none — opt-in nav link dari menu
    auth: links # default no-nav = none — Sign in/Sign up / logout
```

## Atribut

<!-- generated:attributes -->

| Atribut           | Tipe                                 | Wajib | Contoh             | Deskripsi                                                                                                                                                       |
| ----------------- | ------------------------------------ | ----- | ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `version`         | `string`                             | ✅    | 1.0.0              |                                                                                                                                                                 |
| `vendor`          | `string`                             | ✅    | acme-corp          |                                                                                                                                                                 |
| `title`           | `string`                             | —     | Acme Corp Portal   | Human-readable display name (spaces allowed) — brand bar + document.title; metadata.name stays the machine identifier                                           |
| `logo`            | `string`                             | —     | package            | Brand mark icon (lucide name) next to the title in the shell brand bar                                                                                          |
| `root_url`        | `string`                             | ✅    | /app/klinik        |                                                                                                                                                                 |
| `modules`         | []`string`                           | —     | [clinic, pharmacy] | Modules mounted by this App — manifests outside these modules are excluded from the App bundle                                                                  |
| `app_renderer`    | enum (sidebar-nav · topnav · no-nav) | —     | no-nav             | Chrome archetype (frontend/05-app-kinds.md): sidebar-nav \| topnav \| no-nav — no-nav means truly no navigation                                                 |
| `access`          | enum (private · public)              | —     | private            | Auth axis: private (default, secure by default) \| public — orthogonal to app_renderer                                                                          |
| `stack_family`    | `string`                             | —     | react-shadcn       | Shell implementation (frontend/03-renderer-kind.md), e.g. react-shadcn                                                                                          |
| `persist_backend` | `string`                             | —     | jsonb-persist      | Entity persist backend (backend/04-persist-backend.md), e.g. jsonb-persist                                                                                      |
| `theme_ref`       | `string`                             | —     | ocean-blue         | Theme kind name applied per-App (frontend/05-app-kinds.md §6)                                                                                                   |
| `auth_config_ref` | `string`                             | —     |                    | Per-App auth strategy config (kind: Config)                                                                                                                     |
| `renderers`       | map                                  | —     |                    | Renderers maps a VisualSpecKind name → renderer for the whole App                                                                                               |
| `chrome`          | `AppChrome`                          | —     | nav: menu          | Chrome composition: brand/nav/auth/footer/breadcrumbs/theme_switcher, each auto\|show\|hide (auth: auto\|links\|button\|none) — see frontend/05-app-kinds.md §5 |
| `menu`            | []`MenuItem`                         | —     |                    |                                                                                                                                                                 |
| `publishes`       | []`AppInterface`                     | —     |                    | cross-app interfaces offered                                                                                                                                    |
| `consumes`        | []`AppConsume`                       | —     |                    | cross-app interfaces needed → grant request                                                                                                                     |

<!-- /generated:attributes -->

## Gotchas

- **`version`, `vendor`, `root_url` WAJIB.** `root_url` harus mulai `/app/` dan unik di workspace.
- **Menu = struktur pohon Group → Leaf** (1–2 level, maks 3). Jangan leaf gundul di level top — renderer menyelipkannya ke group terakhir (merusak navigasi).
- **`type: module` adopt node hanya di level 1** — splicing `Module.spec.menu` di posisi itu. Adopt node TIDAK boleh punya `label`/`icon`/`view`/`route`/`children`.
- **Leaf node butuh `module` + persis satu dari `view`/`route`.** `view` resolve kind visual terdaftar (Page, Dashboard, Widget, Report, Wizard, Kanban, Timeline, Print — bukan Form/Table); `route` adalah escape hatch.
- **Module tanpa `spec.menu` → adopt node kosong** → module tanpa entri navigasi. Kalau SEMUA module tanpa menu, sidebar + redirect kosong.
- **`view` resolve ALL visual kinds termasuk Form/Table** — masing-masing dapat auto-Page wrapper `/<module>/form/<name>` / `/<module>/table/<name>` (kecuali `public: false`). Tidak perlu `route` escape hatch lagi untuk Form/Table.
- **`no-nav` = tanpa navigasi sama sekali** — tanpa nav link DAN tanpa auth controls secara default. Katalog publik dengan login pakai `chrome: {nav: menu, auth: links}`; kiosk dengan logout pakai `chrome: {auth: button}` (`05-app-kinds.md` §5).
- **`access: public` ≠ renderer** — sumbu auth ortogonal terhadap `app_renderer`: `no-nav` bisa public (landing) maupun private (kiosk, tetap di-redirect ke login saat anonim).
- **Cross-ref:** [`docs/spec/platform/02-workspace-app-module.md`](../spec/platform/02-workspace-app-module.md) §4 · [`docs/spec/frontend/05-app-kinds.md`](../spec/frontend/05-app-kinds.md) §5 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
