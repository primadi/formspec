# 2026-09-15-010 — S5/S11/S12/S14: scope, percent, unit, kontrak summary (+ D1–D7 normatif)

Item `examples/kafe/gaps_found/TODO.md` **1.8** dan **1.9**. Plan:
`docs_internal/plan/s5-s11-s12-s14-scope-percent-unit-summary.md`.

**S5 — `scope` / `row_scope` / `assignments`.** Tiga konstruk terpisah, karena
tiga pertanyaan berbeda: `scope: {dimension, field, required}` menyatakan entity
**dipartisi** (fakta data, tidak memfilter); `row_scope` (nama baru untuk filter
yang ditegakkan, sebelumnya bernama `scope` di Go maupun YAML) adalah otorisasi
server-side; `assignments: [{dimension, field, principal_field}]` menyatakan
**dari mana** nilai dimensi seorang principal berasal. Perubahan nama dilakukan
karena tidak ada satu pun spec di repo yang memakai bentuk filter 1.1 (kafe
menundanya secara sadar), dan satu nama tidak bisa menjadi dua bentuk — preseden
1.7 (menolak union string-atau-objek). Nilai `from: session` kini diselesaikan
dari `assignments` (`internal/api/scope.go` → `internal/entity.FindAssignmentValue`,
memo 30 detik) ketika token tidak membawa atributnya, sehingga `row_scope`
cukup ditulis tanpa `attr` pada entity yang punya `scope`.

**Gerbang validator — inti kejujuran 1.8.** `formspec validate` (layer
cross-manifest baru, `cmd/formspec/validate_scope.go`) **menolak** `row_scope`
`from: session` tanpa `attr` yang atributnya tidak punya sumber. Tanpa gerbang
ini bentuk tersebut 403 untuk semua orang selamanya sementara manifest terlihat
benar — kelas kegagalan yang sama dengan #52/#53/GAP-33.

**S11 — `percent`.** Tipe field baru: secara numerik `decimal`, bedanya
interpretasi/rendering (`10` = 10%). Dipetakan di `fieldTypeToSQL`, agregasi,
`castTypeForField`, `formspec check`, generator TS; klien: `decimalinput`,
zod number, `format: percent` (menampilkan `%`).

**S12 — `unit: {base, convertible}`.** Deklarasi dimensi satuan pada field yang
nilainya adalah nama satuan; `base`/`convertible` wajib ada di `enum_values`
(kalau enum). Satuan di luar grup (mis. `pcs` vs `gram`) berarti dimensi lain —
sengaja tidak dikonversi, bukan error. Konversi menyusul (4.6).

**S14 — `maintained_by` + `invariants`.** `maintained_by` wajib menunjuk script
yang ada dan bisa dikompilasi (dicek honesty scan bersama `impl.ref`);
`invariants[].unique` wajib ditopang unique index yang benar-benar
dideklarasikan — jadi invarian summary ditegakkan **database**, bukan disiplin
script, dan memasang hook yang tak pernah jalan tidak lagi terlihat sebagai
perlindungan. Hanya valid pada `characteristic: summary`.

**D1–D7 normatif** (`docs/spec/backend/01-core-basic.md` §1.2, §1.7, §7, §8.6,
§11): `lifecycle:` = hint UI (penentu adalah aksi mana yang aktif); transisi
**tidak** memancarkan event otomatis; nama state polos bukan konvensi; event
`before_*`/`on_*`; permission `{module}.{plural}.{action}` dengan `submit`
ber-permission sendiri; `settings` di Config level App, dibaca framework
(mata uang `money`).

**Adopsi kafe.** 15 entity menyatakan `scope` (`order`, `shift`, `payment`,
`table-session`, `cash-movement`, `menu-item-price`, `dining-table`,
`stock-movement`, `purchase-order`, `stock-opname`, `waste-entry`,
`stock-level`, `menu-cost`, `promo` opsional); `employee` menyatakan
`assignments`; `branch.tax_percent`/`service_charge_percent` dan
`promo.percent` menjadi `percent`; `ingredient.unit` + `recipe.lines[].unit`
menyatakan satuan gram/kg; ketiga summary menyatakan `invariants`, dan
`stock_level_apply.star` menjadi `maintained_by` `stock-level` (di-rename dari
`guard_stock_level_unique.star`). **Adopsi `row_scope` ditunda ke 3.5** — sesuai
urutan ledger; menyalakannya sekarang akan mem-403 seluruh list kasir dan
identitas dev yang tidak punya baris `employee`.

Test: `pkg/spec` (scope/assignments/unit/invariants/percent, 5 test baru),
`cmd/formspec/validate_scope_test.go`, `internal/api/scope_test.go`
(`TestApplyRowScope_AttributeFromAssignments`). Bukti gate: spec uji dengan
`row_scope` tanpa sumber → `scope: row_scope on "branch_id" resolves attribute
… no entity declares 'assignments'…`; `maintained_by` ke script hilang →
`honesty: … script not found`; `invariants` tanpa unique index → `engine:
invariants[0]: unique(branch_id) is not backed by a unique index`. Regresi:
`go test ./...` hijau, `vitest` 250 lulus, `tsc` bersih, kafe `validate`
**0 problem** (69 manifest).
