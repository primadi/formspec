# 2026-08-25-019 — Fase 7: Subscription Tier 2 Streaming (7.3.2)

## Apa yang diubah

Implementasi **Tier 2 (Streaming)** untuk `kind: Subscription` — kontrak
`docs/spec/backend/02-core-extended.md` §3. Subscription dengan
`durability: durable` kini memakai stream backend (bukan dispatch langsung):
at-least-once, positioned replay, filter/transform Starlark, retry, dan
dead-letter.

**Abstraksi `internal/stream` (KVStore-like seam).** Redis TIDAK diakses
langsung — semua akses lewat interface `stream.Stream` (`Append`/`Read`/`Ack`/
`Trim`/`Close`), implementasi bisa diganti: `memory` (dev default) dan `redis`
(Redis Streams / Valkey). Backend dipilih via env `FORMSPEC_STREAM`
(memory|redis) + `FORMSPEC_REDIS_ADDR` (default `valkey:6379` — service
Redis-compatible di dev container).

**StreamingWorker** (`internal/subscription/stream.go`) — poll semua
subscription durable, baca entry dari stream (group = `{module}/{name}`),
terapkan `filter` (skip + ack kalau false) dan `transform` (ganti payload),
dispatch ke handler (path sama dengan Tier 1), ack kalau sukses. Gagal →
retry sampai `max_retry` (default 3), lalu dead-letter ke `{stream}.dead` +
ack. `position` (`latest`/`earliest`/`<id>`) dipakai saat group baru dibuat;
`retention` (`7d`/`24h`/`1000`) di-enforce via `Trim`.

**Publisher side** — `subscription.Dispatcher` diberi `Stream` opsional;
subscription `durability: durable` di-append ke stream (nama = fq event name)
bukan dispatch langsung. Wiring di `resource/formspec.go` (boot + reload,
worker di-rebuild saat spec reload, backend stream di-reuse).

**Schema fix** — `SubscriptionSpec.Durable` yaml tag `durable` → `durability`
(selaras kontrak spec §3); schema di-regenerate.

## Kenapa

Tier 1 (outbox) transaksional untuk GL/billing/inventory; Tier 2 streaming
untuk analytics/audit/monitoring dengan fan-out banyak subscriber dan
positioned replay. Redis dipakai lewat abstraksi agar implementasi bisa
di-swap (memory/redis/kafka) tanpa menyentuh subscription code.

## File terdampak

- `internal/stream/stream.go` (baru) — interface `Stream` + `Entry` + `ParseRetention`
- `internal/stream/memory.go` (baru) — backend in-memory (dev default)
- `internal/stream/redis.go` (baru) — backend Redis Streams / Valkey
- `internal/stream/stream_test.go`, `redis_test.go` (baru) — unit test
- `internal/subscription/stream.go` (baru) — `StreamingWorker`
- `internal/subscription/stream_test.go` (baru) — unit test
- `internal/subscription/dispatch.go` — durable dispatch append ke stream
- `internal/subscription/registry.go` — `Durable()` helper
- `resource/formspec.go` — `buildStreamBackend` + wiring (boot + reload)
- `pkg/spec/resources.go` — yaml tag `durability`
- `schemas/` — regenerate (Subscription `durability`)
- `go.mod` — `github.com/redis/go-redis/v9`
- `examples/service-demo/` — contoh subscription durable
  (`product-created-stream.yaml` + `handle_product_stream.star`)
- `docs/plan/fase7-subscription-streaming.md` (baru) — plan

## Referensi

- Todo: `docs/plan/todo.md` §7.3.2
- Plan: `docs/plan/fase7-subscription-streaming.md`
- Spec: `docs/spec/backend/02-core-extended.md` §3