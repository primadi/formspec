# Project Layout

**Version:** 0.1.0 · **Status:** Draft (§3–§6 adalah target desain, belum diimplementasikan — lihat §5, §6.5)

> Dokumen ini mengontrakkan struktur folder project aplikasi Forma di disk, dan
> bagaimana satu workspace dengan banyak Module bisa memiliki handler
> implementasi dalam bahasa yang berbeda-beda per Module.

---

## 1. Direktori Standar

```
myapp/
  forma-app.yaml                 # kind: Config — konfigurasi root (dsn, addr, dst.)
  apps/
    internal.yaml                # kind: App
    public.yaml                  # kind: App (App kedua, workspace yang sama)
  modules/
    billing/
      module.yaml                # kind: Module — termasuk deklarasi runtime (§3)
      documents/invoice.yaml      # kind: Entity
      services/tax-calculator.yaml
      scripts/invoice_send.star  # script Starlark
      assets/                    # custom UI component (asset escape hatch)
      impl/                      # kode handler Module ini — bahasa BEBAS per Module (§3)
        src/invoice-handler.ts   # mis. TypeScript, kalau module.yaml runtime: typescript
    pharmacy/
      module.yaml                # runtime: php
      documents/prescription.yaml
      impl/
        composer.json
        src/PrescriptionHandler.php
```

Nama folder adalah konvensi, bukan kontrak keras. Loader **wajib** menemukan
manifest dengan men-scan `*.yaml`, bukan berdasarkan path tetap — tidak ada
yang melarang workspace kecil menyimpan satu `forma.yaml` di root alih-alih
`apps/<name>.yaml`; struktur `apps/` direkomendasikan begitu workspace punya
lebih dari satu App.

## 2. Tiga Jenis File, Plus `impl/` Per Module

Satu-satunya aturan keras: tiga jenis file dalam manifest —`.yaml` (deskripsi),
`.star` (logika, Starlark), `assets/*` (statis/custom UI). `impl/` adalah kode
sumber build-time (Go native, atau bahasa lain untuk `impl.type: sidecar`) —
di-commit, tidak masuk artifact deployment. **`impl/` bersifat per-Module**,
bukan satu folder global di root — setiap Module membawa kode handlernya
sendiri, dalam bahasa yang ia deklarasikan sendiri (§3).

**Git adalah sumber kebenaran.** Manifest selalu berupa file teks di
repositori — tidak pernah format biner proprietary maupun state tersembunyi
di database. Tooling authoring (scaffold `forma new <kind>`, editor visual
di admin panel) **menulis kembali YAML ke file/PR**, bukan ke DB
tersembunyi; git tetap satu-satunya sumber kebenaran. Skema JSON per kind
memberi validasi/autocomplete editor (LSP), tapi tidak menggantikan file
sebagai artifact otoritatif.

## 3. Runtime Per Module

Satu workspace = multi App + multi Module (lihat
[`02-workspace-app-module.md`](02-workspace-app-module.md)), dan tiap Module
bisa dimiliki tim berbeda dengan preferensi bahasa berbeda. Karena itu **setiap
Module mendeklarasikan runtime implementasinya sendiri** — bukan satu runtime
global untuk seluruh project:

```yaml
# modules/billing/module.yaml
apiVersion: forma.dev/v1alpha1
kind: Module
metadata: { name: billing }
spec:
  runtime: typescript        # module ini dilayani proses Node terpisah
```

```yaml
# modules/pharmacy/module.yaml
apiVersion: forma.dev/v1alpha1
kind: Module
metadata: { name: pharmacy }
spec:
  runtime: php                # module ini dilayani proses PHP terpisah
```

Nilai `runtime` yang valid: `local` (default — native compiled-in, tanpa proses
sidecar), `typescript` (alias `node`), `php`, `python`, `go`, `java`, `dotnet`,
`ruby`, `rust` — satu per bahasa yang punya `lib-forma-*` (lihat
[`docs/runtimes/04-forma-sidecar.md`](../../runtimes/04-forma-sidecar.md) §4.4).
Action ber-`impl.type: sidecar` di dalam sebuah Module secara implisit dilayani
runtime yang dideklarasikan Module tersebut — tidak perlu diulang per-action.

## 4. Orkestrasi Multi-Proses

Engine (`forma dev` untuk dev, pod Resource Plane untuk produksi) **wajib**
menjalankan **satu proses sidecar per runtime unik** yang muncul di seluruh
Module workspace — bukan satu proses global untuk seluruh project:

```
Workspace dengan 2 Module, runtime berbeda:
  billing  → runtime: typescript
  pharmacy → runtime: php

Engine start:
  → proses Node untuk billing   (socket: {state-dir}/sidecar/billing.sock)
  → proses PHP untuk pharmacy   (socket: {state-dir}/sidecar/pharmacy.sock)
  → Module dengan runtime: local (atau tidak dideklarasikan) tetap native,
    tidak butuh proses sidecar sama sekali
```

**Kontrak dispatch:** `SidecarExecutor` (lihat
[`docs/runtimes/04-forma-sidecar.md`](../../runtimes/04-forma-sidecar.md)
§3.2/§4.2) me-resolve socket tujuan dari **Module pemilik action**, bukan dari
satu `--app-endpoint` global — rantai resolusi: `action → Module pemilik →
runtime Module → socket proses yang menjalankan runtime itu`. Dua Module
dengan runtime yang sama (mis. dua Module TypeScript) BOLEH berbagi satu
proses sidecar atau dipisah — keputusan ini bukan bagian kontrak, hanya
kebijakan implementasi engine.

## 5. Status Implementasi Hari Ini (Gap)

Model saat ini di `forma dev`/`forma-sidecar`
([`docs/runtimes/04-forma-sidecar.md`](../../runtimes/04-forma-sidecar.md)
§5–§6) hanya mendukung **satu runtime untuk seluruh project**, lewat flag
global `--runtime` (auto-detect dari file marker di root: `composer.json` →
php, `package.json` → node, dst.) dan satu proses app di satu socket. Model
multi-runtime-per-Module di §3–§4 dokumen ini adalah **target desain, belum
diimplementasikan**. Perubahan yang dibutuhkan sebelum ini berjalan:

1. Schema `kind: Module` bertambah field `spec.runtime`.
2. Engine (`forma dev` dan pod Resource Plane produksi) spawn N child process
   sesuai jumlah runtime unik, bukan 1 proses tunggal.
3. `SidecarExecutor` melakukan routing per-Module, bukan satu
   `--app-endpoint` global.

Dicatat normatif di sini — bukan sekadar wishlist — supaya keputusan desain
ini tidak hilang sebelum implementasinya menyusul di fase restrukturisasi
kode.

## 6. Module Lokal vs Vendor, Aktivasi, dan Shadow Copy

> Status: arah desain yang disepakati tim, belum diimplementasikan — lihat
> gap di §6.5. Detail diskusi ada di catatan kerja
> [`docs/technical-notes/Forma-Technical-Note-Module-Vendoring-Aktivasi.md`](../../technical-notes/Forma-Technical-Note-Module-Vendoring-Aktivasi.md).

### 6.1 Struktur Folder

```
project/
  forma.yaml               # kind: App — manifest.spec.modules jadi activation list
  forma.lock                # lockfile: source, versi, checksum, signature, trust_tier per module
  modules/                  # local, hand-authored — source of truth developer
    billing/
      module.yaml
      documents/invoice.yaml
  vendors/                  # eksternal, hasil `forma module install` — read-only
    stripe-connector/
      module.yaml
      *.yaml                # spec tetap terbuka (dibaca boot-time, bukan digenerate)
      impl/handler.so        # impl.compiled — compiled blob, bukan source, untuk vendor komersial
  overrides/                 # shadow copy — lihat §6.4
    stripe-connector/
      form.checkout-form.yaml
```

`vendors/` sengaja **read-only** — integritas checksum/signature
([`07-marketplace.md`](07-marketplace.md) §2 Trust Tier) dan jalur update
versi tetap aman kalau
developer tidak menyentuh isinya secara manual. Beda dari pola `vendor/` Go
standar: vendoring Go membungkus source dependency open-source apa adanya;
di Forma, spec (`*.yaml`) tetap terbuka sesuai filosofi CC0, tapi
implementasi vendor komersial didistribusikan sebagai compiled blob
(`impl.compiled`, [`../backend/01-core-basic.md`](../backend/01-core-basic.md)
§5) alih-alih `impl.native` (source Go di `impl/`), supaya `vendors/` yang
ikut ter-commit ke repo klien tidak membocorkan IP vendor
([`07-marketplace.md`](07-marketplace.md) §2).

**Resolusi module bersifat name-based, bukan path-based:** `forma-server`
scan `modules/**` dan `vendors/**` digabung jadi satu registry saat boot,
key-nya nama efektif module (alias kalau ada — lihat
[`02-workspace-app-module.md`](02-workspace-app-module.md) §2.1). Routing
HTTP, `depends`/`depends_on`, dan referensi menu di App tidak pernah encode
asal folder, hanya nama.

