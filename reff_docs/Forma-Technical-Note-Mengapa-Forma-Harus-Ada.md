# Forma Technical Note: Mengapa Forma Harus Ada — Perspektif Setiap Pemangku Kepentingan Ekosistem

**Catatan internal — hasil diskusi tim, bukan bagian resmi Forma Core Spec**
**Status: bahan argumen inti untuk positioning & Investment Memorandum, melengkapi Technical Note Kedaulatan Data.**

---

## 0. Latar Belakang

Dokumen-dokumen sebelumnya membangun argumen dari sisi teknis (efisiensi, konvensi, sembilan primitif) dan dari satu sudut pandang spesifik (kedaulatan data, App Owner vs Workspace Owner). Dokumen ini menyusun ulang seluruh argumen itu per **pemangku kepentingan** — supaya jelas siapa sebenarnya yang dirugikan kalau Forma tidak ada, dan kenapa masing-masing dari mereka tidak bisa menyelesaikan masalahnya sendiri tanpa sesuatu seperti Forma.

Menariknya, enam pemangku kepentingan inti di bawah **persis memetakan** ke model aktor yang sudah dikunci di arsitektur Forma: Workspace Owner, App Owner, Module Owner, Cloud Owner — ditambah satu pihak di luar model aktor formal (pengguna individu) yang jadi penerima manfaat akhir dari seluruh arsitektur ini. Ini bukan kebetulan — pemetaan ini justru menunjukkan model aktor Forma memang dirancang dari kebutuhan nyata tiap pihak, bukan abstraksi teknis yang dipaksakan.

---

## 1. Pemilik Bisnis Kecil (Workspace Owner Tanpa Tim Teknis)

**Contoh konkret:** pemilik bengkel, klinik, barbershop — subjek utama diskusi kita tentang kedaulatan data dan skenario bengkel-vs-bengkel.

**Masalah hari ini:** dua pilihan yang sama-sama buruk. Manual (kertas/Excel) — rugi waktu, rawan human error, tidak ada insight. Custom dari developer lokal — lebih baik secara fungsi, tapi membuat mereka "telanjang" di hadapan satu pihak: developer yang sama itu bisa membangun aplikasi untuk kompetitor mereka di kota yang sama, dan secara teknis bisa melihat data siapa pun kliennya (lihat Technical Note Kedaulatan Data, bagian 1). Kalau developer itu berhenti kerja atau bisnisnya tutup, aplikasi mereka juga ikut mati — tidak ada jalan lain melanjutkan tanpa developer yang sama.

**Kenapa Forma menjawab ini:** isolasi Workspace membuat App Owner secara struktural tidak bisa mengintip data mereka tanpa consent eksplisit (Technical Note Kedaulatan Data, bagian 2). Konvensi yang seragam berarti kalau developer mereka menghilang, developer Forma lain bisa mengambil alih perawatan aplikasi tanpa belajar ulang dari nol — beda dari kode bespoke yang cuma dipahami satu orang. Forma Cloud, sebagai operator netral, memberi mereka opsi yang benar-benar aman tanpa perlu punya tim infra sendiri.

**Tanpa Forma, alternatifnya:** tetap terjebak antara manual dan custom yang berisiko, atau beralih ke SaaS vertikal jadi (kalau ada) yang kaku, tidak bisa dikustom sesuai kebutuhan spesifik mereka.

**Batasan jujur:** argumen kedaulatan data ini paling kena untuk bisnis yang punya kompetitor di ekosistem yang sama (persis skenario bengkel-vs-bengkel). Untuk bisnis tunggal tanpa kompetitor langsung, nilainya tetap benar secara prinsip tapi kurang jadi pemicu keputusan beli yang konkret (sudah dicatat di Technical Note Kedaulatan Data, bagian 6).

---

## 2. Pemilik Bisnis Besar (Workspace Owner dengan Tim Infra Sendiri)

**Contoh konkret:** perusahaan menengah-besar, institusi keuangan, perusahaan dengan tim IT internal yang mumpuni.

**Masalah hari ini:** mereka *bisa* membangun sendiri atau menyewa tim besar untuk custom development, tapi menghadapi dua tekanan sekaligus — (1) tekanan kecepatan dari era AI coding yang membuat tim mereka menghasilkan lebih banyak kode lebih cepat, tapi berisiko menumpuk utang teknis dan celah keamanan yang tidak konsisten ditangani; (2) tekanan governance — auditor, regulator, atau klien mereka sendiri menuntut bukti kontrol yang jelas (siapa yang approve deployment, apakah ada audit trail, apakah ada pemisahan tugas) yang mahal dibangun dari nol.

