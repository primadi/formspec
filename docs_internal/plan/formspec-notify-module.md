# Plan — module `formspec/notify` + channel `notification` (7.7.6 bagian a)

Status: **selesai** (2026-10-06) — changelog `2026-10-06-003`
Bagian b (`webhook` keluar, unsigned) selesai di changelog yang sama.

## Tujuan

`deliver: {channel: notification}` selama ini **tidak dikirim** — bridge-nya
menunjuk module `formspec/notify` yang belum ada. Akibatnya bukan hanya channel
itu mati: `kind: NotificationCenter` juga zero-config dan mencari entity
`formspec.core.notification` yang sama, jadi halaman itu selalu kosong. **Satu
module menutup keduanya.**

## Keputusan desain

1. **Module resmi, pola yang sudah ada.** `internal/notify/module/` dengan
   `//go:embed module` + `RegisterCoreEntities(reg)` — persis pola
   `internal/period` dan `internal/subscription`. Tidak ada mekanisme baru.

2. **Notifikasi adalah Entity di `formspec.core`.** Bukan tabel framework
   tersembunyi. Konsekuensinya nyata dan diinginkan: ia mendapat `doc_status`,
   permission model, audit, DDL/migrasi otomatis, dan **`row_scope`** — yang
   membuat "hanya pemiliknya yang melihat" ditegakkan server, bukan disaring UI.
   Ditandai `formspec.dev/ui-exposed: "true"` supaya muncul di admin/UI.

3. **Penerima = `row_scope: {field: recipient_id, op: eq, from: session, attr:
user_id}`.** `sessionAttr` sudah memetakan `user_id` → `Identity.UserID`
   (`internal/api/scope.go`), jadi tidak perlu atribut token tambahan. Pemanggil
   tanpa identitas gagal **fail-closed** — perilaku yang sudah ada.

4. **Framework yang membuat barisnya; handler Service bersifat opsional.**
   Inilah yang membuat channel ini berguna tanpa kode apa pun: event → baris
   notifikasi untuk penerima → NotificationCenter menampilkannya. Bila aplikasi
   juga ingin email/WA/push, ia menyebut **Service action** sebagai handler —
   **bentuk yang sama** dengan `job:`, satu kosa kata, bukan dua.

5. **Penerima dan isi dinyatakan di manifest**, karena payload tidak tahu siapa
   yang harus diberi tahu:

   ```yaml
   deliver:
     - channel: notification
       notification:
         recipient: "customer_id" # lintasan payload → recipient_id
         title: "Pesanan {number} dibayar" # template `{path}` atas payload
         body: "Total {total}"
         # opsional: handler Service untuk kanal di luar in-app
       handler: "notify-jobs.send-email"
   ```

   `recipient` wajib: tanpa itu tidak ada yang bisa dituju, dan baris notifikasi
   yang tidak punya penerima adalah baris yang tak pernah bisa dibaca siapa pun
   (fail-closed row_scope) — jadi ditolak validator, bukan dibuat lalu diam.

## Perubahan

| File                                                | Isi                                                                              |
| --------------------------------------------------- | -------------------------------------------------------------------------------- |
| `internal/notify/module/module.yaml`                | deklarasi module resmi                                                           |
| `internal/notify/module/entities/notification.yaml` | Entity `formspec.core.notification` + `row_scope`                                |
| `internal/notify/module.go`                         | `ModuleFS()` + `RegisterCoreEntities`                                            |
| `internal/notify/dispatch.go`                       | `Dispatcher`: resolve config, buat baris, panggil handler opsional               |
| `pkg/spec/entity.go`                                | `EventDeliveryDecl.Notification *NotificationDecl` + `Handler` untuk channel ini |
| `pkg/spec/delivery_channels.go`                     | `notification` keluar dari daftar tak terkirim; `ResolveNotificationRef`         |
| `renderers/jsonb-persist/event_handler.go`          | field `Notifications` + `case "notification"`                                    |
| `resource/formspec.go`                              | wiring (boot + reload) + `RegisterCoreEntities`                                  |
| `cmd/formspec/validate_events.go`                   | `recipient` wajib; handler (bila ada) harus resolve                              |
| `docs/`                                             | channel table, kind NotificationCenter, definisi field                           |

## Verifikasi

- Test: `notification` membuat baris dengan penerima benar; tanpa `recipient`
  ditolak validator; handler opsional dipanggil; `row_scope` isolasi (user lain
  tidak melihat) — di level store, bukan HTTP, agar cepat dan tepat.
- `go test ./...` · kafe/crc/service-demo tidak berubah.
