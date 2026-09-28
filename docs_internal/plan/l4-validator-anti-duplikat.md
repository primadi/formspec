# Plan — L4: validator anti-duplikat + migrasi 83 deklarasi ganda

**Plan induk:** `docs_internal/plan/via-sebagai-action-penuh.md` (L4).
**Prasyarat:** L1 ✅ (field diserap), L2 ✅ (registry union), L3 ✅ (route terdaftar).
**Status:** rencana disetujui-untuk-dikerjakan (belum ada perubahan kode L4).
**Ledger kafe:** **10.51** (L4/L5/L7).

---

## 1. Mengapa L4 tidak sesederhana "hapus duplikat"

Rencana induk menulis L4 = _"Tolak `actions[].name == transitions[].via`"_ lalu
_"migrasi 85 deklarasi"_. Pengukuran 2026-09-27 menunjukkan penolakan buta akan
**merusak otorisasi di 36 dari 77 kasus**, dan bahwa satu kategori justru harus
**diizinkan**.

Tiga measurement, semuanya dijalankan atas repo nyata:

| #   | Pertanyaan                                          | Hasil                                                               |
| --- | --------------------------------------------------- | ------------------------------------------------------------------- |
| M1  | Field apa yang dimiliki deklarasi ganda?            | 83 total: 76 `required_permission`, 11 `uses`, 2 `params`, 9 bersih |
| M2  | Apakah permission route berubah bila entri dihapus? | **41 tidak berubah · 36 berubah**                                   |
| M3  | Berapa duplikat yang memakai nama action reserved?  | 1 nama (`cancel`), **6 kemunculan**                                 |

**Jebakan metodologis yang sempat menipu saya (dicatat supaya tidak terulang):**
pengukuran M2 versi pertama melaporkan **76 dari 77 berubah**. Itu **salah** —
skrip saya memakai `strings.Contains(perm, ".")` sebagai tes "sudah qualified",
padahal aturan sebenarnya (`internal/permission.AutoPrefixPermission`) **juga
mem-prefix nilai 2-segmen**. `visits.cancel` → `clinic.visits.cancel` di kedua
jalur, jadi tidak ada perubahan. Setelah memakai fungsi aslinya, hasilnya 41/36.
**Pelajaran: alat ukur harus memanggil fungsi produksi, bukan meniru logikanya.**

Perbedaan antara nilai hari-ini dan nilai setelah migrasi, menurut generator
(`internal/api/generator.go`):

```
perm := action.RequiredPermission
if perm == "" { perm = module + "." + plural + "." + action.Name }
```

---

## 2. Tiga kategori duplikat (dasar percabangan validator)

### Kategori A — deklarasi murni, aman dihapus (**~77 − 6 − 36 = 35**)

Contoh: `examples/kafe/spec/modules/cafe-master/master/dining-table/entity.yaml`.
`actions:` hanya mengulang `name` + `description` yang **sudah ada di transisi**;
tanpa `required_permission` (gate-nya di transisi). Menghapusnya tidak mengubah
route, permission, maupun perilaku.

**Aksi: hapus entri action. Nol perubahan perilaku.**

### Kategori B — duplikat yang **mengubah permission** bila dihapus (**36**)

Tiga sub-pola, semuanya temuan nyata:

**B1 — mempersempit permission agar operasi admin tidak bisa dijalankan pemegang `update`.**
`examples/cafe/spec/modules/cafe-order/transaction/order/entity.yaml`:

|                                        | permission                               |
| -------------------------------------- | ---------------------------------------- |
| action `cancel` (hari ini)             | `cafe-order.order.cancel` (**singular**) |
| fallback `{module}.{plural}.{action}`  | `cafe-order.orders.cancel` (**plural**)  |
| route lifecycle generik untuk `cancel` | `cafe-order.orders.cancel`               |

Jadi entri action itu **menyengaja** mempersempit dari plural → singular. Alasan
historisnya kuat: route lifecycle generik memakai `{plural}.cancel`, jadi tanpa
entri tersebut **setiap pemegang `update` bisa membatalkan pesanan ber-uang**.
Bentuknya juga **tidak konsisten** dengan `settle` di entitas yang sama
(`cafe-order.order.settle`, singular, karena `settle` bukan nama lifecycle jadi
tidak punya route generik).
**Keputusan yang dibutuhkan pemilik:** mana yang benar, `{module}.{action}` atau
`{module}.{plural}.{action}`? Setelah diputuskan, semua 36 diselaraskan dan
barulah duplikatnya bisa dihapus. Sebelum itu, menghapus = melebarkan akses.

**B2 — dua action memakai SATU permission yang sama (tabrakan).**
`examples/Clinic-UI-Showcase/spec/modules/pharmacy/transaction/prescription/entity.yaml`:

