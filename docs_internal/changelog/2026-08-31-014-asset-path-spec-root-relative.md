# 2026-08-31-014 — Asset path spec-root-relative (hapus hardcoded modules/)

## Apa

Mengubah kontrak path asset custom component dari _module-relative_
(implisit prefix `modules/`) menjadi **spec-root-relative**:

- Sebelum: `asset: portal/assets/profile.js` → handler join
  `root + "modules/" + module + path` (hardcoded `modules/`).
- Sesudah: `asset: modules/portal/assets/profile.js` → handler join
  `root + path` langsung. Root = spec root, sama seperti semua path lain.

Perubahan:

- `internal/api/asset.go`: handler membaca wildcard `*` sebagai path
  lengkap relatif spec root; join `root + path` langsung (traversal check
  tetap).
- `internal/api/router.go`: route `/assets/{module}/*` → `/assets/*`.
- `internal/api/asset_test.go`: test di-update ke URL/path baru.
- `pkg/spec/frontend.go`: doc comment field `Asset` → "spec-root-relative".
- `renderers/react-shadcn/src/shell/AssetRenderer.tsx`: komentar URL
  resolution.
- `registry/spec/modules/portal/pages/profile.yaml`:
  `asset: modules/portal/assets/profile.js`.
- `schemas/` + `docs/kind/` di-regenerate.

## Kenapa

Satu aturan resolusi untuk semua path: root = spec root. Menghapus
hardcoded `modules/` membuat kontrak asset konsisten dengan cara user
memandang tree spec, dan menghilangkan dual-interpretation
("segmen pertama = module") yang sebelumnya jadi sumber bug 404.

## Verifikasi

- `go build ./...` bersih; `go test ./internal/api ./pkg/spec` 162 pass.
- Validasi registry: 13 manifest, 0 problem.
- curl asset baru → 200; browser: halaman profile render penuh.

## File terdampak

- `internal/api/asset.go`, `internal/api/router.go`, `internal/api/asset_test.go`
- `pkg/spec/frontend.go`, `renderers/react-shadcn/src/shell/AssetRenderer.tsx`
- `registry/spec/modules/portal/pages/profile.yaml`
- `schemas/`, `docs/kind/` (regenerated)