**Kenapa Forma menjawab ini:** sembilan primitif infrastruktur dan konvensi spec-first langsung menjawab risiko teknis dari AI coding (bagian 2 Investment Memorandum). Forma Control dengan prinsip "no self-approval," signing artefak, dan audit immutable memberi mereka governance jadi tanpa membangun sendiri dari nol. Dan ini poin baru dari diskusi kita: mereka menghadapi pilihan yang sama seperti memilih MSSP spesialis dibanding SOC internal — mempercayakan operasional ke pihak yang seluruh reputasi bisnisnya bergantung pada tidak pernah gagal, sering kali lebih aman daripada tim internal yang disiplin operasionalnya lebih longgar karena punya banyak prioritas lain (Technical Note Kedaulatan Data, bagian 12).

**Tanpa Forma, alternatifnya:** membangun governance internal sendiri (mahal, lambat, sering kali baru dibangun setelah insiden pertama), atau bergantung ke ERP incumbent besar (SAP, Oracle) yang kaku dan mahal untuk kebutuhan yang sebenarnya bisa lebih sederhana.

**Batasan jujur:** klien di tier ini butuh bukti nyata (sertifikasi SOC 2/ISO 27001, rekam jejak) sebelum mempercayakan operasional kritis — sesuatu yang belum bisa diklaim Forma di tahap awal (M1–M4), harus dibangun lewat waktu.

---

## 3. Developer Kecil / Solo Developer (App Owner Individual)

**Contoh konkret:** developer lepas atau baru lulus yang ingin membangun aplikasi bisnis untuk klien lokal (persis model GTM "komunitas developer jual ke bengkel/klinik" yang kita bahas).

**Masalah hari ini:** untuk membangun aplikasi bisnis yang benar-benar production-grade (tenancy, audit, keamanan, observability), seorang solo developer harus menguasai dan membangun sendiri infrastruktur yang biasanya jadi domain tim besar — atau mengambil jalan pintas yang berisiko (skip validasi, skip audit) karena keterbatasan waktu/skill. Hasilnya sering kali aplikasi yang "cukup jalan" tapi rapuh begitu klien mereka tumbuh.

**Kenapa Forma menjawab ini:** sembilan primitif dan konvensi seragam membuat solo developer bisa menghasilkan output setara tim yang jauh lebih besar — mereka tidak perlu tahu detail implementasi distributed lock atau reliable event delivery, cukup tahu kapan pakai `ctx.lock` atau `publish.durable: true`. Starter template vertikal (`forma/clinic`, dan yang perlu ditambahkan seperti `forma/bengkel`) mempercepat mereka mulai dari sesuatu yang sudah teruji, bukan dari nol. Forma Agent Skill membuat AI assistant yang mereka pakai untuk ngoding tidak lupa aturan penting (event finansial wajib durable, dll).

**Tanpa Forma, alternatifnya:** pakai framework generik (Laravel, Express) yang tidak punya konvensi bisnis bawaan, sehingga tetap harus membangun ulang tenancy/audit/observability sendiri — atau pakai low-code/no-code yang membatasi kustomisasi begitu kebutuhan klien jadi spesifik.

**Batasan jujur:** solo developer di tahap awal masih harus belajar konvensi Forma itu sendiri — ada barrier onboarding yang harus terus diturunkan (dokumentasi, tutorial, `forma dev` yang mulus) sebelum benar-benar jadi jalan pintas, bukan beban tambahan.

---

## 4. Developer Besar / Software House (App Owner Skala Tim)

**Contoh konkret:** software house yang melayani banyak klien sekaligus, kemungkinan lintas-vertikal atau bahkan klien yang saling bersaing.

**Masalah hari ini:** dua sisi mata uang. Di satu sisi, mereka butuh efisiensi tim (konsistensi kode antar developer, onboarding developer baru cepat). Di sisi lain — ini poin yang sering tidak disadari sampai terlambat — **mereka sendiri menanggung risiko hukum dan reputasi** kalau salah satu developer mereka (sengaja atau tidak) membocorkan data satu klien ke klien lain. Ini bukan cuma masalah etika, ini eksposur liabilitas nyata bagi software house itu sendiri.

**Kenapa Forma menjawab ini:** isolasi Workspace yang sama yang melindungi pemilik bengkel (bagian 1) ternyata **juga melindungi software house itu sendiri** — karena secara arsitektur, developer mereka tidak pernah punya akses lintas-klien secara default, software house tidak bisa "kebocoran" data yang secara teknis tidak pernah mereka pegang. ini mengubah argumen isolasi Workspace dari "kewajiban kepatuhan" jadi "perlindungan liabilitas" bagi App Owner sendiri — pembingkaian yang belum eksplisit di dokumen sebelumnya. Verified Badge dan marketplace modul juga mengurangi kerja berulang mereka membangun integrasi yang sama (payment gateway, dll) untuk tiap klien.

