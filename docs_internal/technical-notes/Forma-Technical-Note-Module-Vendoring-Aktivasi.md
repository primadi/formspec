# FormSpec Technical Note: Module Vendoring, Konflik Nama, Aktivasi, Shadow Copy, dan Entity Extension

**Catatan internal — hasil diskusi tim, bukan bagian resmi FormSpec Core Spec**
**Status: arah desain yang disepakati, belum ditulis ke Core Basic/Extended Spec resmi.**

---

## 0. Latar Belakang

Diskusi ini dimulai dari pertanyaan praktis: bagaimana membedakan module lokal (ditulis sendiri) dari module eksternal (di-install dari registry/git), tanpa memaksa siapa pun yang mengonsumsi App — workspace, App author lain, admin panel — untuk tahu atau peduli asal-usul module tersebut. Tiga masalah konkret muncul begitu prinsip itu didalami:

1. Nama module yang di-install bisa bentrok satu sama lain, atau dengan module lokal.
2. Vendor sering mem-publish banyak module sekaligus (bundle) — tidak semua boleh otomatis aktif hanya karena ter-download.
3. Ada kesalahpahaman awal soal peran `formspec generate` yang perlu diluruskan sebelum poin 1 dan 2 bisa dijawab dengan benar.

Ketiganya saling terkait: mekanisme resolusi nama (poin 1) dan mekanisme aktivasi (poin 2) sama-sama terjadi di titik yang sama — boot-time spec loading — bukan di tahap compile/generate.

---

## 1. Koreksi Mendasar: FormSpec Bukan Code Generator dari Spec

Framing sebelumnya keliru menyebut proses sebagai "`formspec generate` membaca `*.resource.yaml` untuk menghasilkan tipe data, endpoint HTTP, dst." Yang benar:

- `formspec-server` **meng-interpretasi spec langsung saat boot** (metadata-driven — pola yang sama dengan Frappe membaca DocType JSON). Routing HTTP, admin panel, permission enforcement, dan CRUD dibaca dari spec di memory saat runtime, **bukan** hasil dari tahap compile YAML → kode Go yang dieksekusi.
- `formspec generate` hanya scaffolding — membuat skeleton file spec baru (mis. `formspec generate resource billing/order` menghasilkan template YAML kosong dengan field dasar terisi). Output-nya adalah spec untuk diedit developer, bukan implementasi yang dijalankan.

**Implikasi langsung ke resolusi module:** karena tidak ada tahap compile, resolusi nama module, alias, dan aktivasi semuanya adalah persoalan **boot-time**, bukan build-time. Tidak ada "step generate" di antara install dan runtime yang bisa dijadikan titik penyelesaian konflik.

> **Perlu diklarifikasi terpisah:** Section 29 Core Extended Spec (v0.1.4) — "Code Generation", memuat Entity type, Query builder, TS types, dll — perlu dipastikan statusnya sebagai **convenience opsional untuk Tier 2/3** (native handler developer yang mau type-safe stub, supaya tidak menulis manual), bukan mekanisme inti yang dipakai Tier 1 (Layer 0, spec-only). Kalimat di spec saat ini terbaca seperti "core mechanism". Ini bukan bagian dari diskusi ini, tapi harus diluruskan sebelum jadi sumber kebingungan lebih lanjut.

---

## 2. Struktur Folder: Local vs Vendor

```
project/
  formspec.yaml              # manifest: activation list ("uses:")
  formspec.lock              # lockfile: source, versi, checksum, signature, trust_tier
  modules/                # local, hand-authored — source of truth developer
    billing/
      billing.module.yaml
      order.resource.yaml
  vendors/                # external, hasil `formspec module install` — read-only
    stripe-connector/
      stripe-connector.module.yaml
      *.resource.yaml     # spec tetap terbuka (dibaca boot-time, bukan digenerate)
      handler.so          # impl native — compiled blob, bukan source, untuk vendor komersial
```

Resolusi module bersifat **name-based, bukan path-based**: `formspec-server` scan `modules/**` dan `vendors/**` digabung jadi satu registry saat boot, key-nya nama efektif module (lihat bagian 3). Ini yang membuat blending otomatis — routing HTTP, `depends_on`, dan referensi menu di App tidak pernah encode asal folder, hanya nama.

**Perbedaan penting dari pola `vendor/` Go standar:** vendoring Go membungkus source dependency open-source apa adanya. Di FormSpec, spec (`*.resource.yaml`) memang tetap terbuka sesuai filosofi CC0 — tapi implementasi `impl.native` milik vendor komersial didistribusikan sebagai compiled blob (`go_plugin`), bukan source, supaya `vendors/` yang ikut ter-commit ke repo klien tidak membocorkan IP vendor.

