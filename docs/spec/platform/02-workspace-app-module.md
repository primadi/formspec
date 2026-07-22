# Workspace, App, Module

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku. Bagian yang masih terbuka
> ditandai eksplisit sebagai **Open**.

## 1. Model Kepemilikan
```
Workspace → App → Module → Resource
```
Satu workspace berisi banyak App dan banyak Module. **Module memiliki
objek** (Entity, Service, dan seluruh instance VisualSpecKind — Page,
Form, Table, dst.) — satu Module = satu bounded context bisnis utuh,
tidak dipecah jadi "module backend" vs "module frontend" (field/schema,
layout form, list view, permission berubah bersamaan; memisahkannya
menambah overhead deklarasi lintas-module yang tidak proporsional untuk
sesuatu yang sebenarnya satu unit). **App adalah kurasi** — keranjang
objek dari module-module yang dideklarasikan lewat `depends_on`
([`../backend/01-core-basic.md`](../backend/01-core-basic.md) §5), bukan
pemilik objek. Satu Module yang sama boleh di-mount lebih dari satu App
dalam workspace yang sama, masing-masing meng-expose subset berbeda (App
internal vs App publik yang mengakses data sama).

**Workspace adalah satu-satunya model multi-tenancy Forma.** Aplikasi
ditulis sepenuhnya *tenancy-blind* — tidak ada kode aplikasi yang memilih
strategi tenancy, dan **setiap Entity tenant-isolated secara default,
tanpa pengecualian**; satu workspace = satu tenant terisolasi. **Tidak ada
akses lintas-workspace dalam bentuk apa pun** di dalam framework — kalau
integrasi lintas-workspace suatu saat dibutuhkan, itu hidup di level
external service, di luar Forma Framework. Data `characteristic: reference`
dimiliki App Owner (di-seed lewat rilis, read-only bagi Data Owner;
backend juga mendukung **find-or-create** otomatis saat pertama kali
diakses — lihat [`backend/01-core-basic.md`](../backend/01-core-basic.md) §1.1);
seluruh data tenant-isolated lainnya dimiliki Data Owner. Strategi dan
topologi isolasi (single vs multi, pooled vs isolated, tiering) **bukan
urusan spec aplikasi** — diputuskan saat deployment oleh Platform Operator
([`04-control-plane.md`](04-control-plane.md) §2). Tenant besar yang ingin
server sendiri memakai lisensi enterprise dan menjalankan Forma Cloud-nya
sendiri sebagai Platform Operator, bukan lewat mode tenancy khusus di dalam
aplikasi.

## 2. Module
Package manifest — identitas, versi, dependency. Isi ditemukan lewat
scanning file, bukan didaftar manual (`metadata.name` = permission
namespace). Struktur di dalamnya adalah *closed set*: Document,
Service, instance VisualSpecKind (Page/Form/Table/dst.), dan deklarasi
permission yang mengikat semuanya — bukan tipe bebas. Module tidak wajib
mengisi semua jenis itu; Module murni integrasi (mis. `forma/tax-calculator`)
boleh cuma berisi Service.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Module
metadata:
  name: general-ledger
spec:
  version: 1.2.0
  vendor: acme-corp
  depends:
    - module: forma/core
    - module: billing
      version: ">=1.0 <2.0"
  datastore: default    # opsional — nama kind: Datastore untuk ctx.db()
                        # module ini; default kalau tidak diisi. Lihat
                        # ../backend/01-core-basic.md §3 dan 06-datastore.md §1.1
  config:
    fiscal_year_start: "04-01"   # module-specific: GL mulai tahun fiskal April
    coa_structure: "4-2-2-2"     # module-specific: struktur kode akun 4 segmen
    # Currency, locale, timezone → settings.* (global), bukan di sini
    # Lihat ../backend/01-core-basic.md §10
  menu: []    # default menu suggestion, module-relative — lihat §4
  ai_index:   # opsional — metadata discovery untuk Forma AI, lihat
              # ../../ai/04-forma-remote-mcp.md §3
    category: payment
    features: [charge, refund, webhook_callback]
    integration_pattern: |
      depends: [{module: payment-gateway-xendit}]
    skills_for_ai: |
      Pakai module ini kalau bisnis butuh terima pembayaran online.
