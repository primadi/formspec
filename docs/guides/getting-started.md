# Membangun App Pertama Anda

Panduan ini membawa Anda dari nol ke **app FormSpec yang berjalan dan lolos
validasi** dalam satu sesi. Setelah selesai Anda punya project yang bisa
dikembangkan — bentuknya app back-office nyata, bukan contoh mainan.

Untuk daftar lengkap perintah, lihat [`../cli-tools/02-formspec-cli.md`](../cli-tools/02-formspec-cli.md).
Untuk menjalankan project yang sudah ada, lihat [`how-to-run.md`](how-to-run.md).

---

## 0. Prasyarat

Install CLI `formspec` (satu perintah, tanpa Go):

```bash
curl -fsSL https://formspec.dev/install.sh | sh
formspec version
```

Detail lengkap (Windows, `go install`, manual): [`install.md`](install.md).

Satu hal yang perlu dipahami sebelum menulis YAML apa pun — **tiga file type
saja** yang dikenal FormSpec:

| Tipe     | Isi                                                    |
| -------- | ------------------------------------------------------ |
| `yaml`   | Deskripsi resource (manifest) — sumber kebenaran       |
| `script` | Logic Starlark yang berjalan di sandbox                |
| `asset`  | Komponen UI statis/custom (JS/TS)                      |

Tidak ada file `.env`, file route, atau file migration manual. Kalau Anda
mencari tempat meletakkan salah satunya, itu tanda Anda harus mengekspresikannya
sebagai manifest.

---

## 1. Scaffold Project

```bash
formspec init tokoku
cd tokoku
```

`formspec init` membuat kerangka standar:

```
tokoku/
├── formspec-app.yaml              # config CLI (`formspec dev`, dsn, runtime)
├── AGENTS.md                      # instruksi untuk AI coding agent
├── spec/
│   ├── apps/tokoku.yaml           # kind: App    — bentuk app, menu, modul
│   ├── modules/tokoku/
│   │   └── module.yaml            # kind: Module — paket yang di-mount App
│   └── workspaces/tokoku.yaml     # kind: Workspace — seed tenant
├── .agents/skills/                # skill AI (workflow 4 fase, katalog kind)
└── .vscode/settings.json          # yaml.schemas → autocomplete + validasi YAML
```

Perhatikan **App dan Module itu berbeda**: App adalah unit deployment
(satu aplikasi yang dijalankan), Module adalah paket manifest (Entity, Service,
Config, …) yang di-mount oleh App. Satu App bisa memount beberapa Module, dan
Module bisa dipublikasikan ke registry secara terpisah.

Validasi kerangka:

```bash
formspec validate --spec spec
# → 3 manifest(s) validated, 0 problem(s) found
```

Jalankan sebagai gerbang di **setiap** langkah berikutnya. Manifest yang gagal
validasi tidak akan boot.

> **Editor YAML?** `.vscode/settings.json` sudah mendaftarkan `yaml.schemas`
> yang menunjuk ke `https://schemas.formspec.dev/v1/formspec.schema.json`, jadi
> `spec/**/*.yaml` mendapat autocomplete + validasi langsung di VS Code.

---

## 2. Tentukan Bentuk App — Sebelum Menulis Manifest

Buka `spec/apps/tokoku.yaml`. Dua sumbu di sini **orthogonal** dan keduanya
keputusan design-time (tidak bisa di-switch saat runtime):

| Sumbu          | Nilai                                                     | Default        |
| -------------- | --------------------------------------------------------- | -------------- |
| `access`       | `private` (login wajib) · `public` (landing anonim)       | `private`      |
| `app_renderer` | `sidebar-nav` · `topnav` · `no-nav`                       | `sidebar-nav`  |

Tanyakan ke diri Anda (atau ke user Anda):

- **Back-office internal?** → `private` + `sidebar-nav` (default) — biarkan apa adanya.
- **Portal publik / landing page?** → `public`, biasanya `no-nav`, dipasangkan
  dengan `kind: Listing`.
- **Dua-duanya?** → **dua** App: portal publik + admin privat yang berbagi
  Module di `root_url` berbeda.

Ini keputusan yang mahal untuk diubah belakangan (semua route dan menu ikut),
jadi ambil sekarang, bukan setelah 20 entity.

---

## 3. Modelkan Domain