**Tanpa Forma, alternatifnya:** membangun kontrol akses internal sendiri (butuh disiplin proses yang konsisten, mudah bolong seiring tim tumbuh) atau menanggung risiko itu diam-diam — dan berharap tidak pernah ada insiden.

**Batasan jujur:** ini butuh software house benar-benar memahami dan mempercayai model isolasi ini — kalau mereka masih menyimpan kebiasaan lama (akses admin manual ke semua database klien "untuk jaga-jaga"), manfaat perlindungan liabilitas ini hilang karena mereka sendiri yang membuka jalan pintasnya.

---

## 5. Vendor/Modul Pihak Ketiga (Module Owner)

**Contoh konkret:** penyedia payment gateway (Midtrans, Xendit), SMS/WhatsApp API, atau penyedia modul kepatuhan lokal.

**Masalah hari ini:** untuk menjangkau banyak developer/software house, vendor harus membangun integrasi terpisah untuk tiap framework atau bahkan tiap developer, dan tidak punya cara mudah membuktikan kualitas integrasinya ke calon pengguna baru.

**Kenapa Forma menjawab ini:** Verified Badge Program memberi mereka jalur distribusi terpercaya — sekali terverifikasi, terlihat kredibel ke seluruh ekosistem developer Forma, bukan harus membangun kepercayaan satu per satu. Mockup System (dengan contract test yang membandingkan mockup vs implementasi asli) mengurangi beban dukungan mereka ke developer yang integrasinya salah pakai.

**Tanpa Forma, alternatifnya:** bergantung ke reputasi brand mereka sendiri dan dokumentasi API konvensional — jalur yang sudah mereka jalani sekarang, jadi ini bukan "masalah mendesak" bagi mereka sampai ekosistem Forma cukup besar untuk sepadan investasi integrasi.

**Batasan jujur:** dari tabel "Tahap Ideal" Investment Memorandum, ini realistis baru bernilai "setelah komunitas inti terbentuk" — vendor tidak akan invest integrasi ke ekosistem yang masih kecil.

---

## 6. Pengguna Individu / Pelanggan Akhir (Pemegang Forma ID)

**Contoh konkret:** orang yang jadi pelanggan tetap di bengkel, langganan klinik, member barbershop — subjek dari diskusi Forma ID.

**Masalah hari ini:** mereka mengisi data yang sama berulang-ulang di tiap bisnis yang mereka datangi, tidak ada kendali granular atas siapa yang tahu apa tentang mereka, dan riwayat aktivitas mereka tersebar di banyak sistem yang tidak saling bicara.

**Kenapa Forma menjawab ini:** Forma ID (Technical Note Kedaulatan Data, bagian 4) memberi mereka satu identitas yang bisa dipakai lintas bisnis dengan consent granular per akses — bukan otomatis semua data terbuka. Prinsip "Jalur B" yang sudah dikunci (fee dari fasilitasi consent, bukan dari jual data perilaku) berarti kepentingan mereka tidak dikorbankan demi monetisasi Forma ID.

**Tanpa Forma, alternatifnya:** terus mendaftar manual di tiap tempat, atau — skenario yang lebih buruk — masuk ke ekosistem loyalty/tracking komersial yang justru memonetisasi data perilaku mereka tanpa transparansi yang sama.

**Batasan jujur:** pihak ini tidak punya kuasa langsung memilih Forma — mereka ikut karena bisnis yang mereka datangi memilih Forma. Nilai bagi mereka baru terasa nyata kalau sudah banyak bisnis di sekitar mereka ikut ekosistem yang sama (masalah chicken-and-egg yang sudah dicatat sebelumnya).

---

## 7. Pemangku Kepentingan Tambahan — Menjawab "Siapa Lagi"

**Pemerintah/regulator (secara tidak langsung).** Ekosistem UMKM yang terformalkan lewat pembukuan digital dan modul kepatuhan (`forma/locale-id` untuk PPN/PPh/e-Faktur/BPJS) mendukung agenda digitalisasi UMKM dan kepatuhan pajak yang selama ini sulit dijangkau bisnis kecil yang masih manual. Ini bukan pengguna langsung Forma, tapi pihak yang kepentingannya sejalan — bisa jadi alasan tambahan untuk kemitraan (mis. dengan asosiasi UMKM atau program pemerintah), meski tidak boleh dijadikan klaim utama tanpa validasi kemitraan nyata.

**Karyawan/staf operasional bisnis kecil.** Ini pengguna harian sebenarnya dari aplikasi yang dibangun di atas Forma (kasir bengkel, resepsionis klinik) — bukan pemilik bisnisnya. Kebutuhan mereka (PWA tanpa install, UI sederhana) sudah masuk pertimbangan desain frontend, tapi mereka sendiri bukan pengambil keputusan pembelian — kepentingan mereka terwakili lewat pemilik bisnis (bagian 1).

