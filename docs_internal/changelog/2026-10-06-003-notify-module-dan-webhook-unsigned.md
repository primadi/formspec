# 2026-10-06-003 — Module `formspec/notify` dan channel `webhook` (unsigned)

Menutup sisa dua kanal pada item todo **7.7.6** (`notification`, `webhook`
keluar). Sebelumnya keduanya menerima deklarasi lalu **tidak melakukan apa-apa**:
`formspec validate` hijau, outbox menandai entry `completed`, tidak ada data yang
menunjukkan konsekuensinya tidak terjadi.

## `notification` — module resmi `formspec/notify`

Module baru dengan pola yang sama seperti `internal/auth` / `internal/period`:
`internal/notify/module/entities/notification.yaml` + `//go:embed module`, di-register
lewat `RegisterCoreEntities` (boot **dan** `ReloadSpec`). Entity
`formspec.core.notification` membawa `recipient_id` (wajib, indexed), `title`
(wajib), `body`, `read` (+`read_at`), `source_event`/`source_resource`/`source_id`,
`level`, dan — yang terpenting — `row_scope: {field: recipient_id, op: eq, from:
session, attr: user_id}`. Jadi "hanya pemiliknya yang melihat" ditegakkan server;
pemanggil tanpa identitas **fail-closed**.

`pkg/spec/entity.go` mendapat `EventDeliveryDecl.Notification *NotificationDecl`
(`recipient`/`title`/`body`/`level`) dan `internal/notify/dispatch.go` menulis
barisnya via `EntityStoreWriter` dengan `SystemCaller: true`. `recipient` **wajib**
dan harus resolve: baris tanpa penerima adalah baris yang tidak bisa dibaca siapa
pun, jadi validator dan runtime sama-sama menolaknya. `handler:` opsional menamai
Service action untuk kanal luar, memakai kosa kata referensi yang sama dengan `job:`.

## `webhook` — POST keluar, UNSIGNED

`internal/webhookout` + `WebhookDeliveryDecl{URL, URLFrom, Headers}`. Tepat satu
dari `url:` / `url_from: {config:}` harus ada; config key di-resolve saat delivery.
Non-2xx = error (outbox retry → dead-letter), bukan sukses. Sementara **tanpa HMAC
signature dan tanpa registry subscriber** — endpoint disimpan di manifest/config,
bukan per-langganan; menandatangani butuh penyimpanan secret per-subscriber dan
belum ada. Batas ini dinyatakan eksplisit di `docs/spec/backend/02-core-extended.md`
§3.

## Konsekuensi kontrak

- `pkg/spec/delivery_channels.go` → `unsupportedChannels` **kosong**. Mekanisme
  pelaporan "dideklarasikan tetapi tidak dikirim" tetap ada (dipakai validator
  dan runtime), siap untuk kanal berikutnya.
- `spec.EventChannel` bertambah `notification` dan `webhook` (tujuh kanal).
- `renderers/jsonb-persist/event_handler.go` mendapat cabang `notification` dan
  `webhook`; `default:` mengembalikan **error** yang menyebut nama kanalnya —
  tidak ada lagi sukses palsu.
- `internal/genjsonschema` `sharedTypes` bertambah `NotificationDecl`,
  `WebhookDeliveryDecl`, `WebhookURLRef` (kalau lupa: `TestGeneratedKindSchemas_HaveNoDanglingRefs`).

## Bug yang ditemukan test suite

`interpolate` (notify) dan `interpolateHeaders` (webhookout) menulis ulang token
yang tidak bisa di-resolve dengan `out = out[:end+1] + out[end+1:]` — **no-op**,
sehingga `{` yang sama ditemukan selamanya. Gejalanya: `go test ./internal/notify/`
menggantung >180 detik. Diperbaiki dengan akumulasi berbasis kursor
(`strings.Builder` + `rest = rest[end+1:]`); regresi dijaga
`TestInterpolateHeaders_UnresolvedTokenTerminates`.

## Bukti

- `go test ./...` → **41 ok, 0 FAIL**; `go vet` bersih; `gofmt -l` bersih.
- Test baru: `internal/notify` 10, `internal/webhookout` 8.
- `formspec validate` per pohon: kafe 88 manifests/0 problem, crc 33/0,
  service-demo 13/0, arisan 17/0, verticals/notifications 3/0;
  `verticals/billing` 35/20 — **20 problem itu pre-existing** (config/forms/mockups/menu),
  bukan dari perubahan ini.
- Tiga asersi test lama diperbarui karena perilaku kanal memang berubah
  (`_InertSubscriptionDelivery`, `_RequiresAnEndpoint`, dan
  `_UnimplementedChannel_*` → `_UnwiredChannel_*` — webhook kini terimplementasi,
  yang tersisa adalah soal wiring).

Referensi: `docs_internal/plan/formspec-notify-module.md`,
`docs_internal/plan/delivery-channel-belum-terkirim.md`. Dokumen kontrak yang
diperbarui: `docs/spec/backend/02-core-extended.md` §3,
`docs/kind/ui/NotificationCenter.md`.