---

## 3. Konflik Nama Module

`metadata.name` yang ditulis pembuat module (mis. `billing`) **tidak dijamin unik secara global** — dua vendor berbeda bisa memilih nama yang sama. Identitas unik sebenarnya ada di source (`github.com/acme/billing-module`), dicatat di `formspec.lock`.

### 3.1 Alias Otomatis saat Konflik

Saat `formspec module install`, kalau nama efektif bentrok dengan module lain yang **sudah pernah ter-install** (aktif maupun belum), installer otomatis memberi alias:

```yaml
uses:
  - module: billing   # local

  # >>> formspec:vendor github.com/acme/billing-module @1.0.0
  # - source: github.com/acme/billing-module
  #   as: acme-billing
  # <<< formspec:vendor

  # >>> formspec:vendor github.com/other/billing-module @2.1.0
  # - source: github.com/other/billing-module
  #   as: other-billing
  # <<< formspec:vendor
```

### 3.2 Keputusan: Alias Dihitung saat Install, Bukan saat Aktivasi

Dua opsi dipertimbangkan:

- **Opsi A** — hitung bentrok hanya terhadap module yang sudah aktif (uncommented). Alias baru dihitung persis saat developer uncomment.
- **Opsi B** — hitung bentrok terhadap semua yang pernah ter-install, aktif maupun masih ter-comment. Alias sudah fixed sejak install.

**Disepakati: Opsi B.** Alasan: nama efektif module tidak boleh berubah tergantung urutan aktivasi developer — kalau dua vendor dengan nama sama sama-sama diaktifkan kapan pun di kemudian hari, tidak boleh ada surprise rename. Ini konsisten dengan prinsip yang sudah dipegang di tempat lain di spec (mis. gap-free guarantee `ctx.next_key` — nomor/nama tidak boleh berubah makna tergantung state runtime). Konsekuensinya: perhitungan alias terjadi di titik **install**, bukan di titik **uncomment**.

### 3.3 Enforcement saat Boot

`formspec-server` cek nama efektif (alias kalau ada, `metadata.name` kalau tidak) hanya untuk set module yang **aktif**. Bentrok di set aktif → refuse to boot dengan pesan jelas, minta alias manual. Module yang belum diaktifkan tidak pernah dicek — dua vendor module bernama sama boleh nangkring bersamaan di `vendors/` selama tidak dua-duanya aktif tanpa alias.

---

## 4. Model Aktivasi: Default Nonaktif, Uncomment untuk Pakai

### 4.1 Alur

- `formspec module install` → fetch ke `vendors/`, catat di `formspec.lock`, tulis entri **ter-comment** di `formspec.yaml` di bawah blok `uses:`. Tidak otomatis aktif.
- Kalau vendor publish bundle berisi banyak module sekaligus, semua ter-download ke `vendors/`, tapi hanya yang di-uncomment di `formspec.yaml` yang benar-benar diregister saat boot — sisanya diam di disk, tidak masuk permission graph, tidak kena license gate (Module tetap unit lisensi: tidak dipakai, tidak perlu bayar).
- Developer cukup buka comment pada entri yang mau dipakai — tidak perlu mengetik ulang nama module atau source.
- Flag eksplisit tetap tersedia untuk kasus ingin langsung aktif tanpa dua langkah: `formspec module install acme/billing --use` (langsung menulis entri ter-uncomment).

### 4.2 Format Marker — Wajib, Bukan Comment Bebas

Blok `>>> formspec:vendor ... <<< formspec:vendor` adalah marker terstruktur, bukan comment bebas developer. Ini yang memungkinkan `formspec module install` mengenali "blok ini milik saya" saat dipanggil ulang (update versi) tanpa menabrak comment manual developer di sekitarnya.

### 4.3 Idempotensi saat Re-install/Update

Kalau developer sudah uncomment (mengaktifkan) suatu module, lalu `formspec module install` dipanggil lagi untuk update versi, installer **tidak boleh** comment-balik blok yang sudah aktif. Yang di-update hanya:
- Versi di dalam marker (`@1.0.0` → `@1.1.0`).
- Entri terkait di `formspec.lock`.

Status aktif/nonaktif adalah properti file yang harus dijaga (preserved), bukan sesuatu yang di-generate ulang setiap kali install/update berjalan.

### 4.4 Konsistensi dengan Prinsip yang Sudah Ada

