# Koreksi: `require_permission` hidup di TRANSISI, bukan di action

**Tip:** koreksi desain dari pertanyaan pemilik.
**Ledger:** kafe **10.47 ⏸️** (baru) · 10.46 ⏸️ · 10.45 ⏸️ · 10.44 ✅
**Pemicu:** _"via di transition harus didefinisikan ulang di action. benar
seperti itu? kalau benar, mengapa di transitions ada require_permission? di
actions sudah ada require_permission."_

## Dua klaim yang perlu diuji — hasilnya berbeda

| Klaim                                                                            | Hasil (terukur)                                                                                                                                   |
| -------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| "`via` harus didefinisikan ulang di `actions`"                                   | **Tidak benar.** Seluruh `actions:` dibuang dari salinan spec → `85 manifest(s) validated, 0 problem(s)`. `via` adalah **nama**, bukan referensi. |
| "`actions` sudah punya `require_permission`, jadi yang di `transitions` mubazir" | **Benar** bahwa itu duplikasi — dan duplikasi itu **tidak berguna**: yang mengikat adalah milik **transisi**.                                     |

### Bukti mana yang benar-benar mengikat

| Bentuk                                                           | kasir `available → not_available` |
| ---------------------------------------------------------------- | --------------------------------- |
| `require_permission` ada di **transisi**                         | **403** (terkunci)                |
| `require_permission` hanya di **action** (dihapus dari transisi) | **200** (tembus)                  |

Jadi field di action **tidak menegakkan apa pun** pada transisi: action itu
tidak punya `impl`, jadi tidak punya route sendiri, dan jalur penerap transisi
adalah `PATCH` — yang diperiksa `{module}.{plural}.update`. Permission per-transisi
harus dibaca dari **transisi**.

## Kesalahan desain saya (revisi pertama 10.44)

Revisi pertama saya **mewajibkan** `require_permission` merujuk permission yang
terdaftar — termasuk `via` action — sehingga duplikasi itu **dipaksa**, dan saya
menuliskannya di manifest sebagai "gate harus ada di action dan di transisi".
Itu salah pada dua hitungan:

1. **Mendorong drift.** Hanya salinan di transisi yang ditegakkan, sehingga dua
   salinan itu bisa menyimpang — dua kali edit, dan yang **terlihat otoritatif
   bukan** yang berlaku. Perbedaannya **tak teramati saat runtime**.
2. **Memaksa duplikasi yang tak berguna.** Sebuah transisi `via: release` pada
   entity yang sudah punya action standar `Delete` tidak bisa digerbangi
   `{plural}.delete` tanpa mengarang action bernama `delete`.

(**Catatan koreksi nyata:** sebelum revisi ini, duplikat `required_permission` di
action memang **tidak** ikut didaftarkan sebagai permission — `registerStandardPermissions`
mendaftarkan permission dari `name` dan `required_permission` action sebagai
**dua entry terpisah**, dan grant materializer mencocokkan lewat nama action.
Terukur: grant `{name: mark-table-served}` menghasilkan
`...mark-table-served`, **bukan** `...mark-served` seperti yang saya tulis.)

## Perubahan

- `pkg/spec`: `ValidateTransitionPermissions` **tidak lagi** mewajibkan rujukan
  terdaftar. Dua pemeriksaan baru: segmen akhir gate harus non-kosong (gate
  setengah jadi tak bisa diberikan ke siapa pun), dan gate **tidak boleh**
  mengulang `required_permission` milik `via` action (dua sumber yang bisa
  menyimpang). Helper `transitionPermissionMatches` dihapus.
- `dining-table`: `require_permission` **dihapus dari semua action**; gate hanya
  di transisi. Nama gate diselaraskan dengan nama action
  (`dining-tables.mark-table-served`, `dining-tables.mark-not-available`) supaya
  grant yang menyebut action menghasilkan permission yang **persis sama **.
- `pkg/spec/transition_permission_test.go` — 10 test, termasuk yang sebelumnya
  menuntut rujukan terdaftar (kini dibalik jadi "gate tidak butuh action").

## Bukti akhir (dev server, terukur)

| Transisi                                   | kasir   | pelayan | manajer |
| ------------------------------------------ | ------- | ------- | ------- |
| `available → occupied` (tanpa gate)        | **200** | —       | —       |
| `occupied → served` (gate pelayan)         | **403** | **200** | —       |
| `occupied/served → available` (gate kasir) | **200** | **403** | **403** |
| `available → not_available` (gate admin)   | **403** | —       | **200** |
| `available ↔ reserved` (gate kasir)        | **200** | —       | **403** |

Grant terukur: `kasir=list,release,reserve,update,view` ·
`pelayan=list,mark-table-served,update,view` ·
`manajer=create,list,mark-available,mark-not-available,update,view`.

Kafe `validate` 85 manifest 0 problem · `gofmt` bersih · `go test ./...` hijau ·
`make generate-schema` + `generate-kind-docs` dijalankan.

## Pelajaran yang bernilai

**Gate harus berada di lapisan yang MENEGAKKANNYA.** Menyimpannya di tempat yang
`required_permission`-nya tidak pernah diminta (action tanpa `impl`) bukan
sekadar berlebihan — ia menciptakan dua sumber kebenaran yang menyimpang tanpa
gejala. Yang terlihat otoritatif bukan yang berlaku.

Ini masih kelas yang sama dengan **10.43** dan **10.46**: gerbang yang hidup di
satu lapisan bukan batas sampai ditegakkan di lapisan tulis.

## Sisa

- **10.47 ⏸️** — nama permission untuk transisi **tidak punya relasi struktural
  dengan nama action**. Grant menyebut **nama action** (materializer mencocokkan
  `fa.Action`), sementara gate menyebut **nama permission**. Kalau keduanya
  berbeda, hasilnya **403** — arah yang aman, tetapi bentuknya persis "gerbang
  yang tak pernah bisa dibuka" yang dulu saya coba cegah. Kandidat penutup:
  validasi warning saat nama permission transisi tidak sama dengan
  `{plural}.{nama action}`.
- **10.46 ⏸️** (jalur script belum menegakkan gate), **10.45 ⏸️**,
  **10.42 ⏸️**, **10.43 ⏸️**, **10.40b ⏸️**, **10.41 ⏸️**, 10.34c/10.36/10.37/
  10.38/10.39/10.20.
