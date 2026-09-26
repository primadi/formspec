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

**Bentuk App — `access` × `app_renderer`.** Dua sumbu ortogonal yang
menentukan perilaku App sebelum menu ditulis. Tanyakan ke user, jangan
default diam-diam:

- `access`: `private` (default — wajib login) · `public` (anonim boleh
  _read + create_ pada semua module yang di-mount; untuk landing/portal/katalog).
- `app_renderer`: `sidebar-nav` (default — back-office, banyak module) ·
  `topnav` (nav sedikit, konten lebar) · `no-nav` (landing/kiosk — tanpa nav
  **dan** tanpa auth kontrol; opt-in via `chrome.nav`/`chrome.auth`).

Pola umum: **satu App private** (back-office), **satu App public** (landing —
pasangkan `kind: Listing`), atau **dua App** (portal `access: public` +
admin `access: private`) berbagi Module yang sama di `root_url` berbeda.

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
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `version` | `string` | — | 1.0.0 | Version is optional — only meaningful when publishing the App to the |
| `vendor` | `string` | — | acme-corp | Vendor is optional — the publishing vendor identity, only required |
| `title` | `string` | — | Acme Corp Portal | Human-readable display name (spaces allowed) — brand bar + document.title; metadata.name stays the machine identifier |
| `logo` | `string` | — | package | Brand mark icon (lucide name) next to the title in the shell brand bar |
| `root_url` | `string` | ✅ | /app/klinik | Mount prefix inside the workspace: \"/\" or any \"/path\" — unique per workspace; reserved segments (_ui, api, _admin, assets, health, login, register, _ws, print) are rejected |
| `workspaces` | — | — | [cafe, kopi] | Optional workspace mount allowlist — absent = all workspaces; [] = staged (mounted nowhere); [slug,...] = only those workspaces |
| `modules` | []`string` | — | [clinic, pharmacy] | Modules mounted by this App — manifests outside these modules are excluded from the App bundle |
| `datastores` | map | — | db: pg-main | Datastores is the App-level App Registry selection — the App Registry |
| `app_renderer` | enum (sidebar-nav · topnav · no-nav) | — | no-nav | Chrome archetype (frontend/05-app-kinds.md): sidebar-nav \| topnav \| no-nav — no-nav means truly no navigation |
| `access` | enum (private · public) | — | private | Auth axis: private (default, secure by default) \| public — orthogonal to app_renderer |
| `stack_family` | `string` | — | react-shadcn | Shell implementation (frontend/03-renderer-kind.md), e.g. react-shadcn |
| `persist_backend` | `string` | — | jsonb-persist | Entity persist backend (backend/04-persist-backend.md), e.g. jsonb-persist |
| `theme_ref` | `string` | — | ocean-blue | Theme kind name applied per-App (frontend/05-app-kinds.md §6) |
| `auth_config_ref` | `string` | — |  | Per-App auth strategy config (kind: Config) |
| `auth` | `AppAuth` | — |  | Per-App auth screen overrides: login_page/setup_page/change_password_page/reset_password_page/oauth_callback_page (kind: Page refs) + chrome_auth (component ref) — empty slots fall back to formspec.core defaults |
| `renderers` | map | — |  | Renderers maps a VisualSpecKind name → renderer for the whole App |
| `chrome` | `AppChrome` | — | nav: menu | Chrome composition: brand/nav/auth/footer/breadcrumbs/theme_switcher, each auto\|show\|hide (auth: auto\|links\|button\|none) — see frontend/05-app-kinds.md §5 |
| `page_transition` | enum (none · fade · slide · slide-up · scale) | — | fade | Page-to-page navigation animation (View Transitions API), scoped to the page content area: none \| fade (default) \| slide \| slide-up \| scale |
| `confirm` | `AppConfirm` | — |  | App-wide default confirm dialogs: create/update/delete message strings — absent = off; forms/actions override per-instance |
| `menu` | []`MenuItem` | — |  |  |
| `publishes` | []`AppInterface` | — |  | cross-app interfaces offered |
| `consumes` | []`AppConsume` | — |  | cross-app interfaces needed → grant request |
| `public_entities` | — | — | [{entity: catalog/product, actions: [list, find]}] | Allowlist of anonymous entity actions for a public App: absent = legacy module-wide, [] = none, list = exactly those pairs |

