# 2026-10-05-005 — Kanal delivery yang belum terkirim: dijujurkan, bukan disembunyikan (7.7.6)

Empat kanal dideklarasikan dan diterima validator, tetapi tidak dikirim:

| Kanak                   | Tempat                       | Keadaan sebelumnya                                                      |
| ----------------------- | ---------------------------- | ----------------------------------------------------------------------- |
| `queue`                 | `events[].deliver[].channel` | `default:` → log warning, **dianggap delivered**                        |
| `webhook` (keluar)      | sama                         | idem (catatan: `callback.channel: webhook` **berbeda** dan sudah jalan) |
| `notification`          | sama                         | idem                                                                    |
| blok `delivery:` Tier-2 | `kind: Subscription`         | field-nya **tidak dibaca siapa pun** (`.Delivery` → 0 pemakaian)        |

Keadaan "diterima tapi tidak dikirim" adalah yang terburuk dari tiga: manifest
terlihat terkonfigurasi, `formspec validate` hijau, dan outbox menandai entry
`completed` — sehingga tidak ada satu pun data yang menunjukkan konsekuensinya
tidak pernah terjadi.

## Perbaikan (increment 1: jujur + ditegakkan)

1. **Satu sumber kebenaran** — `pkg/spec/delivery_channels.go`:
   `ChannelUnsupported(channel) (reason, bool)` + `UnsupportedChannelNames()` +
   `SubscriptionDeliveryIsInert()`. Dipakai validator, runtime, dan docs, jadi
   ketiganya tidak bisa berbeda pendapat tentang kanal mana yang bekerja.
2. **Validator melaporkan** — `cmd/formspec/validate_delivery_channels.go`
   mengeluarkan `[WARN]` untuk tiap kanal yang tidak akan terkirim (menyebut
   alasannya) dan untuk blok `delivery:` Subscription yang inert. Severity
   **warning**, tidak menggagalkan: `queue` masih dipakai `verticals/*` dan
   tutorial, yang belum dimigrasikan — tetapi hijau tidak bisa lagi disalahartikan
   sebagai "kanal ini bekerja".
3. **Runtime berhenti mengaku sukses**:
   - `DeliveryEventHandler` (jalur durable/outbox): kanal tak dikenal → **error**,
     bukan `nil`. Outbox me-retry lalu **dead-letter**
     (`formspec_outbox.status='failed'`) — kegagalannya terlihat dan bisa
     di-query, bukan hilang.
   - `internal/action/deliver.go` (jalur non-durable, tanpa retry): log naik dari
     `Warn` ke **`Error`** dengan kode `event.channel_not_delivered` + alasan.

Catatan koreksi yang penting: `webhook` di daftar "belum" adalah webhook
**keluar** sebagai konsekuensi event. `ServiceAction.callback.channel: webhook`
(`CallbackDecl`, §13.1 — hasil job async ke URL dari header pemanggil) **sudah**
terimplementasi dan tetap bekerja; jangan tertukar.

## Yang belum — butuh keputusan desain, bukan tambalan

Ketiganya masuk **7.7.6 ⏸️** dengan alasan konkret:

- **`queue`** butuh **registry job handler**: `job: generate-receipt` hari ini
  menunjuk apa pun (tidak ada bidang manifest untuk mendeklarasikan handler;
  `verticals/billing/impl/**` hanya stub `// TODO`). Keputusan: apakah `job:`
  menamai action Service, script ref, atau handler Go yang diregistrasi modul?
- **`webhook` keluar** butuh registry subscriber (endpoint + secret) — belum ada
  bidang manifestnya.
- **`notification`** butuh module `formspec/notify` yang **belum ada** —
  `NotificationCenter` pun mencari sumber yang tidak ada (kelas yang sama dengan
  `ApprovalInbox` sebelum 5.13.6).

## Temuan sampingan (bukan dikerjakan di sini)

`verticals/notifications/spec/modules/notifications/subscriptions/wa-on-order-paid.yaml`
memakai `on:` + `deliver:` pada `kind: Subscription`, padahal `SubscriptionSpec`
tidak punya kedua field itu (skemanya `additionalProperties: false` → manifest itu
memang **sudah gagal** validasi, terukur: `missing property 'handler';
additional properties 'deliver', 'on' not allowed`). Vertical itu belum
dimigrasikan ke bentuk `events:` + `handler:` + `delivery:`. Bukan akibat
perubahan ini; dicatat supaya tidak salah diatribusikan.

## Bukti

- `pkg/spec/delivery_channels_test.go` — kanal yang terkirim tidak dilaporkan;
  yang belum dilaporkan **dengan alasan**; `SubscriptionDeliveryIsInert`.
- `cmd/formspec/validate_delivery_channels_test.go` — Entity `queue` → 1 warning;
  kanal terkirim → senyap; Subscription `delivery:` → warning ganda (inert +
  kanal).
- `renderers/jsonb-persist/event_handler_test.go` —
  `_UnimplementedChannel_FailsNotSilentlyDelivered` (dulu memakukan perilaku
  sebaliknya: _"expected nil … treated as delivered"_) dan `_UnknownChannel_FailsToo`.
- `go build ./...` · `go vet ./...` · `gofmt` bersih · `go test ./...` hijau ·
  `formspec validate`: kafe 88/0 · crc 33/0 · service-demo 13/0 (tidak berubah) ·
  `verticals/billing` kini melaporkan 2 kanal `queue` yang tidak terkirim.