```

### 2.1 Identitas Unik & Alias saat Konflik Nama (Module Vendor)
`metadata.name` yang ditulis pembuat module (mis. `billing`) **tidak
dijamin unik secara global** — dua vendor berbeda boleh memilih nama yang
sama. Identitas unik sesungguhnya ada di **source** module (mis.
`github.com/acme/billing-module`), dicatat di `forma.lock`
([`08-project-layout.md`](08-project-layout.md) §6.2). Ini hanya berlaku
untuk module yang diinstal lewat `forma module install`
([`07-marketplace.md`](07-marketplace.md) §3) — Module lokal hand-authored
tetap satu-satunya pemilik `metadata.name`-nya sendiri, tanpa alias.

Saat instalasi bentrok dengan nama efektif module lain yang **sudah pernah
diinstal** (aktif maupun masih nonaktif), installer otomatis memberi alias,
dicatat sebagai blok marker di manifest App
([`08-project-layout.md`](08-project-layout.md) §6.3):

```yaml
spec:
  modules:
    - billing   # module lokal

    # >>> forma:vendor github.com/acme/billing-module @1.0.0
    # - acme-billing
    # <<< forma:vendor
```

Uncomment `- acme-billing` mengaktifkannya — bentuknya tetap string biasa,
konsisten dengan elemen `App.spec.modules` lain (§3). Source dan versi asal
tercatat di baris marker `>>>` dan di `forma.lock`
([`08-project-layout.md`](08-project-layout.md) §6.2), bukan di bentuk
entri itu sendiri.

**Alias dihitung saat install, bukan saat aktivasi** — nama efektif module
tidak boleh berubah tergantung urutan aktivasi developer: kalau dua vendor
dengan nama sama sama-sama diaktifkan kapan pun kemudian, tidak boleh ada
surprise rename. Konsisten dengan prinsip gap-free yang sama dipegang di
tempat lain di spec (mis. `ctx.next_key`, [`../backend/01-core-basic.md`](../backend/01-core-basic.md)) —
nomor/nama tidak boleh berubah makna tergantung state runtime.

**Enforcement saat boot:** `forma-server` mengecek nama efektif (alias
kalau ada, `metadata.name` kalau tidak) **hanya** untuk set module yang
aktif. Bentrok di set aktif → refuse to boot dengan pesan jelas, minta
alias manual. Module yang belum diaktifkan tidak pernah dicek — dua vendor
module bernama sama boleh nangkring bersamaan di `vendors/` selama tidak
dua-duanya aktif tanpa alias.

## 3. App
Root project manifest — unit deployment, trust boundary, dan publikasi
interface. Satu workspace **boleh** berisi lebih dari satu App; seluruh App
di satu workspace berjalan bersamaan dalam proses yang sama, dibedakan
`root_url`.

```yaml
apiVersion: forma.dev/v1alpha1
kind: App
metadata:
  name: klinik-sehat-internal
spec:
  version: 2.1.0
  vendor: acme-corp
  root_url: /app/klinik-internal   # prefiks routing, wajib unik per App dalam satu workspace
  modules: [billing, acme-corp/general-ledger]
  app_renderer: sidebar-nav        # pilih App renderer — lihat spec/frontend/05-app-kinds.md
  menu: []                        # lihat §4
  publishes:                      # interface lintas-app yang ditawarkan
    - service: icd-lookup
      actions: [search, find]
  consumes:                       # interface lintas-app yang dibutuhkan → memicu grant request
    - app: bpjs-gateway
      service: claims
      actions: [submit-claim]
