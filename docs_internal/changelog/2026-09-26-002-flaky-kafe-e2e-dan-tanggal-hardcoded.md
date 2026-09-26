# Flaky kafe E2E: tunggu `status: posted`, dan buang tanggal hardcoded

## Apa yang diubah

**1. `waitForJournal` / `waitForJournalSource` menunggu STATUS, bukan keberadaan
baris** (`resource/o2c_e2e_test.go`, `resource/purchase_journal_e2e_test.go`).
Helper lama hanya menunggu `countJournalEntries() > 0` — yaitu barisnya **ada** —
lalu test meng-assert `status == "posted"`. Dua momen itu berbeda: baris lahir
saat `journalize.star` meng-insert, sedangkan `posted` di-set handler
sesudahnya. Di antara keduanya assertion bisa melihat `"draft"` dan gagal
(`journal status = "draft", want posted`). Sebelumnya ini tercatat sebagai flake
"1/8 run" dengan atribusi yang salah (item kafe 10.7 / `gl/config/gl.yaml`).
Kini kedua helper mem-poll status dengan deadline dan melaporkan status terakhir
yang teramati saat timeout, jadi kegagalan berikutnya membawa buktinya sendiri.

**2. Tanggal hardcoded `2026-09-22` di test E2E adalah bom waktu yang MELEDAK
hari ini.** Default `BackdatePolicy` adalah `max_days_back: 3`
(`renderers/jsonb-persist/transaction_date.go` `DefaultBackdatePolicy`), sehingga
sejak 2026-09-26 tanggal itu ditolak
`FORMSPEC.TXN.BACKDATE_EXCEEDED: transaction_date 2026-09-22 exceeds backdate
limit: max 3 days, got 4 days`. Enam test kafe gagal **permanen** (bukan flaky):
`TestKafe_OnPaidCreatesBalancedJournal`, `TestKafe_OnPaidIsIdempotent`,
`TestKafe_JournalizeRejectsOrderWithNoAmount`,
`TestKafe_PurchaseReceivedCreatesJournal`, `TestKafe_ReceiveGoodsMaintainsStockLevel`,
`TestKafe_LandedCostAllocationStrategy`, `TestKafe_WeightStrategyRefusesUnknownWeight`.
Keempat berkas memakai helper `recentDate()` yang **sudah ada** di paket
(`resource/uses_enforcement_e2e_test.go`, dan disalin di
`examples/Clinic-UI-Showcase/clinic_e2e_test.go`) — jadi perbaikannya adalah
memakai yang sudah tersedia, bukan membuat yang baru. Envelope outbox juga tidak
lagi memakai `emitted: "2026-09-22T00:00:00Z"` melainkan `time.Now()`.

## Kenapa

- Flake (1) bukan regresi siapa pun, tetapi membuat `go test ./...` tidak bisa
  dipakai sebagai gerbang — dan atribusi lamanya menunjuk berkas yang tidak
  berubah sama sekali (`git diff HEAD -- examples/kafe/spec/modules/gl/` kosong).
- Bom waktu (2) adalah kelas yang lebih buruk: test hijau saat ditulis, merah
  permanen beberapa hari kemudian, tanpa ada yang mengubah kode. Ia juga
  menyembunyikan flake (1) — selama enam test gagal karena tanggal, laju flake
  tidak bisa diukur.

## File terdampak

- `resource/o2c_e2e_test.go` — `waitForJournal` + 3 `transaction_date` + `emitted`
- `resource/purchase_journal_e2e_test.go` — `waitForJournalSource` + 1 tanggal
- `resource/allocation_strategy_e2e_test.go` — 2 tanggal
- `resource/script_hooks_e2e_test.go` — 1 tanggal

## Bukti

- **Sebelum fix:** `go test ./resource/ -run TestKafe -count=1` → **6 FAIL**,
  semuanya `BACKDATE_EXCEEDED` (atau timeout menunggu jurnal yang tidak pernah
  lahir karena event-nya gagal).
- **Sesudah fix:** `-run TestKafe` → `ok 8.4s`; `go test ./resource/ -count=1`
  **5/5 run hijau** (sebelumnya `2/8` run gagal di HEAD bersih `dd3adc6`).
- `go build ./...` bersih; `go test ./...` 0 FAIL.

## Rujukan

Todo **5.10.24** (dan **7.8.18**, item duplikat tentang flake yang sama).
