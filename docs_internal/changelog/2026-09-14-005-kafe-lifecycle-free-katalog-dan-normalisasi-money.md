# 2026-09-14-005 — Kafe: referenceability katalog (#44) + normalisasi money (#46, #2)

Tiga gap dari ledger kafe ditutup. `go test ./...` → 35 paket `ok`;
`vitest run` → 171 test lulus; `tsc --noEmit` bersih.

**#44 — `doc_status: draft` membuat data master tak bisa direferensikan.**
`create` selalu menghasilkan `draft`, dan hanya `submitted`/lifecycle-free yang
boleh jadi target `relation` — jadi `menu-item` tidak pernah bisa menunjuk
`menu-category` (422 `is draft`). Ditambahkan satu aturan di
`pkg/spec/entity.go`:

```go
// LifecycleFree: `lifecycle: plain_crud` (atau alias `none`), atau tanpa
// lifecycle eksplisit dengan characteristic master/reference.
func (e *EntitySpec) LifecycleFree() bool
```

Data katalog tidak punya alur draft→submit — lifecycle (draft/submit) adalah
untuk dokumen/transaksi. Ini juga menyelaraskan server dengan semantik frontend,
yang sejak awal memperlakukan `plain_crud` sebagai "tanpa tombol Submit" —
ketidakselarasan itulah akar #44. Dipakai di tiga tempat agar tidak ada drift:

- `renderers/jsonb-persist/crud.go` — `submitEnabled := !entity.LifecycleFree()`;
- `internal/api/generator.go` — `disabledActions()` baru, dipakai kedua surface,
  sehingga rute `submit`/`cancel`/`amend` tidak dibuat untuk entity lifecycle-free;
- `internal/entity/registry.go` — permission lifecycle tidak diregistrasi,
  sehingga permission selalu cocok dengan rute yang benar-benar ada.

Catatan: nilai `lifecycle: plain_crud` sudah ada di kontrak — jadi tidak ada
nilai enum baru, dan entity yang tidak menyatakan apa pun tetap berperilaku
seperti sebelumnya (`two_step_autosave`), sehingga blast radius-nya nol.

**#46 — `money` tidak divalidasi/dinormalisasi.** Tiga bentuk hidup berdampingan
(`{amount, currency}`, `{amount}`, angka polos) dan `settings.currency` tidak
pernah dipakai; `spec.ValidateMoneyValue` sudah ada tetapi **tidak pernah
dipanggil** selain di test. Ditambahkan `spec.NormalizeMoneyValue()` dan
`HandlerFactory.normalizeMoneyFields()` yang dipanggil di `HandleCreate` **dan**
`HandleUpdate`: number/numeric string/objek → selalu `Money{amount, currency}`,
lalu divalidasi (`ValidateMoneyValue`: currency mismatch + scale). Currency yang
tidak bisa ditentukan dari payload, field, maupun settings → error, bukan data
tanpa mata uang.

**#2 — renderer money.** `renderCell.tsx` hanya memformat bila nilainya `number`,
sehingga `{amount, currency}` jatuh ke `JSON.stringify`. Helper `moneyAmount()`
(baru, di `src/lib/format.ts`) menerima number, numeric string, dan objek kanonik;
dipakai `renderCell.tsx`, `DetailPage.tsx` (type `money` sebelumnya **tidak**
ditangani sama sekali sehingga selalu tampil JSON), `ReportRenderer.tsx`, dan
`DashboardRenderer.tsx`.

Verifikasi runtime (spec kafe, `formspec dev`):
`POST …/cafe-master/menu-item` dengan `menu_category_id` → **201** (sebelumnya 422
`is draft`); `value: 25000` → `{"amount":"25000","currency":"IDR"}`; `"Rp25.000"`
→ **422** dengan pesan menuntun.

Test baru: `TestGenerateUIRoutes_LifecycleActions` (katalog tanpa rute lifecycle,
transaksi punya), `TestNormalizeMoneyValue`, 5 case `moneyAmount` di
`format.test.ts`; `TestEntityStore_ResolveRelations_NoDeadlockUnderTxScope`
diperbarui (target relasi kini valid langsung, tanpa `Submit`).

Referensi: `examples/kafe/gaps_found/TODO.md` (2.1, 2.3, 2.4),
`examples/kafe/gaps_found/decisions-needed.md` (D2).
Sisa terkait: #45 (create anonim), #28/#1.3 (aritmetika & agregasi money), #5 (cart).
