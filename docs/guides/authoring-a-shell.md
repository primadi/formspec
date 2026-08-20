# Membangun Shell Baru

**Status:** Draft

> Panduan praktis — bukan kontrak. Definisi normatif ada di
> `docs/spec/frontend/`, dirujuk di tiap langkah.

## 1. Kelas Investasi

Shell = interpreter penuh atas kontrak kind system: Layer 0 auto-generate
([`../spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md)
§9), Navigation, Menu, Auth wiring, permission-aware rendering
([`../spec/frontend/04-spec-resolution-api.md`](../spec/frontend/04-spec-resolution-api.md)
§4) — bukan sekadar "panggil satu endpoint lalu render". Investasinya
setara membangun ulang setengah framework per platform. Karena itu Shell
baru **sebaiknya first-party FormSpec dulu** ("proven first, then offered to
community"), bukan dibuka ke komunitas dengan cara yang sama seperti
Renderer Page/Component biasa
([`../spec/frontend/01-visual-hierarchy.md`](../spec/frontend/01-visual-hierarchy.md)
§4). Kalau kamu bukan tim inti FormSpec, pertimbangkan dulu apakah yang kamu
butuhkan sebenarnya cukup Renderer baru untuk Shell yang sudah ada (lihat
[`authoring-a-page-renderer.md`](authoring-a-page-renderer.md)) — jalur itu
jauh lebih murah daripada Shell baru.

## 2. Kontrak yang Wajib Dipenuhi

1. **Spec Resolution API**
   ([`../spec/frontend/04-spec-resolution-api.md`](../spec/frontend/04-spec-resolution-api.md))
   — endpoint `_meta/apps`, `_meta/ui`, `_meta/me`, `_meta/entities/...`;
   backend-agnostic (§3, dilarang membocorkan detail PersistBackend);
   permission filtering tiga-granularitas (§4); konvensi realtime (§5).
2. **Hirarki App/Page/Component**
   ([`01-visual-hierarchy.md`](../spec/frontend/01-visual-hierarchy.md)) —
   Shell-mu menghosting App renderer, Page renderer, Component renderer;
   aturan `stack_family` (App shell + Page shell-integrated + Component
   satu stack) berlaku identik untuk Shell baru.
3. **FormSpecExpr**
   ([`08-formspec-expr.md`](../spec/frontend/08-formspec-expr.md)) — interpreter
   ekspresi yang perilakunya identik dengan shell resmi untuk grammar yang
   sama (literal, `fields.x`, operator, `len`/`sum`, list literal — **tanpa**
   list comprehension, lihat catatan gap di §2 dokumen itu).
4. **VisualSpecKind & slot system**
   ([`02-visual-spec-kind.md`](../spec/frontend/02-visual-spec-kind.md)) —
   `tier` menentukan di mana kind dipakai; slot filling cuma valid dalam
   satu Shell yang sama.

## 3. Pemetaan Katalog Renderer

Isi katalog app/page/component kind (lihat shell resmi sebagai acuan
struktur, bukan acuan kelengkapan — lihat
[`../renderers/shadcn-shell/03-kind-renderers.md`](../renderers/shadcn-shell/03-kind-renderers.md)
untuk status kelengkapan tiap kind di shadcn-shell hari ini, termasuk gap
yang **belum tentu perlu kamu tiru**, mis. drag-drop Kanban yang diklaim
komentar kode tapi ternyata belum diimplementasikan). Untuk tiap kind di
[`06-page-kinds.md`](../spec/frontend/06-page-kinds.md) dan
[`07-component-kinds.md`](../spec/frontend/07-component-kinds.md), petakan
ke widget native platform target-mu — misalnya untuk Shell Flutter (lihat
contoh perbandingan di
[`01-visual-hierarchy.md`](../spec/frontend/01-visual-hierarchy.md) §4):

| Tier               | shadcn-shell                                          | Flutter (contoh)                              |
| ------------------ | ----------------------------------------------------- | --------------------------------------------- |
| App renderer       | `sidebar-nav`, `topnav`, `no-nav`                     | `bottom-tab`, `drawer-nav`, `onboarding-flow` |
| Page renderer      | `data-entry`, `wizard`, `kanban`, `listing`, `report` | jenis yang sama — satu spec, renderer beda    |
| Component renderer | `textinput`, `dateinput`, `widget`                    | native `TextField`, `DatePicker`, `widget`    |

Write once tetap berlaku (§5 di `01-visual-hierarchy.md`) — satu definisi
Kanban dipakai baik shadcn-shell maupun Shell-mu tanpa ditulis ulang; yang
berubah cuma katalog renderer-nya.

## 4. Navigation Model

**Catatan constraint yang belum final** — sidebar/topnav mengasumsikan
URL-based routing (web); Shell native (mis. Flutter) idiomatik pakai
stack-based push/pop dan bottom-tab/drawer, paradigma navigasi yang berbeda
secara mental. Navigation kind (closed enum di App) kemungkinan perlu
di-namespace per-shell — ini constraint yang harus kamu perhitungkan saat
mendesain Shell barumu, bukan keputusan final yang tinggal diikuti
([`01-visual-hierarchy.md`](../spec/frontend/01-visual-hierarchy.md) §4).

## 5. Konformansi

**Open** — sama seperti Renderer dan PersistBackend, mekanisme verifikasi
formal bahwa sebuah Shell memenuhi kontrak §2 belum dirumuskan
([`../spec/frontend/03-renderer-kind.md`](../spec/frontend/03-renderer-kind.md)
§5 mencatat pertanyaan yang sama untuk Renderer). Sampai itu final, uji
Shell-mu terhadap katalog kind penuh (semua kind di
[`06-page-kinds.md`](../spec/frontend/06-page-kinds.md)/
[`07-component-kinds.md`](../spec/frontend/07-component-kinds.md)) dan
dokumentasikan gap implementasi secara eksplisit — pola yang sama dipakai
`renderers/shadcn-shell/*` §Status Implementasi Hari Ini untuk shell resmi
sendiri, supaya pemakai tahu persis apa yang bisa diandalkan hari ini.
