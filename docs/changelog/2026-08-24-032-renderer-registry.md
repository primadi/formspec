# 2026-08-24-032 — Renderer Registry & Resolution (5.16.1–5.16.3)

## Apa yang diubah

Implementasi renderer registry & resolution engine (frontend/03-renderer-kind.md §3,
02-visual-spec-kind.md §4–§5, 01-visual-hierarchy.md §3).

**`internal/manifest/renderer.go` (baru):**

- `RendererRegistry` — mengindeks `Renderer` + `VisualSpecKind` manifests.
- `ResolveRenderer(implements, stackFamily, explicit)` — **5.16.1**: hanya `trust_tier:
official` yang auto-select; tanpa official → error + sarankan kandidat verified/community
  (tidak pernah silent fallback); override eksplisit menang.
- `ValidateSlotTiers` — **5.16.2**: `accepts_slots` hanya sah dari tier page|app;
  `implements_slot` hanya dari tier component; kombinasi lain ditolak.
- `ValidateStackFamily` — **5.16.3**: App shell + shell-integrated Page wajib satu
  stack_family; mismatch = error.
- `ValidateRendererResolution` — App `renderers:` map + Page `renderer:` field harus resolve
  ke renderer terdaftar.

**Spec:**

- `pkg/spec/spec.go` — `KindRenderer`, `KindVisualSpecKind`, `KindPersistBackend`.
- `pkg/spec/resources.go` — `AppSpec.Renderers` map (implements → renderer).
- `pkg/spec/frontend.go` — `PageSpec.Renderer` (per-instance override).
- `internal/manifest/loader.go` — `KnownKinds`.

**Wiring:** `cmd/formspec/check.go` — `checkRenderers` (slot-tier + resolution +
stack_family) dijalankan saat `formspec check`.

## File terdampak

- `internal/manifest/renderer.go` (baru), `renderer_test.go` (baru)
- `pkg/spec/spec.go`, `pkg/spec/resources.go`, `pkg/spec/frontend.go`
- `internal/manifest/loader.go`, `cmd/formspec/check.go`

## Referensi

- Plan: `docs/plan/fase5-completion.md` (WS-J)
- Todo: `docs/plan/todo.md` §5.16.1–5.16.3
