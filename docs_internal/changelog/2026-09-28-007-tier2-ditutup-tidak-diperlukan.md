# 2026-09-28-007 — Tier 2 ditutup sebagai tidak diperlukan; 1 sisa nyata menggantikannya

**Plan:** — (keputusan scope, bukan implementasi)
**Menutup:** todo `5.24.2` ✅ (sebagai **tidak diperlukan**), membuka `5.24.6` ⏸️

## Pemicu

Pertanyaan pengguna: _"5.24.2 mengapa masih dibiarkan terbuka?"_

Jawabannya: **karena saya salah membiarkannya terbuka.** Item 5.24.2 saya tulis
di `2026-09-28-004` dengan alasan "keputusan D7 menunda bentuk ini" — tetapi D7
adalah rekomendasi **saya sendiri** di percakapan yang sama. Alasan itu
melingkar, jadi ia tidak lolos aturan repo ini sendiri: pekerjaan tertunda harus
punya alasan yang bisa diperiksa (`AGENTS.md` → Workflow Discipline §3).

`⏸️` berarti "akan dikerjakan, ini alasannya". Membiarkan item yang tidak punya
permintaan sekaligus tidak punya rencana adalah misinformasi yang sama
jenisnya dengan `[x]` yang melebih-lebihkan — pembaca mengira ada pekerjaan yang
menunggu, padahal tidak.

## Dua pengukuran yang menggantikan alasan itu

**1. Klaim teknisnya benar (diverifikasi, bukan diulang).** Saya menulis bahwa
`params.form.ref` menuntut `FormRenderer` menerima target submit ketiga. Itu
benar: `doSubmit` (`renderers/react-shadcn/src/kinds/form/FormRenderer.tsx:505`)
punya tepat empat cabang — service `submit.call`, `apiPatch` untuk edit,
`create-submit`, dan POST create — dan **tidak satu pun** mem-POST ke
`{id}/{action}`. Jadi `kind: Form` memang tidak bisa menyasar action entity.

**2. Permintaannya nol.** Jumlah input terdeklarasi terbesar di seluruh
`examples/` + `verticals/` adalah **2** (`service-demo/.../tax-calculator.yaml`).
Kafe `void-order` = 1. Satu-satunya justifikasi Tier 2 adalah ">12 input yang
butuh sections/columns" — tidak ada satu pun yang mendekati, dan tidak ada
manifest yang ingin menautkan `kind: Form` ke sebuah transisi.

Kind yang sudah ada juga bukan penggantinya: `Wizard.action`
(`WizardRenderer.tsx:161`) mem-POST path apa adanya, dirancang untuk "memfinalkan
draft", berupa halaman penuh, dan tidak sadar `{id}`.

## Yang menggantikannya: 5.24.6 ⏸️

Mengganti item yang tidak punya permintaan dengan sisa yang **terukur**:

> Identitas approver tidak pernah masuk ke field record.

`void_approved_by` muncul **3 kali** di seluruh repo (di luar `.git`/
`node_modules`):

| Lokasi                                                           | Isi                                             |
| ---------------------------------------------------------------- | ----------------------------------------------- |
| `examples/kafe/.../cafe-order/transaction/order/entity.yaml:170` | deklarasi field (`relation` → employee)         |
| `examples/kafe/.../cafe-order/forms/order-form-pos.yaml:60`      | entri `read_only: true`, label "Disetujui Oleh" |
| `examples/kafe/docs/domain-model.md:390`                         | dokumentasi: "Supervisor penyetu"               |

**Nol penulis** — bukan script, bukan seed, bukan engine. Field yang sengaja
dideklarasikan `read_only` (author sudah tahu user tidak mengisinya) tetap kosong
selamanya, termasuk **setelah approval supervisor berhasil dijalankan**:
`formspec_workflow_approval.approvals` dan audit trail mencatat _siapa_, tetapi
tidak ada cara deklaratif memproyeksikannya ke record. Tidak ada mekanisme
"approver → field" di `docs/spec/` maupun `docs/kind/`.

## Keputusan & trigger

- **5.24.2 ditutup** (`[x]`, bukan ⏸️). Bila bentuknya suatu hari dibutuhkan,
  arah yang benar adalah **memperluas `ParamInput`** (mis. `sections`) — bukan
  menautkan `kind: Form` yang terikat satu entity + satu mode.
- **Trigger membuka kembali:** ada transisi/action nyata dengan input yang butuh
  sections/columns, atau butuh `default_from` dari render-context
  (`{session.*}`/`{route.*}`) — kemampuan yang `FormField` punya dan
  `ParamInput` belum.

## Berkas yang terdampak

- `docs_internal/plan/todo.md` — 5.24.2 dikunci sebagai keputusan + reasoning,
  5.24.6 dibuka, hitungan sisa dikoreksi dari 63 (unverifiable) ke 37 (terukur,
  disertai metode)

Tidak ada perubahan kode: ini keputusan scope, dan menutup item tanpa mengerjakan
sesuatu adalah hasil yang sah.

## Catatan

Trigger yang eksplisit adalah yang membuat penutupan ini bukan sekadar
menyembunyikan item. `5.24.6` sekarang memuat: apa yang rusak, **bagaimana
mengamatinya** (`grep -rn void_approved_by` → 3 kemunculan, nol penulis), dan
kira-kira berapa biayanya.

## Koreksi kedua: angka "Sisa N item" di header tidak bisa dipertanggungjawabkan

Menutup 5.24.2 berarti menggeser hitungan sisa, dan saat memverifikasinya
ketahuan angkanya **salah** — dan sudah salah sebelum saya menyentuhnya.

Header menyebut **64**, saya geser ke 65, lalu 64, lalu 63, **tanpa sekali pun
mengukur**. Diukur ulang:

| Definisi                                                            | Hasil  |
| ------------------------------------------------------------------- | ------ |
| `- [⏸️]` (marker deferred murni)                                    | **29** |
| `- [ ] ⏸️` (unchecked + emoji)                                      | **8**  |
| **Jumlah item benar-benar deferred**                                | **37** |
| `- [ ]` unchecked (termasuk yang deferred)                          | 48     |
| baris item yang _menyebut_ ⏸️ (banyak item `[x]` merujuk sisa lama) | 78     |
| `- [x]`                                                             | 568    |

**63 tidak cocok dengan definisi mana pun.** Angkanya kini ditulis sebagai
**37**, disertai cara mengukurnya di tempat, dan ditandai sebagai **batas bawah
yang terdefinisi** — sebagian item `[ ]` lain bertanda deferred lewat prosa
("di-defer"), jadi tidak ada satu grep yang menangkap semuanya.

Penggeseran ±1 tanpa pengukuran adalah persis kelas kesalahan yang membuat
5.24.2 ikut ditutup hari ini: sebuah angka yang dibaca sebagai fakta padahal tidak
pernah diperiksa. Karena itu perbaikannya bukan "63 → 37", melainkan
**menyertakan metodenya** supaya pembaca berikutnya bisa menghitung sendiri.
