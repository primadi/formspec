# 6.1 — Keterkaitan transisi ↔ event eksplisit: `emit:` pada transisi (S13)

**Tanggal:** 2026-09-20 · **TODO:** `examples/kafe/gaps_found/TODO.md` 6.1

## Apa yang diubah

`TransitionDecl` mendapat field `Emit`, sehingga transisi state machine dapat
memancarkan event secara **eksplisit** — bukan disimpulkan dari penamaan.

- `pkg/spec/entity.go` — `TransitionDecl.Emit` + `ValidateTransitionEmits`
  (dipanggil dari `ValidateEntitySpec`).
- `internal/action/events.go` — `ResolveTransitionEmission(sm, events, oldState,
  newState, data)`.
- `internal/api/handler.go` — `stateFieldValue` helper; pemancaran di
  `HandleUpdate` (durable → outbox atomik, lalu best-effort delivery).
- `docs/spec/backend/01-core-basic.md` §7 — normatif.
- `examples/kafe/spec/.../order/entity.yaml` — `emit: on_paid` pada
  `confirm-payment`, `emit: on_cancel` pada `cancel-order`/`void-order`.
- `schemas/` — ter-regenerasi.

## Kenapa

Sebelum ini keterkaitan transisi↔event hanya **tersirat dari penamaan** (dugaan:
nama event = nama state), sehingga tidak ada yang bisa memverifikasi bahwa
`order.paid` benar-benar terpancar saat transisi ke `paid` — termasuk
`formspec validate`. Integrasi stok & jurnal bergantung penuh pada ini.

Dengan `emit:`, keterkaitannya dinyatakan dan diverifikasi: validator menolak
`emit` yang menunjuk event yang tidak dideklarasikan, dan runtime memancarkan
event **karena transisi mengatakannya**, bukan karena kebetulan nama cocok.

## Bukti

| Perintah | Hasil |
| --- | --- |
| `go test ./internal/action/ -run TestResolveTransitionEmission` | 5 sub-test PASS |
| `go test ./pkg/spec/ -run TestValidateEntitySpec_TransitionEmits` | PASS |
| `formspec validate` kafe | **0 problem** (69 manifest) |
| `go test ./...` | hijau |

## Catatan

Konvensi penamaan event (`on_*` = async, `before_*` = sync) tetap berlaku dan
kini **terpisah** dari keterkaitan transisi↔event — keduanya terverifikasi
sendiri-sendiri. D3 (transisi tidak memancarkan event *otomatis*) tetap benar:
pemancaran terjadi **karena** `emit:` dinyatakan, bukan otomatis.
