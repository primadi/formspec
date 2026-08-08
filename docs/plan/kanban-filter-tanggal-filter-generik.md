# Plan: Kanban di-scope per tanggal — filter generik (default + fixed)

**Tanggal:** 2026-08-07
**Status:** ✅ Selesai
**Referensi Spec:** `docs/spec/frontend/06-page-kinds.md` §3.3 (kontrak filter) & §4 Kanban
**Changelog:** `docs/changelog/2026-08-07-001-filter-generik-kanban-dan-fixed-filters.md`
**Todo:** `docs/plan/todo.md` §5.5.9

## Masalah

Board `consultation-board` (kanban antrian konsultasi klinik) menampilkan
**semua** kunjungan tanpa batas tanggal. `fetchRecords` di `KanbanRenderer`
memanggil list API tanpa filter apa pun; `filters: [polyclinic_id]` hanya
dropdown client-side di atas record yang sudah ter-fetch. Untuk entity
transaksi (`characteristic: transaction`) ini tidak scalable, dan secara
konsep board antrian bersifat **per hari** (`queue_number` reset daily).

## Solusi — filter generik (bukan hardcode kanban)

Dua kemampuan generik, dipakai identik oleh Table & Kanban:

1. **`filters`** (user-adjustable) kini objek `FilterSpec` — bisa punya
   `default` value. Filter `date` dengan `default: today` membuat board
   terbuka ter-scope ke tanggal server hari ini, tetap bisa diganti user
   via date picker.
2. **`fixed_filters`** (baru) — filter **immutable, server-side**: selalu
   digabung ke request list, tidak dirender sebagai kontrol, tidak bisa
   dihapus user.

Nilai terkirim ke API sebagai `field[op]=value` (mis. `transaction_date[eq]=2026-08-07`),
sehingga DB mem-filter sebelum baris dikirim. Backend **tidak diubah** — list
API sudah mendukung 13 operator filter (`internal/api/handler.go` `filterOps`),
dan `transaction_date` sudah `index: true`.

## Perubahan

### Schema (`pkg/spec/frontend.go`)
- `TableFilter` → `FilterSpec` (generik): tambah `op` (default `eq`) dan
  `default` (mendukung `today()`); `type` opsional, tambah `date`.
- `TableSpec.Filters []TableFilter` → `[]FilterSpec`; tambah `FixedFilters`.
- `KanbanSpec.Filters []string` → `[]FilterSpec`; tambah `FixedFilters`.
- `ApprovalInboxSpec.Filters`, `ListingSpec.Filters` ikut migrasi ke
  `[]FilterSpec` (single shared type).
- `internal/genjsonschema/generator.go` whitelist `"TableFilter"` → `"FilterSpec"`.

### Validasi (`internal/ui/validate.go`)
- Cek field `fixed_filters` Table; cek field `filters` + `fixed_filters`
  Kanban terhadap field entity.

### Renderer
- **`renderers/web/src/lib/filters.ts`** (baru) — helper bersama:
  `serverToday()`, `resolveFilterValue()` (`today()` → tanggal server),
  `buildFixedFilterParams()`, `buildUserFilterParams()`.
- **`KanbanRenderer.tsx`** — `filterValues` di-seed dari `default`; `fetchRecords`
  merge `fixed_filters` + filter user (operator syntax); kontrol filter per tipe
  (`select`/`date`/`text`); `fixed_filters` tidak dirender; `getColumnRecords`
  tetap filter client-side (search + filter user).
- **`TableRenderer.tsx`** — `filterValues` di-seed dari `default`; merge
  manifest `fixed_filters` (menang atas pilihan user) + runtime `fixedFilters`
  prop Page; `FilterControl` dukung type `date`.

### Manifest
- `examples/Clinic-UI-Showcase/.../kanbans/board.yaml` — `filters` objek:
  `transaction_date` (date, default today) + `polyclinic_id` (select).

### Docs
- `docs/spec/frontend/06-page-kinds.md` §3.3 kontrak filter + §4 Kanban.
- `docs/renderers/shadcn-shell/03-kind-renderers.md` baris Kanban dikoreksi.

## Level of Effort

Medium. Perubahan schema + renderer + manifest + docs; backend nol.

## Verification

1. `make generate-schema` — `FilterSpec` + `fixed_filters` di schema Kanban/Table. ✅
2. `npx tsc --noEmit` web — bersih. ✅
3. `go build`/`go test ./...` — hijau.
4. `forma validate` board.yaml — lolos.
5. Manual: board default hari ini, ganti tanggal, polyclinic filter, realtime.
