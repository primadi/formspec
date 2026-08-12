# Spec Compatibility Notes — Hasil Test Drive Examples

**Tanggal:** 2026-07-03
**Spec target:** FormSpec Core Basic v0.2.0 + Core Extended stub (`docs/spec/formspec-core-extended-stub.md`)
**Examples yang diuji:** Customer, Midtrans PG, General Ledger, Inventory, Order-to-Cash

---

## Ringkasan

Semua 5 example **berhasil diekspresikan tanpa konstruksi di luar spec**. Tidak ditemukan gap yang mencegah implementasi. Namun ada beberapa catatan DX dan potensi penyempurnaan.

---

## 1. Gap yang Sudah Diketahui (sudah ada solusi)

| # | Gap | Status | Solusi |
|---|---|---|---|
| G1 | `idempotent: true` — semantik mekanis belum ada | ✅ SELESAI (D32/§11.8) | Framework enforce idempotency store + response replay |
| G2 | Cross-resource validation (blacklist check) — tidak ada di Core Basic | ✅ ACKNOWLEDGED | Rumahnya `conditions` + script; Extended untuk level 4-6 |
| G3 | Cross-module target `deliver.reliable_event` perlu fully qualified | ✅ SELESAI | `resource: gl.journal-entry` format |
| G4 | Webhook signature verification — tidak ada di Core Basic | ✅ Extended | `kind: Webhook` di Core Extended stub |
| G5 | `kind: Subscription` — reaksi dari luar tanpa sentuh source | ✅ SELESAI (D35/§12.5) | Terbukti di O2C §3.7 |
| G6 | HTTP client di Starlark — tidak tersedia | ✅ ACKNOWLEDGED | Harus `impl: native` untuk HTTP call keluar |

---

## 2. Temuan Baru dari Proses Generate

### T1: `impl/` belum punya rumah di spec §5 (Project Layout)

Spec §5 hanya menyebutkan tiga file types (`.yaml`, `.star`, `assets/*`) dan tidak menyebutkan letak Go source code untuk `impl: native`.

**Status:** ✅ SELESAI — §5.1 `impl/` directory ditambahkan ke Core Basic. `impl/` adalah build-time only, committed ke repo, excluded dari deployment artifact. Hasil kompilasi masuk `.formspec/build/`.

### T2: Konvensi `ref` di `impl: native` belum distandarisasi

Dalam examples, kita pakai `ref: "PaymentGateway.CreateSession"` — format `{Type}.{Method}`. Tapi tidak ada aturan bagaimana ref ini di-resolve ke file `.go`.

**Status:** ✅ SELESAI — §6.2 `ref` resolution ditambahkan ke Core Basic. Format `{TypeName}.{MethodName}`; framework scan `impl/**/*.go` untuk exported type+method; duplicate type names = compile error.

### T3: Duplikasi manifest antar example

Customer entity muncul di 2 tempat (Customer/spec dan O2C/spec). GL entity muncul di 2 tempat (GL/spec dan O2C/spec). Ini disengaja agar setiap example self-contained, tapi dalam praktik nyata, module seharusnya tidak diduplikasi.

**Dampak:** Di dunia nyata, module marketplace di-install sekali, bukan di-copy.

**Status:** Bukan gap spec, hanya artefak examples. Sudah dicatat di README masing-masing.

### T4: `kind: Module` tidak mendaftarkan isinya

Spec mengatakan "A Module is a package of manifests — identity, version, and dependencies only. It does NOT list its contents." Tapi saat install, footprint module harus ditampilkan ke user untuk consent.

**Dampak:** Perlu tooling (`formspec module footprint`) untuk mengekstrak footprint dari scanning manifests. Tidak ada di spec.

**Usulan:** Tambahkan section "Module Footprint" atau command reference `formspec module describe`.

### T5: `child: { storage: table }` vs `jsonb` — kapan pakai yang mana?

Spec §10.3 menjelaskan child spec dengan tabel perbandingan teknis tapi tanpa decision guidance.