<!-- /generated:attributes -->

## Gotchas

- **`root_url` wajib dan unik per workspace** — bebas di dalam workspace: `/`, `/barbershop`, `/app/kafe`, dll. (server me-mount SPA dinamis di setiap `root_url`; lihat `docs_internal/plan/flexible-root-url.md`). Reserved segment pertama tidak boleh: `_ui`, `api`, `_admin`, `assets`, `health`, `login`, `register`, `_ws`, `print`. `version` + `vendor` **optional** — metadata publishing marketplace, tidak dikonsumsi runtime.
- **Menu = struktur pohon Group → Leaf** (1–2 level, maks 3). Jangan leaf gundul di level top — renderer menyelipkannya ke group terakhir (merusak navigasi).
- **`type: module` adopt node hanya di level 1** — splicing `Module.spec.menu` di posisi itu. Adopt node TIDAK boleh punya `label`/`icon`/`view`/`route`/`children`.
- **Leaf node butuh `module` + persis satu dari `view`/`route`.** `view` me-resolve route dari registrasi kind visual itu sendiri; `route` adalah escape hatch untuk URL mentah (mis. `/<module>/<plural>` turunan).
- **`view` menerima SEMUA kind visual yang punya route — termasuk Form dan Table.** Keduanya dapat auto-Page wrapper `/<module>/form/<name>` / `/<module>/table/<name>` (kecuali `public: false`), jadi keduanya sah sebagai target `view` dan tidak perlu `route` escape hatch. Kind navigasi memakai `/<kind-lowercase>/<name>` (Dashboard, Widget, Report, Wizard, Kanban, Timeline, Calendar, **Listing**, Print, ApprovalInbox, NotificationCenter); Page memakai `route:`-nya sendiri.
- **Item menu punya dua sumbu visibilitas yang tidak boleh saling menggantikan.** `permissions: [...]` = RBAC, disaring **server** saat bundle dibangun (item tidak pernah terkirim ke pemanggil yang tidak memegangnya, any-of). `when: "<FormSpecExpr>"` = kondisi **bisnis**, dievaluasi **klien**, dan **bukan** gerbang otorisasi: menyembunyikan tautan tidak menolak akses apa pun — route dan datanya tetap dijaga `required_permission`. Untuk membatasi siapa yang boleh membuka sesuatu pakai `permissions`, bukan `when`. Bentuk seperti `when: "user.has('x.y')"` ditolak `formspec check` (`has` bukan callable yang sah; grammar di `08-formspec-expr.md` §2).
- **`when` tidak berlaku di surface `_admin`.** Bundle `?admin=true` unscoped dan menunya dibangun dari daftar entity, jadi `permissions`/`when` hanya berlaku di surface App.
- **Module tanpa `spec.menu` → adopt node kosong** → module tanpa entri navigasi. Kalau SEMUA module tanpa menu, sidebar + redirect kosong.
- **`no-nav` = tanpa navigasi sama sekali** — tanpa nav link DAN tanpa auth controls secara default. Katalog publik dengan login pakai `chrome: {nav: menu, auth: links}`; kiosk dengan logout pakai `chrome: {auth: button}` (`05-app-kinds.md` §5).
- **`access: public` ≠ renderer** — sumbu auth ortogonal terhadap `app_renderer`: `no-nav` bisa public (landing) maupun private (kiosk, tetap di-redirect ke login saat anonim).
- **Cross-ref:** [`docs/spec/platform/02-workspace-app-module.md`](../spec/platform/02-workspace-app-module.md) §4 · [`docs/spec/frontend/05-app-kinds.md`](../spec/frontend/05-app-kinds.md) §5 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
