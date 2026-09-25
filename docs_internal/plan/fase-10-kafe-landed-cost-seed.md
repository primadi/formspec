# Plan — Fase 10 kafe: landed cost, purchase events, seed role & menu

**Sumber:** permintaan pemilik 2026-09-22 (menjawab `examples/kafe/gaps_found/TODO.md`
Fase 10) · **Referensi:** `AGENTS.md` Workflow Discipline
· **Status:** disetujui pemilik (instruksi langsung, item per item)

## Ringkasan permintaan → rencana

| Item | Permintaan pemilik | Rencana |
| --- | --- | --- |
| **10.1** | "Biaya kirim belum dimodelkan, di BOM item, masukkan juga custom item biaya yg bisa diisi bebas" | Tambah baris biaya non-bahan di **child `lines`** resep (`line_type: ingredient\|cost`), biaya kirim di **purchase-order** sebagai baris biaya bebas, lalu alirkan ke `stock-movement`/`stock-level` |
| **10.2** | "purchase jurnal dll berupa published event saja, nanti kalau modul GL dikerjakan, bisa listen event tersebut" | Purchase-order memancarkan **event durable** (`on_received` / `on_cancelled`) lewat `emit:` + `events:`; **tidak** ada Subscription/Integrator di sisi purchase. GL cukup menambah `events:` di manifest-nya sendiri |
| **10.3** | "buat data seed untuk buat role: kasir, dapur, admin, dll. buat hak aksesnya proper, kemudian role2 tersebut bisa dites" | `kind: Seed` untuk role + user + assignment; grants proper per role; **prasyarat: kind `Seed` harus didaftarkan dulu** (lihat Temuan A) |
| **10.4** | "buat saja menu nasi goreng, es teh (misalnya), cari gambar di internet, dan masukkan ke data seed. Kind untuk data seed sudah ada kan?" | Seed master kafe (branch, kategori, menu, harga, bahan, resep, meja) + **gambar diunduh dari Wikimedia Commons** (lisensi bebas) ke `examples/kafe/assets/`, disitir seed |
| **10.5** | "selesaikan semua isu yg masih open" | Tiga sisa 10.5 (dok kontrak REST tulis tangan, kolom turunan `REAL` SQLite, alur QR dua langkah) + semua item ⏸️ hasil audit yang bisa ditutup di mesin |

---

## Temuan riset (wajib dibaca sebelum implementasi)

### A. `kind: Seed` **belum terdaftar** — perintah ada, kind tidak dikenal

Jawaban atas pertanyaan pemilik ("Kind untuk data seed sudah ada kan?"): **setengah**.

- **Ada**: `cmd/formspec/seed.go` — verb `formspec seed [--spec] [--dsn] [--module]`,
  format `kind: Seed` + `spec.entities[].records[]`, idempoten (skip bila natural
  key sudah ada), insert lewat `EntityStore` (jadi natural key, default, validasi
  berlaku).
- **Tidak ada**: `Seed` di `pkg/spec` (`KindSeed` tidak ada) dan tidak di
  `internal/manifest.KnownKinds` (baris 309–326). Diuji langsung:

  ```
  $ formspec validate --spec /tmp/seedtest/spec --schema schemas
  [FAIL] seed.yaml#0
         engine: unknown kind "Seed" for spec version v1
         schema: internal: read Seed.schema.json: open schemas/kinds/Seed.schema.json: no such file or directory
  ```

  Artinya: `formspec seed` bisa menjalankan file itu, tetapi `formspec validate`
  **menolaknya**, dan `schemas/kinds/Seed.schema.json` tidak ada. Tidak ada satu
  pun `kind: Seed` di repo (`grep "^kind: Seed"` → 0 hasil), dan
  `docs_internal/plan/todo.md` 9.4.1a mencatat hal ini untuk bagan akun GL.

**Rencana A — daftarkan kind Seed (prasyarat 10.3 & 10.4):**

| File | Perubahan |
| --- | --- |
| `pkg/spec/seed.go` (baru) | `SeedSpec{Entities []SeedEntity}`, `SeedEntity{Entity, Records}`, `ValidateSeedSpec` (nama entity non-kosong, records non-kosong, module wajib) |
| `pkg/spec/spec.go` | `KindSeed Kind = "Seed"` + masuk `AllKinds()` |
| `internal/manifest/loader.go` | `KnownKinds["Seed"] = true` + jalur validasi ringan (bukan Entity) |
| `internal/genjsonschema/kinds.go` | `{Kind: "Seed", SpecStruct: "SeedSpec"}` |
| `cmd/formspec/seed.go` | Buang `SeedSpec` lokal, pakai `spec.SeedSpec` (satu definisi, tidak dua) |
| `schemas/` | `make generate-schema` → `Seed.schema.json` + root schema |
| `docs/kind/` | `make generate-kind-docs` → entri Seed |

