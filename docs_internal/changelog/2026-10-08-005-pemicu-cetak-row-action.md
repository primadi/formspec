# 2026-10-08-005 — Pemicu cetak deklaratif (`row_action.view`) + kartu nomor meja kafe

**Plan:** `docs_internal/plan/print-row-action.md`.
**Menutup:** master todo **5.25.9** · kafe gap **2.15**.

## Apa yang diubah

`kind: Print` bisa dideklarasikan dan rutenya hidup, tetapi **tidak ada satu pun**
jalan dari klien ke sana (`grep -rn 'print/' src/kinds src/engine` → 0), sehingga
kasir harus mengetik `/kafe/app/pos/print/receipt-thermal/<order-id>` sendiri.
Sisi server **sudah** menyediakan pintu masuknya: `internal/ui/validate.go`
`builtinRowActions` memuat `print` (dan `export`) sebagai action "renderer-provided,
tanpa backing entity action" — yang hilang hanya sisi klien dan cara menyatakan
**tujuan** cetak.

Ditutup dengan satu field deklaratif generik: `TableAction.View` — `view:
"<kind>:<name>"`, memakai kosakata `{kind}:{name}` yang sudah dipakai
`MenuItem.view`/`registered_views`/grants.

- **Go** — `pkg/spec/frontend.go` `TableAction.View` (`@schema`); `internal/ui/validate.go`
  `validateActionView` (+ `viewTargetModule`) memvalidasi target lewat
  `resolveViewRouteLocked` (**bukan** salinan kelima konvensi kind→route), dipanggil
  dari loop Tables & Kanbans.
- **Klien** — `engine/viewTarget.ts` (`resolveViewTarget`, `viewTargetTakesId`,
  `canDoViewTarget`, `canDoTableAction`); `TableRenderer` + `KanbanRenderer`
  mencabang pada `action.view` **sebelum** `canDoEntityAction` (action `print`
  tidak punya permission entity — cabang di belakang guard = kode mati).
- **Kafe** — `cafe-master/prints/table-tent-card.yaml` (kartu meja A5: nomor +
  QR `/kafe/t/{qr_token}`) dan `cafe-master/tables/dining-table-table.yaml`
  (hanya menambah `row_actions`, kolom tetap diwarisi `deriveTable`).

**Kenapa `print` di row action, bukan mekanisme baru:** ia sudah builtin yang
di-whitelist validator; yang belum ada hanya dispatch navigasi di klien. Karena
itu 2.15 (kartu meja QR) bisa ditutup **tanpa** perubahan route kafe — `qr_token`
sudah `natural_key` (10.34b) dan halaman masuk `table-open` sudah mendarat
(`docs_internal/plan/kafe-qr-table-session-flow.md`), jadi QR mengarah ke alur
yang sudah hidup (terukur hidup 2026-10-03).

## Bukti

- `formspec validate --spec examples/kafe/spec --schema schemas` → **90/0**
  (baseline 88 file tracked + 2 baru; kedua manifest baru muncul di output) ·
  `formspec check -f examples/kafe/spec` → **0 error, 0 warning**.
- `go build ./...` · `go test ./...` hijau · `gofmt` bersih ·
  `golangci-lint` **0 issues**.
- Test Go baru: `internal/ui` `TestValidate_ActionView` (target valid lolos;
  malformed + nama tak terdaftar ditolak) dan `internal/app`
  `TestResolve_KafeSpec_PrintTrigger` atas **spec kafe nyata** — memverifikasi
  klaim "tanpa perubahan App config": Print `table-tent-card` terdaftar di module
  `cafe-master` yang memang di-mount `kafe-pos`, payload QR `/kafe/t/{qr_token}`,
  dan row action `print` menunjuk `view: print:table-tent-card`.
- `tsc -b` bersih · `vitest` **695** hijau (naik dari 677: `viewTarget.test.ts`
  12 + `view-row-action.test.ts` 6).
- Guard **dibuktikan menggigit**: cabang `action.view` dihapus + `resolveViewTarget`
  dibuat selalu gagal → 8 test FAIL; `validateActionView` di-no-op → 2 test FAIL
  (`TestValidate_ActionView`); keduanya dipulihkan lalu hijau (file asli di-`cp`
  ke `/tmp` lebih dulu, bukan `git checkout`).
- `make generate-schema` (atribut `view` di `TableAction`) + `make generate-kind-docs`.

## Sisa (item bernomor)

- Pemicu cetak untuk **struk** (`receipt-thermal`/`receipt-digital`) belum
  dipasang di `order-table-pos` → **5.25.11 ⏸️**.
- **`qr_token` tanpa generator** — token diisi manual saat membuat meja →
  kafe **10.83 ⏸️**.
- QR ber-scope sesi (PIN tamu) tetap **10.38/10.39 ⏸️** (tidak termasuk).
