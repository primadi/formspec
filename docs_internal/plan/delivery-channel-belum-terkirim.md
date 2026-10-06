# Plan — 7.7.6: channel delivery yang belum diimplementasikan

Status: **semua kanal terkirim** (2026-10-06) — changelog `2026-10-05-005` (jujur),
`2026-10-05-006` (`queue`), `2026-10-06-003` (`notification` + `webhook` unsigned).
Sisa yang **tetap terbuka**: signature `webhook` (butuh secret per-subscriber) dan
blok `delivery:` Tier-2 pada `kind: Subscription` yang masih inert.

## Masalah

Tiga channel dideklarasikan dan diterima validator, tetapi **tidak dikirim**:

| Channel        | Di mana                            | Yang terjadi sekarang                                            |
| -------------- | ---------------------------------- | ---------------------------------------------------------------- |
| `queue`        | `events[].deliver[].channel`       | `default:` → log warning, **dianggap delivered**                 |
| `webhook`      | `subscriptions[].delivery.channel` | field-nya **tidak dibaca siapa pun** (`.Delivery` → 0 pemakaian) |
| `notification` | `subscriptions[].delivery.channel` | idem                                                             |

Akibatnya: sebuah manifest bisa mendeklarasikan konsekuensi yang tidak pernah
terjadi, `formspec validate` tetap hijau, dan outbox menandai entry-nya
**completed** — jadi kegagalannya tidak terlihat di data mana pun.

Catatan penting: `webhook` **sudah** terimplementasi di jalur lain — itu
`ServiceAction.callback.channel` (callback job async, `CallbackDecl`), yang
memakai URL dari header pemanggil. Yang belum adalah webhook **keluar** sebagai
konsekuensi event (butuh registry subscriber). Jangan tertukar.

## Increment 1 — jujur dan ditegakkan (selesai)

1. **Satu sumber kebenaran** di `pkg/spec`: daftar channel yang **tidak**
   dikirim + alasannya, dipakai validator, runtime, dan docs.
2. **Validator melaporkan, bukan diam.** `formspec validate` menampilkan
   `[WARN]` untuk setiap channel yang tidak akan terkirim, menyebut alasannya.
   Tidak menggagalkan (`queue` dipakai `verticals/*` yang belum dimigrasikan),
   tetapi tidak bisa lagi dikira bekerja.
3. **Runtime berhenti mengaku sukses.** Channel tak dikenal → **error**, bukan
   `nil`: outbox retry lalu **dead-letter** (`formspec_outbox.status='failed'`),
   sehingga kegagalannya terlihat di data. Jalur non-durable menaikkan log-nya
   ke **error** (tidak ada retry di sana).

## Sisa — butuh keputusan desain, bukan sekadar kode

- **`webhook` (keluar)** butuh **registry subscriber** (endpoint + secret per
  langganan). Belum ada bidang manifest untuk itu. **Jangan tertukar** dengan
  `callback.channel: webhook` (§13.1), yang mengirim hasil job async ke URL dari
  header pemanggil — itu sudah jalan.
- **`notification`** butuh module `formspec/notify`, yang **belum ada** —
  `NotificationCenter` pun mencari sumber yang tidak ada (kelas kegagalan yang
  sama dengan `ApprovalInbox` sebelum 5.13.6).

Keduanya tetap **dilaporkan** validator (`[WARN]`) dan **digagalkan** runtime
(outbox retry → dead-letter), jadi tidak bisa dikira bekerja.

## Selesai — `notification` + `webhook` (increment 3)

- **`notification`**: module `formspec/notify` ada (`internal/notify`), entity
  `formspec.core.notification` dengan `row_scope` penerima; framework menulis
  barisnya, `handler:` opsional untuk kanal luar. `NotificationCenter` kini
  mencari sumber yang benar-benar ada.
- **`webhook` (keluar)**: `internal/webhookout`; endpoint dari `url:` atau
  `url_from: {config:}`, **unsigned** untuk sekarang — signature butuh
  penyimpanan secret per-subscriber yang belum ada, dan batas itu dinyatakan di
  `docs/spec/backend/02-core-extended.md` §3.

`unsupportedChannels` di `pkg/spec/delivery_channels.go` kini **kosong**;
mekanismenya tetap dipakai untuk kanal berikutnya.

## Selesai — `queue` (increment 2)

`job:` menamai Service action (`service.action` / `module.service.action`),
dijalankan worker outbox. Detail + alasan: `queue-channel-job-service.md`.

## Tetap terbuka

- **Signature `webhook`** — butuh registry/secret per-subscriber.
- **Blok `delivery:` Tier-2 pada `kind: Subscription`** — masih **inert**; field-nya
  (termasuk `retry`/`dead_letter`) tidak dibaca runtime. Belum jadi item todo
  tersendiri karena belum ada keputusan desain apakah Tier-2 penuh akan
  diimplementasikan atau dipensiunkan.
