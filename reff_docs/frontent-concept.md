# Software Architecture Specification: Declarative UI Framework blueprints
**Kategori:** Arsitektur Sistem / Frontend Automation Framework
**Tujuan:** Panduan standarisasi tipe halaman dan skema deklaratif berbasis YAML untuk otomatisasi pembuatan frontend aplikasi Enterprise (ERP, Sistem Informasi Klinik, dll).

---

## 1. Filosofi & Strategi Pengembangan Framework

Framework ini mengadopsi pendekatan **Hybrid Low-Code (Declarative UI)**. Tujuan utamanya adalah mengotomatisasi **80-90%** komponen antarmuka standar yang berpola, sekaligus menyediakan katup penyelamat (*escape hatch*) berupa area kode manual untuk logika bisnis yang terlalu kompleks.

### Prinsip Utama Pembagian Halaman:
1. **Design-Time Layout Locking:** Penentuan wadah komponen (seperti penggunaan Modal vs Halaman Terpisah) ditentukan saat *design-time* melalui berkas spesifikasi YAML demi menjaga konsistensi *User Experience* (UX) dan stabilitas *state management* aplikasi di *runtime*.
2. **Permission-Based Access Control (PBAC):** Berkas konfigurasi YAML tidak boleh mengikat nama *role* secara kaku (*hardcoded*). YAML hanya mendeklarasikan nama *resource* dan *actions* (hak akses yang tersedia). Pemetaan *role* dilakukan secara dinamis oleh pengguna akhir di tingkat basis data pada saat aplikasi berjalan (*runtime*).

---

## 2. Taksonomi Tipe Halaman (8 Core Page Types)

Aplikasi operasional enterprise dibagi ke dalam 8 tipe halaman spesifik berdasarkan karakteristik manipulasi data dan alur kerjanya:

### 1. Resource Page (Data-Centric)
* **Deskripsi:** Halaman standar untuk mengelola siklus hidup data mentah (*Master Data* / Entitas DB) melalui operasi CRUD (Create, Read, Update, Delete).
* **Karakteristik UI:** Tabel data (*datatable*), filter pencarian, pengurutan, pagination, dan tombol aksi.
* **Opsi Komponen Render (`render_mode`):**
  * `modal` / `drawer`: Cocok untuk data master ringan agar pengguna tidak kehilangan konteks halaman utama. Dikendalikan via *query string* (misal: `?action=edit&id=1`).
  * `separate_page`: Cocok untuk entitas padat dengan formulir input yang panjang dan membutuhkan validasi kompleks. Memiliki alamat *routing* unik (misal: `/pasien/create`).

### 2. Tabbed Resources (Grouped Data-Centric)
* **Deskripsi:** Kategori khusus untuk menyatukan banyak *Resource Page* berskala kecil ke dalam satu halaman induk menggunakan navigasi Tab.
* **Tujuan UX:** Mencegah penumpukan menu pada *sidebar* untuk data-data master sekunder (misal: Master Suku, Master Jenis Kelamin, Master Status Pernikahan).

### 3. Task / Process Page (Workflow-Centric)
* **Deskripsi:** Halaman yang dirancang untuk memandu pengguna menyelesaikan satu proses bisnis multi-langkah (*step-by-step*) yang dapat memengaruhi banyak tabel database dalam satu transaksi.
* **Karakteristik UI:** Komponen *Wizard / Stepper*, form dinamis bertingkat, dan logika ketergantungan antar-input (*dependent fields*).

### 4. Dashboard & Analytics Page (Insight-Centric)
* **Deskripsi:** Halaman agregasi data visual untuk kebutuhan konsumsi informasi dan pengambilan keputusan eksekutif.
* **Karakteristik UI:** Tata letak grid dinamis berisi *metric cards* (KPI), grafik linier/batang (*charts*), tabel ringkasan, dan sistem peringatan (*alerts*).

