# Fase 7 — Dynamic Subscriptions (7.3.4)

**Status:** ✅ Complete · **Tanggal:** 2026-08-25
**Referensi:** `docs/spec/backend/02-core-extended.md` §3 (Subscription & Event
Delivery), `pkg/spec/resources.go` (SubscriptionSpec),
`docs/plan/fase7-subscription-engine.md` (Tier 1),
`docs/plan/fase7-subscription-streaming.md` (Tier 2)
**Todo:** `docs/plan/todo.md` §7.3.4

## Konteks

**Subscription dinamis** (dibuat runtime lewat API/admin panel) adalah _data,
bukan manifest_ — manifest Subscription mendefinisikan apa yang ikut ter-ship
bersama module; subscription dinamis mencatat pilihan operator, hidup di
`formspec.core`.

## Prinsip desain

- Subscription dinamis = **Entity `formspec.core.subscription`** (bundled,
  UIExposed) — dibuat/diubah/dihapus lewat admin panel (surface `/_ui/entity/`),
  disimpan sebagai record, bukan YAML manifest.
- Registry subscription **menggabungkan** manifest (statis) + dinamis (data).
  Dinamis di-key `formspec.core/{name}` — tidak pernah bentrok dengan manifest.
- Perubahan CRUD berlaku tanpa restart: **DynamicRefresher** (worker poll)
  membaca entity store dan re-merge ke registry.

## Scope

### DYN-1 — Entity `formspec.core.subscription` (bundled)

- `internal/subscription/module/` — module manifest + entity manifest
  (UIExposed via `formspec.dev/ui-exposed: "true"`, pola sama seperti
  `formspec.core.api-key`).
- Field memetakan `SubscriptionSpec`: `name` (natural key), `events` (json),
  `handler_type`, `handler_ref`, `durability`, `store`, `position`, `filter`,
  `transform`, `max_retry`, `retention`, `active` (default true).
- `internal/subscription/module.go` — `ModuleFS()` + `RegisterCoreEntities(reg)`
  (pola sama seperti `internal/auth/core.go`).

### DYN-2 — Konversi record → SubscriptionSpec

- `internal/subscription/dynamic.go`:
  - `DynamicSubscription{Name, Spec}`
  - `RecordToSubscription(data map[string]any) (DynamicSubscription, bool)` —
    pure function; skip record non-aktif / tanpa events / tanpa handler.
  - `Registry.MergeDynamic(subs []DynamicSubscription)` — replace semua
    dynamic entry (key `formspec.core/`), manifest tidak tersentuh.
  - `DynamicRefresher` — worker poll (default 5s) yang memanggil
    `DynamicSource` (callback baca entity store) lalu `MergeDynamic`.

### DYN-3 — Wiring (`resource/formspec.go`)

- `subscription.RegisterCoreEntities(reg)` sebelum `LoadEntities()` (pola auth).
- Setelah `SyncSchema` + build subscription registry: load dynamic subs →
  `MergeDynamic`.
- `DynamicRefresher` di-wire ke `App` (start di `StartBackgroundWorkers`, stop
  di `Close`); reload spec → rebuild refresher (source di-reuse).
- `DynamicSource` membaca `formspec.core.subscription` via `GetEntityStore` +
  `List` (PerPage 100).

### DYN-4 — Contoh & verifikasi

- `examples/service-demo/` — seed satu dynamic subscription via API
  (`POST /_ui/entity/formspec.core/subscriptions`) → event `demo.product.on_create`
  → handler di-dispatch (Tier 1) tanpa manifest.
- Verifikasi: `formspec dev` + curl create dynamic sub → create product →
  handler jalan; `go test ./...` hijau.

## Level of effort

| DYN | Effort |
| --- | ------ |
| 1   | small  |
| 2   | medium |
| 3   | small  |
| 4   | small  |

## Verifikasi end-to-end (via `formspec dev` + curl)

- `formspec dev` di `examples/service-demo` → `[subscription-dynamic] started
(poll=5s)`.
- Create dynamic subscription via UI surface
  (`POST /_ui/entity/formspec.core/subscription`) → DynamicRefresher merge ke
  registry → create product → handler `demo/handle_dynamic` di-dispatch →
  outbox `completed` (0 retry). ✅
- `go test ./...` hijau (27 paket, termasuk `internal/subscription` dynamic
  tests); `formspec validate` embedded module OK.

## File terdampak

- `internal/subscription/module/module.yaml` (baru)
- `internal/subscription/module/entities/subscription.yaml` (baru)
- `internal/subscription/module.go` (baru) — ModuleFS + RegisterCoreEntities
- `internal/subscription/dynamic.go` (baru) — konversi + merge + refresher
- `internal/subscription/dynamic_test.go` (baru) — unit test
- `internal/subscription/registry.go` — `MergeDynamic`
- `resource/formspec.go` — register module + wire refresher (boot + reload)
- `examples/service-demo/` — contoh dynamic subscription
- `docs/plan/fase7-dynamic-subscriptions.md` (baru) — plan
