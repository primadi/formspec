# 2026-09-22-012 — Kafe Fase 10: landed cost, purchase events, seed (role/menu/stok/GL)

**Tanggal:** 2026-09-22 · **Trigger:** instruksi pemilik — "10.1 Biaya kirim belum
dimodelkan, di BOM item, masukkan juga custom item biaya yg bisa diisi bebas;
10.2 purchase jurnal dll berupa published event saja; 10.3 buat data seed untuk
role …; 10.4 buat saja menu nasi goreng, es teh …, cari gambar di internet;
10.5 selesaikan semua isu yg masih open"
· **Plan:** `docs_internal/plan/fase-10-kafe-landed-cost-seed.md`

## Yang dikerjakan

**10.1 — Landed cost + baris biaya bebas ("custom item biaya").**
`purchase-order` mendapat `shipping_cost` (money) dan `lines[].line_type`
(`ingredient|cost`) dengan `cost_label` + `cost_amount` + `allocated_to`
opsional. Resep mendapat `line_type` yang sama, jadi biaya non-bahan seperti
"Gas & listrik" dan "Kemasan takeaway" bisa dinyatakan per resep dan ikut
dijumlahkan `menu-cost.cost_per_portion`. `ingredient_id`/`quantity`/`unit_cost`
berubah dari `required: true` menjadi `required_when: line_type == 'ingredient'`
— sebelumnya baris biaya **tidak bisa dinyatakan sama sekali**.

`receive-goods` berubah dari "transisi tanpa `impl`" menjadi action Starlark
(`cafe-stock/purchase_receive.star`), karena penerimaan punya pekerjaan nyata:
membuat `stock-movement` per baris + mengalokasikan landed cost ke `unit_cost`.
Alokasi proporsional terhadap **nilai** baris (qty × unit_cost), dengan **sisa
pembulatan dibebankan ke baris bernilai terbesar** supaya Σ alokasi = faktur
persis — tanpa itu nilai persediaan tidak pernah rekonsiliasi.

**10.2 — Purchase hanya memancarkan event.** `events: [on_po_received,
on_po_cancelled]` (`publish.durable`, `deliver: [reliable_event]`) + `emit:` pada
transisi. **Tidak** ada Integrator/Subscription di sisi `cafe-stock`: saat `gl`
dikerjakan, gl menambah subscription sendiri — tidak ada baris `cafe-stock` yang
perlu diubah.

**10.3 — Seed role + akun.** `formspec.core/seeds/roles.yaml`: role **kasir,
barista, dapur, pelayan, supervisor, manajer** dengan grants yang benar-benar
membedakan hak, plus 7 akun (`kafe123`) dengan `assignments` (role × cabang).

**10.4 — Seed master + menu + gambar.** `cafe-master/seeds/master.yaml` (2
cabang, 4 kategori, 7 menu, 9 harga, 4 meja, 6 karyawan), `cafe-stock/seeds/stock.yaml`
(2 supplier, 10 bahan, 3 resep), `gl/seeds/chart-of-accounts.yaml` (menutup
9.4.1a). 7 foto menu diunduh dari Wikimedia Commons (CC BY / CC BY-SA, atribusi
di `examples/kafe/assets/ATTRIBUTION.md`) dan di-commit — seed tidak boleh
bergantung jaringan.

## Prasyarat yang harus dibangun dulu: `kind: Seed` didaftarkan

Jawaban jujur atas "kind untuk data seed sudah ada kan?": **perintahnya ada,
kind-nya tidak.** `formspec seed` sudah menerima `kind: Seed` sejak lama, tetapi
`Seed` tidak ada di `pkg/spec` maupun `internal/manifest.KnownKinds`, sehingga
**setiap file seed ditolak `formspec validate`** (`unknown kind "Seed"`) dan
`schemas/kinds/Seed.schema.json` tidak pernah ada. Yang dikerjakan:

- `pkg/spec/seed.go` — `SeedSpec`/`SeedEntity` + `ValidateSeedSpec`.
- `KindSeed` di `AllKinds()`/`IsValidKind`, `KnownKinds`, dan `KindMapping()`.
- Loader: validasi bentuk Seed (record payload sengaja TIDAK divalidasi di sini
  — itu butuh registry, dan engine sudah menolaknya saat insert).
- Generator: `SeedEntity` masuk `sharedTypes`; **bug generator ditemukan** —
  `fieldItemsSchema` tidak punya `case *types.Map`, jadi `[]map[string]any`
  jatuh ke `items: {type: string}` dan **setiap record seed gagal schema
  validation padahal engine menerimanya** (kelas "lolos engine, ditolak schema"
  yang generator ini ada untuk mencegahnya).
- CLI: `--workspace` (dulu `"demo"` hardcode — seed kafe akan mendarat di tenant
  yang tidak pernah dibaca App-nya); satu definisi `SeedSpec` (bukan duplikat);
  user di-seed lewat `CreateUser` (hook entity tidak jalan di jalur seed, jadi
  password tidak akan pernah ter-hash).
