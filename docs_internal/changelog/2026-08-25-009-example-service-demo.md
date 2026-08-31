# Contoh Project: service-demo (Service runtime + validation rules)

**Tanggal:** 2026-08-25 · **Todo:** §7.1, §7.9.6

## Apa yang ditambahkan

Contoh project `examples/service-demo` untuk menguji end-to-end fitur backend
baru:

- **Service runtime (7.1)** — `kind: Service` `tax-calculator` dengan dua
  action: `calculate` (sync, script) dan `notify` (`call: async` → 202).
  Script di-resolve module-scoped (`spec/modules/demo/scripts/*.star`) via ref
  slash notation (`demo/calculate_tax`).
- **Validation rules (7.9.6)** — Entity `product` dengan rule `length`,
  `unique`, `in`, `script`.

## Verifikasi end-to-end (via `formspec dev` + curl)

- `POST /api/v1/demo/tax-calculator/calculate` `{"amount":100,"rate":0.1}`
  → `{"tax":10,"total":110}` ✅
- `POST /api/v1/demo/tax-calculator/notify` → `202 Accepted` ✅
- Create product valid (`KOPI0001`) → sukses ✅
- `sku` 7 char → `VALIDATION_ERROR` "length must be exactly 8" ✅
- `status: banned` → `VALIDATION_ERROR` "value must be one of ..." ✅
- `min_stock: -1` → `VALIDATION_ERROR` "script rule failed" ✅
- Duplikat `sku` → `VALIDATION_ERROR` "value must be unique per tenant" ✅

## Fix yang ditemukan selama pengujian

- **Urutan `SetServiceRegistry` vs `BuildRoutes`** — route service tidak
  ter-generate karena `BuildRoutes` dipanggil sebelum `SetServiceRegistry`.
  Diperbaiki di `resource/formspec.go` (boot + reload): set service registry
  SEBELUM `BuildRoutes`.
- **Ref script service** — `resolveScript` memisahkan ref pada `/` (bukan `.`),
  jadi ref module-scoped harus `demo/calculate_tax` (slash), bukan
  `demo.calculate_tax` (dot).

## File terdampak

- `examples/service-demo/` (baru) — App, Module, Service, scripts, Entity, README
- `resource/formspec.go` — urutan SetServiceRegistry sebelum BuildRoutes

## Status

`go test ./...` hijau. Contoh project tervalidasi (`formspec validate` 0
problem) dan teruji end-to-end.