**Kenapa tidak dibiarkan seperti sekarang:** tanpa ini, seed kafe adalah file yang
**gagal `formspec validate`** — persis kelas masalah yang dilarang aturan ledger
kafe ("spec kafe tidak di-degradasi"). Dan 9.4.1a menuntut seeder bagan akun yang
sama bentuknya.

### B. Model biaya saat ini hanya punya satu jenis baris

- `recipe.lines[]` = `{line_no, ingredient_id, quantity, unit, note}` —
  **`ingredient_id` required**. Tidak ada tempat untuk baris biaya non-bahan.
- `purchase-order.lines[]` = `{line_no, ingredient_id(required), quantity, unit_cost, subtotal, received_quantity}` —
  juga hanya bahan; **tidak ada biaya kirim**.
- `stock-movement.unit_cost` = "biaya beku per satuan" dan `source: [purchase, sale, waste, opname, manual]`.
  Sumber `manual` **sudah ada** → landed cost bisa mengalir tanpa enum baru.
- `menu-cost.cost_per_portion` = Σ(qty resep × `stock-level.moving_avg_cost`) ÷ yield.
  Karena HPP berbasis moving-average bahan, menaikkan `unit_cost` pergerakan
  masuk **otomatis** menaikkan HPP — jadi landed cost tidak butuh kolom baru di
  `menu-cost`.

**Rencana B (10.1) — dua sisi:**

1. **Sisi resep (BOM)**: `recipe.lines[]` mendapat `line_type: ingredient|cost`
   (default `ingredient`), `ingredient_id` jadi **kondisional** (`required_when:
   line_type == 'ingredient'`), plus `cost_label` + `cost_amount` untuk
   `line_type: cost` — inilah "custom item biaya yg bisa diisi bebas" (mis.
   "gas & listrik per porsi", "kemasan takeaway").
   ⇒ `menu-cost.cost_per_portion` ikut menjumlahkan baris biaya.
2. **Sisi pembelian**: `purchase-order` mendapat baris biaya bebas
   (`line_type` sama) sehingga biaya kirim bisa dicatat pada PO; script
   `receive-goods` mengalokasikannya **proporsional** ke `stock-movement.unit_cost`
   tiap bahan (landed cost masuk HPP, bukan biaya terpisah). Alokasi proporsional
   dipilih karena moving-average harus tetap per-satuan bahan.

### C. `emit:` sudah ada, dan pola "publish saja" sudah punya preseden

- `TransitionDecl.Emit` ada (`pkg/spec/entity.go:1262`) dan **diwajibkan** lewat
  `UnmarshalYAML` (baris 1279 — pernah hilang senyap di custom unmarshaler, ada
  komentar peringatannya).
- `EventDecl` (`pkg/spec/entity.go:1495`) punya `publish.durable` + `deliver[]`.
- Preseden: `cafe-order` memancarkan `on_paid`/`on_cancel` durable; `gl`
  mendengarkannya lewat `kind: Subscription` (`gl/subscriptions/sales-to-journal.yaml`)
  tanpa `cafe-order` tahu apa-apa soal akuntansi.

**Rencana C (10.2):** `purchase-order` mendapat `events: [on_received, on_cancelled]`
(`publish: {durable: true}`, `deliver: [reliable_event]`) dan `emit:` pada dua
transisinya. **Tidak** ditambahkan Subscription di cafe-stock. Konsekuensi yang
harus dicatat jujur: hari ini `gl` **belum punya** `account` untuk utang/persediaan,
jadi tidak ada yang meng-consume event itu — itu memang keadaan yang diminta
pemilik ("nanti kalau modul GL dikerjakan"). Bukti bahwa kontraknya hidup: event
muncul di outbox, terverifikasi lewat test.

### D. Role: `grants` → permission lewat halaman, bukan per-entity

- `formspec.core.role` (`internal/auth/module/master/role/entity.yaml`) punya
  `name`, `app`, `module`, `description`, `grants` (json).
- `Grant` (`internal/auth/grant.go:16`) = `{page, actions[]|tabs[]}`; `Materializer`
  (`internal/auth/materialize.go:41`) memetakan **page ref** → permission.
  Tiga bentuk page ref diterima: halaman authored (`pos-workbench`), navigation
  kind (`kanban:order-board-kds`), dan **derived entity page** (`order-page`) —
  bentuk terakhir inilah yang paling berguna untuk role kafe.
- `ownerRolePermission` (`internal/auth/role.go:84`) memberi wildcard `*` untuk
  role owner; role biasa **wajib** punya grants agar punya permission.
