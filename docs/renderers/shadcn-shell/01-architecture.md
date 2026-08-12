# Arsitektur shadcn-shell

**Updated:** 2026-07-16 · Status: Draft

> Draft: isi di bawah kondisi kode `web/src` hari ini, bukan rencana desain.
> §5 mencatat kesenjangan terhadap kontrak `docs/spec/frontend/` — bagian itu
> boleh berubah tanpa mengubah kontrak.

## 1. Interpreter Runtime
SPA React yang di-deploy sekali dan me-render App apa pun dari spec saat
runtime — bukan build artifact per-app, konsisten dengan
[`../../spec/frontend/01-visual-hierarchy.md`](../../spec/frontend/01-visual-hierarchy.md).
Dua surface dilayani satu bundle yang sama: `/{workspace}/_admin/*` (100%
derived dari Entity, tanpa manifest UI) dan `/{workspace}/app/*` (manifest
App/Page/dst mengoverride derivasi).

## 2. Struktur Kode
```
web/src/
├── App.tsx / main.tsx        # bootstrap, routing dua surface
├── lib/api/{client,meta}.ts  # ky client, envelope, fetcher _meta/*
├── lib/formspec-expr/            # lexer, parser, eval — interpreter FormSpecExpr
├── types/manifest.ts         # mirror pkg/spec/frontend.go + entity schema
├── stores/{session,meta,prefs}.ts   # zustand
├── engine/{derive,permissions,lifecycle,entityRef}.ts   # lihat §3, 02-derivation-engine.md
├── shell/{AppShell,Sidebar,router,OverlayHost,LoginScreen}.tsx
├── kinds/{page,form,table,dashboard,widget,report,wizard,kanban,timeline,print,theme}/
├── widgets/{TextInput,NumberInput,Select,Switch,Badge,RelationPicker}.tsx
├── hooks/{useMediaQuery,useTheme}.ts
└── components/{ThemeSwitcher,ErrorBoundary}.tsx, components/ui/ (primitif shadcn)
```

Tidak ada `kinds/menu/` — navigasi bukan kind tersendiri, sudah dihapus
mengikuti keputusan `App.spec.menu`/`Module.spec.menu` sebagai satu-satunya
sumber (lihat komentar `types/manifest.ts`: "No KIND_MENU — navigation isn't
a standalone kind"). `engine/registry.tsx` (registry kind→component generik)
dan `web/src/api/`, `web/src/assets/` (sisa scaffold Vite) **ada di repo
tapi tidak dipakai** — lihat §5.

## 3. Bootstrap & Resolusi App
`Root()` (`App.tsx`) mendaftarkan dua route top-level:
`/:workspace/_admin/*` dan `/:workspace/app/*`. `SurfaceShell({surface})`
menjalankan boot:

1. `useSessionStore.boot(workspace)` — fetch `/_meta/me`; gagal/401 → sesi dev
   sintetis `{roles:["admin"], permissions:["*"]}` (bukan blocking error).
2. Muat bundle lewat `useMetaStore.load()`, **beda per surface**:
   - `_admin` — `fetchMetaBundle({admin:true})`: bundle unscoped-App,
     digerbangi satu permission `_admin.access` (bukan filter per-manifest).
   - `app` — panggil `fetchMetaApps()` (`GET .../_meta/apps`) dulu untuk
     enumerasi App yang resolved di workspace, pilih satu lewat
     `detectAppName()` (cocokkan `root_url` terpanjang terhadap path
     browser saat ini), baru `fetchMetaBundle({appName})`.
3. `buildRoutes()` ([`02-derivation-engine.md`](02-derivation-engine.md) §4)
   membangun route table dari bundle: route `kind: Page` dari `spec.route`,
   route CRUD turunan per entity, satu route per entry
   Dashboard/Widget/Wizard/Kanban/Timeline/Report/Print
   (`/dashboard/{name}`, dst).
4. `<AppShell>` membungkus seluruhnya. Path tak cocok dan index jatuh ke
   `DefaultRedirect`: surface `app` menelusuri `bundle.menu` depth-first
   (mendarat di item menu authored pertama); surface `_admin` jatuh ke list
   derived entity non-summary pertama.
5. 403 (`_admin.access` tidak dimiliki) dan error koneksi adalah layar
   eksplisit, bukan crash.

Resolusi multi-App-per-workspace (`_meta/apps`, `root_url`,
`detectAppName()`) mengonsumsi kontrak App yang lebih baru dari yang
didesain semula — lihat `docs/spec/platform/02-workspace-app-module.md`
(masih Outline, menunggu Draft di S8).

## 4. Konsumsi Spec Resolution API
Endpoint yang dipakai (lihat kontraknya di
[`../../spec/frontend/04-spec-resolution-api.md`](../../spec/frontend/04-spec-resolution-api.md)
§2): `_meta/apps`, `_meta/ui` (mode `admin`/`appName`), `_meta/me`,
`_meta/entities/{module}/{name}` (lazy-load, dipanggil `fetchEntitySchema`).
Client `ky` (`lib/api/client.ts`) memprefiks `/{workspace}/api/v1`,
menyuntik `Authorization: Bearer`, unwrap envelope `{data, meta}`/`{data,
meta:{page,...}}`, dan melempar `FormaApiError` typed dari envelope error.
CAS: `version` dikirim sebagai header `If-Match` pada mutasi — **tidak ada
percabangan eksplisit untuk status 409** di mana pun; error CAS conflict
jatuh ke jalur error generik (`toast.error`), bukan alur refetch-khusus.

## 5. Status Implementasi Hari Ini
Bagian yang terbukti belum/tidak sesuai rencana desain awal — dicatat supaya
tidak diam-diam diasumsikan bekerja:

- **`OverlayHost.tsx` (modal/drawer via query string) ada tapi tidak
  terhubung ke jalur hidup mana pun.** Seluruh navigasi create/edit dari
  `TableRenderer` pergi ke route halaman penuh, bukan men-drive
  `OverlayHost` lewat `?action=...`. Isi `Dialog`/`Sheet`-nya masih literal
  placeholder ("Form renderer coming in Fase 4.F3") walau `FormRenderer`
  sendiri sudah selesai dibangun di jalur lain. `Form.render: modal|drawer`
  di manifest karena itu **tidak benar-benar mengubah presentasi** hari ini
  — lihat gap serupa di [`03-kind-renderers.md`](03-kind-renderers.md) §Table.
- **`engine/registry.tsx`** (registry kind→component generik) dan
  **`engine/derive.ts`'s `deriveMenuItems()`** adalah kode mati — wiring
  aktual pakai `lazy()` map hardcoded di `shell/router.tsx`, dan menu
  `_admin` dibangun inline di `Sidebar.tsx`, bukan lewat fungsi derivasi ini.
- **`TableRenderer` hardcode prefiks `/_admin`** pada `navigate()` untuk
  aksi row/New — tabel yang dirender di bawah surface `app` (lewat blok
  Page) ikut ter-navigasi ke `_admin`, kemungkinan bug untuk App-surface.
- Realtime (`realtime: true` di Table/Dashboard) **belum ada implementasi
  apapun** — bukan polling, bukan websocket. Field-nya ada di tipe tapi
  tidak pernah dibaca renderer manapun. Lihat
  [`../../spec/frontend/04-spec-resolution-api.md`](../../spec/frontend/04-spec-resolution-api.md)
  §5 untuk kontrak yang harus dipenuhi nanti.
- Component contract `asset` (mount/unmount, `formspec.ui` service) **belum
  diimplementasikan sama sekali** — lihat
  [`04-theming-assets.md`](04-theming-assets.md) §2.
