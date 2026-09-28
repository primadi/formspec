# 2026-09-27-017 — Alur sesi meja kafe: QR → bayar → occupied → served → release

**Apa:** Alur pemilik (plan `kafe-qr-table-session-flow.md`) mendarat di spec
kafe: enam gap yang selama ini memisahkan "tamu memindai QR" dari "meja kosong
kembali" kini tertutup, **tanpa menurunkan desain kafe**. Kafe: **88 manifest,
0 problem**.

| Gap              | Yang mendarat                                                                      | Bukti                                                                                                                                                                           |
| ---------------- | ---------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **10.34b**       | `dining-table.qr_token` → `natural_key: true`                                      | `GET .../dining-table/JKT-A01-DEMO` → **200** (dulu 404). Batasnya diakui di komentar: jalur `GetByID` mengikuti `WHERE _<natural_key>`, **tanpa** `LIMIT 1` yang diekspresikan |
| **10.34c**       | `table-session` partial unique `{dining_table_id, where: "status = 'open'"}`       | sesi terbuka kedua di meja yang sama → **500 UNIQUE constraint failed**. Dulu 4 request = 4 sesi terbuka                                                                        |
| **10.36**        | `rate_limit: {max, per: 1m, scope: ip}` pada `table-session` (20) dan `order` (30) | intake anonim kini punya pembatas per-IP                                                                                                                                        |
| **10.37**        | `submit.redirect` benar-benar dipakai + token `{uuid}`                             | sesi dibuat → tamu mendarat di `/kafe/menu/{guest_token}`                                                                                                                       |
| **10.35 / 2.15** | Page `table-open` (`route: /t/:qr_token`) + Form `table-open-form`                 | `/kafe/t/JKT-A01-DEMO` me-resolve meja **dari token**, membuat sesi, lalu mengarahkan ke menu                                                                                   |
| **10.40b**       | Subscription `cafe-master/table-occupancy` + script `table_status_from_order.star` | bayar → `table_status` = `occupied` **dalam 1 detik**                                                                                                                           |
| **10.41**        | `emit: on_served` pada `mark-served` + cabang `serve` di script yang sama          | `served` → tabel `served`; `available`/`reserved` dari pembayaran pertama                                                                                                       |

**Keputusan arsitektur yang perlu diingat (bukan pilihan gaya).** Subscription
`table-occupancy` diletakkan di module **`cafe-master`**, bukan `cafe-order`.
Alasannya terukur, bukan estetika: akses lintas-module dari script ditegakkan
saat runtime (`USES_VIOLATION: undeclared cross-module access to
cafe-master.dining-table from module cafe-order`), dan **`SubscriptionSpec`
tidak punya blok `uses`** untuk mendeklarasikannya. Yang memiliki entity adalah
yang menulisnya — pola yang sudah dipakai `gl` (pemilik jurnal mendengarkan
event pesanan) dan `cafe-stock`.

**Cacat yang ikut ditemukan sambil jalan.** `resolveFootprint` menolak seluruh
role bila **satu** grant menyebut page yang tidak dikenal, dan `role.app`
membuat role hanya berlaku pada App-nya. Keduanya nyata:

- `kasir` tanpa `app` pada login → **0 permission**. Dengan `app: kafe-pos` → **45**.
- Longsoran sebelumnya (`inventory-table-stock`) terjadi karena `role.app`
  menyaring role ke App-nya, sehingga page non-auth tidak resolve.
  **Blast radius-nya masih sistemik** (satu page yang tidak resolve = role
  kehilangan seluruh permission-nya) — dicatat sebagai item tersendiri `10.53`.

**Kalau `cafe-master` di-uninstall/di-nonaktifkan, occupancy seluruhnya mati** —
subscription-nya ikut hilang. Ditulis eksplisit di header manifest, bukan
dibiarkan sebagai kejutan.

**Sisa yang tetap terbuka (itemnya sudah ada di ledger kafe):**

- **10.38** — PIN tamu kedua + `guest_token` acak server-side (sekarang
  `{uuid}` di klien) + hashing. Tanpa PIN, tamu berbeda yang memindai saat
  `occupied` tidak dibedakan.
- **10.39** — Service publik `table-status`/`verify-pin` (PIN tidak bisa
  disimpan di entity: `find` mengembalikan seluruh field).
- **10.20** — landing `/kafe` → petunjuk scan QR.
- **10.42** — baris `dining-table` lama tanpa field `table_status` masih bisa
  membuat transisi 500 (seed fresh tidak terpengaruh).

**File:** `examples/kafe/spec/modules/cafe-master/{master/dining-table/entity.yaml,
subscriptions/table-occupancy.yaml, scripts/table_status_from_order.star}`,
`examples/kafe/spec/modules/cafe-order/{pages/table-open.yaml,
forms/table-open-form.yaml, transaction/order/entity.yaml,
transaction/table-session/entity.yaml}`**Rujukan:** plan
`docs_internal/plan/kafe-qr-table-session-flow.md` (urutan kerja §1–5 selesai)
