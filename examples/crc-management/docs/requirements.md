# DOKUMEN KEBUTUHAN SISTEM (BRD)
**Nama Proyek:** HLSD CRC Checklist Management System
**Klien/Perusahaan:** PT Trakindo Utama
**Versi Dokumen:** 1.0 (Berdasarkan Presentasi D&IT, CRC - 2026)

---

## 1. Latar Belakang & Tujuan

### Kondisi Saat Ini (As-Is Process)
Manajemen form *checklist* inspeksi saat ini dilakukan secara manual. Admin membuat form menggunakan Microsoft Word/Excel. Teknisi mengisi dokumen cetak (kertas), dan proses *approval* berjalan menggunakan kertas fisik. Dokumen final kemudian disimpan ke dalam SharePoint secara tidak terpusat dan tidak terstruktur.
*   **Dampak:** Pemborosan kertas yang masif dan data yang tidak terstruktur/sulit dilacak.

### Tujuan Sistem (To-Be Process)
Melakukan transformasi digital ke aplikasi web **CRC Checklist Form** yang mencakup:
*   Pembuatan *master form* secara dinamis oleh Admin.
*   Pengisian form secara digital di lapangan oleh Teknisi.
*   Sistem *Review* dan *Approval* terintegrasi.
*   Penyimpanan file terpusat dan terstruktur (Centralized File Management).

---

## 2. Aktor & Hak Akses (User Management)
Sistem memiliki menu manajemen peran untuk sekitar **250 pengguna aktif**. Role yang tersedia meliputi:
1.  **Administrator:** Membuat dan mengelola *master data*, *template checklist*, mengatur alur persetujuan, dan status dokumen.
2.  **User (Serviceman / Warehouseman):** Menerima jadwal, mencari *task*, dan mengisi form *checklist* di lapangan via aplikasi (Tablet/Laptop/Mobile).
3.  **Foreman / Reviewer (Superior):** Mereview hasil pekerjaan user pada tahapan *Stop and Go*. Dapat menerima tugas *review*, memberikan *performance score*, dan menyetujui/menolak tahapan form.
4.  **Approver Internal (CAP):** Melakukan persetujuan akhir dari sistem eksternal (Centralized Approval Platform).
5.  **Customer (Eksternal):** Memberikan persetujuan jika diperlukan melalui link URL/QR Code tanpa login kompleks.

---

## 3. Alur Kerja Sistem (To-Be Process Workflow)

1.  **Pembuatan Template:** Admin merancang dan menerbitkan *Checklist Template*.
2.  **Penjadwalan:** User mendapatkan *Inspection Schedule*.
3.  **Pencarian Task:** User masuk ke aplikasi dan mencari form tugasnya.
4.  **Eksekusi & Input:** User melakukan inspeksi dan mulai mengisi data di form digital.
5.  **Stop and Go Process (Quality Gate):** Sistem menahan user untuk melanjutkan pekerjaan ke tahap berikutnya dan mengirimkan notifikasi ke Foreman.
6.  **Foreman Review:** 
    *   Jika **Revise**: Dokumen kembali ke User untuk diperbaiki.
    *   Jika **Approve**: User mendapatkan notifikasi untuk *Continue to Next Job* (melanjutkan tahapan).
7.  **Customer Approval:** (Opsional) Jika butuh *approval* kustomer, link dikirim ke kustomer. Kustomer bisa *Approve/Revise*.
8.  **Final Approval (CAP):** Dokumen diteruskan ke *Centralized Approval Platform*. Jika disetujui, masuk status *Completed*.
9.  **Document Archiving:** Sistem otomatis men-*generate* PDF dan menyimpannya di folder SharePoint.

---

## 4. Matriks Data (Document Matrix)

Sistem harus mengakomodasi *dropdown* relasional untuk matriks berikut:

### A. Component Type
*   **Powertrain Components:** Engine, Engine Component, Final Drive, Differential, Transmission, Torque Converter, Drive Axle.
*   **Hydraulic Components:** Hydraulic Cylinder, Hydraulic Pump, Hydraulic Motor, Hydraulic Control Valve.

### B. Work Type
1. Disassemble | 2. Cleaning | 3. Inspection | 4. Assemble | 5. Testing | 6. Installation | 7. General | 8. DCI