```yaml
- name: start-compounding
  required_permission: prescriptions.compound # → pharmacy.prescriptions.compound
- name: mark-ready
  required_permission: prescriptions.compound # → pharmacy.prescriptions.compound
```

Dua action berbeda, satu permission. Artinya **`mark-ready` tidak terbedakan
dari `start-compounding` untuk otorisasi** — tidak ada cara memberi satu tanpa
yang lain. Setelah migrasi, keduanya jadi
`pharmacy.prescriptions.{start-compounding,mark-ready}` dan **justru bisa
dipisah** — jadi migrasi **memperbaiki** ini. Tapi memperbaiki berarti
**mengubah** himpunan siapa-yang-boleh: pemegang `prescriptions.compound` akan
kehilangan akses ke kedua action kecuali grant-nya diperbarui.

**B3 — nama pluralisasi salah, sehingga permission bergantung pada fallback.**
`cmd/formspec-registry/.../module-version.yaml`: `plural: moduleversion`
(tanpa dash) → permission jadi `registry.moduleversion.deprecate`, sedangkan
`name` adalah `module-version` dan fallback memberi `registry.module-versions.*`.
Akar masalahnya `plural` yang salah, bukan duplikasinya. **Perbaiki `plural`
dulu**, lalu duplikatnya boleh dihapus.

**Aksi: butuh keputusan + grant diperbarui. Bukan murni mekanis.**

### Kategori C — duplikat yang **HARUS tetap diizinkan** (**6**, nama `cancel`)

`transitions[].via: cancel` bertemu entri `actions: [{name: cancel, required_permission: …}]`.
`cancel` adalah **action reserved** dengan route lifecycle **generik** dari
`generateRESTRoutes` (`{module}.{plural}.{action}`, `entity.go:222`).

Validator yang menolak buta akan **menghapus satu-satunya cara mempersempit
permission action lifecycle** — dan itu **regresi otorisasi langsung** pada enam
entitas. Karena itu validator **wajib** memiliki pengecualian untuk
`spec.ReservedActionNames`.

**Aksi: pengecualian eksplisit di validator + alasan tertulis di komentar.**

---

## 3. Bentuk validator ✅ mendarat 2026-09-27

`ValidateActionTransitionDuplication(d *EntitySpec) error` — dipanggil dari
`ValidateEntitySpec` (setelah `ValidateTransitionPermissions`).

| kondisi | hasil |
| --- | --- |
| `via` tidak ada di `actions` | ✅ sah (inti L3) |
| `via` kosong | ✅ sah (keputusan C) |
| `actions[].name` ∈ `ReservedActionNames` | ✅ **dikecualikan** — action lifecycle punya route generik; entri nyata bisa mempersempit/mengganti permission-nya |
| entri membawa `impl` yang tidak dimiliki transisi | ✅ dikecualikan — entri itulah sumber handler route |
| entri membawa `required_permission` ≠ gate transisi | ✅ dikecualikan — route akan memakai nama entri |
| nama sama, entri **tidak** menambahkan apa pun | ❌ **ditolak** |

Pengunci: `pkg/spec/duplication_test.go` (4 test), terkalibrasi — gagal saat
penolakan dinetralkan, dan gagal dengan "`occupy` appears 3 times" saat dedup
`ActionSources()` dihapus.

---

## 4. 🔴 Pembalikan analisis: pertanyaannya bukan "mana yang benar"

Bagian 2 menyimpulkan migrasi Kategori B **butuh keputusan pemilik** — plural
atau singular. **Pembacaan `docs/spec/backend/01-core-basic.md` §8.6
membatalkan premisnya.** Spec itu **normatif**:

> Bentuk kanonik satu-satunya adalah `{module}.{plural}.{action}` ... Bentuk
> **singular** (`{module}.{entity}.{action}`) **bukan** varian yang setara dan
> tidak pernah cocok — registry mendaftarkan plural, jadi permission berbentuk
> singular hanya menghasilkan 403 yang sulit dilacak.

**Terukur:** dari 21 permission yang dideklarasikan, **20 berbentuk singular**
(melanggar §8.6); hanya **1** kanonik. Jadi tidak ada keputusan desain yang
terbuka — yang ada **dua cacat yang harus diperbaiki**, dan urutannya
kebalikan dari tebakan awal.

### Dua cacat terpisah

**(a) Route action kustom tidak pernah mendaftarkan permission-nya.** Registry
kaya, route miskin (terukur):

| sumber | `cafe-order.orders.submit-order` |
| --- | --- |
| blob grant (`entityFootprint`) | ✅ ada |
| `authorized_actions` (bundle, supervisor) | `[list, find, create, update]` — **tidak ada** |
| registry permission (`registerStandardPermissions`) | **tidak didaftarkan** (hardcoded list/view/create/update/delete/read_all/submit/cancel/amend + soft-deactivate) |