**Investor** — ini kategori berbeda dari lima pemangku kepentingan operasional di atas. Investor tidak "butuh Forma ada" untuk menyelesaikan masalah operasional mereka sendiri, tapi mereka butuh keenam argumen di atas **koheren dan saling menguatkan** sebagai bukti bahwa Forma bukan solusi mencari masalah — enam pihak berbeda ini punya alasan independen untuk butuh Forma, dan itu yang membuat thesis investasi lebih kuat dibanding produk yang cuma punya satu jenis pengguna.

---

## 8. Sintesis: Kenapa Enam Pihak Ini Saling Menguatkan, Bukan Berdiri Sendiri

Ini bagian yang penting untuk pitch investor: masing-masing pemangku kepentingan di atas punya alasan **independen** untuk butuh Forma — tidak ada yang cuma ikut-ikutan karena pihak lain pakai. Tapi begitu digabung, mereka membentuk lingkaran yang saling menguatkan:

```
Developer kecil/besar butuh Forma → makin banyak aplikasi vertikal dibangun
      → makin banyak pemilik bisnis kecil punya opsi selain manual/custom berisiko
      → makin banyak Workspace aktif → makin bernilai bagi vendor (Module Owner)
      untuk integrasi Verified Badge → makin lengkap ekosistem modul
      → makin menarik bagi developer baru bergabung (kembali ke awal siklus)

Sementara itu:
Pemilik bisnis besar butuh governance Forma Control → jadi rujukan kredibilitas
      → memperkuat kepercayaan pemilik bisnis kecil ke Forma Cloud
      → Forma ID makin bernilai begitu cukup banyak bisnis (kecil & besar) ikut ekosistem
```

Ini bukan klaim network effect yang taken-for-granted — ini konsekuensi logis dari enam kebutuhan independen yang saling terhubung lewat platform yang sama. Kalau salah satu pihak hilang (mis. tidak ada developer yang mau pakai Forma), seluruh lingkaran ini tidak terbentuk — itu sebabnya urutan milestone (M1–M5 di Investment Memorandum) dimulai dari developer/komunitas dulu, baru menyusul lapisan lain.

---

## 9. Siapa yang Kemungkinan TIDAK Butuh Forma — Kejujuran yang Perlu Diakui

Supaya dokumen ini tidak overclaim, penting mencatat siapa yang argumen di atas **tidak** kena:

- **Bisnis sangat kecil tanpa niat digitalisasi sama sekali** (masih nyaman manual, tidak melihat urgensi) — argumen kedaulatan data tidak relevan kalau mereka bahkan belum mau pakai software apapun.
- **Perusahaan dengan software custom yang sudah matang dan puas** — biaya migrasi ke Forma harus lebih kecil dari manfaat yang didapat; tidak semua akan melihat itu sepadan, terutama kalau sistem lama mereka sudah battle-tested.
- **Developer yang sudah sangat nyaman dengan stack sendiri** (budaya anti-framework di komunitas Go, sudah dicatat sebagai risiko di bagian 10 Investment Memorandum) — bagi mereka, argumen efisiensi Forma justru terasa seperti pembatasan, bukan bantuan.
- **Bisnis yang justru butuh visibilitas lintas-tenant sebagai fitur utama** (mis. platform benchmark data yang sengaja menjual insight agregat) — model ini bertentangan langsung dengan prinsip isolasi Workspace, jadi bukan target pasar yang cocok untuk Forma sebagai infrastruktur inti mereka.

---

## 10. Pertanyaan Terbuka

- Bagian mana dari dokumen ini yang paling pas masuk revisi Investment Memorandum — apakah jadi bagian tersendiri, atau disebar ke bagian Diferensiasi (per-persona) dan Visi & Misi (sintesis di bagian 8)?
- Argumen "isolasi Workspace melindungi App Owner dari liabilitas hukum" (bagian 4) ini cukup kuat untuk jadi pitch tersendiri ke software house — perlu divalidasi dengan software house sungguhan apakah ini benar-benar dianggap masalah nyata oleh mereka, atau cuma masuk akal secara teori.
- Apakah kemitraan dengan pemerintah/asosiasi UMKM (bagian 7) layak dieksplorasi sebagai jalur distribusi tambahan, atau terlalu dini dibahas sebelum ada traksi organik?
- Perlu riset lebih lanjut: dari enam pemangku kepentingan ini, mana yang paling realistis jadi early adopter pertama di M1–M3 — apakah urutan "developer dulu, baru bisnis kecil, baru bisnis besar" di Investment Memorandum masih paling masuk akal setelah pemetaan lebih detail ini?

---

*Dokumen ini adalah rangkuman kerja dari sesi diskusi. Tujuannya menyimpan alur penalaran dan argumen inti agar tidak hilang. Bukan keputusan final — perlu direview sebelum masuk sebagai revisi resmi Investment Memorandum.*