### 6.2 `forma.lock`
Mencatat, per module terinstal: `source` (mis.
`github.com/acme/billing-module` — identitas unik sesungguhnya, §2.1 di
atas), `version`, `checksum`, `signature`, dan `trust_tier`
([`07-marketplace.md`](07-marketplace.md) §2). Alias hasil resolusi
konflik nama juga dicatat di sini, mengikat ke `source` yang sama supaya
`forma module install` berikutnya untuk source yang sama tidak menghasilkan
alias baru. Format skema lengkap belum dituliskan — lihat §6.5.

### 6.3 Model Aktivasi: Default Nonaktif, Uncomment untuk Pakai
`forma module install` menulis entri **ter-comment** di `App.spec.modules`
([`02-workspace-app-module.md`](02-workspace-app-module.md) §3) di bawah
blok marker terstruktur `>>> forma:vendor ... <<< forma:vendor` — bukan
comment bebas developer. Ini yang memungkinkan `forma module install`
mengenali "blok ini milik saya" saat dipanggil ulang (update versi) tanpa
menabrak comment manual developer di sekitarnya. Developer cukup uncomment
entri yang mau dipakai — tidak perlu mengetik ulang nama/source. Flag
`forma module install <source> --use` menulis langsung entri ter-uncomment,
untuk kasus ingin langsung aktif tanpa dua langkah
([`07-marketplace.md`](07-marketplace.md) §3).

**Idempotensi saat re-install/update:** kalau developer sudah uncomment
(mengaktifkan) suatu module, `forma module install` untuk update versi
**tidak boleh** comment-balik blok yang sudah aktif — yang diperbarui
hanya versi di dalam marker (`@1.0.0` → `@1.1.0`) dan entri `forma.lock`
terkait. Status aktif/nonaktif adalah properti file yang dijaga, bukan
sesuatu yang di-generate ulang setiap kali install/update berjalan.

Model ini extend prinsip `depends`/`depends_on` yang sudah ada
([`02-workspace-app-module.md`](02-workspace-app-module.md) §2, §7) satu
level di atasnya — sekarang butuh **activation list** eksplisit (App
manifest, atau Module lain via `depends`) sebelum sebuah module bahkan bisa
jadi target dependency. Aktivasi adalah keputusan developer/project yang
eksplisit, bukan efek samping instalasi.

### 6.4 Kustomisasi Vendor Module Tanpa Edit Langsung (Shadow Copy)
`vendors/` read-only (§6.1), tapi kebutuhan riil developer tidak berhenti
di "pakai apa adanya" — sering perlu ubah layout form, caption, urutan
section, atau visibility field tertentu dari module vendor, tanpa
menyentuh file di `vendors/` sama sekali.

**Mekanisme: copy file spec asli, replace total saat boot.** Tidak ada
merge logic (bukan JSON merge patch). Kalau ada file dengan
module+kind+name yang sama di `overrides/`, dia **menggantikan total**
file asli dari `modules/`/`vendors/` saat boot. Trade-off yang disadari:
shadow copy tidak otomatis dapat perubahan aditif dari versi vendor
berikutnya (§6.4.2) — beda dari model patch yang menambah di atas versi
apa pun; dipilih karena lebih sederhana, tidak butuh semantik merge
(strategic merge, array-by-key, dst.) yang masih jadi open question.

Perintah khusus, bukan `cp` manual:

```bash
forma override adopt stripe-connector Form checkout-form
```

Meng-copy file + mencatat checksum spec asli sumbernya ke `forma.lock`
(sebagai "asal fork"). Copy manual pakai `cp` tidak meninggalkan jejak
checksum → tidak ada deteksi drift di §6.4.2.

**6.4.1 Whitelist per Kind (ditegakkan saat boot, bukan konvensi):**

| Kind | Boleh di-shadow-copy | Tidak boleh |
|---|---|---|
| `Form` | Layout (section, `columns`, grouping), caption/label, urutan field, visibility (`hidden`) | — (kind ini murni presentation) |
| `Menu`/`Navigation` (`App.spec.menu`) | Label, icon, urutan, visibility | — |
| Instance `VisualSpecKind` lain (Table/Kanban/Calendar/dst., [`../frontend/02-visual-spec-kind.md`](../frontend/02-visual-spec-kind.md)) | Kolom ditampilkan, default sort | — |
| `Entity` (termasuk `business_rules`/L4–L6 validation, [`../backend/02-core-extended.md`](../backend/02-core-extended.md) §14) | — (tidak ada jalur shadow-copy) | Semua — field/validasi tambahan pakai **Entity Extension** ([`../backend/03-entity-extension.md`](../backend/03-entity-extension.md)) |
| `Service`/`Workflow` | — (tidak ada jalur shadow-copy) | Semua — perilaku lintas-module pakai pola **Integrator** ([`../backend/02-core-extended.md`](../backend/02-core-extended.md) §5) |

