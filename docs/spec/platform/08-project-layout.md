# Project Layout

**Version:** 0.2.0 · **Status:** Draft (§1–§2 menggambarkan layout yang
terimplementasi — lihat contoh `examples/Clinic-UI-Showcase/`; §3–§6 adalah
target desain, belum diimplementasikan — lihat §5, §6.5)

> Dokumen ini mengontrakkan struktur folder project aplikasi FormSpec di disk, dan
> bagaimana satu workspace dengan banyak App/Module menata manifest serta kode
> handler-nya. Contoh kanonik layout ini hidup di
> [`examples/Clinic-UI-Showcase/`](../../../examples/Clinic-UI-Showcase/) —
> `spec/`, `app/`, dan `formspec-app.yaml` di sana adalah referensi utama untuk
> section §1–§2.

---

## 1. Direktori Standar

Layout berikut adalah struktur yang dipakai contoh resmi
[`examples/Clinic-UI-Showcase/spec/`](../../../examples/Clinic-UI-Showcase/spec/)
— satu workspace, dua App (`klinik-internal`, `klinik-portal`), dua Module
(`clinic`, `pharmacy`), satu runtime sidecar (`app/`):

```
klinik-sehat/
  formspec-app.yaml               # config dev/serve — BUKAN manifest (lihat §1.1)
  spec/                        # container semua manifest — lokasi bisa diubah (§1.2)
    apps/
      klinik-internal.yaml     # kind: App — staf klinik, akses penuh ke dua module
      klinik-portal.yaml       # kind: App — portal publik, mount module yang SAMA
    modules/
      clinic/
        module.yaml            # kind: Module — termasuk spec.menu (menu default, §2.2)
        master/                #  → folder entity ber-characteristic: master
          patient/
            entity.yaml        #    kind: Entity
            forms/             #    UI yang melekat ke entity ini (§2.2)
            pages/
            tables/
          doctor/
            entity.yaml
            tables/
          polyclinic/
            entity.yaml
        transaction/           #  → folder entity ber-characteristic: transaction
          visit/
            entity.yaml
            forms/
            kanbans/
            pages/
            prints/
            scripts/           #    *.star colocated dengan entity
            tables/
            widgets/
            wizards/
        reference/             #  → folder entity ber-characteristic: reference
          setting/
            entity.yaml
            forms/
            pages/
        summary/               #  → folder entity ber-characteristic: summary
          daily-visit-summary/
            entity.yaml
            widgets/
        config/                # kind: Config — konfigurasi module (bukan per-entity)
          clinic.yaml
        dashboards/            # kind: Dashboard — level module, bukan per-entity
          clinic-dashboard.yaml
        pages/                 # kind: Page komposit lintas-entity (data-master)
          data-master.yaml
        reports/               # kind: Report — level module
          revenue-by-polyclinic.yaml
        themes/                # kind: Theme — level module
          showcase-theme.yaml
      pharmacy/
        module.yaml
        master/
          medicine/
        transaction/
          otc-sale/
          prescription/
  app/                         # kode handler sidecar (satu proses per runtime) — §2.3
    package.json
    src/
      app.ts                   # entrypoint (app-entrypoint di formspec-app.yaml)
      handlers/otc_sell.ts
  .formspec/                      # runtime state (sqlite, pid) — gitignored, bukan source of truth
```

Pola di atas adalah **konvensi, bukan kontrak keras** — loader wajib menemukan
manifest dengan men-scan `*.yaml` secara rekursif (§1.2), bukan berdasarkan
path tetap. Organisasi di sini adalah yang disarankan supaya project mudah
dibaca dan supaya konvensi penamaan (§2.2) bisa dipakai renderer untuk derive
UI.

### 1.1 `formspec-app.yaml` adalah Config Dev/Serve, Bukan `kind: Config`

File `formspec-app.yaml` di root (legacy: `formspec-sidecar.yaml`) adalah **config
untuk tooling** (`formspec dev`/`formspec serve`), diparse oleh CLI — bukan manifest
resource `kind: Config`. Isinya mengarahkan engine ke mana spec, datastore,
runtime, dan kode app berada:

