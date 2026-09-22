# Fase 6 kafe — akuntansi & integrasi lintas-app (6.1–6.5)

**Tanggal:** 2026-09-20 · **TODO:** `examples/kafe/gaps_found/TODO.md` Fase 6

## Apa yang diubah

| Item | Hasil |
| --- | --- |
| **6.1** transisi↔event eksplisit | ✅ `TransitionDecl.Emit` + validasi + pemancaran runtime |
| **6.2** pemetaan payload Integrator | ✅ `IntegratorCall.Map` + interpolasi template |
| **6.3** cross-app grant + SyncAgent | ⏸️ DEFERRED — butuh Control Plane (cloud phase) |
| **6.4** kepemilikan `publishes` | ⏸️ DEFERRED — butuh keputusan desain pemilik proyek |
| **6.5** vertical `purchase` | ✅ keputusan tertulis: tetap model sendiri (opsi b) |

## Perubahan kode

- `pkg/spec/entity.go` — `TransitionDecl.Emit` + `ValidateTransitionEmits`.
- `internal/action/events.go` — `ResolveTransitionEmission`.
- `internal/api/handler.go` — `stateFieldValue`; pemancaran di `HandleUpdate`.
- `pkg/spec/resources.go` — `IntegratorCall.Map`.
- `internal/integrator/dispatch.go` — `applyCallMap` + interpolasi rekursif.
- `docs/spec/backend/01-core-basic.md` §7 + `02-core-extended.md` §5 — normatif.
- `examples/kafe/spec/.../order/entity.yaml` — `emit:` pada transisi.
- `examples/kafe/spec/.../order-paid-to-journal.yaml` — `map:` pesanan → jurnal.
- `schemas/` — ter-regenerasi.

## Kenapa

- **6.1** — keterkaitan transisi↔event hanya tersirat dari penamaan, sehingga
  tidak ada yang bisa memverifikasi `order.paid` benar-benar terpancar. `emit:`
  membuatnya eksplisit dan terverifikasi.
- **6.2** — pemetaan "omzet → kredit 4-1000" adalah pengetahuan akuntansi, bukan
  penamaan field; tanpa `map:` ia harus hidup di script `gl` (pihak ketiga) atau
  action kustom sisi kafe — yang kedua membuat `kind: Integrator` kehilangan
  gunanya.
- **6.3/6.4** — keduanya butuh keputusan arsitektur (Control Plane / desain
  bahasa spec), bukan tebakan. Dicatat deferred dengan alasan.
- **6.5** — kafe sudah memodelkan purchase lengkap (`supplier`, `purchase-order`,
  `receive-goods`); vertical reusable adalah keputusan produk yang lebih besar.

## Bukti

| Perintah | Hasil |
| --- | --- |
| `go test ./internal/action/ -run TestResolveTransitionEmission` | 5 sub-test PASS |
| `go test ./pkg/spec/ -run TestValidateEntitySpec_TransitionEmits` | PASS |
| `go test ./internal/integrator/ -run TestApplyCallMap` | 2 PASS |
| `formspec validate` kafe | **0 problem** (69 manifest) |
| `go test ./...` | hijau |
| `make lint` | 0 issues |

## Sisa (dicatat)

- **6.3** — cross-app grant & SyncAgent: deferred ke Control Plane (cloud phase).
- **6.4** — kepemilikan `publishes`: butuh keputusan desain (module vs App).
- **6.5** — integrasi purchase → stock & purchase → jurnal belum dinyatakan di
  spec kafe (jalurnya ada: `emit:` + `Integrator` + `map:`); landed cost belum
  dimodelkan.
