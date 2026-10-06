# Plan — 7.7.6 increment 2: channel `queue`

Status: **selesai** (2026-10-05) — changelog `2026-10-05-006`
Sisa: `webhook` (keluar) + `notification` — lihat `delivery-channel-belum-terkirim.md`

## Keputusan: `job:` menamai Service action

`deliver: {channel: queue, job: <ref>}` — `<ref>` adalah **`service.action`**
(module = module publisher) atau **`module.service.action`** (eksplisit).

Alasannya, bukan sekadar pilihan:

- Handler job adalah **komputasi tanpa state** — tepatnya definisi `kind:
Service` di AGENTS.md ("Service defines: inputs, outputs, dan handler, no
  state"). Ia sudah punya `impl` (native/script/script_ref/sidecar), penegakan
  permission/`uses`, dan dispatcher yang jalan.
- **Reuse, bukan registry baru.** `invokeServiceAction` + `service.Registry`
  sudah ada dan sudah dipakai jalur lain (`resource.call`, `Integrator`,
  `deliver: target`). Registry job handler yang parallel akan jadi mesin kedua
  untuk pekerjaan yang sama.
- **Konsisten dengan target lain.** `deliver[].target: {resource, action}` dan
  `Integrator.call` sudah menyebut resource+action; `job:` kini menyebut hal
  yang sama, hanya saja rumahnya Service (stateless) alih-alih Entity
  (stateful).

`job:` **wajib** pada entry `queue`: tanpa nama, worker tidak punya apa pun
untuk dipanggil, dan `formspec validate` menolaknya.

## Kenapa lewat outbox, bukan queue baru

`ValidateEventDurability` sudah **mewajibkan** `publish.durable: true` untuk
channel `queue` (`pkg/spec/entity.go`). Artinya event-nya toh sudah masuk outbox.
Outbox **adalah** antreannya: worker mem-poll, memanggil handler, dan me-retry
dengan backoff lalu dead-letter. Menambah tabel queue kedua berarti dua
mekanisme retry untuk satu jaminan — persis duplikasi yang plan ini hindari.

Konsekuensinya jujur dan dicatat: **tidak ada worker paralel**; pekerjaan
berjalan di worker outbox yang sama, jadi throughput-nya terikat poll interval
outbox (default ~1s). Untuk kebutuhan throughput tinggi, queue sungguhan
(Redis/Kafka) adalah pekerjaan tersendiri — bukan yang diklaim selesai di sini.

## Perubahan

| File                                       | Isi                                                                |
| ------------------------------------------ | ------------------------------------------------------------------ |
| `pkg/spec/entity.go`                       | doc `EventDeliveryDecl.Job` (kontrak ref)                          |
| `pkg/spec/delivery_channels.go`            | keluarkan `queue` dari daftar tak terkirim; tambah `ResolveJobRef` |
| `internal/action/deliver.go`               | `case "queue"`: durable → enqueue outbox (pola `reliable_event`)   |
| `renderers/jsonb-persist/event_handler.go` | field `Jobs JobDispatch` + `case "queue"`                          |
| `resource/formspec.go`                     | `newJobDispatch` + wire (boot + reload)                            |
| `cmd/formspec/validate_events.go`          | `job:` wajib, resolve ke Service action, tolak yang tak ada        |
| `verticals/billing`                        | Service `receipt-jobs` (2 action) + kualifikasi `job:`             |
| `docs/`                                    | channel table §3, definisi `job:`, tutorial                        |

## Verifikasi

- Test: `ResolveJobRef` (bentuk ref + error), validator (`job:` kosong / tak
  resolve / resolve), event handler (`queue` memanggil `Jobs`), dan satu test
  end-to-end lewat outbox worker bila murah.
- `go test ./...` · `formspec validate` kafe/crc/service-demo · `verticals/billing`.