```yaml
# formspec-app.yaml — contoh Clinic-UI-Showcase
spec: spec # lokasi container spec (§1.2)
dsn: sqlite:.formspec/clinic.db # datastore engine
# schema-registry: https://schemas.formspec.dev   # registry JSON Schema (default; override FORMSPEC_SCHEMA_REGISTRY)
runtime: node # runtime sidecar untuk impl.type: sidecar
app-dir: app # lokasi kode app sidecar (§2.3)
app-entrypoint: src/app.ts # entrypoint app sidecar
listen: unix_socket # socket server engine
app-endpoint: unix_socket # socket invoke sidecar
dev: true # mode dev (hot-reload spec, dst.)
themes: # direktori theme tambahan (bisa di luar spec/)
  - ../../ui-theme/batik-theme
```

`runtime` di sini masih **satu untuk seluruh project** (lihat §5) — model
multi-runtime per Module di §3–§4 tetap target desain.

### 1.2 Kontrak Loader: Zero Folder Assumption

`formspec` menemukan manifest dengan `filepath.Walk` rekursif dari root spec,
mengumpulkan semua `.yaml`/`.yml`. Direktori yang **di-skip**: folder hidden
(berawalan `.`), `node_modules`, dan `impl/` (kode build-time, bukan manifest).
Semua folder lain — termasuk `apps/`, `modules/`, dan subfolder entity — bebas
ditata; resolver action/script menyelesaikan referensi **relatif ke direktori
entity** (§2.2), bukan dari path tetap.

Konsekuensinya: workspace kecil boleh menyimpan satu App manifest di root spec
tanpa folder `apps/`, dan module tunggal boleh tanpa subfolder characteristic —
struktur `apps/` + `modules/` + characteristic direkomendasikan begitu
workspace tumbuh melewati satu App/satu entity.

## 2. Tiga Jenis File, Plus Kode Handler

Satu-satunya aturan keras: tiga jenis file dalam manifest — `.yaml`
(deskripsi), `.star` (logika, Starlark), dan `assets/*` (statis/custom UI).
Kode handler non-Starlark adalah tipe keempat, di luar manifest — lokasinya
di §2.3.

### 2.1 Tiga Jenis File dan Tempatnya

| Jenis            | Ekstensi       | Isi                                                    | Lokasi umum                                                 |
| ---------------- | -------------- | ------------------------------------------------------ | ----------------------------------------------------------- |
| Deskripsi        | `.yaml`/`.yml` | Seluruh manifest (App, Module, Entity, UI kinds, dst.) | `spec/apps/`, `spec/modules/<module>/**`                    |
| Logika           | `.star`        | Script Starlark untuk action/guard                     | `scripts/` di dalam folder entity (§2.2), atau level module |
| Statis/Custom UI | `assets/*`     | Aset biner + komponen UI custom (asset escape hatch)   | dalam module, di luar `spec/`                               |

**Git adalah sumber kebenaran.** Manifest selalu berupa file teks di
repositori — tidak pernah format biner proprietary maupun state tersembunyi
di database. Tooling authoring (scaffold `formspec new <kind>`, editor visual di
admin panel) **menulis kembali YAML ke file/PR**, bukan ke DB tersembunyi; git
tetap satu-satunya sumber kebenaran. Skema JSON per kind memberi
validasi/autocomplete editor (LSP), tapi tidak menggantikan file sebagai
artifact otoritatif.

### 2.2 Konvensi Folder Entity (entity-centric)

Di dalam `spec/modules/<module>/`, entity dikelompokkan per **characteristic**
(master/transaction/reference/summary), dan setiap entity adalah **satu
folder** bernama sama dengan entity-nya:

