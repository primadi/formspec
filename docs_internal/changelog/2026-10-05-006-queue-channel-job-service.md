# 2026-10-05-006 — Channel `queue` terkirim: `job:` menamai Service action (7.7.6)

Increment 2 dari 7.7.6. Increment 1 (`2026-10-05-005`) membuat ketiga kanal
**jujur** (dilaporkan + gagal, bukan diam-diam "delivered"). Ini menutup salah
satunya dengan benar.

## Kontrak: `job:` menamai Service action

```yaml
deliver:
  - { channel: queue, job: receipt-jobs.generate-receipt } # billing.receipt-jobs.generate-receipt
  - { channel: queue, job: gl.journal-jobs.post } # lintas module
```

`service.action` (module = publisher) atau `module.service.action`. Nama polos
(`job: generate-receipt`) **ditolak**: tidak ada Service yang bisa diatribusikan,
dan menebaknya berarti mengarang konvensi.

Alasan bentuk ini, bukan registry job handler tersendiri:

- Sebuah job adalah **komputasi tanpa state** — tepatnya definisi `kind: Service`.
- Service sudah punya `impl` (native/script/script_ref/sidecar), penegakan
  `uses`/permission, dan dispatcher yang **sudah jalan** (`resource.call`,
  `Integrator`, `deliver: target`). Registry kedua = mesin kedua untuk pekerjaan
  yang sama.
- Konsisten dengan target lain yang juga menyebut resource+action.

## Antreannya adalah outbox — dan itu dinyatakan

`ValidateEventDurability` sudah mewajibkan `publish.durable: true` untuk channel
`queue`, jadi event-nya toh sudah masuk outbox. Worker outbox yang memanggil job,
me-retry, lalu dead-letter. Tidak ada tabel queue kedua.

Batas yang dicatat di spec: **tidak ada worker paralel** — throughput terikat
poll interval outbox (default ~1s). Queue sungguhan (Redis/Kafka) tetap pekerjaan
tersendiri, bukan yang diklaim di sini.

## Perubahan

| File                                       | Isi                                                                                                             |
| ------------------------------------------ | --------------------------------------------------------------------------------------------------------------- |
| `pkg/spec/entity.go`                       | doc kontrak `EventDeliveryDecl.Job`                                                                             |
| `pkg/spec/delivery_channels.go`            | `queue` keluar dari daftar tak terkirim; `ResolveJobRef`                                                        |
| `internal/action/deliver.go`               | `case "queue"`: durable → enqueue outbox; non-durable → error (tak boleh ada, tapi struct bisa dibangun manual) |
| `renderers/jsonb-persist/event_handler.go` | `Jobs JobDispatch` + `case "queue"`                                                                             |
| `resource/formspec.go`                     | `newJobDispatch` + wiring di boot **dan** reload                                                                |
| `cmd/formspec/validate_events.go`          | `job:` wajib; harus resolve ke Service action; bare name ditolak                                                |
| `verticals/billing`                        | Service `receipt-jobs` (2 action, idempoten) + `job:` dikualifikasi                                             |
| `docs/spec/backend/02-core-extended.md`    | tabel kanal: `queue` → terkirim + subbagian `job:`                                                              |
| `docs/guides/order-to-cash-tutorial.md`    | `job:` dikualifikasi                                                                                            |

## Temuan yang ikut diperbaiki (dinyatakan, bukan disembunyikan)

Tutorial Order-to-Cash mengajarkan `kind: Subscription` dengan `on:` + `deliver:`
— **bukan** bentuk yang spec punya (`SubscriptionSpec` = `events:` + `handler:`;
skemanya `additionalProperties: false`, jadi contoh itu gagal validasi). Karena
saya menyentuh baris itu, contohnya saya perbaiki ke bentuk nyata dan batasnya
ditulis eksplisit. `verticals/notifications` + dua integrator memakai bentuk yang
sama dan **belum** dimigrasikan (kelas yang sama dengan 7.7.6; tidak dikerjakan
di sini supaya perubahan ini tetap satu topik).

## Bukti

- `TestResolveJobRef` — dua bentuk sah; `""`, bare name, 4 segmen, segmen kosong
  ditolak.
- `TestValidateEventTargets_QueueJobResolves` / `_Missing` / `_DoesNotExist` /
  `_BareNameRejected` / `_ModuleQualifiedJob`.
- `TestDeliveryEventHandler_QueueChannelRunsTheJob` / `_WithoutJobFails` /
  `_WithoutDispatcherFails`.
- Test lama yang memakai `queue` sebagai contoh "tak terkirim" dialihkan ke
  `webhook` (masih tak terkirim) — perilaku barunya sudah dipin test tersendiri.
- **Terukur di tree nyata:** `verticals/billing` — **20 problem sebelum dan
  sesudah** (manifest 34 → 35, nol problem baru; 20 itu drift skema pra-ada di
  config/forms/mockups/menu), dan warning kanal **2 → 0** karena `job:` kini
  resolve.
- `go build ./...` · `go vet ./...` · `gofmt` bersih · `go test ./...` hijau ·
  kafe 88/0 · crc 33/0 · service-demo 13/0.