```

Default private. Akses lintas-app hanya lewat publish → request → **grant
disetujui Data Owner**, tercatat, revocable, metered
([`04-control-plane.md`](04-control-plane.md) §5 Contracts).

`app_renderer` memilih Renderer tier `app` yang dipasang App ini
([`../frontend/01-visual-hierarchy.md`](../frontend/01-visual-hierarchy.md),
[`../frontend/05-app-kinds.md`](../frontend/05-app-kinds.md)) — nilainya
nama Renderer terdaftar (`sidebar-nav`, `topnav`, `landing-page`, dst),
bukan enum tertutup di level kontrak App (Renderer baru bisa didaftarkan
kapan saja tanpa mengubah skema App).

**Theme adalah app-specific (normatif).** Theme di-resolve di **level App**
lewat field `theme_ref` di `App.spec` — bukan per workspace. Alasannya:
beda App bisa memakai Shell berbeda, dan bahkan dua App di Shell yang sama
bisa punya kebutuhan brand berbeda (App internal vs App publik satu vendor).
Workspace boleh menetapkan Theme default sebagai fallback untuk App yang
tidak mendeklarasikan `theme_ref`, tapi keputusan akhir selalu di manifest
App. ([`../frontend/05-app-kinds.md`](../frontend/05-app-kinds.md) §5
mengikuti keputusan ini.)

**Auth & authorization per-App (normatif).** Autentikasi dan otorisasi
di-resolve di level App, bukan enum tertutup:

- **Authentication** — App mendeklarasikan skenario auth yang dipakainya
  lewat konfigurasi auth (`auth_config_ref`), memilih dari strategy yang
  terpasang: `basic-auth`, `sso` (OIDC/SAML), `social-sso` (Google,
  Facebook, GitHub, dst), `passwordless` (magic link/OTP), `passkey`
  (WebAuthn), dan seterusnya. **Set strategy terbuka untuk ditambah**
  (bukan closed enum) — strategy baru didaftarkan sebagai artifact, mengikuti
  trust tier yang sama dengan artifact lain.
- **Authorization** — dievaluasi di level permission module
  (`{module}.{entity}.{action}`, mis. `invoice.create`) dan **boleh juga
  memeriksa atribut** App, user, membership, atau konteks lain
  (attribute-based) di samping pengecekan permission berbasis role.

**"Forma-ID" bukan primitive tersendiri (normatif).** Konsep identitas
lintas-workspace dengan consent ledger portable (identitas manusia yang
dikenali lintas banyak workspace, membawa riwayat consent) **di luar
scope Forma Framework** — bertentangan langsung dengan prinsip tenancy §1:
workspace adalah satu-satunya batas isolasi, tanpa akses lintas-workspace
dalam bentuk apa pun. Kalau kebutuhan semacam itu muncul, tempatnya di
level external service ([`../backend/01-core-basic.md`](../backend/01-core-basic.md)
§3), bukan diselesaikan sebagai fitur Forma Framework.

Yang tetap relevan dan **tidak butuh konsep baru**: Forma Cloud **boleh**
menawarkan server OIDC/OAuth terkelola sebagai kenyamanan infra — supaya
App Owner tidak perlu memasang identity provider sendiri — persis seperti
menawarkan Postgres terkelola. Ini cukup jadi **satu pilihan** di bawah
strategy `sso` yang sudah ada di atas, dikonfigurasi lewat `auth_config_ref`
seperti provider OIDC lain mana pun — tanpa nama/branding atau kontrak
tersendiri.

**Multi-cabang (branch) — pola yang direkomendasikan.** Cabang **bukan**
alasan membuat App terpisah per cabang (App per cabang menduplikasi kurasi
dan menu hanya demi satu kode pembeda). Cabang adalah **scoping data**:
entity yang branch-aware memakai field cabang (di-scope lewat
`scope_field` pada natural key bila perlu nomor urut per cabang), membership
user membawa atribut cabang, dan authorization attribute-based (di atas)
mengevaluasi atribut itu — satu App, banyak cabang. Kalau satu cabang butuh
kurasi UI yang benar-benar berbeda, barulah App terpisah dipertimbangkan —
sebagai keputusan kurasi, bukan keharusan teknis.

## 4. Menu
**Menu milik App, independen dari Module** — bukan keputusan estetika,
konsekuensi langsung dari fakta bahwa View/Action yang di-expose bisa
berbeda per App-mount (App publik cuma expose `wizard`+`cek-status`, App
internal expose `list`+`approve` dari Module yang sama). Menu — enumerasi
"apa yang bisa dicapai lewat navigasi" — harus ditentukan di level yang sama
dengan keputusan visibility itu, yaitu App. Analogi: **Module = katalog,
App.menu = daftar belanja dari katalog itu.**

`App.spec.menu` dan `Module.spec.menu` sama-sama `[]MenuItem` — array
= `App.spec.menu` (otoritatif), atau saran default `Module.spec.menu` yang
diadopsi App lewat `type: module` Adopt node.

Urutan item di list = urutan tampil (tidak ada field `order` terpisah).
Supaya App Owner tidak dibebani wiring manual dari nol, **Module boleh
menyediakan default menu suggestion** (`Module.spec.menu`) yang bisa langsung
diadopsi App lewat Adopt node — App tetap bebas override/restrict/rearrange.

`MenuItem` (dipakai identik di `App.spec.menu` dan `Module.spec.menu`):

```go
type MenuItem struct {
    Type     string      // "module" = adopt-shorthand node; kosong = grup/leaf biasa
    Label    string
    Icon     string
    Module   string      // wajib di leaf & node type:module; terlarang di grup
    View     string      // nama View terdaftar (Page/Table/Wizard/Kanban/Dashboard/Report/Timeline)
    Route    string      // escape hatch: URL mentah untuk leaf tanpa View terdaftar
    When     string      // kondisi bisnis FormaExpr
    Children []MenuItem
}
```

Nesting dibatasi **3 level**; tiap node wajib salah satu dari tiga bentuk
(divalidasi saat load, bukan diam-diam dipaksa jadi bentuk lain):
1. **Adopt node** (`type: module`, level 1 saja) — wajib `module`; menyisipkan
   seluruh menu suggestion Module itu di posisi ini.
2. **Group node** (punya `children`, level 1/2) — wajib `label` +
   `children` tak kosong; terlarang `module`/`view`/`route` di node itu
   sendiri (grup boleh berisi children dari module berbeda).
3. **Leaf/action node** (tanpa `children`, bukan `type: module`, level 2/3) —
   wajib `label`, `module`, dan **tepat satu** dari `view`/`route`. Leaf
   level-3 tidak boleh punya `children` — ini yang menegakkan batas 3 level.

Resolusi route: leaf ber-`view` me-resolve route dari registrasi View itu
sendiri (Page pakai `route:`-nya; Dashboard/Widget/Wizard/Kanban/Timeline/
Report/Print pakai konvensi `/<kind-lowercase>/<name>`) — route tidak pernah
diduplikasi ke item menu supaya tidak bisa drift. `Form` dan `Table` **bukan**
target `view` yang valid — keduanya tidak pernah dapat route standalone
(cuma tampil embedded di blok Page, atau lewat derived CRUD route). Tidak
ada `kind: Menu` standalone — sudah dilebur seluruhnya ke `App.spec.menu`
(otoritatif) dan `Module.spec.menu` (saran default).

## 5. Qualifier Referensi Antar Module
Notasi `module/resource` untuk referensi lintas module — konsisten dengan
`sources.resource`, penamaan named script `{module}/{script-name}`
([`../backend/02-core-extended.md`](../backend/02-core-extended.md) §7),
dan qualifier entity di menu App multi-module. Referensi di dalam module
sendiri tanpa qualifier (`resource: invoice`, konteks sudah jelas satu
module) — analog package-qualified reference di Go (`Invoice` dalam package
sendiri vs `billing.Invoice` dari luar).

## 6. Validasi `forma apply`
- Setiap `module` yang direferensikan di manapun dalam `App.spec.menu`
  (leaf atau adopt node) wajib anggota `App.spec.modules`.
- `root_url` wajib unik lintas seluruh App dalam satu workspace dan diawali
  `/app/`.

## 7. Akses Lintas-Module
Tiga jenis interaksi, level coupling berbeda — disarankan urutan preferensi
dari longgar ke erat: **event subscribe** (paling longgar, async, tanpa
dependency waktu-boot — `kind: Subscription`,
[`../backend/01-core-basic.md`](../backend/01-core-basic.md) §7), **action/
service call** (A cukup tahu kontrak Action, tidak tahu skema internal B),
**entity read langsung** (paling erat — dibatasi untuk kasus read-only
sederhana, mis. cek data referensi). Framework tidak melarang entity read
lintas-module, tapi konvensi mengarahkan ke pola lebih longgar untuk apa pun
yang menyangkut *behavior*, bukan sekadar baca.

**Kalau kedua Module beda `spec.datastore` (§2), preferensi di atas jadi
keharusan (normatif):** *entity read langsung* dan *action/service call*
lewat `ctx.db` sama-sama **tidak tersedia** lintas-Datastore — satu-satunya
jalur yang tersisa adalah **event subscribe**
([`../backend/01-core-basic.md`](../backend/01-core-basic.md) §3,
[`06-datastore.md`](06-datastore.md) §1.1). Beda Datastore = beda
deployment boundary; tidak ada tingkat consent yang membuka akses langsung
ke sana.

Deklarasi dependency di level Module (`spec.depends`, §2) memberi
visibilitas dependency graph untuk tooling registry/marketplace — bukan
cuma untuk enforcement runtime.

**Consent lintas Module Owner berbeda.** Akses antar-Module dengan Module
Owner yang **sama** cukup lewat `depends_on` + deklarasi permission biasa.
Akses antar-Module dengan Module Owner **berbeda** (mis. workspace
menginstal module dari dua vendor marketplace berbeda, keduanya perlu saling
baca data) butuh **consent eksplisit dari Workspace Owner** — bukan dari
Module Owner asal data, konsisten dengan prinsip data adalah milik workspace
(lihat [`04-control-plane.md`](04-control-plane.md) §5 soal model contract/
consent). Module Owner cuma menyediakan *permukaan publik* (subset Entity/
Action yang sengaja di-expose untuk dikonsumsi Module lain); keputusan
"boleh dikonsumsi atau tidak" tetap di tangan Workspace Owner.

**Bentuk artefak consent (normatif).** Consent lintas-Module-Owner memakai
**flag pada `depends_on` yang sudah ada** (mis.
`depends_on: [{module: inventory, owner_consent: required}]`) dan mendaur
ulang alur approval Policy — bukan kind artefak baru (reuse mekanisme,
bukan tambah primitive).

**Akses lintas-module dalam satu workspace didukung penuh & terverifikasi
tooling.** Selama masih dalam satu workspace, module boleh saling mengakses
sesuai deklarasi `depends_on` + permission. Kejujuran deklarasi ditegakkan
statis oleh **`forma check`** ([`../../cli-tools/02-forma-cli.md`](../../cli-tools/02-forma-cli.md)),
yang wajib melaporkan minimal: (a) seluruh *unresolved varname* di script,
(b) akses lintas-module yang **belum di-approve** (dipakai di kode tapi tak
dideklarasikan/di-consent), dan (c) deklarasi lintas-module yang **tidak
terpakai** (declared tapi tak pernah diakses — kandidat dicabut).
`forma check --fix` memperbaiki yang bisa diperbaiki otomatis (menambah
deklarasi yang kurang setelah konfirmasi, menghapus yang tak terpakai).

## 8. Identitas User & Membership

Identitas user hidup di **level workspace** — satu akun per manusia,
prasyarat untuk grant lintas-app, audit yang konsisten, dan SSO.
**Membership dan penetapan role bersifat per-App**: populasi user tiap App
dalam satu workspace boleh sepenuhnya berbeda (App internal dipakai staf,
App publik dipakai pelanggan) walau keduanya me-mount Module yang sama.
Definisi role tetap dimiliki Module → otomatis ter-scope per-App saat
Module itu di-mount. Membership boleh membawa atribut (mis. kode cabang)
yang dievaluasi authorization attribute-based (§3). Membership disimpan di
`forma.core` dan didistribusikan ke Resource Plane lewat snapshot Plane
Protocol ([`04-control-plane.md`](04-control-plane.md) §1,
[`05-plane-protocol.md`](05-plane-protocol.md) §4.1).

## 9. Resource Bawaan `forma.core`

`forma.core` adalah namespace resource **yang selalu ada di setiap workspace**,
terlepas dari Module apa pun yang terpasang — tidak perlu di-`depends_on`, tidak
perlu diinstal, tidak dideklarasikan oleh Module mana pun. Enumerasi ini adalah
referensi "apa yang selalu tersedia" bagi developer yang perlu merujuk resource
ini (mis. mengueri audit log, mengecek role assignment) tanpa harus memiliki
Module sendiri yang mendeklarasikannya.

| Resource | Isi | Kaitan |
|---|---|---|
| `workspace` | Identitas dan metadata tenant — unit multi-tenancy tunggal Forma | §1 Model Kepemilikan |
| `user` | Akun manusia di level workspace — satu akun per manusia | §8 Identitas User & Membership |
| `app-membership` | Record membership per-App (populasi user + atribut mis. kode cabang, ter-scope per App) | §8 Identitas User & Membership |
| `role` | Definisi role — dimiliki Module, otomatis ter-scope per-App saat Module di-mount | §8 |
| `role-assignment` | Penetapan role ke user dalam konteks App tertentu | §8 |
| `api-key` | Kredensial akses non-interaktif (service/integrasi) | [`04-control-plane.md`](04-control-plane.md) §5 |
| `session` | Sesi login aktif user | §3 (auth per-App) |
| `job` | Pelacakan async job — mengikat kontrak wire async action ([`../backend/02-core-extended.md`](../backend/02-core-extended.md)) | — |
| `audit-log` | Jejak audit bisnis append-only, immutable | [`../backend/02-core-extended.md`](../backend/02-core-extended.md) §11 Business Audit Trail |
| `setting` | Namespace global-settings `settings.*` (workspace/App Config) | [`../backend/01-core-basic.md`](../backend/01-core-basic.md) §10 Config & Global Settings |

Selain resource di atas, `forma.core` juga meng-expose **service endpoint
bawaan** `health` dan `metrics` untuk observability — kosakata health
machine-readable dan set metric Prometheus didefinisikan di
[`09-observability.md`](09-observability.md) (§5 Kosakata Health, §3 Metrics).

Resource `forma.core` mengikuti model kepemilikan data workspace yang sama
(§1): data ini milik workspace, bukan milik Module Owner mana pun. Akses tetap
tunduk pada permission — merujuk resource ini bukan berarti bebas otorisasi.