```
spec/modules/clinic/transaction/visit/
  entity.yaml         # kind: Entity — manifest entity (wajib bernama entity.yaml)
  forms/create.yaml   # kind: Form  — metadata.name: visit-create  (resolveForm mode create)
  forms/edit.yaml     # kind: Form  — metadata.name: visit-edit
  forms/quick.yaml    # kind: Form  — metadata.name: visit-quick (wizard step / quick-create)
  tables/list.yaml    # kind: Table — metadata.name: visit-table
  pages/list.yaml     # kind: Page  — metadata.name: visit-list
  kanbans/board.yaml  # kind: Kanban — metadata.name: consultation-board
  prints/queue-ticket.yaml   # kind: Print
  widgets/today.yaml  # kind: Widget
  wizards/registration.yaml  # kind: Wizard
  scripts/cancel.star # script colocated — ref cukup "cancel", resolve relatif ke folder entity
```

Konvensi penamaan (dikodifikasi sebagai rekomendasi; dipakai renderer untuk
derive UI dan resolve referensi):

1. **Manifest entity bernama `entity.yaml`** di dalam folder entity.
2. **Nama file UI = peran (role)**, bukan `{entity}-{kind}`: `create.yaml`,
   `edit.yaml`, `quick.yaml`, `list.yaml`, `board.yaml`, `detail.yaml`,
   `registration.yaml`, `card.yaml`, dst. — boleh diberi kata sifat
   (`quick-create.yaml`, `by-polyclinic.yaml`).
3. **`metadata.name` membawa nama resolve** = `{entity}-{role}` (mis.
   `visit-create`, `visit-table`, `patient-detail`, `patient-card`). Pola
   `{entity}-create`/`{entity}-edit` inilah yang dipakai `resolveForm()` untuk
   memilih form per mode — authoring memakai nama file pendek, resolver memakai
   `metadata.name` penuh.
4. **Script direferensikan dengan nama file saja** (mis. `ref: cancel`) dan
   di-resolve **relatif ke direktori entity** (folder `scripts/` di dalam
   entity didahulukan) — bukan path penuh dari spec root.

Entity yang **tidak punya manifest UI apa pun** sengaja sah: renderer
men-derive Table, Form, detail Page, dan menu entry-nya (derived by default —
[`03-kind-system.md`](03-kind-system.md)). Contoh `clinic/polyclinic` dan
`clinic/daily-visit-summary` menguji jalur derived ini.

Kind **level module** (tidak melekat ke satu entity) diletakkan langsung di
bawah folder module, bukan di dalam folder entity: `config/`, `dashboards/`,
`pages/` (page komposit seperti data-master), `reports/`, `themes/`.

> Menu default sebuah Module kini dideklarasikan di `module.yaml → spec.menu`
> — **tidak ada `kind: Menu` standalone**; sudah dilebur ke `App.spec.menu`
> (otoritatif) dan `Module.spec.menu` (saran default), lihat
> [`02-workspace-app-module.md`](02-workspace-app-module.md) §4. App mengadopsi
> menu default module lewat entri `menu: [{type: module, module: ...}]`
> (`spec/apps/*.yaml`).

### 2.3 Lokasi Kode Handler: `app/` Root vs `impl/` per Module

Handler non-Starlark (`impl.type: native` atau `impl.type: sidecar`) hidup di
kode sumber build-time — di-commit, tidak masuk artifact deployment. Dua pola
lokasi:

- **`app/` di root project (model yang dipakai contoh sekarang).** Satu folder
  app sidecar per workspace, ditunjuk lewat `app-dir`/`app-entrypoint` di
  `formspec-app.yaml` (§1.1). Cocok untuk model saat ini yang masih **satu runtime
  per project** (§5) — seluruh handler sidecar satu bahasa di satu proses.
- **`impl/` per Module (native Go, dan target multi-runtime).** Setiap Module
  membawa kodenya sendiri dalam bahasa yang ia deklarasikan (§3). Loader
  **men-skip `impl/`** saat scan manifest (§1.2) — folder ini murni untuk
  kompiler/build, bukan dibaca sebagai spec. Contoh yang memakai pola ini:
  `examples/Midtrans-Payment-Gateway/impl/billing/`.

