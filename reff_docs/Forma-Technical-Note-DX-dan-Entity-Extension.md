# Forma Technical Note: Developer Experience & Mekanisme Entity Extension

**Catatan internal — hasil diskusi tim, bukan bagian resmi Forma Core Spec**
**Status: bahan pertimbangan desain, belum committed ke spesifikasi. Perlu direview dan diformalkan ke Core Basic/Extended spec kalau disepakati.**

---

## 0. Latar Belakang

Dokumen ini merangkum kesimpulan diskusi tentang dua topik yang saling terkait:

1. **Developer experience (DX)** — bagaimana Forma seharusnya menyeimbangkan barrier-to-entry (git, devcontainer, CLI) dengan kebutuhan audit/type-safety yang jadi nilai jual utama.
2. **Mekanisme entity extension** — bagaimana modul lain menambahkan field ke entity milik modul lain (misal modul vertikal `invoice`) tanpa fork, tanpa merusak isolasi modul, dan tanpa mengorbankan performa.

Kedua topik ini dibahas sebagai perbandingan terhadap PocketBase dan Frappe sebagai referensi DX, dan terhadap Odoo (`_inherit`) sebagai referensi pola extension.

---

## 1. Posisi DX Forma vs PocketBase vs Frappe

| | PocketBase | Frappe | Forma |
|---|---|---|---|
| Setup awal | Satu binary, SQLite embedded | `bench` CLI + MariaDB/Postgres + Redis + Node build | `forma dev` — Docker Compose (Postgres, Redis, Mailpit, MinIO) + `forma-control` |
| Definisi data | Collection via Admin UI/schema | DocType (metadata-driven) | Resource YAML → codegen tipe Go |
| Business logic | JS hooks / embed Go library | Server script Python + hook lifecycle | Starlark (`script`/`script_ref`), editable dari admin panel tanpa redeploy |
| Type safety | Rendah–menengah | Rendah (Python dinamis) | Tinggi (codegen dari resource definition) |
| Kedalaman untuk app bisnis kompleks | Terbatas | Tinggi (terbukti di ERPNext) | Tinggi, target setara Frappe tapi dengan type safety Go |

**Kesimpulan:** Forma mengambil posisi tengah secara sadar — kedalaman ala Frappe untuk business logic, dengan type safety dan performa Go, dengan ongkos setup yang lebih dekat ke Frappe daripada ke PocketBase. Ini trade-off yang diterima, bukan kekurangan yang perlu ditutupi.

---

## 2. Git sebagai Satu-satunya Source of Truth untuk Struktur

Pertanyaan awal: apakah editing bisa dilakukan lewat web-app (Forma Cloud) yang terpisah dari git, untuk menurunkan barrier masuk devcontainer/VS Code?

**Kesimpulan: tidak, untuk perubahan struktur.** Semua alur inti (`forma apply`, `forma diff`, signing di `forma-control`, audit trail) beroperasi di atas file yang diasumsikan versioned. Git bukan preferensi gaya kerja, tapi prasyarat teknis:

- `forma diff` butuh state "sebelum" yang bisa dibandingkan — itu commit git, bukan state di database editor web.
- Signing artifact di `forma-control` butuh sumber yang immutable dan jelas asalnya (commit SHA).
- Kalau ada dua sumber kebenaran (web editor + git), drift antara keduanya merusak jaminan audit yang jadi nilai jual utama Forma.

**Yang tetap didukung secara konsisten dengan model ini:**
- Editing via GitHub Codespaces (tetap commit ke git di baliknya) — bukan editor lepas.
- "Forma Studio" (opsional, belum dirancang detail): web GUI drag-and-drop untuk desain resource, tapi output-nya tetap commit YAML ke git lewat Git API, bukan tulis langsung ke database.

---

## 3. Dua Lapisan Barrier-to-Entry — Sudah Terpisah secara Alami

Barrier devcontainer/git itu nyata, tapi cuma berlaku untuk satu dari dua lapisan perubahan:

| Lapisan | Contoh | Jalur | Butuh git/devcontainer? |
|---|---|---|---|
| **Struktur** | Field, entity, relation, migrasi DB | `forma apply` dari resource YAML | Ya — risiko setara ubah skema produksi |
| **Business logic** | Validation rule, condition, action handler | Starlark `script`/`script_ref`, admin panel | **Tidak** — sudah bisa diedit dari admin panel tanpa redeploy, dengan versioning & rollback bawaan |