- **`$ref` untuk relasi.** Relasi disimpan sebagai ID record (UUID yang
  digenerate engine), yang tidak bisa diketahui penulis YAML — tanpa ini, seed
  yang berguna (harga per cabang, baris resep, PO) **tidak bisa dinyatakan sama
  sekali**. Sintaks `{ $ref: "module.entity:field=value" }`, diselesaikan
  terhadap baris yang sudah ada + yang baru dibuat, dengan iterasi sampai titik
  tetap **supaya urutan blok di file tidak menentukan hasil**.
- Idempotensi diperluas: natural key → field `unique` tunggal (role/user) →
  `invariants`/`indexes` komposit (harga per cabang). Hasil: run kedua =
  **0 inserted, 69 skipped, 0 failed**.

## Dua bug mesin yang ditemukan (dan diperbaiki)

1. **`CreateUser` menulis `"active": u.Active`** — zero-value Go = `false`,
   menimpa default `true` di entity spec. Setiap akun yang dibuat lewat jalur itu
   mendarat **nonaktif**, dan login menolaknya dengan *"invalid username or
   password"* — pesan yang menunjuk password padahal masalahnya akun mati.
   `UpdateUser` punya trap yang sama (map-nya menggantikan seluruh data, jadi
   caller yang hanya mengubah `roles` mematikan akunnya). Fix: `active`
   di-hardcode `true` pada create; `activeForUpdate` mempertahankan nilai
   tersimpan pada update. Deaktivasi tetap lewat `status` — kanal yang
   didokumentasikan entity.
2. **`resource.create` dari script tidak menjalankan hook `after create`.** Jalur
   HTTP menjalankannya, jalur script tidak. Akibat konkret: `receive-goods`
   membuat `stock-movement` (baris ada, `unit_cost=40` benar) tetapi `stock-level`
   — proyeksi yang dipelihara hook `after create` pada `stock-movement` —
   **tidak pernah ter-update**, tanpa error di mana pun. Fix di
   `SetCreateHandler` (`resource/formspec.go`); lihat master todo 7.8.8 untuk
   bukti test (gagal sebelum, hijau sesudah) dan **7.8.9 ⏸️** untuk jalur
   `save`/`update` yang belum diperiksa.

Plus: **grant yang gagal materialisasi dilewati tanpa suara** (`resolver.go`
`continue // malformed grant — skip role`). `approval-inbox:` bukan navigation
kind yang didukung, sehingga role `supervisor` DAN `manajer` kehilangan SELURUH
grant dan semua request-nya membalas 404 tanpa satu pun error. Resolver kini
menerima `SetLogger` dan melaporkannya.

## 10.5 — isu terbuka yang ditutup

- **`trial-balance` + 3 widget GL** merujuk `entity: ledger` yang **tidak pernah
  ada** → diperbaiki ke `gl.gl-balance` dengan field yang benar. `formspec check`
  turun dari 6 error → 0.
- **`journal-table`** merujuk `journal_entry` (underscore) dan mendeklarasikan
  kolom `debit`/`credit` yang field child, bukan field header → diperbaiki;
  peringatan `validate warning` di boot dev turun dari 2 → **0**.
- **Widget `recent-journals`** dirujuk `gl-dashboard` tetapi manifest-nya tidak
  pernah ada → dibuat.
- `make seed-kafe` + `make seed-kafe-assets` ditambahkan (seed target sengaja
  **tidak** bergantung pada build SPA — lihat item 5.15.3).

## Bukti

`formspec validate --spec examples/kafe/spec --schema schemas` → **83 manifest,
0 problem** · `formspec check` → **0 error, 0 warning** · `make seed-kafe` →
**69 inserted**, rerun → **0 inserted / 69 skipped / 0 failed** · login 6 role di
dev server dengan `app` scope → permission berbeda sesuai tabel di kafe TODO 10.3
· `kasir.dua` → **409 CONTEXT_REQUIRED + 2 choices** · `GET menu-item/{id}/photo`
→ **200 `image/jpeg` 83.189 byte** · PO E2E: `unit_cost` **15 → 40** dan
`stock-level` `qty=10000 avg=40 value=400000` · `go test ./...` hijau (test baru:
`TestKafe_ReceiveGoodsMaintainsStockLevel`, `TestValidateSeedSpec`,
`TestSeedIsRegisteredKind`, `TestSeedResolvesRefs`, `TestSeedUnresolvedRefFails`).

## Sisa (semua bernomor)

Kafe **10.6 ⏸️** (alokasi landed cost hanya proporsional nilai) · **10.7 ⏸️**
(belum ada consumer `on_po_received`) · **10.8 ⏸️** (verifikasi katalog di browser)
· master todo **7.8.9 ⏸️** (`resource.save`/`update` belum diperiksa untuk gap
hook yang sama) · **5.15.3 ⏸️** (SPA tidak bisa di-build — refactor permission
setengah jalan di working tree; bukan dari sesi ini).

Kafe **2.15 / 6.3 / 6.4 / 9.4** tidak disentuh: ketiganya deferred beralasan
(kontrol plane, keputusan bahasa spec) dan 9.4 menunggu skenario walkthrough.
