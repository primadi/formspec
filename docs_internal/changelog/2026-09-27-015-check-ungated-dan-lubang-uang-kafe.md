# 2026-09-27-015 — check "permission yang tidak ditegakkan" + lubang uang kafe ditutup

**Apa:** check baru `formspec check` memperingatkan action yang mendeklarasikan
`required_permission` tetapi **tidak ada yang menegakkannya**, lalu tujuh gate
kafe dipindahkan ke transisi sehingga benar-benar berlaku.

**Kenapa — O6 dari plan L4 terbukti sebagai lubang, bukan pertanyaan desain.**
Rencana L4 mencatat "50 action impl-less: putuskan bagaimana
`required_permission` ditegakkan, atau jangan deklarasikan". Terukur 2026-09-27
di server hidup:

> kasir **tidak punya** `confirm-payment` dalam bentuk apa pun, tetapi
> `PATCH status=paid` pada `order` (`awaiting_payment → paid`) → **200,
> status=paid**.

Siapa pun pemegang `update` bisa **menandai pesanan lunas**. `required_permission`
pada action tanpa `impl` tidak diperiksa di jalur mana pun: action itu tidak punya
route, dan jalur penerapannya (`PATCH status`) hanya diperiksa `{plural}.update`.
Ini jalur uang, jadi keputusannya tidak bisa ditunda.

**Check baru** (`cmd/formspec/check_ungated.go`): memperingatkan, bukan
menolak — bentuk action-impl-less memang sah bila gate-nya hidup di transisi.
Pesannya menunjuk tepat apa yang harus dilakukan. Dua test pengunci
(`check_ungated_test.go`), terkalibrasi.

**Tujuh gate kafe dipindahkan ke transisi** (gate → dihapus dari `actions:`):

| entity    | action                          | kenapa penting                                   |
| --------- | ------------------------------- | ------------------------------------------------ |
| `order`   | `confirm-payment`               | menandai **LUNAS** (uji: kasir 200, pelayan 403) |
| `order`   | `cancel-order`                  | membatalkan pesanan                              |
| `order`   | `start-preparing`, `mark-ready` | pemisahan peran **dapur/barista**                |
| `order`   | `mark-served`, `complete-order` | pemisahan peran **pelayan**                      |
| `payment` | `settle`, `fail`                | uang masuk                                       |
| `payment` | `refund`                        | **uang keluar** (uji: kasir 403, supervisor 200) |
| `shift`   | `close-shift`                   | rekonsiliasi kas                                 |

Karena `barista`/`dapur`/`pelayan` semuanya hanya di-grant `update` pada
`order-page`, empat transisi dapur/sajian **tidak terbedakan** sebelum ini —
grant-nya kini eksplisit per peran (`buildEntitySchema`/`authorizedActions`
membaca union sejak L5).

**Bukti end-to-end (server hidup):**

| siapa                             | aksi                      | hasil                             |
| --------------------------------- | ------------------------- | --------------------------------- |
| pelayan (tanpa `confirm-payment`) | `awaiting_payment → paid` | **403**                           |
| kasir (punya)                     | `awaiting_payment → paid` | **200** `paid`                    |
| kasir (tanpa `refund`)            | `settled → refunded`      | **403** `…payments.refund`        |
| supervisor (punya)                | `settled → refunded`      | **200** `refunded`                |
| pelayan (tanpa `start-preparing`) | `paid → in_kitchen`       | **403** `…orders.start-preparing` |
| barista (punya)                   | `paid → in_kitchen`       | **200** `in_kitchen`              |

**Warning `formspec check` kafe: 21 → 10.** `formspec validate` 85 manifest /
0 problem. `go test ./internal/... ./pkg/... ./cmd/...` hijau.

⚠️ **Regresi yang saya perkenalkan lalu perbaiki:** edit di `check.go`
membuat pemanggilan `checkDatastores` **menempel ke baris komentar** di
atasnya, sehingga check itu tidak pernah jalan dan linter melaporkannya sebagai
`unused`. Sudah dipisah ke barisnya sendiri, dengan komentar yang menyebutkan
mengapa. **Pelajaran: `golangci-lint` menangkap dead code; jangan hanya
membaca diff.**

**Blind spot yang tetap ada (dicatat, bukan diklaim selesai):** check ini
menangkap **kontradiksi** (permission dideklarasikan tetapi tidak ditegakkan),
bukan **omisi**. Terukur **43 transisi tanpa gate sama sekali** di `examples/` +
`verticals/` (15 file) — termasuk `arisan/draw.mark-paid`,
`stock-opname.post-opname`, `billing/order.void`, dan 11 transisi approval CRC.
Semuanya diotorisasi `{plural}.update` saja. → item baru **O9**.

**File:** `cmd/formspec/check.go`, `cmd/formspec/check_ungated.go` (baru),
`cmd/formspec/check_ungated_test.go` (baru),
`examples/kafe/spec/modules/cafe-order/transaction/{order,payment,shift}/entity.yaml`,
`examples/kafe/spec/modules/formspec.core/seeds/roles.yaml`.

**Sisa → kafe 10.51 + O9** (transisi tanpa gate di contoh/vertikal lain).
