# Gap #18, #19, #20 — Drift Dokumen & Skill AI

Ini gap lapisan **dokumentasi & skill** — bukan kode engine, tapi berdampak
langsung: agent AI (termasuk saya) menulis YAML berdasarkan dokumen ini. Kalau
dokumennya salah, spec yang dihasilkan salah.

Untuk test case "agent-assisted app development", gap di lapisan ini justru
yang paling relevan.

---

## Gap #18 — Skill `entity-authoring` mengajarkan `relation` wajib `target` ⚠️ DIKOREKSI

> **KOREKSI setelah verifikasi (2026-09-14).** Klaim di bawah perlu diperbaiki:
> `formspec validate` **MENANGKAP** kesalahan ini. Diuji dengan entity probe
> berisi `target:`:
>
> ```
> [FAIL] spec\modules\cafe-master\master\probe-target\entity.yaml#0
>        schema: /spec/fields/0: validation failed
> ```
>
> Jadi ini **bug dokumentasi, bukan kegagalan senyap**. Tingkat keparahan turun
> dari HIGH ke **MEDIUM**: penulis spec langsung dapat error saat validasi,
> bukan menghasilkan data rusak tanpa gejala.
>
> **Yang tetap berlaku:** dua sumber resmi mengajarkan bentuk yang ditolak
> validator — skill `entity-authoring` **dan landing page FormSpec**
> (`site/src/components/Hero.tsx`). Biaya nyatanya adalah trial-and-error, bukan
> korupsi relasi. Karena itu usulan "perbaiki skill" tetap prioritas.
>
> **Catatan usability:** pesan errornya tidak menyebut nama key yang salah
> (`/spec/fields/0: validation failed`), sehingga penulis harus menebak bahwa
> `target:` penyebabnya. Nama key akan membuat perbaikan jauh lebih cepat.
>
> **Pelajaran metodologis:** klaim "gagal senyap" di GAP-18 dan GAP-21 berasal
> dari pembacaan dokumen/kode, bukan pengujian. Setelah diuji, GAP-18 gugur dan
> GAP-21 berdiri. Verifikasi runtime memang membedakan keduanya.

### Bukti

`.agents/skills/entity-authoring/SKILL.md` (file yang **ada di workspace ini**,
jadi ini yang benar-benar saya baca saat menulis YAML) — tabel tipe field:

| Tipe | Catatan |
| --- | --- |
| `relation` | referensi entity lain; **wajib `target`** |

Hanya itu. Tidak ada contoh `relation:` object.

Namun skill `formspec-kinds` — **skill lain di folder yang sama** — justru
memperingatkan sebaliknya, cukup keras:

> **`target:` on a field is silently ignored by the YAML loader, producing a
> dangling relation. Use `relation: { type: belongs_to, resource: <mod.entity> }`.**

Dan loader-nya memang begitu — `Cmd/formspec/generate_test.go` serta
`verticals/gl`/`billing` semuanya memakai bentuk objek:

```yaml
- { name: customer_id, type: relation, relation: { type: belongs_to, resource: billing.customer }, required: true }
```

`pkg/spec/entity.go`:

```go
Relation *RelationDecl `yaml:"relation,omitempty" json:"relation,omitempty"`
```

Tidak ada field `Target` pada `Field`.

### Dampak ke aplikasi kafe: **HIGH**

Ini bukan sekadar salah tulis dokumen — ini **memproduksi bug yang gagal
senyap**:

1. Agent (saya) membaca `entity-authoring` (skill yang secara deskripsi
   menangani "buat entity", "tambah field") → menulis `target: menu-item`.
2. YAML **lolos loading** tanpa error — `target` diabaikan.
3. `formspec validate` **tidak melaporkan apa pun**, karena `target` memang bukan
   key yang dikenal, dan `type: relation` tanpa `relation:` tidak dianggap
   invalid.
4. Hasilnya: **relasi menggantung.** `order.menu_item_id` berisi UUID, tetapi
   tidak ada relasi → `RelationPicker` tidak tahu entity target → tidak ada
   pencarian/pemilihan, dan label relasi tidak pernah muncul di Table/Detail.

Untuk kafe, hampir **semua** relasi adalah `belongs_to` lintas-entity
(`order → menu-item`, `order → table`, `order → member`, `payment → order`,
`stock-movement → product`). Jadi bug ini akan mengenai nyaris seluruh model
data, dan tidak muncul di validasi.

### Usulan

1. **Perbaiki tabel di `entity-authoring/SKILL.md`** — ganti baris `relation`
   dengan bentuk objek + contoh kode, dan tambahkan peringatan `target:`
   diabaikan.
