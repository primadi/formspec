# 2026-08-17-003 — Fix 9 Clinic-UI-Showcase e2e failures

**Apa:** Memperbaiki 9 test gagal di `examples/Clinic-UI-Showcase` (sebelumnya
"pre-existing"). Akar masalahnya ternyata **bukan** satu bug, tapi empat bug
nyata yang saling menutupi — script-resolution failure menutupi tiga bug lain.

## Akar masalah & perbaikan

1. **Hook script tidak resolve** (7 test: prescription/otc-sale create hooks
   `derive-patient-name`, `stock-guard`).
   - `HandleCreate`/`HandleUpdate` tidak mengisi `SpecDir` di `ExecuteParams`,
     jadi hook script jatuh ke resolusi spec-root dan tidak menemukan script
     yang nested di direktori entity.
   - Fix: `HandlerFactory.SetSpecDirLookup` + `entitySpecDir()`; di-wire di
     `NewRouterBuilder` dari `registry.GetEntity(...).Source`; `HandleCreate`/
     `HandleUpdate` mengisi `execParams.SpecDir`.

2. **`start-consultation` gagal "unknown field: patient"** (2 test visit).
   - `GetByID` → `resolveRelations` menyuntik `Data["patient"]` (objek relasi
     ter-enrich) ke map record; `resource.save()` (dan PATCH merge) menulis
     balik seluruh map → `validateKnownFields` menolak `patient` sebagai field
     tak dikenal.
   - Fix: `EntityStore.stripEnrichedRelations()` — menghapus alias relasi
     (nested object) sebelum validasi di `Update` dan `Insert`. Alias relasi
     adalah convenience read-side; caller menulis `*_id` scalar, bukan objek.

3. **Guard state machine `!empty(items)`** (otc-sale sell).
   - `!` bukan operator Starlark (harus `not`). Guard evaluator memakai
     `internal/starlark.EvalExpr` yang punya `empty` predeclared, tapi sintaks
     `!empty(...)` invalid.
   - Fix: manifest otc-sale `!empty(items)` → `not empty(items)`.

4. **Prescription mereferensikan visit draft** (prescription lifecycle).
   - Visit lifecycle-active (`doc_status='draft'`) tapi tidak punya route
     submit (tidak ada `expose` submit), jadi prescription tak pernah bisa
     mereferensikannya (referenceability guard butuh submitted/lifecycle-free).
   - Fix: visit memakai state machine `status` sendiri (bukan document
     lifecycle) → `submit: disabled: true` (lifecycle-free, `doc_status` NULL).

5. **Test time-dependent** (5 test: otc-sale + prescription scenarios).
   - Test hardcode `transaction_date: 2026-07-12` (36 hari lalu > limit 3 hari
     backdate). Visit punya `override_permission`, tapi otc-sale/prescription
     tidak.
   - Fix: helper `recentDate()` (kemarin) di `clinic_e2e_test.go`; ganti semua
     hardcoded date di 3 file test.

## File terdampak

- `internal/api/handler.go` — `SetSpecDirLookup`, `entitySpecDir`, `SpecDir` di
  HandleCreate/HandleUpdate
- `internal/api/router.go` — wire `SetSpecDirLookup`
- `internal/action/hooks.go` — hook mewarisi `uses` action enclosing
- `renderers/jsonb-persist/crud.go` — `stripEnrichedRelations` di Update/Insert
- `examples/Clinic-UI-Showcase/spec/.../visit/entity.yaml` — `submit: disabled`
- `examples/Clinic-UI-Showcase/spec/.../otc-sale/entity.yaml` — `not empty(items)`
- `examples/Clinic-UI-Showcase/spec/.../prescription/entity.yaml` — `uses.resources: [clinic.patient]`
- `examples/Clinic-UI-Showcase/clinic_e2e_test.go` — `recentDate()`
- `examples/Clinic-UI-Showcase/pharmacy_otc_sale_e2e_test.go` — `recentDate()`
- `examples/Clinic-UI-Showcase/pharmacy_prescription_scenarios_e2e_test.go` — `recentDate()`

**Catatan:** hook kini mewarisi `uses` action enclosing (perbaikan #1 juga
membuka jalur hook yang sebelumnya tak pernah jalan; `derive-patient-name`
melakukan cross-module fetch `clinic.patient` → manifest prescription kini
mendeklarasikan `uses.resources: [clinic.patient]`).

**Verifikasi:** `go test ./...` → **571 passed, 0 failed** (sebelumnya 562 pass
/ 9 fail). `go build` + `go vet` hijau.

**Referensi:** todo 2.6.4, `docs/plan/uses-enforcement.md`.