Kedua pola sah hari ini; `impl/` per Module adalah arah yang dikontrakkan di
§3–§4 ketika multi-runtime diimplementasikan.

## 3. Runtime Per Module

Satu workspace = multi App + multi Module (lihat
[`02-workspace-app-module.md`](02-workspace-app-module.md)), dan tiap Module
bisa dimiliki tim berbeda dengan preferensi bahasa berbeda. Karena itu **setiap
Module mendeklarasikan runtime implementasinya sendiri** — bukan satu runtime
global untuk seluruh project:

```yaml
# modules/billing/module.yaml
apiVersion: formspec.dev/v1
kind: Module
metadata: { name: billing }
spec:
  runtime: typescript # module ini dilayani proses Node terpisah
```

```yaml
# modules/pharmacy/module.yaml
apiVersion: formspec.dev/v1
kind: Module
metadata: { name: pharmacy }
spec:
  runtime: php # module ini dilayani proses PHP terpisah
```

Nilai `runtime` yang valid: `local` (default — native compiled-in, tanpa proses
sidecar), `typescript` (alias `node`), `php`, `python`, `go`, `java`, `dotnet`,
`ruby`, `rust` — satu per bahasa yang punya `lib-formspec-*` (lihat
[`docs/runtimes/04-formspec-sidecar.md`](../../runtimes/04-formspec-sidecar.md) §4.4).
Action ber-`impl.type: sidecar` di dalam sebuah Module secara implisit dilayani
runtime yang dideklarasikan Module tersebut — tidak perlu diulang per-action.

## 4. Orkestrasi Multi-Proses

Engine (`formspec dev` untuk dev, pod Resource Plane untuk produksi) **wajib**
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
[`docs/runtimes/04-formspec-sidecar.md`](../../runtimes/04-formspec-sidecar.md)
§3.2/§4.2) me-resolve socket tujuan dari **Module pemilik action**, bukan dari
satu `--app-endpoint` global — rantai resolusi: `action → Module pemilik →
runtime Module → socket proses yang menjalankan runtime itu`. Dua Module
dengan runtime yang sama (mis. dua Module TypeScript) BOLEH berbagi satu
proses sidecar atau dipisah — keputusan ini bukan bagian kontrak, hanya
kebijakan implementasi engine.

## 5. Status Implementasi Hari Ini (Gap)

Model saat ini di `formspec dev`/`formspec-sidecar`
([`docs/runtimes/04-formspec-sidecar.md`](../../runtimes/04-formspec-sidecar.md)
§5–§6) hanya mendukung **satu runtime untuk seluruh project** — ditetapkan
lewat `runtime:` di `formspec-app.yaml` (§1.1) atau flag global `--runtime`
(auto-detect dari file marker di root: `composer.json` → php, `package.json` →
node, dst.), dengan satu proses app di satu socket. Contoh
[`examples/Clinic-UI-Showcase/`](../../../examples/Clinic-UI-Showcase/) persis
menjalankan model ini: satu `runtime: node`, satu `app/` sidecar, socket
`unix_socket`. Model multi-runtime-per-Module di §3–§4 dokumen ini adalah
**target desain, belum diimplementasikan**. Perubahan yang dibutuhkan sebelum
ini berjalan:

1. Schema `kind: Module` bertambah field `spec.runtime`.
2. Engine (`formspec dev` dan pod Resource Plane produksi) spawn N child process
   sesuai jumlah runtime unik, bukan 1 proses tunggal.
3. `SidecarExecutor` melakukan routing per-Module, bukan satu
   `--app-endpoint` global.

Dicatat normatif di sini — bukan sekadar wishlist — supaya keputusan desain
ini tidak hilang sebelum implementasinya menyusul di fase restrukturisasi
kode.

## 6. Module Lokal vs Vendor, Aktivasi, dan Shadow Copy