2. **Tambahkan deteksi di loader/validator**: key `target` pada field →
   `formspec validate` harus **error** ("use `relation:` object; `target:` is
   ignored"). Ini kelas validasi yang sama dengan "unknown key" — dan justru
   karena `target` muncul di **dokumentasi publik** (`site/src/components/Hero.tsx`
   menampilkan contoh manifest dengan `target: customer`!) maka pesan error-nya
   sangat berharga.
3. Audit semua salinan skill di repo: `ai_skills/entity-authoring/` (sumber),
   `examples/*/.agents/skills/entity-authoring/`, dan salinan di project
   `kafe/` ini. Sinkronkan lewat proses yang sudah ada agar tidak terulang.

> **Temuan tambahan:** contoh di `site/src/components/Hero.tsx` (landing page resmi
> FormSpec) juga memakai bentuk lama:
> ```yaml
> - name: customer
>   type: relation
>   target: customer
> ```
> Jadi bentuk yang salah ini adalah **bentuk yang paling terlihat secara publik**,
> sekaligus yang paling berbahaya. Untuk project baru, semua relasi akan ditulis
> dengan cara ini dan gagal senyap.

---

## Gap #19 — Drift dokumen renderer 📄

### `docs/renderers/shadcn-shell/03-kind-renderers.md` §4

Klaimnya:

> Widget field yang **ada**: `TextInput`, `NumberInput`, `Select` (enum),
> `Switch` (boolean), `Badge`, `RelationPicker`. Widget yang derivation engine
> _tunjuk_ tapi **tidak ada komponennya**: `datepicker` (field
> `date`/`datetime`), `json`, `child-grid` (field `child`) — ketiganya jatuh
> diam-diam ke `TextInput` polos.

**Kondisi sekarang:** `DateInput`, `JsonInput`, dan `ChildTable` **sudah ada**
(terekspor di `widgets/index.ts`, dan dengan `case` di `FormRenderer.tsx`).
Jadi bagian ini kedaluwarsa.

**Tapi poin konseptualnya masih benar dan berharga** — paragrafnya bahkan
merumuskan masalahnya dengan tepat:

> Ini kesenjangan nyata antara apa yang derivation engine _bilang_ harus dipakai
> vs yang benar-benar dirender — bukan sekadar "belum ditulis", karena **user
> melihat input teks polos untuk field tanggal, bukan pesan error atau
> placeholder yang jujur.**

Pola "silent downgrade" itu **masih hidup**, hanya pindah korban: sekarang
`money` dan `time` (lihat `01-widget-money-time.md`). Dokumen yang menyebut
daftar widget outdated membuat pembaca mengira masalahnya sudah selesai.

### `docs/renderers/realtime.md` §5

Klaim: "Calendar / ApprovalInbox / NotificationCenter **renderer belum ada**".

**Kondisi sekarang:** `shell/router.tsx` sudah lazy-import ketiganya
(`CalendarRenderer`, `ApprovalInboxRenderer`, `NotificationCenterRenderer`).
Dokumen belum diperbarui. Yang **masih** benar dari gap list itu: Timeline
belum di-wire realtime, dan heartbeat belum ada.

### Usulan

- Regenerate/perbarui bagian manual dokumen; tapi yang lebih baik: tambahkan
  **test yang memverifikasi setiap `FieldType` punya widget** (atau terdaftar
  eksplisit sebagai "belum ada"), sehingga dokumen dan kode tidak bisa
  berbeda tanpa gagal build.

---

## Gap #20 — Inkonsistensi kecil yang menumpuk 📄

Beberapa hal kecil yang berulang dan membingungkan saat menulis spec:

| Hal | Inkonsistensi | Bukti |
| --- | --- | --- |
| Versi apiVersion | `entities` di dokumen/skill `formspec.dev/v1`, tapi banyak test & fixture memakai `formspec.dev/v1alpha1`, dan `site/src/components/Hero.tsx` memakai `forma.dev/v1alpha1` (prefix lama) | `internal/manifest/*_test.go`, `internal/ui/ui_test.go`, `site/src/components/Hero.tsx` |
| `lifecycle` | Skill bilang "STRING enum" dan ada nilai `plain_crud`; contoh `arisan` memakainya; tapi ada dokumentasi lama yang menyebut `lifecycle: {doc_status: true}` sebagai bentuk yang SALAH. Interpretasi berbeda antara `Characteristic` dan `Lifecycle` untuk entity `transaction` | `entity-authoring/SKILL.md`, `ai_skills/formspec-kinds/SKILL.md` (gotchas) |
| Jumlah kind | Skill menyebut **34 kind**; `renderers/react-shadcn/src/types/manifest.ts` ≥ 32, dan `docs/kind/` menyebut 34; `ResourceKind` di types belum memuat `Calendar`, `ApprovalInbox`, `NotificationCenter`, `Mockup`, `Integrator`, `KindDefinition` | `types/manifest.ts` `ResourceKind` (26 nilai) vs `docs/kind/` (34) |
| Field `required` | Dua cara: key `required: true` **dan** `rules: [required]`. `fieldIsRequired()` di `cmd/formspec/generate.go` menangani keduanya, dengan komentar bahwa manifest nyata "exclusively use the latter" | `cmd/formspec/generate.go` |
| `natural_key` | Bisa sebagai **atribut field** (`natural_key: true`), tapi dokumen lama menunjukkan `natural_key: [field]` (array) | changelog `2026-08-28-003`: "`natural_key` adalah atribut field" |

### Dampak ke aplikasi kafe: **MEDIUM**

Masing-masing kecil, tapi digabung biaya trial-and-error-nya nyata: agent harus
menebak bentuk mana yang benar, dan error-nya sering tidak muncul di
`formspec validate` (lihat Gap #18).

### Usulan

- Satukan sumber kebenaran: `schemas/kinds/*.schema.json` (yang di-generate dari
  `pkg/spec`) sudah jadi acuan otomatis — **arahkan skill AI untuk membacanya**
  sebagai ganti tabel manual. Jadi satu tabel per tipe field bisa (dan
  seharusnya) digenerate, bukan ditulis tangan.
- Untuk test case ini: jadikan `formspec validate --schema schemas` sebagai gate
  wajib, dan simpan `schemas/` di project (skill `formspec init` sudah
  memuatnya).
- Tambahkan rule deteksi "unknown key" yang **error**, bukan diabaikan — ini
  menutup Gap #18 dan sebagian besar Gap #20 sekaligus.

---

## Gap #21 — Validator tidak menangkap referensi menggantung ✅ Pasti

Ditemukan saat menulis Tahap 1 Draft.

### Bukti

Scaffold project ini (`spec/apps/kafe.yaml`, sebelum diubah) berisi:

```yaml
spec:
  modules:
    - kafe
  menu:
    - type: module
      module: kafe
```

Sementara **tidak ada** `spec/modules/kafe/` sama sekali — module itu tidak
pernah ada.

`formspec validate --spec spec` melaporkannya:

```
[OK]   spec\apps\kafe.yaml#0
2 manifest(s) validated, 0 problem(s) found
```

**Hijau.** Referensi ke module yang tidak ada lolos validasi.

Kelas yang sama juga muncul di `spec.modules` saat jumlah module bertambah:
pindah ke `modules: [cafe-master, cafe-order]` juga tidak pernah dilaporkan
walau `view:` di menu sempat menunjuk manifest yang belum ada.

### Dampak ke aplikasi kafe: **HIGH**

Ini pola **gagal senyap** yang persis sama dengan Gap #18:

1. App menunjuk module yang salah ketik / belum dibuat → validasi hijau.
2. Saat runtime, menu adopt node mengembang jadi **kosong** — sidebar tidak
   punya entri untuk module itu.
3. Skill `formspec-kinds` sudah memperingatkan gejala akhirnya (*"If ALL
   modules in an App lack menus, the UI sidebar and default redirect will be
   empty"*), tetapi pencegahannya hanya konvensi authoring, bukan validasi.

Untuk aplikasi ber-5-module dan 3-App, satu salah ketik nama module berarti
bagian UI yang hilang tanpa satu pun pesan error.

### Usulan

Tambah ke `formspec validate` (lintas-manifest — sudah ada mekanisme
referensi lintas-manifest untuk entity/field/action/route, jadi tinggal
diperluas):

| Referensi | Harus dilaporkan bila |
| --- | --- |
| `App.spec.modules[].<module>` | Module tidak terdaftar |
| `Module.spec.depends[].module` | Module tidak terdaftar |
| Menu leaf `view:` | Manifest view tidak terdaftar di module itu |
| `relation.resource` | Entity target tidak terdaftar (lihat Gap #12) |

Ini satu perubahan yang menutup **seluruh keluarga** bug "referensi menggantung"
— termasuk bentuk `target:` di Gap #18 dan resolusi tabel naif di Gap #12.

---

## Gap #37 — Shorthand yang didokumentasikan schema ditolak schema itu sendiri ✅ Terverifikasi

### Bukti

Deskripsi `FormRenderDecl` di schema resmi (`$defs.FormRenderDecl`) menyatakan:

> "FormRenderDecl is the design-time container declaration of a Form
> (Frontend §1.6). **YAML accepts both `render: separate_page` (shorthand) and
> `render: { mode: separate_page }`**; JSON always serializes the object form"

Tapi JSON Schema-nya sendiri menuntut objek:

```json
"FormRenderDecl": {
  "properties": { "mode": { "$ref": "#/$defs/FormRender" } },
  "required": ["mode"],
  "additionalProperties": false
}
```

Ditulis persis seperti yang didokumentasikan:

```yaml
spec:
  render: drawer      # bentuk singkat yang disebut diterima
```

hasilnya:

```
[FAIL] spec\modules\cafe-order\forms\order-form-pos.yaml#0
       schema: /spec/render: validation failed
[FAIL] spec\modules\cafe-master\forms\menu-item-form.yaml#0
       schema: /spec/render: validation failed
```

Bentuk yang lolos: `render: { mode: drawer }`.

### Analisis

Ini bukan bug dokumen biasa — **deskripsi dan validasi berada di artefak yang
sama** dan saling bertentangan. Kemungkinan penyebabnya: `UnmarshalYAML` khusus
pada tipe Go menerima skalar, tetapi JSON Schema yang **digenerate** dari struct
hanya melihat bentuk objek. Jadi loader dan validator memakai aturan berbeda.

Pola ini kemungkinan besar tidak tunggal. Kandidat lain yang punya
`UnmarshalYAML` penerima-multi-bentuk:

| Tipe | Bentuk alternatif yang diterima loader |
| --- | --- |
| `TransitionDecl` | `via:` **dan** alias lama `action:` |
| `StateList` | skalar `"draft"` **dan** list `[draft, x]` |
| `GuardDecl` | string ekspresi **dan** map `{expression, message}` |
| `ConditionDecl` | `{script, message}` dan bentuk lama `{field, expression}` |

### Dampak ke aplikasi kafe: **MEDIUM**

Biaya nyata: developer menulis bentuk yang secara eksplisit didokumentasikan
sebagai sah, lalu ditolak — dan pesannya hanya `/spec/render: validation failed`,
tanpa menyebut bahwa masalahnya "harus objek, bukan string".

Lebih buruk: karena `formspec check` **0 error**, satu-satunya gerbang yang
menangkapnya adalah `validate` — dan hanya bila dijalankan.

### Usulan

1. **Satukan aturan**: entah schema menerima kedua bentuk (`oneOf: [string,
   object]`), atau `UnmarshalYAML` longgar itu dihapus dan dokumen diperbaiki.
   Kondisi sekarang—dokumen menjanjikan, validator menolak—adalah yang terburuk.
2. Audit seluruh tipe ber-`UnmarshalYAML` penerima-multi-bentuk dan pastikan
   JSON Schema-nya sejalan.
3. Perbaiki pesan error menjadi menyebutkan penyebab: "`render` harus objek
   `{mode: ...}`, bukan string".

---

## Prioritas Perbaikan (rekomendasi untuk FormSpec)

Berdasarkan seluruh temuan, urutan yang paling banyak membuka jalan untuk
aplikasi bisnis nyata seperti kafe:

| Prioritas | Item | Alasan |
| --- | --- | --- |
| **P0** | Unknown-key **error** di validator (`target:`, salah ketik, key usang) | Menutup seluruh kelas bug "gagal senyap" — termasuk Gap #18, #20, dan sebagian #10 |
| **P0** | Widget `money` + pemetaan nilai `{amount, currency}` | Prasyarat semua aplikasi transaksional (Gap #1, #2) |
| **P1** | Render `file`/gambar di Table/Listing/Detail (`widget: image`) | Membuka katalog publik & menu bergambar (Gap #4) |
| **P1** | `format: thermal` — implementasi ATAU tolak di validate | Struk POS; hari ini validate bohong (Gap #10) |
| **P1** | Perbaiki `target:` di semua skill + landing page | Mencegah bug relasi menggantung massal (Gap #18) |
| **P2** | Widget/kanal QR code | Blokir alur QR order (Gap #3) |
| **P2** | Blok cart/checkout untuk halaman publik | Blokir alur pesan pelanggan (Gap #5) |
| **P2** | Wire komposisi multi-App (SyncAgent → router) | Membuka integrasi vertical akuntansi (Gap #15) |
| **P3** | Row-level scope / `TenantDecl` untuk outlet/cabang | Multi-outlet aman (Gap #8) |
| **P3** | Valuasi stok (FIFO/moving-avg) + HPP | Laporan margin menu (Gap #13) |
