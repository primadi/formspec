# Hirarki Visual

**Version:** 0.1.0 · **Status:** Outline

> Dokumen berstatus Outline: heading di bawah menetapkan cakupan final; isi
> ditulis bertahap.

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
nilai `tier` dan tidak dideklarasikan lewat VisualSpecKind.

## 3. Aturan `stack_family`
App shell + Page shell-integrated + Component wajib satu stack resmi yang sama
(berbagi render tree). Mismatch = compile-time error saat `forma apply`. Page
yang benar-benar lepas dari App bebas stack apa saja — konsumsi API generik,
tanpa Renderer kind.

## 4. Shell Baru
Hirarki berlaku sama lintas platform (mis. Flutter: bottom-tab/drawer-nav di
tier app; katalog page yang sama). Shell baru adalah interpreter penuh atas
kontrak kind system — investasi first-party dulu, bukan community renderer.
Catatan constraint: model navigation kemungkinan perlu namespace per-shell
(URL-based vs stack-based).

## 5. Write Once
Satu definisi kontrak (mis. Kanban) dipakai semua shell tanpa ditulis ulang —
web app dan mobile app dari spec yang sama.
