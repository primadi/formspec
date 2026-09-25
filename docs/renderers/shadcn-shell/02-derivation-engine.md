# Derivation Engine

**Updated:** 2026-07-16 · Status: Draft

> Draft: isi di bawah kondisi kode `engine/derive.ts` hari ini.

## 1. Layer 0 Otomatis

Dari Entity manifest → Table, Form, Page detail, tanpa spec eksplisit —
mewujudkan kontrak "Derivasi Otomatis" di
[`../../spec/frontend/06-page-kinds.md`](../../spec/frontend/06-page-kinds.md)
§9. Tipe output derivasi **sama persis** dengan tipe manifest hasil YAML,
sehingga kind renderer tidak tahu bedanya derived vs authored.

## 2. Aturan Derivasi

**Table** (`deriveTable`): kolom = field non-child non-computed, **dipotong
ke 8 pertama** (field selebihnya _tidak_ ditampilkan di mana pun — bukan
"sisanya pindah ke detail" seperti niat awal). `enum` atau field bernama
`doc_status` → widget `badge`; `boolean` → widget boolean; `datetime` →
format relative; `date` → format date; `decimal` → format currency **hanya
kalau** field itu punya rule `min` numerik (heuristik kasar, bukan deteksi
"mirip uang" yang sesungguhnya). `search: true` kalau ada field string
non-enum. `default_sort: -created_at` kalau field `created_at` ada (kalau
tidak, tidak ada default sort sama sekali). Row action beda per
`characteristic`: `summary` → view saja; `reference` → view+edit tanpa
delete; lainnya → view+edit+delete dengan pesan confirm hardcoded.

**Form** (`deriveForm`): satu section, semua field editable non-computed
(mode create mengecualikan computed saja). Heuristik `render`: **>12 field
ATAU punya child field ber-`storage: table`** → `separate_page`; **>5
field** → `drawer`; selain itu → `modal`. (Field child ber-`storage: jsonb`
**tidak** ikut dihitung sebagai alasan `separate_page` — beda dari niat awal
yang menghitung "punya child table" secara generik.)

**Caption & help field** (`entityFieldLabel`, `entityFieldHelp`,
`withEntityFieldDefaults`, `withEntityColumnLabels`): caption memakai
`label` manifest → `title` field Entity → nama field yang di-humanise; help
memakai `help` manifest → `description` field Entity → tanpa elemen. Presedensi
ini **wajib sama** di jalur derivasi dan authored, sesuai
[`../../spec/frontend/06-page-kinds.md`](../../spec/frontend/06-page-kinds.md)
§2. Dulu hanya `fieldLabel()` (jalur derivasi) yang membaca `title` entity;
jalur authored memakai `field.label ?? field.name`, sehingga mendeklarasikan
`kind: Form`/`Table` menurunkan seluruh caption menjadi nama mentah — 111 dari
161 field form authored di `examples/` tidak menulis `label:`, jadi ini jalur
umum, bukan kasus tepi. `resolveForm()`/`resolveTable()` kini meng-enrich
manifest authored (mengembalikan salinan, bukan memutasi entry bundle yang
dibagi antar render).

Sisi `help`-nya punya riwayat yang sama persis: `formField()` (jalur derivasi)
membaca `field.description`, sedangkan form authored tidak pernah lewat sana —
jadi `description` entity hilang begitu entity-nya punya Form, dan penulis
menyalinnya manual (`promo-form.yaml` dan `promo/entity.yaml` memuat kalimat
yang identik). `withEntityFieldDefaults()` kini mengisi `label` **dan** `help`
sekaligus, plus `sections[0].description` dari `entity.description` — yang
terakhir adalah subtitle drawer/dialog
(`shell/OverlayHost.tsx`), yang tanpa itu jatuh ke teks Inggris generik.
`WizardFormStep` dan jalur `WizardRenderer` untuk `steps[].fields` membaca Form
langsung dari bundle (`useMetaStore.getForm`) sehingga melewati `resolveForm()`
— keduanya memanggil resolver yang sama.

Renderer merender help **hanya** di permukaan input (Form create/edit + langkah
Wizard, di bawah kontrol). Mode `view` dan permukaan baca-saja (Table, Listing,
detail Page, Kanban, ChildTable) tidak merendernya — lihat §2 spec di atas.

**Menu**: fungsi `deriveMenuItems()` ada di kode tapi **tidak dipanggil
siapa pun** — lihat [`01-architecture.md`](01-architecture.md) §5. Menu
surface `app` murni dari `bundle.menu` (resolved server-side dari
`App.spec.menu`) tanpa fallback derived untuk entity yang belum masuk menu
manapun; menu surface `_admin` dibangun terpisah, inline, di `Sidebar.tsx`
(grouped per module).

**Lifecycle buttons** (`engine/lifecycle.ts`): tipe `Lifecycle` di kode
hanya punya **dua** nilai — `plain_crud` dan `two_step_autosave` (bukan
empat pola yang sempat didesain). `getLifecycle()` jatuh ke
`two_step_autosave` untuk apa pun selain `plain_crud`. Pola "2-step manual"
dan "1-step create-submit" dari
[`../../spec/frontend/06-page-kinds.md`](../../spec/frontend/06-page-kinds.md)
§2.1 **belum** jadi cabang lifecycle terpisah di renderer ini — yang ada
cuma flag `has_quick_submit` sebagai tambahan di dalam `two_step_autosave`,
bukan pola independen dengan UI sendiri.

## 3. Override

Presedensi authored > derived (`resolveTable`/`resolveForm`), dengan
konvensi penamaan lebih kaya dari yang didesain semula: form authored
spesifik-mode (`{entity}-create`/`-edit`/`-view`) → form authored generik
(`{entity}-form`) → derive. Blok Page/Tab yang menyebut Form lewat `ref`
eksplisit selalu menang di atas keduanya.

## 4. Route Table

`buildRoutes()` (`shell/router.tsx`) membangun daftar route sekali per
surface dari bundle: route `kind: Page` dari `spec.route`, route CRUD
turunan per entity (`list`/`new`/`:id`/`:id/edit`), plus satu route per
entry Dashboard/Widget/Wizard/Kanban/Timeline/Report/Print
(`/dashboard/{name}`, dst — mengikuti konvensi
[`../../spec/platform/02-workspace-app-module.md`](../../spec/platform/02-workspace-app-module.md)
soal resolusi route `view` menu, menunggu Draft).

## 5. Status Implementasi Hari Ini

Lihat [`01-architecture.md`](01-architecture.md) §5 untuk gap lintas-file
(registry mati, menu derivation mati). Spesifik ke derivasi: kolom Table
yang terpotong di atas 8 field hilang tanpa jejak (bukan sekadar
"belum ditampilkan di detail" — detail page tidak menerima limpahannya);
ini gap nyata terhadap niat "Table" di
[`../../spec/frontend/06-page-kinds.md`](../../spec/frontend/06-page-kinds.md)
§3 yang tidak menyebut batas kolom.
