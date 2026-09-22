# 3.12 — Hook `after create` senyap karena script memakai `float()` pada nilai `money`

**Tanggal:** 2026-09-20 · **TODO:** `examples/kafe/gaps_found/TODO.md` 3.12 (temuan walkthrough 9.4, skenario 7)

## Gejala

Dua `POST /_ui/entity/cafe-stock/stock-movement` (in, 1000@50 lalu 500@80)
membalas **201**, tetapi `cafe_stock_stock_levels` tetap **0 baris** — proyeksi
saldo/moving-average (dasar HPP & margin) tidak pernah ditulis. Tidak ada error di
keluaran server, tidak ada berkas log.

## Diagnosis

Log sementara satu baris di `RunAfterPhase` (`internal/action/hooks.go`) sudah
cukup:

```
[hook-debug] after action="create" hooks=1 selected=1
[hook-debug] dispatch implNil=false err=action cafe-stock.create (script_ref, 2.17ms):
  script failed: script runtime error: float got money, want number or string
```

- Hook **dipilih dan dijalankan** (`hooks=1 selected=1`) — jadi bukan masalah
  registrasi hook.
- Script-nya yang gagal: `float(resource.field.unit_cost)` — `unit_cost` adalah
  nilai **money** (objek `{amount, currency}`), dan `float()` menolaknya
  (S7 / 1.3: operand tidak sah = **error**, bukan `0`).
- Kegagalannya **senyap bagi pemanggil**: `RunAfterPhase` sengaja tidak
  membatalkan respons (baris sudah commit; tidak ada yang bisa di-rollback), ia
  hanya mencatatnya lewat logger engine — itulah sebabnya create tetap 201 dan
  mengapa tidak ada jejak di stdout.

## Perbaikan

`examples/kafe/spec/modules/cafe-stock/scripts/stock_level_apply.star`:

- helper `money_amount(value, default)` — membaca uang lewat `amount()`;
- helper `money_like(angka, sumber, default_currency)` — menulis bentuk kanonik
  `{amount, currency}` (mata uang ikut sumber, bilangan bulat tanpa `.0`);
- semua pembacaan uang (`unit_cost`, `moving_avg_cost`) dan penulisan
  `moving_avg_cost` memakai helper itu.

Log debug dicabut kembali setelah diagnosis.

## Bukti

| Perintah | Hasil |
| --- | --- |
| `formspec validate --schema ../../schemas` | **0 problem** (script dikompilasi) |
| dua movement in (1000@50, 500@80) | **201** keduanya |
| `cafe_stock_stock_levels` | **1 baris**: `quantity_on_hand 1500`, `moving_avg_cost {"amount":"60","currency":"IDR"}` — persis (1000×50 + 500×80) ÷ 1500 |
| `go test ./...` | hijau |
| `make lint` | 0 issues |

## Catatan

- Kelas bug ini (after-hook senyap) layak diingat: **create 201 bukan bukti hook
  berhasil**. Untuk hook `after`, satu-satunya jejaknya adalah efeknya (atau log
  engine).
- Sisa kecil: script belum mengisi `stock_value` (`qty × avg`),
  `last_movement_at`, dan `is_below_min` — proyeksi berjalan tetapi tiga kolom
  itu masih kosong, jadi laporan nilai persediaan belum bisa memakainya.
