# Marketplace

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku.

## 1. Artifact yang Didistribusikan
Satu infrastruktur marketplace untuk seluruh artifact terdistribusi:
Module, App template, dan artifact visual — **VisualSpecKind** (jenis view
baru) serta **Renderer** (implementasi view,
[`../frontend/03-renderer-kind.md`](../frontend/03-renderer-kind.md)) —
komunitas boleh menerbitkan keduanya, juga **PersistBackend**
([`../backend/04-persist-backend.md`](../backend/04-persist-backend.md) §7).
Semuanya artifact bertanda tangan di registry yang sama; marketplace adalah
**lapisan listing dan metering** di atas registry, bukan pengganti registry.

Ketentuan komersial (harga, revenue share, masa trial) hidup di **listing**,
tidak pernah di manifest — module yang sama boleh gratis+self-hosted lewat
binary FSL, atau berbayar di marketplace dengan revenue sharing, tanpa
perubahan YAML apa pun.

## 2. Trust Tier
`official` / `verified` / `community` — berlaku seragam untuk Module,
Renderer, PersistBackend. **Verified Badge** wajib untuk listing berbayar
apa pun: artifact ditandatangani kunci ed25519 vendor, signature chain
terverifikasi terhadap public key vendor di registry, plus proses review
(manual atau otomatis, kebijakan Platform Operator).

Manifest **tidak pernah dienkripsi** (keterbacaan adalah fitur — consent,
`formspec validate`, review diff, AI). Proteksi nilai komersial datang dari:
(1) IP sesungguhnya boleh biner lewat `impl` native/compiled tanpa source;
(2) signed provenance — salinan yang di-rename tidak bisa memalsukan
signature dan tidak bisa masuk environment governed Verified-only; (3)
ekonomi update+support+liability tidak ikut berpindah bersama salinan; (4)
proteksi legal lewat lisensi module.

**Trust tier menggerbang tipe `impl` yang boleh dijalankan.** Pertahanan
sesungguhnya untuk handler biner adalah *provenance*, bukan sandbox
sempurna: enforcement native bersifat *best-effort* (kode in-process
dipercaya penuh — kredensial di-broker tapi memori bisa diinspeksi; ini
hukum alam semua plugin biner). Karena itu tipe impl digerbangi trust tier:
unverified/gratis **hanya** boleh sandbox (`script`/`script_ref`/WASM);
**Verified Badge** menambah `sidecar` (terisolasi proxy); **Verified +
security scan + approval berlapis** menambah `native`/`compiled`. Injeksi
tenant di lapisan `ctx.*` membatasi blast-radius ke workspace yang consent;
pemulihan lewat audit + backup eksternal
([`04-control-plane.md`](04-control-plane.md) §6.1).

## 3. Penerbitan dan Instalasi
**Syarat listing:** listing gratis — signature ed25519 valid; listing
berbayar — Verified Badge + signature valid; semua listing — metadata
artifact publik (nama, vendor, versi, deskripsi, dependency graph,
footprint permission).

**Alur instalasi:** `formspec module install` menampilkan footprint permission
(agregat `required_permission` + `uses`) untuk consent Workspace Owner
sebelum instalasi selesai
([`04-control-plane.md`](04-control-plane.md) §3 consent gate). Update versi
yang memperluas footprint memicu re-consent.

**Instalasi tidak sama dengan aktivasi (normatif).** `formspec module install`
mem-fetch artifact ke `vendors/`
([`08-project-layout.md`](08-project-layout.md) §6.1), mencatat provenance
di `formspec.lock` (§6.2), dan menulis entri **ter-nonaktif** (marker blok
ter-comment) di manifest App
([`02-workspace-app-module.md`](02-workspace-app-module.md) §2.1,
[`08-project-layout.md`](08-project-layout.md) §6.3) — bukan otomatis aktif.
Ini yang membuat bundle vendor (satu source, banyak module sekaligus) aman
di-install: semua ter-download, tapi hanya module yang eksplisit
diaktifkan developer yang masuk registry, kena permission graph, dan kena
license gate (§9) — sisanya diam di disk tanpa konsekuensi apa pun. Flag
`formspec module install <source> --use` melewati dua langkah ini — langsung
menulis entri ter-aktif.

Re-install/update (mis. naik versi) **tidak boleh** mengubah status
aktif/nonaktif entri yang sudah ada — hanya versi di dalam marker dan
entri `formspec.lock` yang diperbarui; status aktivasi adalah properti file
yang dijaga (preserved), bukan digenerate ulang. Detail mekanisme folder,
format marker, dan idempotensi ada di
[`08-project-layout.md`](08-project-layout.md) §6.

**Uninstall bersih** — kontrak "extension harus bisa di-uninstall tanpa
sisa" ada di [`../backend/03-entity-extension.md`](../backend/03-entity-extension.md)
§2; marketplace mewajibkan setiap artifact terdaftar bisa dilepas lewat
jalur itu, bukan meninggalkan sisa yang cuma bisa dibersihkan manual.

**Peran Platform Operator:** mengoperasikan infrastruktur marketplace,
menetapkan persentase fee platform per kategori, memfasilitasi settlement
antara vendor dan konsumen. **Tidak** menetapkan harga per-module — vendor
mengontrol harganya sendiri.