Akibat: nama yang diturunkan dari konvensi dan nama yang didaftarkan berbeda.

**(b) 50 dari 77 duplikat tidak punya `impl`** → tidak ada route → maka
`required_permission`-nya terdaftar dan bisa di-grant, tetapi **tidak pernah
dieksekusi**. Kelas yang sama dengan 10.46, pada action alih-alih gate
transisi. Sementara **27 duplikat punya `impl`** (route nyata → permission
benar-benar ditegakkan).

**Kesimpulan urutan:** perbaiki **(a) dulu** — jadikan bentuk permission
kanonik (plural) di generator, daftarkan permission action kustom, dan tambahkan
validator yang menolak bentuk singular sesuai §8.6. Baru setelah itu migrasi
Kategori B kehilangan bahayanya: yang berubah hanyalah nama, menuju bentuk yang
spec sudah nyatakan wajib.

---

## 5. Item terbuka (temuan lanjutan 2026-09-27)

| # | Item | Ukuran |
| --- | --- | --- |
| O1 | Route action kustom mendaftarkan bentuk kanonik `{module}.{plural}.{action}` | small |
| O2 | `registerStandardPermissions` mendaftarkan permission action kustom (registry = route) | small |
| O3 | Validator menolak `required_permission` berbentuk singular (§8.6) | small |
| O4 | Validator menolak entri `actions:` yang menamai action tidak-ada di halaman yang di-grant (10.47) — butuh materializer di `check` | medium |
| O5 | 20 manifest diperbaiki ke bentuk plural + grant diselaraskan | medium |
| O6 | ~~50 action impl-less: putuskan bagaimana `required_permission` ditegakkan~~ **✅ sebagian 2026-09-27** — check `check_ungated.go` memperingatkan bentuk ini, dan **7 gate kafe dipindahkan ke transisi** (lubang uang `confirm-payment`/`refund`/`settle`/`close-shift` + pemisahan peran dapur/pelayan). Warning kafe 21 → 10, lalu **21 → 0** (`-016`: 10 warning terakhir tuntas; semua gate pindah ke transisi) | medium |
| O9 | *(masih terbuka, di luar kafe)* **43 transisi tanpa gate sama sekali** di `examples/` + `verticals/` (15 file) — diotorisasi `{plural}.update` saja. Check hanya menangkap kontradiksi, bukan omisi. Termasuk `arisan/draw.mark-paid`, `stock-opname.post-opname`, `billing/order.void`, 11 transisi approval CRC | medium |
| O7 | `buildTransitionIndex` menimpa pada tabrakan `via`; `ActionSources()` memakai yang pertama — selaraskan | small |
| O8 | `cancel` yang genuinely tidak ada di 2 entitas (grant error?) | small |

---

## 6. Risiko

| Risiko | Mitigasi |
| --- | --- |
| Menghapus entri Kategori B = melebarkan akses ke pemegang `update` | Jangan migrasi sebelum O1–O3; setelah bentuk kanonik didaftarkan, migrasi hanya mengganti nama ke bentuk yang §8.6 sudah nyatakan wajib |
| Validator tanpa pengecualian reserved = regresi otorisasi 6 entitas | Pengecualian eksplisit + test yang mem-pin-nya |
| `plural` salah (B3) memperbaiki nama tapi memutus grant lama | Perbaiki `plural` di langkah terpisah, catat entitas yang terkena |
| Alat ukur meniru logika produksi | Setiap measurement memanggil fungsi produksi (lihat jebakan M2 di bagian 1) |

## Files

- `pkg/spec/entity.go` — validator baru; `ReservedActionNames` dipakai eksplisit
- `pkg/spec/` — test pengunci
- `internal/api/generator.go` — dibaca untuk memastikan aturan permission; tidak diubah
- `examples/kafe/spec/modules/cafe-master/master/dining-table/entity.yaml` — target migrasi pertama (Kategori A)
- `examples/...` + `cmd/formspec-registry/...` — 36 Kategori B + 6 Kategori C
- `docs_internal/plan/via-sebagai-action-penuh.md` — L4 diperbarui
- `examples/kafe/gaps_found/TODO.md` — 10.51

## Bukti yang harus ada saat L4 ditutup

1. Test yang **gagal** saat duplikat murni diizinkan (kalibrasi).
2. Test yang **gagal** saat named reserved dikecualikan salah (regresi 6 entitas).
3. `formspec validate` hijau per-module yang dimigrasi.
4. Server hidup: `POST /kafe/_ui/entity/gl/journal-entry/1/post` tetap **403**
   `missing permission: gl.journal-entrys.post` (L3 tidak mundur).
5. Grant `cafe-order.order.cancel` masih mempersempit seperti sebelumnya, atau
   grant diperbarui + dicatat.
