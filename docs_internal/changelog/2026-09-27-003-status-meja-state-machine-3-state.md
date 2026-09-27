# Status meja kafe: `state_machine` 3-state (10.40)

**Plan:** `docs_internal/plan/kafe-qr-table-session-flow.md` (revisi 2)
**Ledger:** kafe **10.40 ✅** · sisa → **10.40b ⏸️, 10.42 ⏸️, 10.43 ⏸️**

## Perubahan

Menerapkan keputusan pemilik 2026-09-27 ("status meja pakai `state_machine`",
pemicu pembayaran, 4 status termasuk `served`) pada `cafe-master.dining-table`:

- **Field** `table_status` — `enum [available, occupied, served]`,
  `default: available`, `index: true`.
- **`state_machine`** — `available → occupied` (`occupy`),
  `occupied → served` (`mark-table-served`), `served → occupied` (`occupy`,
  untuk tamu yang menambah pesanan), `occupied|served → available` (`release`).
- **Actions** bernama (`occupy`, `mark-table-served`, `release`) supaya tiap
  `via:` menunjuk sesuatu dan terbaca di manifest.
- **Grant** `update` pada `dining-table-page` untuk kasir, pelayan, supervisor,
  manajer.

File: `examples/kafe/spec/modules/cafe-master/master/dining-table/entity.yaml`,
`examples/kafe/spec/modules/formspec.core/seeds/roles.yaml`.

## Bukti (dev server, terukur 2026-09-27)

| Langkah                                            | Hasil                              |
| -------------------------------------------------- | ---------------------------------- |
| `available → occupied` (kasir)                     | **200** → `_status = occupied`     |
| `occupied → served` (pelayan)                      | **200** → `_status = served`       |
| `served → occupied` (pelayan, tamu tambah pesanan) | **200** → `_status = occupied`     |
| `occupied → available` (kasir, clear meja)         | **200** → `_status = available`    |
| `available → served` (ilegal)                      | **422** `invalid state transition` |
| `PATCH` anonim                                     | **401**                            |

Kafe `validate` 85 manifest 0 problem · `go build ./...` · `go test ./...` hijau
· `gofmt` bersih.

**Efek keamanan (sebab utama rancangan ini lebih baik):** sumbu "meja terisi"
tidak lagi _keberadaan sesi_ — yang bisa dibuat anonim (terbukti `POST
table-session` → **201**) — melainkan field ber-permission. Terukur: anonim
`PATCH` → **401**, jadi tamu tidak bisa mengunci meja.

## Tiga temuan yang lahir dari implementasi (semuanya dicatat)

1. **10.43 — gerbang transisi adalah `update`, bukan `required_permission`
   action.** `kasir` yang memegang `dining-tables.release` tetap **403**
   `missing permission: ...update`; `pelayan` yang memegang `occupy` +
   `mark-served` juga **403**. Konvensi repo membenarkannya: `shift` juga
   mendeklarasikan `shifts.close-shift` sementara nol role memegangnya. Jadi
   `required_permission` pada action **tanpa `impl`** adalah dekorasi yang
   menyesatkan (grant editor menawarkannya, tapi tak ada yang menegakkan).
   Grant akhirnya memakai `update` — dan itu yang terverifikasi jalan.
2. **10.42 — tidak ada backfill field enum baru.** Baris yang dibuat sebelum
   `table_status` ada membuat transisi **500** `initial state must be
"available"` (6 dari 7 meja di DB dev). Seed **fresh** aman (`default`
   diterapkan engine; bundle memuat `"default": "available"`). Form Edit
   mem-PATCH seluruh field → _self-heal_; bahayanya di jalur non-UI, dan
   **10.40b menempuh jalur itu**.
3. **10.41 — "semua pesanan disajikan" butuh agregat lintas-record.** Starlark
   tidak punya list/query (`ResourceAPI.AttrNames` → `find` saja, satu record),
   `EvaluateGuard` tidak punya agregat, dan `RebuildSpec` tidak punya `trigger`.
   Jalan: `ctx.db().query` (GAP-30 sudah diperbaiki 2026-09-18) atau tambah
   `Rebuild.Trigger` di `pkg/spec`.

## Sisa

- **10.40b ⏸️** — pemicu otomatis `order.on_paid → occupied` (subscription).
- **10.42 ⏸️** — backfill kolom untuk DB lama (putuskan sebelum 10.40b).
- **10.43 ⏸️** — putuskan apakah action impl-less boleh membawa
  `required_permission` yang tidak ditegakkan.
- **10.41 ⏸️ / 10.34c / 10.36 / 10.37 / 10.38 / 10.39 / 10.20** — tidak berubah.