Satu Entity = satu tabel = satu resource dengan CRUD, state machine, permission,
dan event yang diturunkan otomatis. Anda **hanya** menulis field, aturan, dan
lifecycle-nya.

Pilih **characteristic** yang tepat — ini menentukan strategi penyimpanan dan
perilaku API:

| Characteristic | Sifat                                                         |
| -------------- | ------------------------------------------------------------- |
| `master`       | Data stabil — kategori, produk, customer                     |
| `transaction`  | Append-heavy, time-partitioned — order, invoice, jurnal       |
| `reference`    | Read-only seed — provinsi, tarif pajak, chart of accounts     |
| `summary`      | Projection yang dikelola sistem — tidak ada CUD via API       |

Buat entity pertama:

```bash
formspec new entity product
# ✓ spec/modules/tokoku/master/product/entity.yaml
```

Scaffold-nya sudah berisi `version`, `characteristic`, `plural`, `display_field`,
dan blok `expose` — isi field-nya, jangan tulis dari nol. Hasil akhir yang **lolos
`formspec validate`** (terverifikasi) terlihat seperti ini:

```yaml
apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: product
  module: tokoku
  description: "Produk yang dijual"
spec:
  version: v1              # WAJIB — satu-satunya properti `required` di spec
  characteristic: master
  plural: products
  fields:
    - name: sku
      type: string
      required: true
      unique: true
      title: "SKU"
    - name: name
      type: string
      required: true
      title: "Nama"
      rules: [{ min_length: 2 }]
    - name: price
      type: money
      required: true
      title: "Harga"
      rules: [positive]
    - name: stock
      type: integer
      title: "Stok"
      rules: [{ min: 0 }]
  state_machine:
    field: status
    initial: draft
    states:
      - { name: draft, label: "Draft" }
      - { name: active, label: "Aktif" }
      - { name: archived, label: "Diarsipkan" }
    transitions:
      - { from: draft, to: active, via: activate }
      - { from: active, to: archived, via: archive }
  expose:
    - type: rest
      actions: [list, find, create, update, delete]
```

Dua hal yang mudah salah dan **ditolak dengan keras**:

- `version` wajib. Tanpa itu: `schema: /spec: missing property 'version'`.
- `states` butuh `name` **dan** `label`. `{ name: draft }` saja ditolak.

Perhatikan juga: **permission bukan bagian dari spec Entity.** Permission =
resource + action, dan nilainya diturunkan framework dari nama module/entity
(mis. `tokoku.products.create`). Siapa yang boleh melakukan apa diatur lewat
**Role** di admin panel, bukan di YAML — supaya kebijakan bisa berubah tanpa
menyentuh spec. (Menambahkan blok `permissions:` ke Entity akan ditolak:
`additional properties 'permissions' not allowed`.)

Validasi lagi:

```bash
formspec validate --spec spec
# → 4 manifest(s) validated, 0 problem(s) found
```

Kesalahan paling sering di tahap ini — contoh pesan nyata dari validator:

| Pesan                                                              | Artinya                                                          |
| ------------------------------------------------------------------ | ---------------------------------------------------------------- |
| `schema: /spec: missing property 'version'`                        | `spec.version` belum diisi (satu-satunya field wajib)             |
| `schema: /spec/characteristic: validation failed`                  | Nilai `characteristic` bukan `master`/`transaction`/…             |
| `additional properties 'permissions' not allowed`                  | Permission tidak dideklarasikan di Entity (lihat di atas)        |
| `reference: App mounts module(s) X, which no kind: Module declares` | Nama di `App.spec.modules` tidak cocok dengan Module mana pun    |

> **Belum divalidasi (jujur):** dua Entity dengan `metadata.name` sama di satu
> module **tidak** dilaporkan sebagai error oleh `formspec validate`.
> Hasil runtime-nya belum diuji, jadi jangan mengandalkannya sebagai gerbang —
> pakai nama unik dan biarkan AI/agent Anda (atau review) yang menjaganya.
> Dicatat sebagai item terbuka di todo (`9.4.1 ⏸️`).

---

## 4. Jalankan

```bash
formspec dev --dev-ui
```

- **Admin panel** → `http://localhost:5173/default/_admin`
- Entity yang baru dibuat otomatis muncul di menu (derived by default) — lengkap
  dengan Table, Form create/edit, dan halaman detail. Anda tidak menulis satu
  baris pun UI untuk itu.

