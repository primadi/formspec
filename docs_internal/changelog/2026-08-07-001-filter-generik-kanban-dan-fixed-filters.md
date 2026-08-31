# Feat: Filter generik (default + fixed_filters) — Kanban di-scope per tanggal

## Perubahan

Model filter data kind dibuat **generik** (`FilterSpec`), dipakai identik oleh
Table & Kanban, untuk menjawab: board kanban konsultasi klinik harus tampil
per tanggal tanpa menarik semua record kunjungan.

### Schema (`pkg/spec/frontend.go`)
- `TableFilter` → `FilterSpec` (field, label, type `text|select|date|date_range`,
  op default `eq`, default mendukung `today()`).
- `TableSpec.Filters` → `[]FilterSpec` + `FixedFilters []FilterSpec` (baru).
- `KanbanSpec.Filters []string` → `[]FilterSpec` (breaking — hanya
  `board.yaml` yang kena) + `FixedFilters []FilterSpec` (baru).
- `ApprovalInboxSpec`/`ListingSpec` ikut migrasi ke `[]FilterSpec`.
- `internal/genjsonschema/generator.go` whitelist diupdate.
- `make generate-schema` — schema Kanban/Table kini punya `filters` objek +
  `fixed_filters`.

### Renderer (`renderers/web/`)
- `lib/filters.ts` (baru): `serverToday()`, `resolveFilterValue()` (`today()` →
  tanggal server UTC, konvensi sama dengan widget query), `buildFixedFilterParams()`,
  `buildUserFilterParams()`.
- `KanbanRenderer` — `filterValues` di-seed dari `default`; fetch kirim
  `fixed_filters` + filter user sebagai `field[op]=value` (server-side);
  kontrol filter per tipe (`select`/`date`/`text`); `fixed_filters` tidak
  dirender (immutable).
- `TableRenderer` — `filterValues` di-seed dari `default`; merge manifest
  `fixed_filters` (menang atas pilihan user) + runtime `fixedFilters` prop Page;
  `FilterControl` dukung type `date`.

### Manifest & Docs
- `board.yaml`: `filters` objek — `transaction_date` (date, `default: today`) +
  `polyclinic_id` (select). Board antrian hari ini, navigable.
- `docs/spec/frontend/06-page-kinds.md` §3.3 kontrak filter + §4 Kanban.
- `docs/renderers/shadcn-shell/03-kind-renderers.md` baris Kanban dikoreksi.

## Kenapa

Board antrian bersifat per hari (`queue_number` reset daily); mem-filter di
client atas semua record tidak scalable untuk entity transaksi. Backend list
API sudah mendukung `field[op]=value` (13 operator) — perubahan murni
schema/renderer/manifest/docs, backend nol.

## File Terkena

- `pkg/spec/frontend.go`, `internal/genjsonschema/generator.go`,
  `internal/ui/validate.go`, `schemas/*`
- `renderers/web/src/lib/filters.ts` (baru), `.../kinds/kanban/KanbanRenderer.tsx`,
  `.../kinds/table/TableRenderer.tsx`, `.../types/manifest.ts`
- `examples/Clinic-UI-Showcase/spec/modules/clinic/transaction/visit/kanbans/board.yaml`
- `docs/spec/frontend/06-page-kinds.md`, `docs/renderers/shadcn-shell/03-kind-renderers.md`,
  `docs/plan/kanban-filter-tanggal-filter-generik.md`, `docs/plan/todo.md`

Referensi: `docs/plan/kanban-filter-tanggal-filter-generik.md`, todo §5.5.9.