> Status: arah desain yang disepakati tim, belum diimplementasikan — lihat
> gap di §6.5. Detail diskusi ada di catatan kerja
> [`docs/technical-notes/FormSpec-Technical-Note-Module-Vendoring-Aktivasi.md`](../../technical-notes/FormSpec-Technical-Note-Module-Vendoring-Aktivasi.md).

### 6.1 Struktur Folder

```
project/
  formspec.yaml               # kind: App — manifest.spec.modules jadi activation list
  formspec.lock                # lockfile: source, versi, checksum, signature, trust_tier per module
  modules/                  # local, hand-authored — source of truth developer
    billing/
      module.yaml
      documents/invoice.yaml
  vendors/                  # eksternal, hasil `formspec module install` — read-only
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
di FormSpec, spec (`*.yaml`) tetap terbuka sesuai filosofi CC0, tapi
implementasi vendor komersial didistribusikan sebagai compiled blob
(`impl.compiled`, [`../backend/01-core-basic.md`](../backend/01-core-basic.md)
§5) alih-alih `impl.native` (source Go di `impl/`), supaya `vendors/` yang
ikut ter-commit ke repo klien tidak membocorkan IP vendor
([`07-marketplace.md`](07-marketplace.md) §2).

**Resolusi module bersifat name-based, bukan path-based:** `formspec-server`
scan `modules/**` dan `vendors/**` digabung jadi satu registry saat boot,
key-nya nama efektif module (alias kalau ada — lihat
[`02-workspace-app-module.md`](02-workspace-app-module.md) §2.1). Routing
HTTP, `depends`/`depends_on`, dan referensi menu di App tidak pernah encode
asal folder, hanya nama.

### 6.2 `formspec.lock`

Mencatat, per module terinstal: `source` (mis.
`github.com/acme/billing-module` — identitas unik sesungguhnya, §2.1 di
atas), `version`, `checksum`, `signature`, dan `trust_tier`
([`07-marketplace.md`](07-marketplace.md) §2). Alias hasil resolusi
konflik nama juga dicatat di sini, mengikat ke `source` yang sama supaya
`formspec module install` berikutnya untuk source yang sama tidak menghasilkan
alias baru. Format skema lengkap belum dituliskan — lihat §6.5.

### 6.3 Model Aktivasi: Default Nonaktif, Uncomment untuk Pakai

`formspec module install` menulis entri **ter-comment** di `App.spec.modules`
([`02-workspace-app-module.md`](02-workspace-app-module.md) §3) di bawah
blok marker terstruktur `>>> formspec:vendor ... <<< formspec:vendor` — bukan
comment bebas developer. Ini yang memungkinkan `formspec module install`
mengenali "blok ini milik saya" saat dipanggil ulang (update versi) tanpa
menabrak comment manual developer di sekitarnya. Developer cukup uncomment
entri yang mau dipakai — tidak perlu mengetik ulang nama/source. Flag
`formspec module install <source> --use` menulis langsung entri ter-uncomment,
untuk kasus ingin langsung aktif tanpa dua langkah
([`07-marketplace.md`](07-marketplace.md) §3).

**Idempotensi saat re-install/update:** kalau developer sudah uncomment
(mengaktifkan) suatu module, `formspec module install` untuk update versi
**tidak boleh** comment-balik blok yang sudah aktif — yang diperbarui
hanya versi di dalam marker (`@1.0.0` → `@1.1.0`) dan entri `formspec.lock`
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
formspec override adopt stripe-connector Form checkout-form
```

Meng-copy file + mencatat checksum spec asli sumbernya ke `formspec.lock`
(sebagai "asal fork"). Copy manual pakai `cp` tidak meninggalkan jejak
checksum → tidak ada deteksi drift di §6.4.2.

**6.4.1 Whitelist per Kind (ditegakkan saat boot, bukan konvensi):**

| Kind                                                                                                                                    | Boleh di-shadow-copy                                                                      | Tidak boleh                                                                                                                           |
| --------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| `Form`                                                                                                                                  | Layout (section, `columns`, grouping), caption/label, urutan field, visibility (`hidden`) | — (kind ini murni presentation)                                                                                                       |
| `Menu`/`Navigation` (`App.spec.menu`)                                                                                                   | Label, icon, urutan, visibility                                                           | —                                                                                                                                     |
| Instance `VisualSpecKind` lain (Table/Kanban/Calendar/dst., [`../frontend/02-visual-spec-kind.md`](../frontend/02-visual-spec-kind.md)) | Kolom ditampilkan, default sort                                                           | —                                                                                                                                     |
| `Entity` (termasuk `business_rules`/L4–L6 validation, [`../backend/02-core-extended.md`](../backend/02-core-extended.md) §14)           | — (tidak ada jalur shadow-copy)                                                           | Semua — field/validasi tambahan pakai **Entity Extension** ([`../backend/03-entity-extension.md`](../backend/03-entity-extension.md)) |
| `Service`/`Workflow`                                                                                                                    | — (tidak ada jalur shadow-copy)                                                           | Semua — perilaku lintas-module pakai pola **Integrator** ([`../backend/02-core-extended.md`](../backend/02-core-extended.md) §5)      |

Field-field yang boleh di-shadow-copy pada dasarnya presentation-layer
saja — sejalan dengan lingkup `kind: Form` yang memang didesain untuk
kustomisasi layout, bukan logic. Field yang memang tidak boleh pernah
tampil (bukan "ubah tampilannya", tapi "jangan tampil sama sekali") tidak
butuh shadow copy Form — cukup `exclude: [ui]` di level field
([`../backend/05-field-types.md`](../backend/05-field-types.md) §5.3).

**6.4.2 Deteksi Drift saat Vendor Update.** Setiap `formspec module
install`/update, checksum base spec baru dibandingkan checksum "asal fork"
tercatat. Kalau beda → warning eksplisit saat boot (bukan hard-fail,
karena developer memang sudah sengaja ambil alih penuh file itu):

```
⚠ overrides/stripe-connector/form.checkout-form.yaml adalah shadow copy
  dari checkout-form versi 1.0.0 — vendor sudah rilis versi 2.1.0.
  Shadow copy Anda TIDAK otomatis dapat perubahan upstream.
  → formspec override diff stripe-connector Form checkout-form
```

### 6.5 Status Implementasi Hari Ini (Gap)

Seluruh §6 adalah **target desain, belum diimplementasikan** — `vendors/`,
`overrides/`, `formspec.lock`, marker aktivasi, dan `formspec override
adopt|diff` tidak ada di `formspec dev`/CLI hari ini
([`../../cli-tools/02-formspec-cli.md`](../../cli-tools/02-formspec-cli.md) §9
baru mencakup `formspec module list|install|uninstall` dasar). Pertanyaan
terbuka yang masih perlu didalami sebelum implementasi (lihat catatan
kerja untuk detail):

- `vendors/` di-commit ke git, atau di-gitignore dan direstore murni dari
  `formspec.lock` (pola `node_modules`/vendor PHP)?
- Mekanisme `formspec verify` (cek checksum tree `vendors/` vs `formspec.lock`,
  tolak build kalau ada modifikasi manual) belum dispesifikasikan detail
  teknisnya.
- Format persis entri `formspec.lock` per module (§6.2) belum dituliskan
  skema lengkapnya.
- Bagaimana `formspec module install` menangani bundle (satu source, banyak
  module) secara teknis — manifest bundle terpisah, atau `ModulePublish`
  boleh mendeklarasikan banyak module sekaligus?

**Catatan 2026-08-19 — `external/` (module user-kustom):** Sebagai langkah
pertama menuju §6, konsep **`external/`** diperkenalkan dan diimplementasikan
untuk auth (todo 6.1, `docs/plan/auth-login-token.md`): folder module
external yang **dikustomisasi user dan wajib di-commit ke git** — berbeda
dari `vendors/` (readonly, tidak di-commit). Loader men-scan `external/`
sebagai root tambahan; entity di sana **menang** atas default bawaan
`formspec.core` (user override menang). Alur DX: `formspec generate auth`
meng-scaffold auth module ke `external/auth` untuk dikustomisasi. Konsep
`external/` ini diharapkan menjadi fondasi `overrides/` §6.4 yang lebih
luas (shadow copy per-kind) di fase berikutnya.

**Catatan 2026-08-21 — `formspec.core` sebagai bundled module (dogfooding):**
`formspec.core` kini didefinisikan sebagai **bundled module YAML** yang
di-embed ke binary (`internal/auth/module/`, `//go:embed module`) dan dimuat
lewat manifest loader — jalur yang sama dengan modul user (dogfooding).
Entity auth (`user`, `session`, `role`, `api-key`,
`app-membership`, `workspace`) diekspresikan sebagai YAML manifests, bukan
registrasi programatik Go. `formspec generate auth` menyalin module ini ke
`external/auth` untuk dikustomisasi (selalu sinkron dengan bundled). Detail:
`docs/plan/fase6-dogfooding-auth-module.md`.

- Apakah shadow-copy (§6.4) perlu dilacak versinya sendiri secara
  eksplisit di `formspec.lock` (bukan cuma checksum "asal fork"), supaya
  `formspec override diff` bisa tunjukkan riwayat?
- Apakah ada kebutuhan override di level tenant/runtime (bukan cuma level
  project/deploy) — mis. tenant admin ganti caption sendiri lewat admin
  panel? Kemungkinan sumbu terpisah mirip resolusi `ctx.config`
  ([`../backend/01-core-basic.md`](../backend/01-core-basic.md) §10),
  belum dibahas hubungannya dengan shadow copy.
- Apakah whitelist §6.4.1 perlu dideklarasikan vendor sendiri (module
  menandai field mana yang "override-safe"), atau cukup aturan generik per
  kind yang sama untuk semua vendor?

## 7. Referensi

| Dokumen                                                                             | Isi                                                                                       |
| ----------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| [`examples/Clinic-UI-Showcase/`](../../../examples/Clinic-UI-Showcase/)             | Contoh kanonik layout §1–§2: dua App, dua Module, spec entity-centric, app sidecar `app/` |
| [`03-kind-system.md`](03-kind-system.md)                                            | Taksonomi kind, derived-by-default, pemetaan kind → plane                                 |
| [`02-workspace-app-module.md`](02-workspace-app-module.md)                          | Model workspace/App/Module yang jadi dasar §3; menu (§4); alias saat konflik nama (§2.1)  |
| [`07-marketplace.md`](07-marketplace.md)                                            | Instalasi, trust tier, dan model aktivasi module vendor (§6 di atas)                      |
| [`../backend/03-entity-extension.md`](../backend/03-entity-extension.md)            | Entity Extension — jalur aditif untuk field/validasi tambahan pada Entity vendor          |
| [`../backend/06-script-runtime.md`](../backend/06-script-runtime.md)                | Resolusi `ref` handler native di `impl/` (§7) — memakai §2 dokumen ini                    |
| [`docs/runtimes/04-formspec-sidecar.md`](../../runtimes/04-formspec-sidecar.md)     | Protokol sidecar per proses (§4), mode eksekusi (§5), gap implementasi (§8)               |
| [`docs/architecture/08-repo-structure.md`](../../architecture/08-repo-structure.md) | Bagaimana `sdk/*` dan `internal/sidecar` merealisasikan kontrak ini di kode               |

> **Dua konvensi folder di repo:** contoh `examples/Clinic-UI-Showcase/`
> memakai folder entity-centric + characteristic grouping (kanonik, §2.2).
> Contoh lama (`examples/Midtrans-Payment-Gateway/`, `verticals/billing/spec/`)
> masih memakai grouping kind-based (`entities/`, `forms/`, `tables/`,
> `services/`) — keduanya sah di loader (§1.2), dan migrasi ke konvensi kanonik
> adalah pekerjaan lanjutan, bukan bagian kontrak.
