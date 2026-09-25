# 2026-09-22-006 — Verifikasi kontrak permission filtering (todo 5.12.4)

**Plan/Todo**: item **5.12.4** (`docs_internal/plan/todo.md`).

Item ini bertuliskan "**sebagian**" sejak `?grants=true` mendarat (2026-08-26),
tetapi tanpa penjelasan bagian mana yang kurang — dan ketiga klaimnya ternyata
**sudah berlaku**, hanya saja tidak ada yang meng-assert-nya. Alih-alih
menambah kode, tiap klaim diverifikasi terhadap kode + test:

| Klaim | Temuan |
| --- | --- |
| entity → 404 kalau tanpa list/view | `BuildBundle` (`internal/ui/meta.go:398`) menyaring entitas tanpa `list`/`view`; `internal/api/handler.go` → **404** di surface UI, **403** di surface eksternal (surface-aware, Core §15.2). Test: `TestEnforcement_UISurface_EntityList_NoPerm_404`, `TestEnforcement_External_EntityList_NoPerm_403`. |
| page → disembunyikan | `allowedPage` (`meta.go:634`) menyembunyikan Page yang `permissions`-nya tidak dimiliki. Test: `TestBuildBundlePermissionFiltering` (sub-test `no permissions sees nothing entity-backed` → hanya `settings` lolos). |
| action → permission dikirim, tidak difilter | `buildEntitySchema` (`meta.go:684`) **selalu** menambahkan `ActionSummary{Permission: …}` tanpa `can()` — memang aditif. |

**Yang ditambahkan** hanya pengunci untuk klaim ketiga (satu-satunya yang belum
punya test): `TestBuildBundle_ActionPermissionsAreAdditiveNotFiltered`
(`internal/ui/ui_test.go`) — action list + permission string tetap utuh untuk
caller yang cuma punya `list`, dan invariant terhadap permission caller. Tanpa
ini, sebuah edit di masa depan bisa mulai memfilter action dan **merusak
GrantsEditor diam-diam** (admin tidak lagi bisa menawarkan action yang belum ia
pegang).

**File terkena dampak**: `internal/ui/ui_test.go` (test baru),
`docs_internal/plan/todo.md`.

**Bukti**: `go test ./internal/ui/ ./internal/api/` hijau; `go test ./...` hijau.

**Dicatat sebagai konsekuensi, bukan celah tersembunyi**: karena aditif, caller
yang boleh `view` sebuah entity menerima **semua** action-nya di bundle selama
entity itu kelihatan; objek `permission` per-action juga belum dibaca renderer
(`grep '\.permission'` di `renderers/react-shadcn/src` → 0 hit), jadi tombol
action hari ini tidak di-gate klien. Enforcement tetap 100% di server; yang
belum ada hanya penyembunyian tombol. Dinyatakan apa adanya supaya tidak
diklaim lebih dari yang benar.