---

## 5. Struktur Form Checklist (Dinamis)
Form terdiri dari 4 bagian (Section) utama:

1.  **Document Header:** Doc Number, Revision Date, Responsible By, Process, Model, Prefix, Part Number.
2.  **General Section:** 
    *   *Job Identity:* Trakindo WO, Customer, PEX/Non PEX, TU ID Actual, Component Hours, dll.
    *   *Special Tools:* Tools Name, Part No, Qty.
    *   *Chemical:* Chemical Name, Chemical Serial No, MSDS No.
    *   *Document Reference & Weight of Part.*
3.  **Specific Section (Bergantung pada Work Type):**
    *   Berisi tabel inspeksi, tahapan persiapan, tabel bongkar pasang (disassembly).
    *   List gambar panduan (Image List).
    *   List abnormalitas alat (*Abnormal Part List, Condition, Reason*).
    *   Hasil rekaman spesifikasi teknis dan verifikasi foto.
    *   *Overall Judgement:* (Reuse / Reject / Repair).
4.  **Approval List:** Tanda tangan dan nama terang pelaksana, foreman, dan approver.

---

## 6. Spesifikasi & Kebutuhan Modul (Features Requirements)

### A. Modul Administrator (Master Data & Template Builder)
*   **Input Data:** Input *Master Data* (Job Identity, Tools, dll). Mendukung tarikan data otomatis (sinkronisasi) dari **SAP via EDW**.
*   **Builder Engine:** Admin dapat membuat form yang berisi: Tabel, Textbox, Text Area, Checkbox, Multiple Checklist, File Upload, dan Take/Upload Picture.
*   **Visual Guide:** Admin dapat mengunggah gambar sebagai panduan (*guide sequence*) untuk teknisi.
*   **Workflow Config:** Admin bisa mengatur urutan form, alur persetujuan, dan siapa *approver*-nya.
*   **Lifecycle:** Form dapat di-*Preview* PDF, diduplikasi (Duplicate), diubah, dan di-*set* *Active/Inactive*.

### B. Modul User Input (Aplikasi Lapangan)
*   **Dashboard My Task:** Tampilan tugas berjalan dengan status.
*   **Filter/Search:** Filter berjenjang berdasarkan: *Work Type, Process Type, SN Prefix, Model, Part Number*.
*   **Multi-User & Sequential:** Form mendukung kolaborasi *(multi-user)* dan pengisian harus berurutan (*sequential*).
*   **Validasi:** Fitur wajib isi (*mandatory flag*) dengan *pop-up/icon warning* jika terlewat.
*   **Offline/Online capability:** Fitur *Save as Draft*.
*   **Export:** *Preview* hasil dalam PDF dan fitur *Export* ke Excel/PDF.

### C. Modul Report & Integrasi Eksternal
*   **Summary Report:** Dasbor yang melacak status seluruh dokumen form *(In progress, Waiting Foreman Review, Waiting Approval, Completed)*.
*   **Integrasi CAP:** Mengirim status/dokumen untuk persetujuan internal.
*   **Portal Kustomer:** Sistem *Email notification* berisi *secure URL/QR code* ke *landing page* minimalis khusus kustomer (*Approve/Revise*).

---

## 7. Integrasi Sistem Penyimpanan (SharePoint)
Aplikasi harus otomatis menyimpan file PDF final (dan *attachment*) ke SharePoint Service Engineering dengan hierarki *auto-folder*:
*   `Level 1: [Work Type]` (cth: /Disassemble/)
*   `Level 2: [Doc Type]` (cth: /Engine/)
*   `Level 3: [Year]` (cth: /2026/)

---

## 8. Arsitektur Teknis & Kapasitas (Technical Specifications)

*   **Kapasitas Pengguna:** ±250 Users.
*   **Jam Operasional:** 24 Jam.
*   **Infrastruktur (Existing Server):** CPU 4 Core, RAM 4 GB (Minimum).
*   **Backend Framework:** Minimal **.NET Core 8**
*   **Database:** **MS SQL Server 2019 - 2022**
*   **Frontend Web:** **AngularJS / jQuery / Bootstrap** (Responsive untuk Tablet & Mobile Web).
*   **Autentikasi:** SSO Microsoft **Office 365 (O365)**.