# 2026-08-24-023 — Track C: Chrome Struktural (5.9.3, 5.2.7, 5.10.8)

## Apa yang diubah

Implementasi Track C widget strategy (docs/plan/widget-strategy.md) — chrome struktural:

**`5.9.3` formspec.ui centralized service:**

- `lib/ui.ts` (baru) — `toast` re-export (wrapper sonner, API identik) + `confirm`/`dialog`/
  `drawer` promise-based via event-bus (`onConfirm`/`onDrawer`). `ui` namespace = permukaan
  `formspec.ui` yang nanti di-inject ke component `asset` (5.9.2).
- `shell/UiHost.tsx` (baru) — host yang merender request confirm/dialog (via `ConfirmDialog`)
  dan drawer (via `Sheet`), me-resolve promise saat user bertindak. Di-mount di `App.tsx`.
- 9 renderer dimigrasi: `import { toast } from "sonner"` → `import { toast } from "@/lib/ui"`
  (FormRenderer, KanbanRenderer, DetailPage, PrintRenderer, ReportRenderer, TableRenderer,
  TimelineRenderer, SearchSelect, WizardRenderer). Call site tidak berubah.

**`5.2.7` Page banner/alert/notice block:**

- `SectionBlocks.tsx` — `AlertBlock` (variant `info`/`success`/`warning`/`destructive`, ikon +
  warna sesuai variant); dispatcher menerima `banner`/`alert`/`notice`.
- `manifest.ts` — komentar closed set `SectionBlock` diperluas.

**`5.10.8` empty-state:**

- `components/ui/empty-state.tsx` (baru) — komponen `EmptyState` reusable (icon/title/
  description/action).

## Kenapa

Memberi developer chrome UI yang lebih kaya: block alert deklaratif di Page, empty-state
reusable, dan layanan `formspec.ui` terpusat sebagai fondasi injeksi ke component `asset`
(5.9.2) — renderer kini tidak mengimpor `sonner` langsung.

## File terdampak

- `renderers/react-shadcn/src/lib/ui.ts`, `shell/UiHost.tsx`, `components/ui/empty-state.tsx` — baru
- `renderers/react-shadcn/src/components/sections/SectionBlocks.tsx` — `AlertBlock` + dispatcher
- `renderers/react-shadcn/src/types/manifest.ts` — komentar closed set
- `renderers/react-shadcn/src/App.tsx` — mount `UiHost`
- 9 renderer — migrasi import toast ke `@/lib/ui`
- `docs/plan/todo.md` — tandai 5.9.3, 5.2.7, 5.10.8 ✅

## Verifikasi

- `npx vitest run` — 144 test lulus
- `npx tsc --noEmit` — bersih
- `go test ./...` — tidak ada perubahan backend

## Catatan

- `5.9.2` `formspec.components` (expose widget dasar ke asset) dan `5.9.1` asset loader belum
  dikerjakan — `formspec.ui` sudah siap sebagai fondasi; injeksi penuh menyusul saat asset
  loader landing.
