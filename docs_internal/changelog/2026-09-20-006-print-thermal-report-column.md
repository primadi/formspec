# 7.1 & 7.2 — Print thermal (ESC/POS) + ReportColumn vs TableColumn (S16)

**Tanggal:** 2026-09-20 · **TODO:** `examples/kafe/gaps_found/TODO.md` 7.1, 7.2

## 7.1 — Print `thermal` diimplementasikan (#10)

### Apa yang diubah

`internal/api/print.go` kini bercabang pada `output.format`:

- `thermal` → `renderPrintThermal` (ESC/POS byte stream: init `ESC @`, align,
  bold, cut `GS V 0`), `Content-Type: application/octet-stream`.
- `pdf` → `renderPrintPDF` (perilaku lama).
- `html` → dirender klien oleh PrintRenderer.
- lainnya (`dotmatrix`) → **501 NOT_IMPLEMENTED**, bukan diam-diam jadi PDF.

### Kenapa

Handler SELALU membalas PDF tanpa melihat `output.format`, sehingga
`format: thermal` lolos validasi dan menghasilkan PDF bernama `.pdf` — struk
kafe tidak bisa dicetak ke printer thermal. Ini kelas "validate bohong".

### Bukti

| Perintah | Hasil |
| --- | --- |
| `go test ./internal/api/ -run TestRenderPrintThermal` | PASS (prefix ESC/POS, cut command, bukan PDF, konten terinterpolasi) |
| `formspec validate` kafe | 0 problem |

## 7.2 — `ReportColumn` vs `TableColumn` diselaraskan (S16)

### Apa yang diubah

- `ReportColumn` mendapat `Widget TableCellWidget` (set yang sama dengan
  `TableColumn.widget`).
- `Aggregate`/`Format` menjadi **himpunan tertutup**: `ReportAggregate`
  (`sum`/`avg`/`count`/`min`/`max`) dan `ReportFormat`
  (`currency`/`date`/`datetime`/`percentage`) di `pkg/spec/widget.go`.
- `docs/spec/frontend/06-page-kinds.md` — tabel `ReportColumn` vs `TableColumn`
  berdampingan.
- `schemas/` — ter-regenerasi (`$defs/ReportAggregate`, `$defs/ReportFormat`,
  `ReportColumn.widget → $ref TableCellWidget`).

### Kenapa

`ReportColumn` tidak punya `widget` sama sekali, dan `aggregate`/`format` adalah
string bebas — sehingga menyalin bentuk `TableColumn` ke report gagal validasi,
dan salah ketik format lolos lalu mencetak nilai mentah.

### Bukti

| Perintah | Hasil |
| --- | --- |
| `go test ./pkg/spec/ -run TestReportColumn_ClosedSets` | PASS |
| schema `$defs/ReportAggregate` / `ReportFormat` | enum lengkap |
| `formspec validate` kafe | 0 problem |
| `go test ./...` | hijau |
