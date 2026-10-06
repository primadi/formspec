# 2026-10-05-007 — Tiga vertical dimigrasikan ke bentuk Subscription yang sah + cek `events:` resolve

Dua hal, satu akar: **deklarasi yang tidak melakukan apa-apa tidak boleh terlihat bekerja.**

## 1. Tiga vertical memakai bentuk Subscription yang tidak ada

`verticals/notifications`, `verticals/sales-gl-integrator`, dan
`verticals/sales-inventory-integrator` menulis Subscription dengan bentuk lama:

```yaml
spec:
  on: { resource: billing.order, event: paid } # ← bukan field SubscriptionSpec
  deliver: # ← bukan field SubscriptionSpec
    - channel: queue
      job: create-sales-journal
```

`SubscriptionSpec` = `events:` + `handler:`; skemanya `additionalProperties:
false`, jadi ketiga manifest itu **gagal validasi** — terukur **1 problem
masing-masing**, dan sudah begitu sebelum sesi ini. Bentuk itu juga salah
konsep: `deliver:` adalah daftar konsekuensi milik **publisher**, bukan consumer.

Bentuk yang benar, dan kenapa:

```yaml
spec:
  events:
    - billing.order.paid # nama penuh {module}.{entity}.{event}
  handler:
    type: native
    ref: "SalesGlIntegrator.CreateSalesJournalHandler"
```

Handler Subscription **sudah berjalan di worker outbox** (at-least-once), jadi ia
sendiri adalah eksekusi latar — `queue` + `job` tidak perlu di sisi consumer.

**Terukur: ketiga vertical 1 → 0 problem.** Header ketiga file `impl` yang masih
menjelaskan bentuk lama ikut diperbarui.

## 2. Cek baru: `events:` Subscription harus resolve

Sebelumnya **tidak ada** yang memeriksa nama event di `events:`. Sama kelasnya
dengan target `deliver` dan `job:`: typo atau event yang di-rename menghasilkan
subscription yang **tidak pernah menyala**, manifest terlihat benar, validate
hijau.

`validateSubscriptionEvents` (`cmd/formspec/validate_subscriptions.go`) menolak
event yang tidak dikenal pada entity yang ada di tree. Dua hal menjaganya dari
crying wolf:

1. **Event reserved life-cycle bersifat IMPLIED** — `before_{action}` /
   `on_{action}` untuk delapan reserved action ada di setiap Entity tanpa
   dideklarasikan (Core §7), jadi listening ke `on_submit` tanpa deklarasi itu
   sah. Ditambahkan ke himpunan yang dikenal.
2. **Entity di luar tree → di-skip**, bukan ditolak (module yang dikirim
   terpisah tidak bisa diverifikasi; aturan yang sama dengan cek target delivery).

Pesan error menyebut nama yang dicari **dan** daftar event yang memang ada, jadi
typo bisa langsung dibetulkan.

**Blast radius diukur:** nol false positive di **sepuluh** tree
(kafe, crc, service-demo, arisan, cafe, billing, notifications, dua integrator,
reference-app) — 0 kegagalan `subscription:` di semuanya.

## Bukti

- `cmd/formspec/validate_subscriptions_test.go` — 5 test: resolve, event reserved
  implied, typo ditolak (pesan menyebut `paid` sebagai yang dimaksud), nama
  pendek ditolak, entity luar tree di-skip.
- Baseline terukur: tiga vertical **1 problem → 0**; sepuluh tree **0**
  kegagalan `subscription:`.
- `go build ./...` · `go vet ./...` · `gofmt` bersih · `go test ./...` hijau ·
  kafe 88/0 · crc 33/0 · service-demo 13/0 (tidak berubah).