**Kesimpulan:** kalau target pengguna termasuk non-developer, jawabannya bukan menghilangkan git, tapi memastikan lapisan business logic (yang memang didesain untuk berubah tanpa deploy) punya GUI yang baik di admin panel — ini sudah ada secara konsep di Starlark scripting, tinggal dipastikan UX admin panel-nya memadai.

---

## 4. Performa Model Storage Hybrid (Fixed Columns + JSONB)

Forma sudah memakai model hybrid: kolom fixed (`id`, `tenant_id`, `version`, timestamps) + satu kolom `data jsonb` untuk semua field bisnis, dengan generated column untuk field yang di-`index: true`.

**Kesimpulan performa:**

- Field dengan `index: true` → generated column (`GENERATED ALWAYS AS (data->>'field') STORED`) diperlakukan Postgres planner **persis seperti kolom native**. Tidak ada penalti untuk filter/sort/join pada field yang sudah diindeks dengan benar.
- Risiko nyata ada di dua tempat:
  1. Field yang dipakai untuk filter/sort/join tapi **lupa** di-`index: true` — full JSONB parsing per baris, mahal di skala besar.
  2. Cross-resource query lintas `persist.category` (schema berbeda) — tidak pernah jadi SQL JOIN, selalu app-level merge. Ini biaya by-design (isolasi kategori data), bukan bug, tapi perlu diperhitungkan untuk dataset besar.
- **Rekomendasi tooling:** `forma apply`/observability sebaiknya bisa mendeteksi dan warning field yang dipakai di query builder tapi tidak diindeks — mengubah risiko dari "developer lupa" jadi "sistem yang menegur," mengurangi ketergantungan pada disiplin manual.

**Verdict:** model ini bukan ide buruk — pola "fixed columns + JSONB + generated indexed columns" adalah pola yang matang untuk flexible-schema-on-Postgres, dengan syarat disiplin indexing dijaga (idealnya dibantu tooling, bukan cuma dokumentasi).

---

## 5. Mekanisme Entity Extension

### 5.1 Masalah yang diselesaikan

Programmer yang memakai modul vertikal (misal `billing/invoice`) perlu menambahkan field bisnis kastem sendiri, tanpa fork modul, tanpa merusak upgrade path modul asal, dan idealnya tanpa biaya join tambahan saat baca data.

### 5.2 Opsi yang dipertimbangkan

| Opsi | Type-safe? | Sentuh tabel modul asal? | Risiko upgrade | Cocok untuk |
|---|---|---|---|---|
| `extra_fields` bucket generik | Tidak | Ya (kolom `data` sama) | Rendah | Tenant admin nambah atribut ad hoc via UI, tanpa developer |
| `extendEntity` ala Odoo `_inherit` | Ya | Ya, field menyatu ke tabel yang sama | **Tinggi** — melanggar isolasi modul yang sudah jadi prinsip Forma | Tidak direkomendasikan sebagai default |
| `containEntity` — tabel terpisah, relasi 1:1 | Ya | Tidak | Rendah | Ekstensi berbentuk one-to-many atau butuh lifecycle independen |
| **Kolom JSONB terpisah per-extend (final)** | Ya | Ya, tapi terkontrol & terbatas ke kolomnya sendiri | Rendah–menengah | **Kasus umum: nambah beberapa field kastem ke entity modul vertikal** |

### 5.3 Desain final: kolom JSONB terpisah per-namespace extend

Setiap extend menambah **kolom fisik baru** di tabel yang sama (bukan tabel terpisah, bukan nested path di dalam `data`):

```sql
CREATE TABLE financial.billing_invoices (
  id          uuid PRIMARY KEY DEFAULT gen_uuid_v7(),
  tenant_id   uuid NOT NULL,
  ...,
  data        jsonb NOT NULL DEFAULT '{}',   -- field inti, milik modul billing
  ext_kastem1 jsonb NOT NULL DEFAULT '{}'    -- field ekstensi, milik modul my-customization
);
```

```yaml
resource:
  name: invoice-ext
  type: entity
  module: my-customization
  extend_storage:
    target: billing/invoice
    namespace: kastem1
  fields:
    - name: project_code
      type: string
      index: true
```

**Kenapa kolom terpisah, bukan nested path (`data->'ext'->'kastem1'`):**

