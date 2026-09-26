# `core.idempotency_retention` dibaca dari manifest (todo 2.1.6)

## Apa yang diubah

Key `core.idempotency_retention` — yang **sudah** dinormatifkan
`01-core-basic.md` §5 ("Entry kedaluwarsa lewat retention (default 24 jam,
**dibaca dari `core.idempotency_retention`**)") dan sudah disalin ke
`docs/renderers/jsonb-persist/04-query-and-keys.md` — kini benar-benar dibaca.
Sebelumnya `grep idempotency_retention` di seluruh Go hanya menemukan
**komentar**, jadi TTL hanya bisa diatur dari Go (`Config.IdempotencyTTL`) dan
manifest yang tampak mengonfigurasi retention tidak melakukan apa pun.

Rantainya: `resolveIdempotencyTTL(cfgReg, fallback)` (`resource/formspec.go`)
di-wire di **dua** titik — boot (`New`) dan `ReloadSpec` — sehingga mengubah
nilainya berlaku pada hot-reload, bukan hanya saat restart. Presedensi:
`core.idempotency_retention` → `Config.IdempotencyTTL` → `db.DefaultIdempotencyTTL`.

**Dua keputusan perilaku yang eksplisit, bukan default implisit:**

1. **Nilai tak terbaca dilaporkan, lalu default dipakai.** `"banana"` tidak
   diam-diam berarti "tanpa kedaluwarsa" maupun "24 jam tanpa pemberitahuan" —
   ia mencetak warning ke stderr. Mode kegagalan yang berbahaya di sini adalah
   salah ketik yang **mematikan** retention tanpa jejak; itu yang dicegah.
2. **`0` berarti tanpa kedaluwarsa dan dihormati.** Ini menyelaraskan diri
   dengan `BackdatePolicy.MaxDaysBack = 0` yang sudah berarti "tanpa batas".
   Aturan yang sama juga menolak bare integer (`"7"`) — pada parser retention,
   `7` adalah **count**, bukan durasi, dan menerimanya akan menghasilkan TTL
   ~7 nanodetik, yang jauh lebih buruk daripada menolak.

Lookup memakai `Registry.ResolveKeyAny` (baru, `internal/config/registry.go`):
key framework hidup di namespace `formspec.core` tanpa nama Config yang stabil
(manifest kafe menamainya `app-settings`), jadi mencari per-nama akan mengikat
framework ke penamaan author. Iterasi deterministik (urut nama manifest).

Konsumsi nyata: kafe mendeklarasikan `keys.idempotency_retention: "24h"` di
`examples/kafe/spec/config/app.yaml`, sehingga bentuknya terverifikasi lewat
`formspec validate` pada spec sungguhan — bukan hanya di test.

## Kenapa

Menutup item 2.1.6 (audit `docs_internal/plan/audit-open-items-prosa.md`
kategori A #12). Item itu mencatat bahwa prasyaratnya — runtime `kind: Config`
(7.2) — sudah landing 2026-08-25, sehingga satu-satunya yang tersisa adalah
pemetaannya. Sejak itu tidak ada pembaca sama sekali, jadi spec mendokumentasikan
sebuah setting yang tidak berfungsi.

## File terdampak

- `internal/config/registry.go` — `ResolveKeyAny` (key by bare name, deterministik)
- `resource/formspec.go` — `resolveIdempotencyTTL` + wiring di `New` dan `ReloadSpec`
- `renderers/jsonb-persist/idempotency.go` — komentar `DefaultIdempotencyTTL` (bukan lagi "SHOULD", kini "dibaca")
- `examples/kafe/spec/config/app.yaml` — deklarasi `keys.idempotency_retention: "24h"`
- `docs/renderers/jsonb-persist/04-query-and-keys.md` — §3 tidak lagi menyebut "menunggu runtime Config-kind"
- `internal/config/registry_resolveany_test.go` — **baru**
- `resource/idempotency_ttl_test.go` — **baru** (7 sub-test)

## Bukti

- `TestResolveIdempotencyTTL` **dibuktikan gagal** saat pemetaan dinetralkan
  (3 sub-test: `7d: got 24h0m0s, want 168h`, `30m: got 24h0m0s`,
  `0: got 24h0m0s, want 0`), hijau sesudahnya.
- `TestRegistry_ResolveKeyAny` — key lintas-manifest, tidak ada, manifest kosong.
- `go build ./...` bersih; `go test ./internal/config/ ./resource/` hijau;
  `gofmt -l` bersih.
- `formspec validate --spec examples/kafe/spec --schema schemas` → **85 manifest,
  0 problem**; `formspec check -f examples/kafe/spec` → **0 error / 0 warning**.

## Rujukan

Todo **2.1.6** (tertutup) · **2.1.4** (teks "menunggu runtime Config-kind" kini
basi — dikoreksi di baris itemnya).
