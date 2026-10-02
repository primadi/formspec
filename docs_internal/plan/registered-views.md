# Plan — `registered_views`: allowlist permukaan `kind: App`

**Status:** ✅ landed 2026-10-02 (changelog `2026-10-02-001`, todo 5.25.1;
sisa 5.25.2/5.25.3 + 14.c.4) · **Tanggal:** 2026-10-02
**Referensi spec:** `docs/spec/frontend/05-app-kinds.md` (tier App) §1.2,
`docs/spec/platform/02-workspace-app-module.md` §3/§4,
`docs_internal/plan/app-entry-gate.md` (batas otorisasi).

## Masalah

Permukaan sebuah App ditentukan secara kasar: `App.spec.modules` memilih
modul, lalu `BuildBundle` (`internal/ui/meta.go`) mengirim **semua** entity,
Page, Form, Table, dan kind navigasi lain di modul itu yang lolos permission
pemanggil. Klien (`shell/router.tsx` `buildRoutes`) membuat route CRUD turunan
untuk **setiap** entity di `bundle.entities`.

Akibatnya entity yang sama sekali tidak dipakai App (mis. `Category`) tetap
punya route yang bisa dibuka langsung lewat URL — selama pemanggil memegang
`list`/`view` pada entity itu. Menu tidak membantu: menu hanya navigasi, dan
`filterMenu` sudah menjatuhkan item yang route-nya tidak ada, tapi tidak
membatasi apa yang boleh ada di bundle.

## Tujuan

Dari sebuah App, nyatakan **view apa saja yang dapat diakses**. Yang tidak
dinyatakan di menu maupun `registered_views` **tidak diberi route** — tidak
bisa diakses langsung lewat URL (SPA 404).

## Aturan (hasil diskusi)

1. **Boleh akses = target menu ∪ `registered_views`.** Semua leaf menu — baik
   `view:` (nama kind terdaftar) maupun `route:` mentah — otomatis terdaftar.
   `registered_views` adalah **tambahan** untuk view yang tidak ada di menu,
   atau untuk App yang tidak punya menu sama sekali.
2. `registered_views` **opsional** (tanpa error). Error hanya untuk entri
   yang salah bentuk. Warning bila App punya `modules` tapi menu dan
   `registered_views` dua-duanya kosong (tidak ada apa pun yang dapat diakses).
3. **Grain entity = per-entity**: satu entri mencakup semua derived view-nya
   (list/detail/create/edit). Pembatasan aksi tetap tugas RBAC.
4. **Jenis view**: semua kind navigable — Entity, Page, Form, Table,
   Dashboard, Report, Wizard, Kanban, Timeline, Calendar, Listing, Print,
   ApprovalInbox, NotificationCenter. Kosakatanya sama dengan `view:` MenuItem.
5. Leaf menu `route:` mentah **boleh** dan **tidak wajib** didaftarkan —
   digerbang oleh `routeExists`.
6. Surface `_admin` (`?admin=true`) dan editor grants (`?grants=true`)
   **tidak** digerbang `registered_views`.

## Bentuk manifest

```yaml
kind: App
spec:
  modules: [cafe-master, cafe-order]
  menu: [] # opsional; target menu otomatis terdaftar
  registered_views: # opsional; tambahan di luar menu
    - { view: cafe-order/menu-catalog } # kind Page (atau kind navigable lain)
    - { entity: cafe-master/menu-item } # entity → semua derived view
```

Tepat satu dari `view`/`entity` per entri. `entity` menerima `module/entity`
atau `module.entity` (dinormalisasi seperti `public_entities`).

## Batasan yang disadari

- Ini **kurasi permukaan / least-privilege**, **bukan** gerbang otorisasi data.
  Route data (`/_ui/entity/...`) berskope workspace/module, bukan per-App.
  RBAC dan `public_entities` tetap penjaga data.
- Entity non-routable **tetap dikirim** di bundle (agar relasi/picker tetap
  berfungsi) tetapi ditandai `routable: false`; klien tidak mendaftarkan
  route-nya. Konsekuensi: picker yang menavigasi ke detail entity non-routable
  akan 404. Trade-off sengaja — alternatifnya membuang schema entity sehingga
  picker rusak total.
- Widget & Theme tidak digerbang (building block / bukan view).

## Perubahan

### Fase 1 — `pkg/spec`

- `RegisteredViewDecl{Entity, View string}` + `AppSpec.RegisteredViews
[]RegisteredViewDecl` (`pkg/spec/resources.go`, dekat `PublicEntityDecl`).
- `ValidateAppSpec`: tepat satu field; format `module/x`; module ∈
  `spec.modules`; tolak entri duplikat.
- Anotasi `// @schema`; regenerate `schemas/` + `docs/kind/`.

### Fase 2 — gating bundle (Go)

- `internal/ui/meta.go`:
  - `AppContext` += `RegisteredViews []spec.RegisteredViewDecl`, `Unfiltered bool`.
  - `BuildBundle`: bangun himpunan reachable sebelum loop section —
    `reachableViewRoutes` (dari `leaf.Route` menu + `ResolveViewRoute` tiap
    registered `view`) dan `reachableEntities` (dari `{entity:}` + reverse-map
    `leaf.Route` via `entityIndex.moduleOfPlural`). Aktif hanya bila
    `Modules != nil && !Unfiltered`.
  - Gating: entity → `Routable`; Page authored → route terdaftar, kecuali home
    `"/"` dan auth screen; derived wrapper Form/Table & nav kinds → hanya bila
    route terdaftar.
  - `EntitySchema.Routable bool json:"routable"` (tanpa `omitempty`).
  - `routeExists` langkah 4: lewati entity `Routable == false`.
- `internal/api/meta.go`: isi `RegisteredViews`; `Unfiltered = true` pada
  `?grants=true`.
- Resolusi/validasi existence entri (view resolve, entity ada di module
  ter-mount) di `internal/app/resolve.go`.

### Fase 3 — frontend

- `types/manifest.ts`: `EntitySchema.routable?: boolean`.
- `shell/router.tsx`: registrasi route CRUD hanya bila `routable !== false`.
- `shell/landing.ts` `canLandOnList`: tolak `routable === false`.
- Test vitest.

### Fase 4 — validasi, schema, docs

- Warning di pipeline check (`cmd/formspec/check.go`).
- `make generate-schema` + `make generate-kind-docs`.
- Update `docs/spec/frontend/05-app-kinds.md` + `docs/spec/platform/02-workspace-app-module.md`.

### Fase 5 — migrasi example

- `examples/kafe/spec/apps/kafe-qr.yaml`: tambah `registered_views`.
- Review `kafe-pos`/`kafe-kds`.

## Efek samping yang diketahui (akan jadi item `[⏸️]` bila belum ditutup)

- Memo: `docs_internal/plan/app-entry-gate.md` (14.c.4) — request-time App
  gate (403 di `/_meta/ui`) tetap belum ada.
