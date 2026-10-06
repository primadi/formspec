# 2026-10-02-007 — Validator UI membaca union action (`actions:` ∪ `via`)

**Apa yang diubah.** `actionExists` (`internal/ui/validate.go`) kini menelusuri
`EntitySpec.ActionSources()` — gabungan `actions:` yang dideklarasikan dan
setiap `via` transisi — bukan `EntitySpec.Actions` langsung. Pesan kedua
validator diperjelas menyebut union sebagai sumber yang sah.

**Kenapa.** Menjalankan `examples/kafe` mencetak **8 warning palsu** saat boot:

```
ui validate warning: Form "order-form-pos": action "start-preparing" not on entity "cafe-order.order"
ui validate warning: Table "order-table-pos": action "mark-ready" not on entity "cafe-order.order" and not a builtin (view|edit|delete|export|print)
```

Delapan nama yang dilaporkan (`start-preparing`, `mark-ready`, `mark-served`,
`complete-order`) **semuanya ada** — sebagai `via:` transisi di
`cafe-order.order`. Sejak L2/L3 `via` adalah action penuh, dan sejak L4 menulis
ulang nama itu di `actions:` justru **ditolak**
(`ValidateActionTransitionDuplication`), jadi via-only adalah bentuk yang
diminta. Bukti bentuk bug-nya jelas dari daftar itu sendiri: `confirm-payment`
dan `void-order` — satu-satunya dua nama di form/table yang sama yang **masih**
punya entri `actions:` (entri itu load-bearing, membawa `conditions`/`params`)
— tidak pernah diperingatkan. Selaras dengan temuan L3/L5 sebelumnya
(`registerRouteWithPattern`, `generatePrepareRoutes`, `buildEntitySchema`,
`authorizedActions`, `GenerateCustomActionRoutes`), ini pembaca union yang
terlewat.

**Dampak.** `internal/ui/validate.go` (2 situs pesan) · test baru
`internal/ui/validate_transition_action_test.go`.

**Bukti.** Probe yang memanggil jalur yang sama dengan `resource/formspec.go`
(`ui.LoadDir` + `ui.Validate` atas resolver manifest `examples/kafe/spec`):
sebelum → 8 masalah `not on entity`; sesudah → **0** masalah untuk manifest
kafe (sisa 5 adalah entity framework `role`/`user` yang memang tidak ada di
resolver spec-tree — prasangka yang sama juga muncul di server, di luar
cakupan ini). Test pengunci terkalibrasi: dengan regresi disuntikkan kembali ia
gagal dengan pesan yang persis sama seperti warning aslinya.

Rujukan: plan `docs_internal/plan/via-sebagai-action-penuh.md` (L8), kafe 10.59.