**Status:** ✅ SELESAI — §10.3 ditambahkan "When to use `jsonb` vs `table`" dengan rule of thumb: jsonb untuk item sedikit (<100), table untuk item banyak atau butuh query independen.

---

## 3. Temuan DX (Developer Experience)

### DX1: Struktur folder intuitif

Struktur `spec/modules/<name>/entities/`, `spec/modules/<name>/services/`, dll. terasa **natural** setelah pattern-nya dipahami. Mirip dengan struktur project pada umumnya.

**Skor DX:** ⭐⭐⭐⭐ (4/5) — perlu dokumentasi mapping antara `kind` dan subfolder.

### DX2: Banyak boilerplate di YAML

Setiap Entity butuh 4 top-level keys (`apiVersion`, `kind`, `metadata`, `spec`). Untuk 20+ entity di satu App, ini repetitif.

**Skor DX:** ⭐⭐⭐ (3/5) — wajar untuk deklaratif, tapi mungkin butuh scaffolding tool.

**Usulan:** `formspec generate entity customer --module billing` untuk generate boilerplate.

### DX3: Permission auto-prefix sangat membantu

`required_permission: orders.checkout` → otomatis jadi `billing.orders.checkout`. Tidak perlu menulis namespace berulang-ulang.

**Skor DX:** ⭐⭐⭐⭐⭐ (5/5)

### DX4: Script `.star` vs Go `impl/` — dua mindset

Developer harus paham kapan pakai Starlark (simple, hot-updatable) vs Go (performa, network, kompleks). Ini dual mindset yang bisa membingungkan.

**Status:** ✅ SELESAI — §6.3 "Choosing `script_ref` vs `native`" ditambahkan sebagai non-normative guidance. Litmus test: hanya baca/tulis field resource sendiri atau panggil resource FormSpec lain → `script_ref`; butuh network/filesystem/library eksternal → `native`.

### DX5: `deliver` blok sangat powerful

Blok `deliver` di event menjadi **peta lengkap konsekuensi** — 4 kelas jaminan berbeda dalam satu tempat. Handler tidak perlu memanggil job/generate event secara manual.

**Skor DX:** ⭐⭐⭐⭐⭐ (5/5)

---

## 4. Rekomendasi untuk Spec v0.3.0

| # | Rekomendasi | Prioritas | Status |
|---|---|---|---|
| R1 | Tambahkan `impl/` directory di §5 Project Layout | HIGH | ✅ Done (§5.1) |
| R2 | Standarisasi mekanisme resolusi `ref` di `impl: native` | HIGH | ✅ Done (§6.2) |
| R3 | Tambahkan `formspec module footprint` atau command serupa | MEDIUM | Tunda — sudah cukup di §4.5 prose |
| R4 | Tambah decision tree `child: storage` (jsonb vs table) | MEDIUM | ✅ Done (§10.3) |
| R5 | Tambah decision matrix `.star` vs Go `impl/` | MEDIUM | ✅ Done (§6.3, non-normative) |
| R6 | Scaffolding command: `formspec generate entity/module/service` | LOW | Tunda — nice-to-have |
| R7 | Global `.devcontainer/` untuk DX2 testing | TUNDA | Menunggu kejelasan DX2 |

---

## 5. Kesimpulan

**Spec v0.2.0 + Extended stub LULUS test drive.** Semua 5 example berhasil diekspresikan. 4 dari 6 temuan sudah di-address di spec:

| Temuan | Klasifikasi | Section | Status |
|---|---|---|---|
| G1-G6 | Sudah diketahui | — | ✅ dari awal |
| T1: `impl/` directory | **Basic** | §5.1 | ✅ addressed |
| T2: `ref` resolution | **Basic** | §6.2 | ✅ addressed |
| T5: `child` storage guidance | **Basic** | §10.3 | ✅ addressed |
| DX4: `.star` vs Go | **Basic** (non-norm) | §6.3 | ✅ addressed |
| T3: Duplikasi manifest | Artefak examples | — | By design |
| T4: Module footprint | **Basic** | §4.5 prose | Cukup (sudah ada) |

Tidak ada gap yang perlu masuk ke Extended. Semua temuan memiliki rumah di Core Basic.
