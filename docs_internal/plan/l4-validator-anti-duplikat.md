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

## 3. Bentuk validator

```
ValidateActionTransitionDuplication(d *EntitySpec) error
```

Dipanggil dari `ValidateEntitySpec` (dekat `ValidateTransitionPermissions`).

| kondisi                                                         | hasil                                                                                                              |
| --------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| `via` tidak ada di `actions`                                    | ✅ sah (inti L3)                                                                                                   |
| `via` kosong                                                    | ✅ sah (keputusan C)                                                                                               |
| `actions[].name` ∈ `ReservedActionNames`                        | ✅ **dikecualikan** — action lifecycle punya route generik; entri nyata bisa mempersempit/mengganti permission-nya |
| `via` = nama reserved (mis. `cancel`)                           | ⚠️ **warning**: entri ini jadi satu-satunya sumber permission route `/{id}/cancel`; pastikan disengaja             |
| nama sama, entri **tanpa** `required_permission`                | ❌ tolak — deklarasi murni, hapus                                                                                  |
| nama sama, permission identik dengan fallback                   | ❌ tolak — deklarasi murni, hapus                                                                                  |
| nama sama, `required_permission` + `impl` berbeda dari transisi | ❌ tolak                                                                                                           |

Agar bisa membedakan baris terakhir dari "murni", validator memerlukan
`TransitionDecl` untuk menyatakan `impl`-nya; itu sudah ada sejak L1.

**Urutan wajib:** validator dulu → kalibrasi (test gagal saat duplikat murni
diizinkan) → baru migrasi per-module. Tanpa validator, bentuk lama dan baru hidup
berdampingan tanpa penjaga dan deklarasi ganda bisa ditulis ulang kapan saja.

---

## 4. Urutan pengerjaan

| #   | Langkah                                                                 | Ukuran | Dependensi |
| --- | ----------------------------------------------------------------------- | ------ | ---------- |
| 1   | `ValidateActionTransitionDuplication` + pengecualian reserved + warning | small  | —          |
| 2   | Test pengunci (kalibrasi: gagal saat duplikat murni diizinkan)          | small  | 1          |
| 3   | Putuskan bentuk permission untuk Kategori B1 (butuh pemilik)            | small  | —          |
| 4   | Selaraskan 36 Kategori B + grant terkait; perbaiki `plural` B3          | medium | 3          |
| 5   | Migrasi 35 Kategori A mulai dari `examples/kafe` (1 entitas)            | medium | 2          |
| 6   | `formspec validate` per-module setelah tiap langkah + `go test ./...`   | small  | 4,5        |
| 7   | Tutup kafe 10.51 (bagian L4) + changelog                                | small  | 5          |

**Langkah 4 dan 5 sengaja terpisah dari 1–2:** validator bisa mendarat lebih dulu
tanpa memaksa migrasi, sehingga migrasi dapat dilakukan per-module dengan aman.

---

## 5. Risiko

| Risiko                                                              | Mitigasi                                                                              |
| ------------------------------------------------------------------- | ------------------------------------------------------------------------------------- |
| Menghapus entri Kategori B = melebarkan akses ke pemegang `update`  | Validator menolak; migrasi hanya setelah keputusan B1 dan grant diperbarui            |
| Validator tanpa pengecualian reserved = regresi otorisasi 6 entitas | Pengecualian eksplisit + test yang mem-pin-nya                                        |
| `plural` yang salah (B3) memperbaiki nama tapi memutus grant lama   | Perbaiki `plural` di langkah terpisah, catat di changelog, sebut entitas yang terkena |
| Alat ukur meniru logika produksi (jebakan M2)                       | Setiap measurement berikutnya memanggil fungsi produksi, bukan menyalin logikanya     |

---

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