Anda juga tidak perlu menulis CRUD: `formspec new entity` sudah menyertakan blok
`expose` dengan `actions: [list, find, create, update, delete]`, dan route
ter-generate saat boot (terlihat di log: `engine loaded: N routes`). Tanpa
`expose`, entity-nya ada tapi tidak punya permukaan API sama sekali
(deny-by-default) — kalau endpoint-nya 404, cek blok ini dulu.

`formspec dev` juga menyalakan hot-reload spec: ubah YAML, simpan, dan manifest
dimuat ulang tanpa restart.

> Untuk mode dua terminal (engine + Vite terpisah), mode statis, dan opsi lain:
> [`how-to-run.md`](how-to-run.md).

---

## 5. Menulis Logic Bisnis

Saat behavior lupa tidak cukup (mis. "diskon maksimal 10%", "void transaksi
butuh approval"), ada beberapa jalur — pilih yang paling sempit:

| Kebutuhan                                        | Jalur                                        |
| ------------------------------------------------ | -------------------------------------------- |
| Validasi satu field (presence, panjang, range)   | `required` + `rules` pada field              |
| Validasi antar-field dalam satu record           | `rules` field (`after:`/`before:`)           |
| Logic di satu transisi state                     | `guard` pada transition                      |
| Side-effect setelah action                       | `hooks: [{ point: after, script: ... }]`     |
| Logic lintas-resource, sandboxed                 | `kind: Service` + Starlark                   |
| Butuh performa / library Go                      | `kind: Service` + `impl: { type: native }`   |
| Persetujuan sebelum transisi                     | `kind: Workflow`                             |

> Level validasi **L4–L6** (`business_rules`, `cross_validate`, `consistency`)
> sudah ada di kontrak tapi **belum bisa dideklarasikan** di `pkg/spec` —
> keputusan desainnya masih terbuka (todo 7.9.1–7.9.4). Untuk sekarang, validasi
> yang menjangkau beberapa field atau entity lain ditulis sebagai `guard`,
> `hook`, atau `kind: Service`.

Contoh — guard pada transisi, di `entity.yaml` (terverifikasi lolos validasi):

```yaml
    transitions:
      - from: draft
        to: active
        via: activate
        guard:
          expression: "resource.stock >= 0"
          message: "Stok tidak boleh negatif saat mengaktifkan produk"
```

Script Starlark berjalan di sandbox dengan batas keras (wall-clock, memori,
jumlah query) dan **hanya** bisa menyentuh infrastruktur lewat enam primitif
tertutup: `ctx.db`, `ctx.cache`, `ctx.lock`, `ctx.queue`, `ctx.pubsub`,
`ctx.storage`. Jangan menulis SQL mentah — gunakan `ctx.db`.

---

## 6. Berikutnya

| Saya ingin…                                        | Lihat                                                        |
| -------------------------------------------------- | ------------------------------------------------------------ |
| Referensi satu kind (atribut, contoh, gotcha)      | <https://docs.formspec.dev/kind/>                            |
| Kontrak normatif (backend/frontend/platform)       | [`../spec/`](../spec/README.md)                              |
| Tutorial alur bisnis lengkap                       | [`order-to-cash-tutorial.md`](order-to-cash-tutorial.md)     |
| Bangun app dibantu AI agent                        | [`agent-assisted-app-development.md`](agent-assisted-app-development.md) |
| Atur login, role, dan permission                   | [`authentication.md`](authentication.md)                     |
| Reproduksi/ubah renderer atau persist backend      | [`authoring-a-page-renderer.md`](authoring-a-page-renderer.md), [`authoring-a-persist-backend.md`](authoring-a-persist-backend.md) |
| Deploy ke produksi (`serve --mode=production`)     | [`../spec/platform/`](../spec/platform/README.md)            |

### Membangun dengan AI agent

`formspec init` sudah menyiapkan jalur ini: `AGENTS.md` + `.agents/skills/`
membuat coding agent mengikuti workflow 4 fase (Discovery → Proposal → Draft →
Iterate) dengan `formspec validate --spec spec` sebagai gerbang. Di Copilot Chat,
ketik `/skills` untuk melihat skill yang tersedia, lalu mulai dengan permintaan
dalam bahasa biasa:

> buat FormSpec app untuk manajemen inventory

Lihat [`agent-assisted-app-development.md`](agent-assisted-app-development.md)
untuk detailnya.
