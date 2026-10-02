# 2026-10-02-001 — `registered_views`: allowlist permukaan `kind: App`

Plan: `docs_internal/plan/registered-views.md` · Todo: **5.25.1**

## Apa

Tambah field opsional `spec.registered_views` pada `kind: App`. Permukaan App
yang boleh diakses menjadi **semua target menu ∪ `registered_views`**: target
leaf menu (baik `view:` maupun `route:` mentah) otomatis terdaftar, dan
`registered_views` menambah view yang tidak ada di menu (khususnya App tanpa
menu seperti `kafe-qr`). View/entity di luar himpunan itu **tidak diberi route**
(SPA 404); entity non-routable tetap dikirim di bundle dengan penanda
`routable: false` agar relasi/picker tetap berfungsi.

## Kenapa

Sebelumnya `App.spec.modules` mengirim seluruh entity modul ke bundle, dan
klien mendaftarkan route CRUD turunan untuk setiap entity — sehingga entity
yang tidak dipakai App tetap dapat dibuka lewat URL langsung. Menu tidak
membatasi apa pun. Ini menutup celah itu dengan kurasi permukaan per-App.

Batas: ini **least-privilege permukaan**, BUKAN gerbang otorisasi data
(lihat `docs_internal/plan/app-entry-gate.md`); RBAC/`public_entities` tetap
penjaga data.

## File terkena

- `pkg/spec/resources.go` — `RegisteredViewDecl`, `AppSpec.RegisteredViews`,
  validasi bentuk di `ValidateAppSpec`.
- `internal/ui/registry.go` — `resolveViewRouteLocked` (BuildBundle memegang
  RLock; memanggil `ResolveViewRoute` publik akan RLock rekursif → deadlock).
- `internal/ui/meta.go` — `AppContext.{RegisteredViews,Unfiltered}`,
  `EntitySchema.Routable`, gating di `BuildBundle`, `routeExists`.
- `internal/api/meta.go` — isi `RegisteredViews`; `Unfiltered` pada `?grants=true`.
- `internal/app/resolve.go` — `validateRegisteredViews` (existence).
- `renderers/react-shadcn/src/{types/manifest.ts,shell/router.tsx,shell/landing.ts}`.
- `cmd/formspec/check.go` — `checkAppSurface` (warning permukaan kosong).
- `internal/genjsonschema/generator.go` — daftar `sharedTypes`.
- `examples/kafe/spec/apps/{kafe-qr,kafe-kds,kafe-pos}.yaml`.
- Docs: `docs/spec/frontend/05-app-kinds.md` §1.2,
  `docs/spec/platform/02-workspace-app-module.md` §3/§4.

## Bukti

- `go build ./...`, `gofmt -l` bersih; `go test ./...` 0 FAIL.
  Test baru: `pkg/spec/registered_views_test.go`,
  `internal/ui/registered_views_test.go`, `internal/app/registered_views_test.go`,
  `internal/app/kafe_spec_test.go` (memuat **tree kafe nyata** dan
  me-resolve App-nya — typo di manifest contoh akan gagal di sini).
- Frontend: `npx tsc -b` bersih; `npx vitest run` 46 file / 597 test lulus
  (ditambah kasus `routable` di `router.shapes.test.tsx` + `landing.test.ts`).
- `formspec check -f examples/kafe/spec` (binary `go run`, jadi kode baru):
  **0 error, 0 warning**. Warning permukaan kosong diverifikasi memicu pada spec
  minimal App-tanpa-menu.
- `make generate-schema` (171 shared defs) + `make generate-kind-docs`.

## Sisa

- **Gerbang permukaan ini tidak menutup akses API langsung.** Route data
  `/_ui/entity/...` berskope workspace/module; request-time App gate
  (`access_permission` → 403 di `/_meta/ui`) tetap belum ada → **14.c.4 ⏸️**
  (`docs_internal/plan/app-entry-gate.md`). `registered_views` mengurasi
  permukaan SPA, bukan endpoint data.
- Picker yang menautkan ke halaman detail entity non-routable → 404 → **5.25.2 ⏸️**.
- Lint pre-existing (`isModule`, `assetSource` tak terpakai) → **5.25.3 ⏸️**.