- `user.roles` (json) + `user.permissions` (json) + `user.assignments` (json,
  `[{role, dimension, value}]`).

**Rencana D (10.3):** seed berisi role + user + assignment:

| Role | Cakupan | Grants (ringkas) |
| --- | --- | --- |
| `kasir` | `kafe-pos` | `pos-workbench` (create/update order, submit), `payment-page`, `shift-page` |
| `barista` | `kafe-kds` | `kanban:order-board-kds` (start-preparing/mark-ready/mark-served), `order-page` view |
| `dapur` | `kafe-kds` | sama dengan barista, tanpa bar |
| `pelayan` | `kafe-pos` | `order-page` view + mark-served |
| `supervisor` | `kafe-pos` | + `approval-inbox:supervisor-inbox` (approve/reject void) |
| `manajer` | `kafe-pos` | hampir penuh (laporan, master, stok) tanpa hapus |

Password user seed: `password` plaintext di record (hook `formspec.core.user.hash-password`
yang meng-hash) — **tetapi** seed insert lewat `EntityStore.Insert` langsung,
sehingga perlu diverifikasi apakah hook `before` dijalankan jalur itu; bila
**tidak**, seed harus menulis `password_hash` hasil `auth.HashPassword` (dan seed
tidak bisa berupa YAML murni). **Ini titik risiko pertama — diuji lebih dulu.**

### E. Gambar: internet tersedia, lisensi harus aman

- `curl` ke Wikipedia/Commons API **200** (terverifikasi 2026-09-22).
- Rencana: unduh dari **Wikimedia Commons** (lisensi CC-BY-SA/public domain),
  simpan ke `examples/kafe/assets/menu/*.jpg`, dan catat atribusi di
  `examples/kafe/assets/ATTRIBUTION.md`.
- Seed menyitir **path**, bukan URL (spec kafe tidak boleh bergantung jaringan).
  Format nilai field `file` = object key → seed harus memakai key storage yang
  benar; **titik risiko kedua** — diverifikasi dengan upload sungguhan atau
  dengan membaca `internal/api/file.go`.
- Ukuran ≤2 MB (`menu-item.photo.max_size_mb: 2`), resize bila perlu.

---

## Urutan pengerjaan (dependensi)

```
A. daftarkan kind Seed ──┬─► D. seed role + user + assignment (10.3)
                         └─► E. seed master + menu + gambar (10.4)
B. landed cost (10.1) ──┬─► C. purchase events (10.2)   [B dulu: PO berubah bentuk]
                        └─► menu-cost ikut menjumlah
F. sisa 10.5
G. verifikasi + changelog + todo
```

**Estimasi:** A small–medium · B medium · C small · D medium · E medium ·
F small · G small.

## Risiko & titik verifikasi lebih dulu

| # | Risiko | Cara memastikan |
| --- | --- | --- |
| 1 | Hook `before create` user (hash password) tidak jalan di jalur seed → password tidak bisa login | Baca `resource/formspec.go` + `EntityStore.Insert`; kalau tidak jalan, tulis hash via Go atau jalankan hook eksplisit di `seed.go` |
| 2 | Nilai field `file` tidak bisa diisi dari seed (butuh upload nyata) | Baca `internal/api/file.go`; kalau perlu, seed menulis key + file disalin ke storage root |
| 3 | Grants yang menunjuk page/tab yang tidak ada → `Materialize` **error** dan role dilewati senyap (`resolver.go:114`: `continue // malformed grant — skip role`) | Uji login nyata tiap role; jangan percaya "seed berhasil" saja |
| 4 | Alokasi landed cost proporsional bisa menghasilkan pembulatan yang tidak menjumlah tepat | Uji dengan angka yang tidak bulat; tetapkan aturan sisa (selisih ke baris terbesar) |
| 5 | `recipe.lines.ingredient_id` jadi kondisional — validator lama mungkin tetap menolak `required: true` + `required_when` | Uji `formspec validate` lebih dulu dengan bentuk minimal |

## Bukti yang harus ada saat item ditutup

- `formspec validate --spec examples/kafe/spec --schema schemas` → **0 problem**
- `formspec seed --spec examples/kafe/spec --dsn …` → semua inserted, idempoten
  pada run kedua (0 inserted, N skipped)
- Login nyata untuk **setiap** role seed → permission berbeda terlihat
  (endpoint yang ditolak role lain → 403)
- Pembelian dengan biaya kirim → `stock-level.moving_avg_cost` naik sesuai
  alokasi (angka sebelum/sesudah dicatat)
- `go test ./...` hijau · `make lint` 0 issues
- Changelog `docs_internal/changelog/2026-09-22-NNN-*.md` + update
  `docs_internal/plan/todo.md` + `examples/kafe/gaps_found/TODO.md` Fase 10