Model ini extend prinsip `depends_on` yang sudah ada di spec (cross-module coupling: entity read, service call, event subscribe) satu level di atasnya — sekarang butuh **activation list** eksplisit sebelum sebuah module bahkan bisa jadi target `depends_on`. Aktivasi adalah keputusan developer/project yang eksplisit, bukan efek samping instalasi — sejalan dengan prinsip "no self-approval" dan gate di control plane yang sudah dipegang di bagian lain arsitektur.

---

## 5. Kustomisasi Vendor Module Tanpa Edit Langsung (Shadow Copy)

`vendors/` sengaja read-only (bagian 2) supaya integritas checksum/signature terjaga dan update versi tetap aman. Tapi kebutuhan riil tidak berhenti di "pakai apa adanya" — developer sering perlu ubah layout form, caption, urutan section, atau sembunyikan field tertentu dari module vendor, tanpa menyentuh file di `vendors/` sama sekali.

**Revisi dari draf awal:** desain pertama mengusulkan kind `Override` berbasis merge-patch (JSON merge patch di atas base spec). Setelah didiskusikan, ini diganti dengan model **shadow copy full-replace** — lebih sederhana, tidak butuh semantik merge (strategic merge, array-by-key, dst yang masih jadi open question). Trade-off yang disadari: shadow copy tidak otomatis dapat perubahan aditif dari versi vendor berikutnya (lihat 5.3), beda dari model patch yang sifatnya menambah di atas versi apa pun.

### 5.1 Mekanisme: Copy File Spec Asli, Replace Total saat Boot

Tidak ada merge logic. Kalau ada file dengan module+kind+name yang sama di `overrides/`, dia **menggantikan total** file asli dari `modules/`/`vendors/` saat boot:

```
project/
  overrides/
    stripe-connector/
      form.checkout-form.yaml     # copy penuh, sudah diedit bebas isinya
```

### 5.2 Perintah Khusus — Bukan `cp` Manual

```bash
formspec override adopt stripe-connector Form checkout-form
```

Meng-copy file + mencatat checksum spec asli sumbernya ke `formspec.lock` (sebagai "asal fork"). Kalau developer copy manual pakai `cp`, tidak ada jejak checksum → tidak ada deteksi drift di 5.3. Jadi tetap disarankan lewat CLI ini.

### 5.3 Deteksi Drift saat Vendor Update

Setiap `formspec module install`/update, checksum base spec baru dibandingkan dengan checksum yang tercatat sebagai "asal fork". Kalau beda → warning eksplisit saat boot (bukan hard-fail, karena developer memang sudah sengaja ambil alih penuh file itu):

```
⚠ overrides/stripe-connector/form.checkout-form.yaml adalah shadow copy
  dari checkout-form versi 1.0.0 — vendor sudah rilis versi 2.1.0.
  Shadow copy Anda TIDAK otomatis dapat perubahan upstream.
  → formspec override diff stripe-connector Form checkout-form
    untuk lihat apa yang berubah di upstream.
```

### 5.4 Whitelist per Kind Tetap Berlaku

Shadow copy **tidak boleh** jadi celah untuk mengambil-alih logic vendor. Kalau bebas copy-paste file spec apa saja, developer bisa "shadow" sebuah `Entity` atau `BusinessRule` vendor dan diam-diam ubah validasi/logic — itu membatalkan jaminan contract-test/verified badge tanpa ada yang tahu. Mekanisme ini hanya boleh berlaku untuk kind yang di-whitelist, ditegakkan lewat pengecekan `kind:` saat boot — bukan sekadar konvensi:

| Kind | Boleh di-shadow-copy | Tidak boleh |
|---|---|---|
| `Form` | Layout (section, `columns`, grouping), caption/label, urutan field, visibility (`hidden`) | — (seluruh file boleh diganti, tapi kind ini murni presentation) |
| `Menu`/`Navigation` | Label, icon, urutan, visibility | — |
| `ViewKind` | Kolom yang ditampilkan di list, default sort | — |
| `Entity` | — (tidak ada jalur shadow-copy) | Semua — kalau butuh field tambahan atau validasi tambahan, pakai **Entity Extension** (bagian 6) |
| `BusinessRule`/`BusinessService` | — (tidak ada jalur shadow-copy) | Semua — kalau butuh ubah perilaku lintas-module, pakai pola **Integrator** yang sudah ada |

Field-field ini pada dasarnya presentation-layer saja, sejalan dengan lingkup "Layer 1 Form kind" yang memang didesain untuk kustomisasi layout, bukan logic.

### 5.5 Tidak Wajib untuk Field yang Memang Tidak Terlihat