**Unit lisensi (normatif).** Lisensi menempel ke **Module + environment**,
tidak pernah ke App. App = resep komposisi (menu, Auth, Theme, referensi
Module — [`02-workspace-app-module.md`](02-workspace-app-module.md) §3) dan
**selalu gratis**; ia tidak pernah punya skema lisensi sendiri. Konsekuensinya:
begitu sebuah Workspace punya lisensi produksi untuk Module M, M bebas
dipakai App mana pun dalam Workspace itu — termasuk App custom yang
me-remix M. Menjual-ulang App (App-reselling) bukan pelanggaran: selama
Module di dalamnya tetap Module bertanda-tangan asli, Module Owner tetap
menerima fee di titik gate produksi.

**Gate lisensi hanya di production.** Module bebas dipakai tanpa gate di
environment non-produksi (`development`/`test`/`staging`,
[`04-control-plane.md`](04-control-plane.md) §2) — gate lisensi aktif
hanya saat label environment adalah `production`, dievaluasi di titik
`formspec promote --to production`
([`10-deployment-operations.md`](10-deployment-operations.md) §5), bukan
di dalam handler saat runtime.

## 4. Pricing Vocabulary (Closed)
Marketplace **tidak boleh** mengizinkan model harga custom di luar
vocabulary tertutup ini:

| Model | Unit |
|---|---|
| `free` | — |
| `one_time` | Per lisensi (perpetual) |
| `subscription` | Per periode (bulanan/tahunan) |
| `per_seat` | Per membership user aktif |
| `per_call` | Per panggilan grant lintas-app |
| `per_transaction` | Per event metered (hitungan saja) |
| `metered_passthrough` | Data metering mentah, vendor set tarif sendiri |

## 5. Verifiable Metering
Tiga jaminan yang membuat metering **verifiable oleh semua pihak terhadap
operator** (tidak butuh percaya ke Platform Operator): (1) **hitungan
saja** — rekaman metering tidak pernah berisi payload data bisnis, Control
Plane tidak pernah tahu apa yang dibeli, cuma tahu N transaksi terjadi; (2)
**ditandatangani Resource Plane** — tiap rekaman metering ditandatangani
kunci instance; (3) **dijangkarkan di transparency log** — batch metering
masuk evidence channel dan dijangkarkan
([`05-plane-protocol.md`](05-plane-protocol.md) §4.4,
[`04-control-plane.md`](04-control-plane.md) §7).

## 6. Ledger & Settlement
Satu **ledger per owner** (Workspace Owner dan Module Vendor punya ledger
terpisah): sisi debit (pemakaian infra, langganan module/app, charge
per-seat/per-call), sisi kredit (top-up prepaid, payout marketplace untuk
vendor). Pendapatan vendor mengurangi tagihan infra-nya sendiri.

**Model settlement:** prepaid (default — top-up saldo lokal, saldo habis →
masa tenggang → degradasi read-only; `list/find/export/backup` tidak
pernah digerbang) untuk semua akun; postpaid (tier trust — invoice/PO,
net terms) untuk enterprise/vendor terverifikasi.

**Budget cap:** Workspace Owner menetapkan batas anggaran — charge berulang
dalam batas itu auto-approve (top-up prepaid = persetujuan billing itu
sendiri); di atas batas butuh approval eksplisit.

## 7. Revenue Sharing
Berbasis **dependency graph**: edge dependency diketahui dari deklarasi
`depends`/`depends_on` di manifest Module
([`02-workspace-app-module.md`](02-workspace-app-module.md) §2, §7).
Metering melacak pemakaian per dependency; revenue share dihitung sesuai
ketentuan listing (bukan manifest); payout dikreditkan ke ledger vendor.

## 8. Versi dan Kompatibilitas
Semver artifact — dependency Module (`depends`) menyatakan rentang versi
(`">=1.0 <2.0"`). Kompatibilitas Renderer terhadap `stack_family` Shell
diatur di [`../frontend/03-renderer-kind.md`](../frontend/03-renderer-kind.md)
§1–§2 — marketplace memverifikasi field itu ada dan konsisten saat listing
diterbitkan, tapi resolusi kompatibilitasnya sendiri kontrak frontend, bukan
kontrak marketplace.

Seluruh pricing model resolve ke satu **license token** dengan tipe dan
masa berlaku (`free`/`one_time` → perpetual; `subscription` → rolling,
expire kalau tidak diperpanjang; `per_seat`/`per_call`/`per_transaction` →
valid selama saldo/prepaid mencukupi). Token **wajib** divalidasi lokal
oleh Resource Plane (tanpa call-home, air-gap safe), **wajib** ditolak kalau
mencoba menggerbang `list/find/export/backup`, dan portable — dokumen
bertanda tangan yang bisa diverifikasi implementasi konform manapun.

## 9. Guardrail Tetap (non-negotiable)
Degradasi read-only + export tidak pernah digerbang; Verified Badge wajib
untuk listing berbayar; token portable & air-gap safe; jalur free/self-hosted
FSL tetap utuh; module yang sama boleh gratis-self-hosted dan berbayar-
marketplace tanpa perubahan manifest.
