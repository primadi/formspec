# Plan — Flexible `root_url` (bebas di dalam workspace)

Status: **Implemented** (2026-08-30, lihat `docs/changelog/2026-08-30-002-flexible-root-url.md`) · Tanggal: 2026-08-30
Referensi spec: `docs/spec/platform/02-workspace-app-module.md` §4 · `docs/spec/frontend/05-app-kinds.md` §1

## Motivasi

Skenario single-app: satu App `barbershop` di workspace dipaksa mount di
`/{ws}/app/barbershop` padahal `/{ws}` (atau `/{ws}/barbershop`) lebih
sederhana. Constraint lama (private App wajib `/app/*`) adalah artefak
implementasi — SPA hanya di-mount hard-coded di `/{ws}/app` — bukan keputusan
kontrak.

## Desain

`root_url` menjadi **prefix virtual bebas di dalam workspace**: `/`,
`/barbershop`, `/app/kafe`, dll. Server me-mount SPA shell dinamis di setiap
`root_url`; SPA di browser tetap memilih App via longest-prefix match
(`detectAppName` — sudah generic, tidak berubah).

### Aturan validasi baru (`internal/app/resolve.go`)

- Tetap required, unik per workspace (exact match).
- Normalisasi: trailing `/` di-strip.
- Harus `/` atau prefix `/`.
- **Reserved first segment** (bentrok dengan surface tetap engine):
  `_ui`, `api`, `_admin`, `assets`, `health`, `login`, `register`, `_ws`,
  `print`. (`app` TIDAK reserved — tetap valid sebagai konvensi lama.)
- Overlap prefix (mis. `/app` vs `/app/kafe`) tetap diizinkan — resolusi
  longest-prefix menang; `/` (public landing) memang by-design meng-claim
  semua subpath.

### Dynamic mount (`internal/api/router.go`)

`RouterBuilder` sudah punya `b.apps` (via `SetApps`). Di `BuildHTTP`, mount
SPA di: `/_admin` (tetap) ∪ `/app` (legacy, backward compat) ∪ setiap
`root_url` App. Dedupe sebelum register (chi panic pada duplikat). Untuk
`root_url: /` → `r.Get("/", spa)` + `r.Get("/*", spa)` (splat hanya menang
bila tidak ada route lebih spesifik — API routes tetap menang).

### SPA (`renderers/react-shadcn/src/App.tsx`)

`RootSurface` (route catch-all `/:workspace/*`) digeneralisasi: render
best-match App apapun `access`-nya — public → anonymous boot, private →
session boot + redirect ke `{surfacePath}/login`. Tidak ada match → redirect
`/{ws}/_admin` (perilaku lama). Route `/:workspace/app/*` dan
`/:workspace/_admin/*` tetap (lebih spesifik, menang untuk App ber-prefix
`/app`).

### Schema (`pkg/spec/resources.go`)

Pattern `@schema` → `^(/|/[^/]+(/[^/]+)*)$` (tanpa trailing slash; reserved
segment divalidasi runtime). Regenerate `make generate-schema` +
`make generate-kind-docs`.

## File terkena dampak

| File                                                         | Perubahan                         | Effort |
| ------------------------------------------------------------ | --------------------------------- | ------ |
| `internal/app/resolve.go`                                    | validasi baru + reserved segments | small  |
| `internal/api/router.go`                                     | dynamic mount + dedupe            | small  |
| `renderers/react-shadcn/src/App.tsx`                         | generalisasi RootSurface          | small  |
| `pkg/spec/resources.go`                                      | @schema pattern                   | small  |
| `internal/app/resolve_test.go`                               | test kasus baru                   | small  |
| `internal/api/router_test.go` (jika ada)                     | test mount dinamis                | small  |
| docs (kind/App.md, spec platform 02, frontend 05, ai_skills) | kontrak                           | small  |

## Keputusan terbuka

- Legacy mount `/app` dipertahankan tanpa syarat — bookmark lama & SPA
  fallback tetap berfungsi.
- JSON 404 untuk GET API path yang tidak dikenal berubah jadi HTML index
  HANYA di workspace yang punya App `root_url: /` (konsekuensi splat).
  Diterima — sama dengan perilaku SPA-fallback di `/_admin/*`.
