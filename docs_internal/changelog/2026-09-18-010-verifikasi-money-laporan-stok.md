# 4.8 — Verifikasi agregasi `money` pada bentuk laporan stok kafe (#28)

**Tanggal:** 2026-09-18 · **Plan:** `docs_internal/plan/fase-4-kafe-stok-hpp.md`
(4.8) · **TODO:** `examples/kafe/gaps_found/TODO.md` 4.8

## Apa yang diubah

Tidak ada perubahan kode — item ini murni **verifikasi**. Ditambahkan
`TestAggregate_StockReportShape` (`renderers/jsonb-persist/aggregate_money_test.go`).

## Kenapa

Item 1.3 menetapkan semantik aritmetika/agregasi `money`, tetapi belum
diverifikasi pada **bentuk laporan stok kafe yang sebenarnya**:
`cafe-report/reports/stock-usage.yaml` mengagregasi `total_cost` (money) dengan
`sum`, dikelompokkan per `ingredient_id`, berdampingan dengan `quantity`
(decimal). Test baru memakai bentuk itu — bukan kasus sintetis satu field —
supaya semantiknya terbukti terhadap apa yang benar-benar diminta aplikasi.

## Bukti

| Perintah | Hasil |
| --- | --- |
| `go test ./renderers/jsonb-persist/ -run TestAggregate_StockReportShape` | PASS |
| SUM(total_cost) per grup | kopi **75000**, susu **30000** (bukan 0, bukan teks) |
| Grand total (report `totals:`) | **105000** |
| SUM(quantity) decimal | **6** |
| `go test ./...` | hijau |
