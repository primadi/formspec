# 2026-09-22-003 — Guard parity mirror TS ↔ Go + tutup drift (todo 9.3.2)

**Plan/Todo**: item **9.3.2** (`docs_internal/plan/todo.md`); menyinggung 9.3.1.

Item 9.3.2 meminta "validate generated types against manual `types/manifest.ts`".
`formspec generate` sudah ada dan terverifikasi (`examples/cafe/spec` → 12
`export interface`), tetapi menghasilkan **artefak berbeda**
(`formspec-client.ts`, client per-entitas) dari mirror hand-written
`renderers/react-shadcn/src/types/manifest.ts` — jadi perbandingan
"generated vs manual" tidak punya objek. Dikerjakan hal yang setara dan lebih
berguna: **guard lintas-bahasa** yang mempin mirror itu ke sumber Go.

**Guard baru** — `pkg/spec/renderer_parity_test.go`:

- `TestTSManifest_APIVersionMatchesGo` — `API_VERSION` TS vs `spec.APIVersion`.
- `TestTSManifest_KindsCoverGoCatalog` — himpunan `KIND_*` TS vs
  `spec.AllKinds()`, **dua arah** (kind Go yang tak bisa disebut renderer, dan
  kind TS tanpa padanan Go).

Guard ini **gagal lebih dulu** dan mengungkap drift nyata:

1. `API_VERSION = "formspec.dev/v1alpha1"` — nilai pra-stabil yang
   **ditolak** `internal/schemaregistry.ParseVersion` (dan sudah dibetulkan di Go
   pada 8.2) → kini `formspec.dev/v1`.
2. `KIND_MIGRATION = "Migration"` — **phantom**: `kind: Migration` tidak pernah
   ada di katalog engine (`internal/manifest.KnownKinds`) → dihapus.
3. 10 kind Go tak punya konstanta TS: `Integrator`, `KindDefinition`, `Mockup`,
   `Renderer`, `VisualSpecKind`, `PersistBackend`, `Workspace`, `Calendar`,
   `ApprovalInbox`, `NotificationCenter` → ditambahkan; union `ResourceKind`
   ikut diselaraskan.

**Temuan menyertai (Go)**: `spec.IsValidKind` adalah pengecekan yang **tidak
pernah dipanggil** — katalog efektifnya `internal/manifest.KnownKinds` — dan
daftarnya sudah **drift**: `Workspace` diterima engine (`kind: Workspace` adalah
seed platform yang nyata; `KindWorkspace` ada di `pkg/spec/workspace.go`) tetapi
`IsValidKind` bilang *tidak*. Diperbaiki + `spec.AllKinds()` baru (enumerator
sorted, mem-paritas-kan `AllFormWidgets`), dipin dua arah oleh
`TestIsValidKind_MatchesKnownKinds` + `TestKnownKinds_ContainsWorkspace`
(`internal/manifest/kind_catalog_test.go`).

**File terkena dampak**: `pkg/spec/spec.go` (`IsValidKind` + `AllKinds`),
`pkg/spec/renderer_parity_test.go` (baru),
`internal/manifest/kind_catalog_test.go` (baru),
`renderers/react-shadcn/src/types/manifest.ts`.

**Bukti**: kedua guard TS gagal lebih dulu dengan daftar drift yang persis, lalu
hijau; `TestIsValidKind_MatchesKnownKinds` + `TestKnownKinds_ContainsWorkspace`
hijau; `go test ./...` hijau; `npx tsc --noEmit` bersih; `vitest run` 288 lulus.

**Sisa** → item **9.3.1 ⏸️** di todo: mengganti mirror hand-written dengan tipe
yang di-generate dari `pkg/spec` (atau memutuskan mirror itu final). `make
generate` masih stub dan target path-nya (`renderers/web/`) sudah tidak ada.