Kalau kebutuhan cuma "field ini jangan tampil sama sekali" (bukan "ubah tampilannya"), shadow copy Form **tidak diperlukan** — lihat 6.3, cukup atribut `exclude: [ui]` di level field itu sendiri.

---

## 6. Entity Extension: Tambah Field & Validasi

Kebutuhan paling umum saat pakai vendor module — tambah field, tambah rule validasi — **bukan** kasus untuk shadow copy di atas. Ini mekanisme terpisah, sifatnya aditif, bukan replace.

### 6.1 Kind `Extension`

```yaml
apiVersion: formspec/v1
kind: Extension
metadata:
  target:
    module: billing
    entity: invoice
  name: shipping-info        # nama extension — jadi namespace

spec:
  fields:
    - name: shipping_method
      type: enum
      enum_values: [regular, express]
      rules:
        - required
    - name: shipping_note
      type: string

  validate:
    - script: |
        def validate(resource, params, ctx):
          if resource.ext.shipping_info.shipping_method == "express" \
             and resource.total < 100000:
            return fail("Express shipping butuh minimum order")
          return ok()
      on: [create, update]
```

### 6.2 Kenapa Ini Berbeda dari Shadow Copy

- **Storage** — setiap Extension bikin kolom JSONB baru di level top (`ext_shipping_info`), bukan nested di dalam `data` entity asli. Uninstall = `ALTER TABLE DROP COLUMN`, bersih, tanpa join tambahan ke query normal. Semua extension pada entity yang sama tetap flat sebagai siblings — tidak ada extend-of-extend.
- **Akses field dinamespace** — `resource.ext.shipping_info.xxx`, bukan langsung `resource.xxx`. Mencegah tabrakan nama antar dua Extension berbeda pada entity yang sama.
- **Validasi bersifat tambahan, bukan pengganti** — `validate:` di Extension tidak bisa mengubah/menghapus validasi bawaan entity asli. Dia hanya menambah pemeriksaan baru yang boleh membaca field asli (read-only) tapi hanya berhak menuntut field miliknya sendiri. Urutan eksekusi: validasi entity asli jalan dulu (kontrak module asal tetap utuh), baru validasi Extension — pola yang sama dengan priority handler yang sudah ada di Document Model, bukan mekanisme baru.
- **Tidak menyentuh `vendors/` sama sekali** — murni menambah artifact baru di sisi, tidak ada "versi mana yang dipakai" untuk dilacak driftnya seperti shadow copy.

### 6.3 Visibility Default — Tidak Wajib Shadow Copy Form

Field Extension otomatis ikut Layer 0 (spec-only, auto-generated CRUD) — begitu Extension aktif, field-nya otomatis muncul di form/list yang di-generate, dikelompokkan default di section bernama sesuai `metadata.name` Extension (mis. "Shipping Info"), tanpa developer perlu bikin apa pun.

Dua kasus turunan:

- **Mau tampil tapi beda posisi/caption dari default** → shadow copy Form (bagian 5), opsional.
- **Memang tidak boleh pernah terlihat** (internal/computed/API-only) → bukan urusan Form sama sekali, cukup atribut di field spec Extension itu sendiri:

```yaml
fields:
  - name: internal_sync_token
    type: string
    exclude: [ui]        # perluasan dari exclude: [public_api, audit_log, webhook]
                          # yang sudah ada di Advanced Field Types (Extended Spec §2.4)
```

**Konsekuensi:** `exclude: [ui]` diusulkan sebagai perluasan resmi ke `Advanced Field Types` (bukan konsep khusus Extension) — berlaku juga untuk field bawaan entity biasa, bukan cuma field Extension.

---

## 7. Ringkasan Keputusan