1. **Uninstall jadi DDL murni.** `ALTER TABLE ... DROP COLUMN ext_kastem1` adalah operasi metadata-only, instan. Nested path butuh `UPDATE` di setiap baris untuk strip key — mahal dan berisiko di tabel besar.
2. **Isolasi baca.** Query umum terhadap `data` tidak perlu ikut men-detoast kolom ekstensi yang jarang dipakai.
3. **Prefix wajib** (`ext_kastem1`, bukan `kastem1` polos) — supaya tidak pernah bentrok dengan kolom reserved framework (`data`, `tenant_id`, `version`, dst).

### 5.4 Reservasi nama namespace

Nama namespace (`kastem1`) yang pernah dipakai **tidak boleh dipakai ulang**, aktif maupun sudah di-drop, kecuali di-purge eksplisit. Ditegakkan lewat tabel registry, pola serupa `forma_migrations` yang sudah ada:

```sql
CREATE TABLE forma_extensions (
  resource    text NOT NULL,   -- billing/invoice
  namespace   text NOT NULL,   -- kastem1
  module      text NOT NULL,   -- my-customization
  status      text NOT NULL,   -- active | dropped
  created_at  timestamptz NOT NULL,
  PRIMARY KEY (resource, namespace)
);
```

`forma apply` menolak namespace yang sudah tercatat di tabel ini untuk resource yang sama.

### 5.5 Nested extend (extend dari extend) — tidak direkomendasikan

Pertimbangan: extend dari extend (`kastem1` di-extend lagi jadi `kastem1_special1`) secara teknis mungkin, tapi punya tiga masalah:

- Mengikat dependency antar modul ke dalam **nama kolom fisik** secara permanen — sulit di-rename/diorphan-kan dengan aman kalau modul dasar diganti/dihapus.
- Menciptakan urutan dependency di level migration yang sebelumnya tidak perlu ada (`kastem1` harus ter-apply dulu sebelum `special1`).
- Membocorkan abstraksi — modul `special1` jadi tahu bahwa `kastem1` adalah sebuah extend, bukan entity biasa.

**Alternatif yang dipilih:** semua extend tetap **flat**, sibling terhadap base entity, berapa pun jumlahnya. Dependency antar modul ekstensi (kalau ada) dinyatakan lewat `module.requires` yang sudah ada di Module Spec, dan akses lintas ekstensi (kalau perlu) lewat resource API biasa dengan permission eksplisit:

```yaml
# modul special1 — extend langsung ke base entity yang sama,
# bukan ke kastem1
extend_storage:
  target: billing/invoice
  namespace: special1

module:
  requires:
    - module: kastem1-module
      version: ">=1.0.0"
```

Field akses lintas ekstensi lewat kode, bukan lewat penamaan kolom:
```
invoice.ext("kastem1").some_field
```

### 5.6 Field yang di-`index: true` pada kolom ekstensi

Konsekuensi yang perlu disadari: field ekstensi yang di-index tetap berarti mengubah DDL tabel milik modul lain (`ALTER TABLE billing_invoices ADD COLUMN _project_code ... GENERATED ALWAYS AS (ext_kastem1->>'project_code') STORED`). Ini titik kopling yang mirip risiko `extendEntity`, tapi terkontrol karena:

- Terjadi di level migration yang bisa direview via `forma apply --dry-run`, bukan akses runtime bebas.
- Field non-indexed (default) tidak menyentuh DDL sama sekali.

---

## 6. Ringkasan Keputusan

| Topik | Kesimpulan |
|---|---|
| Editing struktur resource | Wajib lewat git (CLI/devcontainer/Codespaces) — tidak ada jalur web terpisah dari git |
| Editing business logic | Sudah bisa lewat admin panel (Starlark), tanpa redeploy — ini jalur penurun barrier yang sudah ada |
| Storage field bisnis | Hybrid fixed columns + JSONB tetap dipertahankan; field yang di-query wajib `index: true`, idealnya ditegakkan tooling |
| Cross-category join | Tetap app-level merge by design; perlu warning eksplisit untuk dataset besar |
| Entity extension | Kolom JSONB terpisah per-namespace extend, flat (tidak nested), dengan reservasi nama via registry |

---

*Dokumen ini adalah catatan diskusi, belum bagian resmi Forma Core Spec. Perlu diformalkan ke Core Extended Spec (kemungkinan sebagai bagian baru "Extension Spec") kalau desain di bagian 5 disepakati.*
