# 6.2 — Pemetaan payload pada Integrator: `call.map` (S6)

**Tanggal:** 2026-09-20 · **TODO:** `examples/kafe/gaps_found/TODO.md` 6.2

## Apa yang diubah

`IntegratorCall` mendapat `Map`, sehingga pemetaan payload dari event sumber ke
params action target dinyatakan **di manifest**.

- `pkg/spec/resources.go` — `IntegratorCall.Map`.
- `internal/integrator/dispatch.go` — `applyCallMap` + interpolasi rekursif
  (`interpolateValue`/`interpolateString`/`resolvePath`); diterapkan di
  `dispatchOne`.
- `docs/spec/backend/02-core-extended.md` §5 — normatif.
- `examples/kafe/spec/.../order-paid-to-journal.yaml` — `map:` pesanan → jurnal.
- `schemas/` — ter-regenerasi.

## Kenapa

`call` hanya menyebut resource + action, sehingga pemetaan "omzet → kredit
4-1000, pajak → kredit 2-2000, kas → debit 1-1000" harus hidup di salah satu
sisi: script milik `gl` (vertical pihak ketiga) atau action kustom sisi kafe —
dan yang kedua membuat `kind: Integrator` kehilangan gunanya. Pemetaan itu
adalah **pengetahuan akuntansi**, bukan penamaan field, jadi tempatnya di
manifest.

## Semantik

- Nilai adalah template: string berisi `{dotted.path}` diinterpolasi terhadap
  payload event; map dan list diinterpolasi rekursif.
- Nilai yang **persis satu token** mempertahankan tipe aslinya — `debit:
  "{total_amount}"` menghasilkan objek `money`, bukan teksnya. Untuk field
  target bertipe skalar, ambil komponennya (`{total_amount.amount}`).
- Token yang tak ter-resolve **dibiarkan verbatim** (terlihat di payload),
  bukan menjadi string kosong yang diam.
- Tanpa `map`, payload diteruskan apa adanya (perilaku lama).

## Bukti

| Perintah | Hasil |
| --- | --- |
| `go test ./internal/integrator/ -run TestApplyCallMap` | 2 PASS |
| `formspec validate` kafe | **0 problem** (69 manifest) |
| `go test ./...` | hijau |
