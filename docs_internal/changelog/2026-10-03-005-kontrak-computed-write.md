# 2026-10-03-005 — Kontrak §5.1 `computed` ditegakkan: tidak bisa ditulis klien, dihitung ulang saat simpan (kafe 10.68/10.70)

**Keputusan yang ternyata sudah ada di spec.** 10.70/10.68 saya bingkai sebagai
"pilihan desain: persist derivasi atau tidak". Setelah membaca ulang
`docs/spec/backend/05-field-types.md` **§5.1 Computed field**, ternyata **tidak
ada yang perlu diputuskan** — kontraknya sudah normatif:

> - **Never client-writable** — nilai `computed` di payload klien **diabaikan**.
> - **Recomputed on save** — formula dievaluasi ulang pada tiap `create`/`update`
>   **sebelum persist**, sehingga nilai tersimpan selalu konsisten dengan input
>   terkini.

Implementasinya melanggar ketiganya. Jadi pekerjaan ini **menegakkan spec**, bukan
memilih desain baru.

## Tiga pelanggaran yang diukur sebelum perbaikan

| # | Pelanggaran | Terukur (dev server, `order.total_amount`) |
| --- | --- | --- |
| 1 | **Computed bisa ditulis klien** | CREATE membawa `total_amount: 999999` → **tersimpan 999999**, sementara respons API menunjukkan `1155`. Storage **berbeda dari API**. |
| 2 | Sama, di jalur update | PATCH membawa `777777` → tersimpan `777777`. |
| 3 | **Tidak dihitung saat create** | Baris hasil CREATE **tanpa** nilai computed di storage; ia hanya muncul sebagai efek samping read-modify-write PATCH (kafe 10.70). |

Pelanggaran 1–2 bukan sekadar pelanggaran kontrak: kolom turunan numerik
`_total_amount` — yang dibaca **sortir, filter rentang, dan report SQL** — memuat
angka palsu itu, sementara API menjawab dengan angka benar. Itu kelas "jawaban
salah tanpa gejala" yang §2.2 peringatkan.

## Perbaikan

- Helper baru `EntityStore.stripComputedValues(data)` — menghapus nilai
  computed yang dikirim klien di **tingkat induk DAN di setiap baris child**
  (`Child.Fields`), karena keduanya berbagi kontrak yang sama.
- Helper baru dipanggil di **Insert**, **Update**, dan **UpsertProjection**
  (jalur tulis summary), **sebelum** validasi/default, sehingga tidak ada yang
  bisa menyimpannya.
- `evaluateComputed` **dipanggil di jalur tulis**, sebelum persist:
  - **Insert**: **di dalam transaksi** dan **setelah `applyFinancialSnapshot`** —
    posisi itu wajib, sebab formula boleh membaca field hasil snapshot
    (`tax_amount` membaca `tax_percent` yang disalin dari cabang).
  - **Update**: setelah period guard, sebelum child diekstrak (agar baris
    storage `table` ikut membawa nilai computed).
  - **UpsertProjection**: sebelum tulis.
- **`resource`/`data` (FieldMap) kini juga disuntikkan ke env formula CHILD**,
  bukan hanya induk. Tanpa itu formula child tidak bisa menyebut operand
  opsional dengan aman — dan saya menemukannya justru karena test child saya
  gagal (`resource.qty` undefined → computed child absen). Kedua tingkat kini
  memakai kontrak yang sama.

**Catatan yang sengaja dibiarkan:** validasi field (`validateFieldRules`) berjalan
**sebelum** `evaluateComputed` pada Insert, karena snapshot baru tersedia di dalam
transaksi. Akibatnya `rules` pada sebuah field *computed* akan melihat nilai
kosong. Tidak ada manifest yang melakukannya (checked: seluruh `computed` kafe
tidak ber-`rules`), tetapi batasnya dicatat, bukan disembunyikan.

## Bukti

**Unit `renderers/jsonb-persist/computed_write_contract_test.go`** (baru, 4
subtest) membaca **payload MENTAH dari tabel**, bukan respons dan bukan hasil
baca, sehingga perbaikan yang hanya membetulkan salinan hasil tidak bisa lulus:

| Subtest | Sebelum | Sesudah |
| --- | --- | --- |
| create dengan computed palsu | tersimpan `999999` | tersimpan `2000` (hasil formula) |
| create **tanpa** menyebut computed | **absen** di storage | tersimpan `2000` |
| update dengan computed palsu | tersimpan `777777` | `1000` (dihitung ulang dari input BARU `500×2`) |
| child computed palsu | tersimpan `123456` | `300` (hasil formula) |

Keempatnya **gagal lebih dulu**, dengan pesan yang menyebut §5.1.

**Verifikasi dev server pada DB bersih** (seed ulang, agar tidak tercemar baris
pra-perbaikan — pengukuran pertama saya tercemar residu `777777` dan membuat
sortir tampak rusak; itu **salah baca saya**, bukan bug):

| Pemeriksaan | Hasil |
| --- | --- |
| CREATE qty=3, membawa total palsu `999999` | respons & tersimpan & kolom turunan = **34650** (30000 + 5% + 10%) |
| CREATE qty=1 **tanpa angka uang** | payload **11550** *dan* `_total_amount` **11550** — tanpa PATCH |
| PATCH dengan `777777` | dihitung ulang → **34650** |
| `?sort=-total_amount` | 57750 → 34650 → 11550 (monoton) |
| `?sort=total_amount` | kebalikannya (monoton) |

**Dampak ke 10.68:** kolom turunan kini terisi **sejak create**, jadi
`?sort=total_amount` dan filter rentang bekerja untuk baris baru tanpa
bergantung pada efek samping PATCH. Sisa yang tetap terbuka: baris lama
(**10.69**) dan nilai bisa basi bila formula berubah tanpa menulis ulang baris.

`go test ./...` hijau · `gofmt` bersih · `formspec validate` 89/0 ·
`formspec check` 0/0 · frontend `tsc -b` bersih · `vitest` 616 lulus.
Diperiksa juga: **tidak ada script kafe yang menulis field `computed`**
(`grep 'set("(difference|change|subtotal|total_amount|line_total|…)"'` → nol),
jadi penghapusan nilai kiriman tidak menghilangkan tulisan script mana pun.

**Dampak.** `renderers/jsonb-persist/crud.go` ·
`renderers/jsonb-persist/computed_write_contract_test.go` (baru) ·
`examples/kafe/gaps_found/TODO.md` (10.70 ✅, 10.68 diperbarui).
