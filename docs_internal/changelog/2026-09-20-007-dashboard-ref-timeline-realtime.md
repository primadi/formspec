# 7.3 & 7.4 — DashboardWidget.ref module-qualified + realtime Timeline

**Tanggal:** 2026-09-20 · **TODO:** `examples/kafe/gaps_found/TODO.md` 7.3, 7.4

## 7.3 — `DashboardWidget.ref` module-qualified (#16)

### Apa yang diubah

`renderers/react-shadcn/src/stores/meta.ts` — lookup widget memakai `byQualified`
(baru): entri di-key oleh nama polos **dan** bentuk module-qualified
(`module/name` dan `module.name`). `createLookups` diekspor untuk testabilitas.

### Kenapa

`byName` hanya memakai `item.name` polos, sehingga `DashboardWidget.ref`
dicocokkan dengan nama polos — dua widget bernama sama di module berbeda akan
bertabrakan, dan `ref: cafe-report/omzet-hari-ini` tidak resolve.

### Bukti

| Perintah | Hasil |
| --- | --- |
| `vitest run src/stores/meta.test.ts` | 3 PASS (module/name, module.name, bare name) |
| `tsc -p tsconfig.app.json --noEmit` | bersih |

## 7.4 — Realtime untuk Timeline (#17)

### Apa yang diubah

- `pkg/spec/frontend.go` — `TimelineSpec.Realtime bool`.
- `renderers/react-shadcn/src/types/manifest.ts` — `TimelineSpec.realtime`.
- `renderers/react-shadcn/src/kinds/timeline/TimelineRenderer.tsx` — `useRealtime`
  + reset cursor & refetch saat event entity cocok.
- `schemas/` — ter-regenerasi.

### Kenapa

Realtime hanya di-wire di Table/Kanban/Dashboard; Timeline tidak. Timeline
append-only dengan cursor pagination, jadi entri baru harus diambil dari atas —
reset cursor + items lalu refetch.

### Catatan adopsi

Kafe memakai **Kanban** untuk KDS (sudah `realtime: true`), bukan Timeline — jadi
tidak ada Timeline kafe untuk diadopsi. Ini kemampuan engine yang kini tersedia
untuk semua kind.

### Bukti

| Perintah | Hasil |
| --- | --- |
| `tsc -p tsconfig.app.json --noEmit` | bersih |
| `vitest run` | **276 lulus** |
| `formspec validate` kafe | 0 problem |
| `go test ./...` | hijau |
