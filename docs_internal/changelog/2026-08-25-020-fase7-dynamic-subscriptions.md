# 2026-08-25-020 — Fase 7: Dynamic Subscriptions (7.3.4)

## Apa yang diubah

Implementasi **dynamic subscriptions** — subscription yang dibuat runtime
lewat API/admin panel sebagai _data, bukan manifest_ (kontrak
`docs/spec/backend/02-core-extended.md` §3), hidup di `formspec.core`.

**Entity `formspec.core.subscription` (bundled, UIExposed).** Module baru
`internal/subscription/module/` (module.yaml + entities/subscription.yaml)
di-register via `subscription.RegisterCoreEntities(reg)` (pola sama seperti
`internal/auth/core.go`). Field memetakan `SubscriptionSpec`: `name` (natural
key), `events`, `handler_type`, `handler_ref`, `durability`, `store`,
`position`, `filter`, `transform`, `max_retry`, `retention`, `active`.
UIExposed (`formspec.dev/ui-exposed: "true"`) → dikelola lewat admin panel
(surface `/_ui/entity/`), tanpa route external API.

**Konversi + merge.** `internal/subscription/dynamic.go`:

- `RecordToSubscription(data)` — pure function record → `DynamicSubscription`;
  skip record non-aktif / tanpa events / tanpa handler.
- `Registry.MergeDynamic(subs)` — replace semua dynamic entry (key
  `formspec.core/{name}`), manifest tidak tersentuh.
- `DynamicRefresher` — worker poll (default 5s) yang memanggil `DynamicSource`
  (callback baca entity store) lalu `MergeDynamic`; `Refresh()` sinkron untuk
  boot/reload.

**Wiring (`resource/formspec.go`).** Register embedded module sebelum
`LoadEntities()` (boot + reload — reload sebelumnya drop entity
`formspec.core.subscription` karena tidak di-register ulang); `DynamicSource`
membaca `formspec.core.subscription` via `GetEntityStore` + `List`; refresher
di-wire ke `App` (start di `StartBackgroundWorkers`, stop di `Close`, rebuild
saat reload).

## Kenapa

Manifest Subscription mendefinisikan apa yang ikut ter-ship bersama module;
dynamic subscription mencatat pilihan operator (dibuat runtime tanpa redeploy).
Perubahan CRUD berlaku tanpa restart via DynamicRefresher.

## File terdampak

- `internal/subscription/module/module.yaml` (baru)
- `internal/subscription/module/entities/subscription.yaml` (baru)
- `internal/subscription/module.go` (baru) — ModuleFS + RegisterCoreEntities
- `internal/subscription/dynamic.go` (baru) — konversi + merge + refresher
- `internal/subscription/dynamic_test.go` (baru) — unit test
- `internal/subscription/registry.go` — `MergeDynamic`
- `resource/formspec.go` — register module + wire refresher (boot + reload)
- `examples/service-demo/` — contoh handler dynamic subscription
  (`handle_dynamic.star`)
- `docs/plan/fase7-dynamic-subscriptions.md` (baru) — plan

## Referensi

- Todo: `docs/plan/todo.md` §7.3.4
- Plan: `docs/plan/fase7-dynamic-subscriptions.md`
- Spec: `docs/spec/backend/02-core-extended.md` §3