| # | Topik | Keputusan |
|---|---|---|
| D-a | Peran `formspec generate` | Scaffolding spec saja (template kosong). Tidak ada codegen dari spec ke kode yang dieksekusi — `formspec-server` interpretasi spec langsung saat boot. |
| D-b | Struktur folder | `modules/` (lokal, hand-authored) vs `vendors/` (eksternal, hasil install, read-only). Resolusi module name-based, bukan path-based. |
| D-c | Distribusi impl. vendor komersial | Spec (`*.resource.yaml`) tetap terbuka; `impl.native` didistribusikan sebagai compiled blob (`go_plugin`), bukan source — proteksi IP vendor. |
| D-d | Identitas unik module | Bukan `metadata.name`, tapi source (`github.com/...`) dicatat di `formspec.lock`. `metadata.name` boleh bentrok antar vendor. |
| D-e | Titik hitung alias saat bentrok | Saat **install**, terhadap semua module yang pernah ter-install (aktif maupun belum) — bukan saat uncomment/aktivasi. |
| D-f | Model aktivasi | Default nonaktif. `formspec module install` menulis entri ter-comment berformat marker di `formspec.yaml`; uncomment untuk aktifkan. Flag `--use` untuk langsung aktif. |
| D-g | Idempotensi update | Re-install/update tidak boleh mengubah status aktif/nonaktif blok yang sudah ada — hanya update versi di marker dan `formspec.lock`. |
| D-h | Kustomisasi vendor module (presentation) | **Shadow copy full-replace** (bukan merge-patch) — file di-copy ke `overrides/` via `formspec override adopt`, checksum asal dicatat di `formspec.lock` untuk deteksi drift saat vendor update. Hanya berlaku untuk kind whitelist (`Form`, `Menu`/`Navigation`, `ViewKind`) — `Entity` dan `BusinessRule`/`BusinessService` tidak punya jalur shadow-copy. |
| D-i | Tambah field/validasi ke entity vendor | Kind `Extension` terpisah, aditif (bukan replace) — kolom JSONB baru per extension (`ext_{name}`), akses dinamespace (`resource.ext.{name}.field`), `validate:` hanya boleh menambah pemeriksaan baru, tidak bisa override validasi bawaan. Field Extension otomatis muncul di Layer 0 tanpa perlu shadow copy Form. |
| D-j | Field yang tidak boleh terlihat visual | `exclude: [ui]` di level field (perluasan dari `exclude: [public_api, audit_log, webhook]` yang sudah ada) — berlaku untuk field Extension maupun field entity biasa, bukan lewat shadow copy Form. |

---

## 8. Pertanyaan Terbuka untuk Iterasi Berikutnya

- Apakah `vendors/` di-commit ke git, atau di-gitignore dan direstore murni dari `formspec.lock` (pola node_modules/vendor PHP)? Belum diputuskan — tergantung ukuran blob compiled dan preferensi soal reproducible air-gapped build.
- Mekanisme `formspec verify` (cek checksum tree `vendors/` vs `formspec.lock`, tolak build kalau ada modifikasi manual) belum dispesifikasikan detail teknisnya.
- Format persis entri `formspec.lock` per module (field checksum, signature, trust_tier — merujuk ke Trust Tier model yang sudah ada) belum dituliskan skema lengkapnya.
- Bagaimana `formspec module install` menangani bundle (satu source, banyak module) secara teknis — apakah satu manifest bundle terpisah dari `ModulePublish`, atau `ModulePublish` sendiri boleh mendeklarasikan banyak module sekaligus?
- Perlu diklarifikasi status Section 29 Core Extended Spec (Code Generation) — convenience Tier 2/3 vs core mechanism (lihat catatan di bagian 1).
- Apakah `Override`/shadow-copy perlu dilacak versinya sendiri secara eksplisit di `formspec.lock` (bukan cuma checksum "asal fork"), supaya `formspec override diff` bisa tunjukkan riwayat, bukan cuma versi terbaru vs versi asal?
- Apakah ada kebutuhan override di level tenant/runtime (bukan cuma di level project/deploy) — mis. tenant admin ganti caption sendiri lewat admin panel? Ini kemungkinan sumbu terpisah, mirip resolusi `ctx.config` (bootstrap > tenant > global > default), belum dibahas hubungannya dengan shadow copy di sini.
- Apakah whitelist permukaan shadow-copy di bagian 5.4 perlu dideklarasikan oleh vendor sendiri (module bisa menandai field mana yang "override-safe"), atau cukup aturan generik per kind yang berlaku sama untuk semua vendor?
- Format `formspec.lock` untuk Extension — bagaimana relasi antara Extension dan module target dicatat (Extension bisa jadi milik project sendiri, atau dipublish sebagai module terpisah yang extend module vendor lain)?
- Batasan `validate:` di Extension — apakah boleh membaca hasil business_rules milik entity asli (mis. status setelah validasi bawaan lolos), atau harus benar-benar independen dan hanya baca raw field?
- Apakah `exclude: [ui]` (D-j) butuh granularitas lebih detail (exclude dari form saja vs list saja), atau cukup satu flag untuk semua permukaan UI?

---

*Dokumen ini adalah catatan kerja, bukan keputusan final untuk semua detail. Keputusan D-a sampai D-j di bagian 7 sudah disepakati sebagai arah desain; detail teknis di bagian 8 masih perlu didalami sebelum masuk ke Core Basic/Extended Spec resmi.*
