# Hirarki Visual

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku. Bagian yang masih terbuka ditandai
> eksplisit sebagai **Open**.

## 1. Empat Tingkat
| Tingkat | Definisi | Contoh |
|---|---|---|
| **Shell** | Stack teknologi + kontrak bootstrap penuh (routing awal, Layer 0 auto-generate). Satu App = satu Shell | shadcn/React (resmi) |
| **App renderer** | Asumsi bootstrap: siapa menguasai bootstrap subtree — chrome penuh (Auth/Menu/Nav) atau minimal | `sidebar-nav`, `topnav`, `landing-page` |
| **Page renderer** | Isi konten utama sebuah route | `data-entry`, `wizard`, `kanban`, `table-list`, `report`, `listing` |
| **Component renderer** | Elemen granular, reusable di Page manapun dalam Shell yang sama | `textinput`, `dateinput`, `widget` |

Catatan penting: App renderer bukan soal "punya nav atau tidak" — `sidebar-nav`
dan `landing-page` sama-sama App renderer dengan asumsi bootstrap berbeda.
Keputusan "dibungkus chrome atau tidak" selesai di App renderer; tidak ada flag
`route_mode` di level Page.

## 2. Shell Bukan VisualSpecKind
Shell adalah wadah yang menghosting App/Page/Component renderer; ia tidak punya
nilai `tier` dan tidak dideklarasikan lewat VisualSpecKind
([`02-visual-spec-kind.md`](02-visual-spec-kind.md)). Shell dipilih di level
App/Page/Component lewat `stack_family` renderer yang dipasang
([`03-renderer-kind.md`](03-renderer-kind.md)), bukan lewat field terpisah.

## 3. Aturan `stack_family`
App shell + Page shell-integrated + Component wajib satu stack resmi yang sama
(berbagi render tree). Page yang benar-benar lepas dari App bebas stack apa
saja — konsumsi API generik, tanpa Renderer kind sama sekali (tidak ada shared
render tree untuk dijaga konsistensinya).

Validasi `formspec apply`:

```
Jika Page dipasang di dalam App (shell-integrated):
  renderer.stack_family HARUS sama dengan App.stack_family
  → mismatch = compile-time error

Jika Page dikonsumsi independen dari App manapun:
  tidak ada compatibility check — tidak ada shared render tree
```

## 4. Shell Baru
Hirarki berlaku sama lintas platform (mis. Flutter: bottom-tab/drawer-nav di
tier app; katalog page yang sama — `data-entry`/`wizard`/`kanban`/`listing`/
`report` tidak ditulis ulang, cuma renderer barunya). Shell baru adalah
interpreter penuh atas kontrak kind system (Layer 0 auto-generate, Navigation,
Menu, Auth wiring, permission-aware rendering lewat Spec Resolution API,
[`04-spec-resolution-api.md`](04-spec-resolution-api.md)) — investasinya
setara membangun ulang setengah framework per platform, karena itu Shell baru
**sebaiknya first-party FormSpec dulu** ("proven first, then offered to
community"), bukan dibuka ke komunitas dengan cara yang sama seperti Renderer
Page/Component biasa di §3.

**Navigation per-shell (normatif).** Kontrak navigasi dideklarasikan
**abstrak** di level App (menu tree + route kanonik per Page,
[`../platform/02-workspace-app-module.md`](../platform/02-workspace-app-module.md)
§4) — spec tidak pernah mengasumsikan paradigma navigasi tertentu. Tiap
Shell **memetakan** kontrak abstrak itu ke paradigma idiomatiknya sendiri:
Shell web memakai URL-based routing (sidebar/topnav), Shell native (mis.
Flutter) boleh memakai stack-based push/pop dan bottom-tab/drawer. Dua
kewajiban lintas-paradigma: (1) setiap Page tetap punya **alamat kanonik**
(route) yang bisa dituju langsung — deep-link di web, deep-link/intent di
native; (2) menu tree App dihormati sebagai struktur navigasi utama, apa pun
widget navigasinya. Detail pemetaan per paradigma diuji dan dipertajam saat
Shell non-web pertama dibangun, tanpa mengubah kontrak abstraknya.

## 5. Write Once
Satu definisi kontrak (mis. Kanban, dideklarasikan lewat VisualSpecKind) dipakai
semua shell tanpa ditulis ulang — web app dan mobile app dari spec yang sama,
masing-masing dengan Renderer sendiri yang memenuhi `renderer_contract` yang
sama.

## 6. Referensi
| Dokumen | Isi |
|---|---|
| [`02-visual-spec-kind.md`](02-visual-spec-kind.md) | Meta-kind VisualSpecKind: tier, schema, renderer_contract, slot system |
| [`03-renderer-kind.md`](03-renderer-kind.md) | Kind Renderer: implements, stack_family, trust_tier |
| [`04-spec-resolution-api.md`](04-spec-resolution-api.md) | Seam runtime Shell ↔ engine |