Field-field yang boleh di-shadow-copy pada dasarnya presentation-layer
saja — sejalan dengan lingkup `kind: Form` yang memang didesain untuk
kustomisasi layout, bukan logic. Field yang memang tidak boleh pernah
tampil (bukan "ubah tampilannya", tapi "jangan tampil sama sekali") tidak
butuh shadow copy Form — cukup `exclude: [ui]` di level field
([`../backend/05-field-types.md`](../backend/05-field-types.md) §5.3).

**6.4.2 Deteksi Drift saat Vendor Update.** Setiap `forma module
install`/update, checksum base spec baru dibandingkan checksum "asal fork"
tercatat. Kalau beda → warning eksplisit saat boot (bukan hard-fail,
karena developer memang sudah sengaja ambil alih penuh file itu):

```
⚠ overrides/stripe-connector/form.checkout-form.yaml adalah shadow copy
  dari checkout-form versi 1.0.0 — vendor sudah rilis versi 2.1.0.
  Shadow copy Anda TIDAK otomatis dapat perubahan upstream.
  → forma override diff stripe-connector Form checkout-form
```

### 6.5 Status Implementasi Hari Ini (Gap)
Seluruh §6 adalah **target desain, belum diimplementasikan** — `vendors/`,
`overrides/`, `forma.lock`, marker aktivasi, dan `forma override
adopt|diff` tidak ada di `forma dev`/CLI hari ini
([`../../cli-tools/02-forma-cli.md`](../../cli-tools/02-forma-cli.md) §9
baru mencakup `forma module list|install|uninstall` dasar). Pertanyaan
terbuka yang masih perlu didalami sebelum implementasi (lihat catatan
kerja untuk detail):

- `vendors/` di-commit ke git, atau di-gitignore dan direstore murni dari
  `forma.lock` (pola `node_modules`/vendor PHP)?
- Mekanisme `forma verify` (cek checksum tree `vendors/` vs `forma.lock`,
  tolak build kalau ada modifikasi manual) belum dispesifikasikan detail
  teknisnya.
- Format persis entri `forma.lock` per module (§6.2) belum dituliskan
  skema lengkapnya.
- Bagaimana `forma module install` menangani bundle (satu source, banyak
  module) secara teknis — manifest bundle terpisah, atau `ModulePublish`
  boleh mendeklarasikan banyak module sekaligus?
- Apakah shadow-copy (§6.4) perlu dilacak versinya sendiri secara
  eksplisit di `forma.lock` (bukan cuma checksum "asal fork"), supaya
  `forma override diff` bisa tunjukkan riwayat?
- Apakah ada kebutuhan override di level tenant/runtime (bukan cuma level
  project/deploy) — mis. tenant admin ganti caption sendiri lewat admin
  panel? Kemungkinan sumbu terpisah mirip resolusi `ctx.config`
  ([`../backend/01-core-basic.md`](../backend/01-core-basic.md) §10),
  belum dibahas hubungannya dengan shadow copy.
- Apakah whitelist §6.4.1 perlu dideklarasikan vendor sendiri (module
  menandai field mana yang "override-safe"), atau cukup aturan generik per
  kind yang sama untuk semua vendor?

## 7. Referensi

| Dokumen | Isi |
|---|---|
| [`02-workspace-app-module.md`](02-workspace-app-module.md) | Model workspace/App/Module yang jadi dasar §3; alias saat konflik nama (§2.1) |
| [`07-marketplace.md`](07-marketplace.md) | Instalasi, trust tier, dan model aktivasi module vendor (§6 di atas) |
| [`../backend/03-entity-extension.md`](../backend/03-entity-extension.md) | Entity Extension — jalur aditif untuk field/validasi tambahan pada Entity vendor |
| [`docs/runtimes/04-forma-sidecar.md`](../../runtimes/04-forma-sidecar.md) | Protokol sidecar per proses (§4), mode eksekusi (§5), gap implementasi (§8) |
| [`docs/architecture/08-repo-structure.md`](../../architecture/08-repo-structure.md) | Bagaimana `sdk/*` dan `internal/sidecar` merealisasikan kontrak ini di kode |
