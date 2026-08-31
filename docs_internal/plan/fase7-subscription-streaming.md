# Fase 7 — Subscription Tier 2 Streaming (7.3.2)

**Status:** ✅ Complete · **Tanggal:** 2026-08-25
**Referensi:** `docs/spec/backend/02-core-extended.md` §3 (Subscription & Event
Delivery), `pkg/spec/resources.go` (SubscriptionSpec Tier 2 fields),
`docs/plan/fase7-subscription-engine.md` (Tier 1)
**Todo:** `docs/plan/todo.md` §7.3.2

## Konteks

Tier 1 (outbox) sudah selesai: event → match Subscription → call handler,
transaksional. Tier 2 (Streaming) menambah `durability: durable` dengan
`store`, `retention`, `position`, `max_retry`, `dead_letter`, plus
`filter`/`transform` Starlark atas payload event.

Kontrak (02-core-extended.md §3):

|             | Tier 1 — Core (outbox) | Tier 2 — Streaming               |
| ----------- | ---------------------- | -------------------------------- |
| Storage     | Outbox PersistBackend  | Redis Stream / Kafka             |
| Konsistensi | Transaksional          | At-least-once, positioned replay |
| Fan-out     | Satu target per entry  | Banyak subscriber                |
| Pemakaian   | GL, billing, inventory | Analytics, audit, monitoring     |

## Prinsip desain

**Redis TIDAK diakses langsung** — semua akses lewat abstraksi `Stream`
(seperti `KVStore` untuk `ctx.kvstore`). Implementasi bisa diganti:
`memory` (dev default), `redis` (Redis Streams / Valkey), `kafka` (future).
Subscription code hanya bicara ke interface `stream.Stream`.

## Scope

### STR-1 — Abstraksi `internal/stream` (KVStore-like seam)

- `internal/stream/stream.go` — interface `Stream` + tipe `Entry`:
  - `Append(ctx, stream, data) (id, err)` — append event ke stream
  - `Read(ctx, stream, group, consumer, position, count) ([]Entry, err)` —
    claim entry (at-least-once: pending sampai `Ack`); `position` dipakai saat
    group baru dibuat (`latest` | `earliest` | `<id>`)
  - `Ack(ctx, stream, group, id) error` — tandai selesai
  - `Trim(ctx, stream, retention) error` — enforce retention ("7d", "24h", "1000")
  - `Close() error`
  - `Entry{ID, Data, Timestamp, Attempts}` — `Attempts` = jumlah delivery
    (untuk retry/dead-letter)
- `internal/stream/memory.go` — implementasi in-memory (dev default):
  per-stream entry list + per-group cursor + pending map (attempts)
- `internal/stream/redis.go` — implementasi Redis Streams (Valkey):
  `XADD`, `XGROUP CREATE` (MKSTREAM, posisi), `XREADGROUP >` (baru),
  `XAUTOCLAIM` + `XPENDING` (pending + delivery count), `XACK`, `XTRIM`
- `internal/stream/stream_test.go` — unit test (memory + redis via testcontainers
  tidak; redis test di-skip kalau `valkey:6379` tidak reachable)

### STR-2 — StreamingWorker (`internal/subscription/stream.go`)

- Poll semua subscription durable (`Durable == "durable"`) di registry
- Per (subscription, event): `Read` dari stream (nama stream = fq event name),
  group = `{module}/{name}`, consumer = `formspec-worker`
- Per entry:
  1. Bangun env filter/transform: `event.name`, `event.resource`,
     `event.workspace_id`, `event.occurred_at` + field payload langsung
  2. `filter` (Starlark) → false = ack + skip
  3. `transform` (Starlark) → hasil dict = payload baru
  4. Dispatch ke handler (action dispatcher, sama seperti Tier 1)
  5. Sukses → `Ack`
  6. Gagal → kalau `Attempts >= max_retry` (default 3): dead-letter
     (append ke `{stream}.dead` + log) lalu `Ack`; kalau belum → biarkan
     pending (redelivery next poll, at-least-once)
- `Start(ctx)` / `Stop()` — di-wire ke `App.StartBackgroundWorkers`/`Close`

### STR-3 — Publisher side (durable dispatch)

- `subscription.Dispatcher` diberi `Stream` opsional
- `Dispatch`: subscription dengan `Durable == "durable"` → `Append` ke stream
  (bukan dispatch langsung); selain itu → dispatch langsung (Tier 1)
- Entry data menyimpan metadata: `workspace_id`, `resource`, `event`,
  `payload`, `occurred_at`

### STR-4 — Wiring (`resource/formspec.go`)

- Backend stream dipilih via env `FORMSPEC_STREAM` (`memory` default |
  `redis`) + `FORMSPEC_REDIS_ADDR` (default `valkey:6379` — service
  Redis-compatible di dev container; bisa di-override `redis:6379`)
- `subscription.NewDispatcher(reg, disp, stream)` + `subscription.NewStreamingWorker`
- `App.stream` + `App.streamingWorker`; start di `StartBackgroundWorkers`,
  stop di `Close`; reload spec → rebuild worker (reuse stream backend)

### STR-5 — Contoh & verifikasi

- `examples/service-demo/` — subscription durable (`durability: durable`,
  `store: redis`, `position: earliest`, `filter`, `transform`, `max_retry`)
  yang melanggan event `demo.products.on_create`
- Verifikasi: `formspec dev` + curl create product → stream worker dispatch
  handler → outbox status completed; `go test ./...` hijau

## Level of effort

| STR | Effort |
| --- | ------ |
| 1   | medium |
| 2   | medium |
| 3   | small  |
| 4   | small  |
| 5   | small  |

## Verifikasi end-to-end (via `formspec dev` + curl)

- `FORMSPEC_STREAM=redis` + `formspec dev` di `examples/service-demo` →
  `[subscription-stream] started (poll=500ms, batch=10)`.
- Create product → event `demo.product.on_create` di-append ke Redis stream
  (`XLEN=1`, data JSON-encoded) → StreamingWorker read (`entries-read: 1`),
  filter `min_stock >= 0` lolos, transform diterapkan, handler di-dispatch,
  entry di-ack (`pending: 0`, tidak ada dead-letter). ✅
- `go test ./...` hijau (27 paket, termasuk `internal/stream` memory+redis
  dan `internal/subscription` streaming worker).

## File terdampak

- `internal/stream/stream.go` (baru) — interface + Entry
- `internal/stream/memory.go` (baru) — backend in-memory
- `internal/stream/redis.go` (baru) — backend Redis Streams
- `internal/stream/stream_test.go` (baru) — unit test
- `internal/subscription/stream.go` (baru) — StreamingWorker
- `internal/subscription/stream_test.go` (baru) — unit test
- `internal/subscription/dispatch.go` — durable dispatch ke stream
- `internal/subscription/registry.go` — `Durable()` helper
- `resource/formspec.go` — wiring backend + worker (boot + reload)
- `go.mod` — `github.com/redis/go-redis/v9`
- `examples/service-demo/` — contoh subscription durable