### 5. Configuration & System Page (Control-Centric)
* **Deskripsi:** Halaman khusus untuk memodifikasi perilaku atau parameter global aplikasi. 
* **Karakteristik Data:** Bersifat tunggal (*single-row*) atau bertipe *Key-Value*. Strukturnya dikunci oleh developer dan **tidak memiliki tombol "New Item"** karena variabelnya bersifat statis, pengguna hanya berwenang mengubah nilainya (*Update Only*).
* **Karakteristik UI:** Menggunakan tata letak form ber-tab (*Tabbed Key-Value Form*).

### 6. Document & Print Page (Output-Centric)
* **Deskripsi:** Halaman layout statis/presisi yang dioptimalkan khusus untuk media cetak fisik atau konversi dokumen resmi (PDF).
* **Karakteristik Teknik:** Menggunakan CSS `@media print`, menyembunyikan navigasi global (sidebar/navbar), dan mendukung ukuran kertas spesifik (A4, A5, Thermal 58mm/80mm).

### 7. Timeline & Journal Page (Event-Centric)
* **Deskripsi:** Halaman yang menyajikan rekaman aktivitas atau data historis secara kronologis vertikal berdasarkan urutan waktu.
* **Karakteristik Data:** Bersifat *append-only* (tidak boleh di-edit/hapus untuk menjaga validitas audit).

### 8. Kanban / Board Page (Status-Centric)
* **Deskripsi:** Halaman manajemen berbasis visual kartu untuk memantau pergeseran status operasional secara *real-time*.
* **Karakteristik UI:** Kolom status dengan fitur *drag-and-drop* kartu yang memicu pembaruan field status di database secara otomatis.

### 9. Custom Page (The Escape Hatch)
* **Deskripsi:** Kanvas kosong yang didaftarkan ke sistem *routing* framework. Framework hanya menyediakan kerangka luar (otentikasi, sidebar, layout dasar), sementara area konten utama diserahkan sepenuhnya kepada programmer menggunakan kode manual (*pure code*).

---

## 3. Blueprint Spesifikasi YAML (`app_spec.yaml`)

Berikut adalah standardisasi struktur dokumen YAML yang akan dibaca oleh mesin *parser generator* frontend:

```yaml
version: "1.0"
app_name: "Klinik Medika Utama"
theme:
  primary_color: "#2563eb"
  density: "compact" # opsi: comfortable, compact

# ==========================================
# SPESIFIKASI HALAMAN (PAGES DECLARATION)
# ==========================================
pages:

  # 1. RESOURCE PAGE (Render Mode: Modal)
  - id: master_obat
    title: "Master Data Obat"
    type: "resource"
    datasource: "tbl_obat"
    render_mode: "modal" # modal | drawer | separate_page
    auth:
      resource_name: "obat"
      actions: ["read", "create", "update"] # Tombol delete otomatis tidak digenerate jika absen
    features:
      allow_export: true
    fields:
      - name: "kode_obat"
        label: "Kode Obat"
        type: "text"
        required: true
        searchable: true
      - name: "nama_obat"
        label: "Nama Obat"
        type: "text"
        required: true
        searchable: true
      - name: "stok"
        label: "Stok Minimal"
        type: "number"
        default: 0

  # 2. RESOURCE PAGE (Render Mode: Halaman Terpisah)
  - id: manajemen_pasien
    title: "Data Pasien"
    type: "resource"
    datasource: "tbl_pasien"
    render_mode: "separate_page"
    auth:
      resource_name: "pasien"
      actions: ["read", "create", "update", "delete"]
    fields:
      - name: "nik"
        label: "No. NIK (KTP)"
        type: "text"
        required: true
      - name: "nama_lengkap"
        label: "Nama Lengkap"
        type: "text"
        required: true

  # 3. TABBED RESOURCES (Pengelompokan Data Master Banyak)
  - id: data_master_klinik
    title: "Data Master Ops"
    type: "tabbed_resources"
    auth:
      resource_name: "master_ops"
      actions: ["read", "write"]
    tabs:
      - id: tab_spesialisasi
        title: "Spesialisasi Dokter"
        datasource: "tbl_spesialisasi"
        render_mode: "modal"
        fields:
          - name: "nama_spesialisasi"
            label: "Nama Spesialisasi"
            type: "text"
            required: true
      - id: tab_jenis_pasien
        title: "Kategori Pasien"
        datasource: "tbl_jenis_pasien"
        render_mode: "modal"
        fields:
          - name: "nama_kategori"
            label: "Nama Kategori (Umum/BPJS)"
            type: "text"
            required: true

  # 4. PROCESS / WIZARD PAGE
  - id: pendaftaran_wizard
    title: "Pendaftaran Pasien & Poli"
    type: "process"
    auth:
      resource_name: "pendaftaran_loket"
      actions: ["execute"]
    steps:
      - step: 1
        title: "Cari / Pilih Pasien"
        layout: "search_select"
        datasource: "tbl_pasien"
      - step: 2
        title: "Pilih Poli & Dokter"
        fields:
          - name: "id_poli"
            label: "Poliklinik Tujuan"
            type: "dropdown"
            source: "tbl_poliklinik"
          - name: "id_dokter"
            label: "Dokter Jaga"
            type: "dropdown"
            source: "tbl_dokter"
            depends_on: "id_poli" # Logika frontend: filter dokter berdasarkan id_poli

  # 5. DASHBOARD PAGE
  - id: dashboard_direktur
    title: "Dashboard Analitik Klinik"
    type: "dashboard"
    auth:
      resource_name: "dashboard_eksekutif"
      actions: ["read"]
    grid_layout: "auto"
    widgets:
      - id: widget_total_pendapatan
        type: "metric_card"
        title: "Pendapatan Bulan Ini"
        query: "SELECT SUM(total) FROM tbl_invoice"
        icon: "currency-dollar"
        size: "w-1/3"
      - id: widget_tren_kunjungan
        type: "chart_line"
        title: "Tren Kunjungan Pasien"
        query: "SELECT tanggal, COUNT(*) FROM tbl_kunjungan GROUP BY tanggal"
        size: "w-2/3"

  # 6. CONFIGURATION PAGE
  - id: pengaturan_sistem
    title: "Konfigurasi Sistem"
    type: "configuration"
    auth:
      resource_name: "system_setting"
      actions: ["read", "update"]
    tabs:
      - id: tab_klinik
        title: "Profil Klinik"
        fields:
          - key: "sys_nama_klinik"
            label: "Nama Klinik"
            type: "text"
      - id: tab_integrasi
        title: "API Satusehat"
        fields:
          - key: "satusehat_base_url"
            label: "Production URL"
            type: "text"

  # 7. DOCUMENT / PRINT PAGE
  - id: cetak_nota_kasir
    title: "Nota Pembayaran"
    type: "document"
    paper_size: "thermal_58mm"
    datasource: "tbl_invoice"
    layout:
      header: ["logo", "nama_klinik", "alamat"]
      body: "table_items"
      footer: ["total_tagihan"]

  # 8. TIMELINE PAGE
  - id: rekam_medis_history
    title: "Riwayat Klinis Pasien"
    type: "timeline"
    datasource: "tbl_rekam_medis"
    bind_param: "id_pasien"
    display:
      title_field: "tgl_periksa"
      subtitle_field: "nama_dokter"
      content_field: "anamnesa_dan_diagnosa"

  # 9. KANBAN / BOARD PAGE
  - id: antrean_farmasi
    title: "Board Antrean Apotek"
    type: "kanban"
    datasource: "tbl_resep"
    status_column: "status_resep"
    columns:
      - value: "antri"
        label: "Belum Diproses"
      - value: "peracikan"
        label: "Sedang Diracik"
      - value: "siap_ambil"
        label: "Siap Diambil"

  # 10. CUSTOM / FULL CODE ESCAPE HATCH
  - id: modul_bridging_bpjs_custom
    title: "Bridging Khusus BPJS"
    type: "custom"
    route: "/bpjs/custom-bridging"
    auth:
      resource_name: "custom_bpjs"
      actions: ["access"]
    target_file: "src/views/custom/BpjsCustomBridging.vue"