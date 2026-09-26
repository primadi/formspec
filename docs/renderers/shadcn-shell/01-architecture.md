# Arsitektur shadcn-shell

**Updated:** 2026-07-16 · Status: Draft

> Draft: isi di bawah kondisi kode `renderers/react-shadcn/src` hari ini, bukan rencana desain.
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
renderers/react-shadcn/src/
├── App.tsx / main.tsx        # bootstrap, routing dua surface
├── lib/api/{client,meta}.ts  # ky client, envelope, fetcher _meta/*
├── lib/formspec-expr/            # lexer, parser, eval — interpreter FormSpecExpr
├── types/manifest.ts         # mirror pkg/spec/frontend.go + entity schema
├── stores/{session,meta,prefs}.ts   # zustand
├── engine/{derive,permissions,lifecycle,entityRef}.ts   # lihat §3, 02-derivation-engine.md
├── shell/{SideNavShell,Sidebar,router,OverlayHost,LoginScreen}.tsx
├── kinds/{page,form,table,dashboard,widget,report,wizard,kanban,timeline,print,theme}/
├── widgets/{TextInput,NumberInput,Select,Switch,Badge,RelationPicker}.tsx
├── hooks/{useMediaQuery,useTheme}.ts
└── components/{ThemeSwitcher,ErrorBoundary}.tsx, components/ui/ (primitif shadcn)
```

Tidak ada `kinds/menu/` — navigasi bukan kind tersendiri, sudah dihapus
mengikuti keputusan `App.spec.menu`/`Module.spec.menu` sebagai satu-satunya
sumber (lihat komentar `types/manifest.ts`: "No KIND_MENU — navigation isn't
a standalone kind"). `engine/registry.tsx` (registry kind→component generik)
dan `renderers/react-shadcn/src/api/`, `renderers/react-shadcn/src/assets/` (sisa scaffold Vite) **ada di repo
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
4. `<SideNavShell>` membungkus seluruhnya. Path tak cocok dan index jatuh ke
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

- **`OverlayHost` sudah terhubung** — dipasang di ketiga shell
  (`SideNavShell`, `TopNavShell`, dan otomatis ikut `AuthPage`) dan dibuka lewat
  query string `?action=&form=&mode=` yang dikirim `TableRenderer`; `Form.render:
modal|drawer` di manifest karena itu benar-benar mengubah presentasi. Jalur
  derivasi juga bekerja saat pemanggil mengirim `entity=module.name` sebagai
  ganti `form` (lihat `OverlayHost.tsx` + `formspec-client.ts`).
- **`engine/registry.tsx` sudah dihapus.** Wiring aktual memakai `lazy()` map
  hardcoded di `shell/router.tsx`; tidak ada file registry generik lagi. (Item
  ini dulu menyebutnya "kode mati" — sekarang tidak ada sama sekali.)
- **`deriveMenuItems()` sudah dipakai** — `hooks/useResolvedMenu.ts` memanggilnya
  untuk cabang `_admin`, bukan lagi membangun menu inline di `Sidebar.tsx`.
- **`TableRenderer` tidak lagi hardcode prefiks `/_admin`** — navigasi memakai
  `useSurface().surfacePath`, sehingga tabel di surface `app` tetap di `app`.
- **Realtime sudah ada** — `hooks/useRealtime.ts` (subscriber union + delta) dan
  sudah dipakai Table, Kanban, Calendar, Dashboard, Timeline, ApprovalInbox, dan
  NotificationCenter; `realtime: true` di manifest dibaca, bukan field mati.
- **Component contract `asset` sudah ada** — `shell/AssetRenderer.tsx` memuat ES
  module dari path `asset`, memanggil `mount(el, props, formspec)` /
  `unmount(el)`, dan menyuntikkan `formspec` client sesuai
  [`07-component-kinds.md`](../../spec/frontend/07-component-kinds.md) §4
  ([`04-theming-assets.md`](04-theming-assets.md) §2).

**Cara memakai section ini:** ia memuat **divergensi yang masih berlaku**.
Tiap baris di atas adalah divergensi yang sudah **tertutup** dan sengaja
dipertahankan sebagai jejak singkat — bila Anda menemukan barisnya tidak lagi
benar, hapus baris itu (jangan tambahkan baris "dulu X sekarang Y" ke `docs/`,
cukup rujuk changelog penutupnya). Untuk routing dan visibilitas menu, dokumen
otoritatifnya [`05-routing.md`](05-routing.md).
