# 2026-10-07-002 — 10.76 (cabang dari meja) · 10.79 (harga dari katalog) · 6.5.10 (pengalih konteks)

**Plan:** `docs_internal/plan/public-scope-enforcement.md` (fase B/C), `docs_internal/plan/session-context-role-branch.md` (tahap 4)
**Konteks:** tindak lanjut tiga permintaan pemilik proyek — "10.76 cabang harus diambil
dari informasi QR di meja", "10.79 unit price harus ikut server — harus pakai script?",
"6.5.10 selesaikan juga".

## 1. 10.76 ✅ — cabang diambil dari meja, bukan dari klien

**Mekanisme baru:** `FilterSpec.Via`/`ViaField` untuk `from: route` (`pkg/spec/frontend.go`).
`param` menjadi **rujukan** (id atau natural key) dan nilai scope dibaca dari
`via.via_field` record itu — jadi pemanggil menyebut SESI-nya (sudah ada di URL
`/menu/:session_id`), tidak pernah cabang yang ingin dilihatnya. `via` ditolak
validator pada sumber selain `route`.

**Deklarasi:** `PickerDisplay.PriceScope` (`price_scope`) pada picker, karena
picker-lah yang menyebabkan fetch — sehingga deklarasi dan penegakannya tidak
bisa datang dari dua tempat. Grant publik turunan mewarisinya
(`internal/ui/surface.go` `visitField`).

> **DIKOREKSI 2026-10-07 (changelog `2026-10-07-003`):** nama itu **tidak ada lagi**.
> Seluruh kosakata `price_*` pada picker dicabut karena mengunci satu domain ke dalam
> `pkg/spec`; yang berlaku `picker.lookup.scope` (perilaku identik, kosakata netral).
> Sisa entry ini — aturan `via`, `resolveRouteScopeValue` sebagai satu implementasi,
> fail-closed, dan harga yang BEKU — tidak berubah.

```yaml
price_scope:
  {
    field: branch_id,
    from: route,
    param: session_id,
    via: cafe-order.table-session,
    via_field: branch_id,
  }
```

`price_filter` yang lama **dihapus** dari manifest kafe: ia filter klien, dan di
POS pun tidak bekerja (`order-form-pos` tidak punya context `session`, jadi token
`{session.branch_id}` terkirim verbatim).

**Sisi staf:** `menu-item-price` kini juga punya `row_scope: [{field: branch_id,
from: session}]` — tanpa itu kasir melihat harga semua cabang (`price_filter`
adalah satu-satunya pembatas, dan ia tidak resolve di POS). Untuk pemanggil
anonim ber-grant, `row_scope` entity dilewati (tidak ada atribut sesi untuk
dibaca) dan scope grant yang berlaku; itulah sebabnya kedua sisi bisa hidup
berdampingan.

**Koreksi penting (satu implementasi, bukan dua):** `applyPublicScope` masih
membaca nilai param **mentah** setelah jalur predikat belajar tentang `via` —
dua pembacaan atas pertanyaan yang sama, dan yang satu (grant) memfilter dengan
id sesi sehingga **nol baris**. Disatukan jadi `resolveRouteScopeValue`, dipakai
kedua jalur. Kelas drift yang sama dengan 10.46.

## 2. 10.79 ✅ — harga baris dari katalog, **tanpa script**

Jawaban atas pertanyaan "harus pakai script load data dan replace?": **tidak**.
Picker sudah mendeklarasikan sumber harga (`price_entity`, `price_match_field`,
`price_field`, dimensi), jadi harga adalah nilai **turunan** — dan framework
sudah punya aturan untuk itu: `computed` di-strip karena "nilai turunan tidak
pernah milik pemanggil". Harga picked diperlakukan sama.

`resolvePickerPrices` (`internal/api/pickerprice.go`), dipanggil di `HandleCreate`
dan `HandleUpdate`:

- baris **baru** → harga dibaca dari katalog (`price_match_field` = ref baris,
  dimensi = nilai ortu), nilai kiriman klien **diganti**;
- baris yang **sudah ada** → snapshot tersimpan **dipertahankan**. Ini yang
  membuat harganya BEKU, bukan dihitung ulang: mengedit kuantitas tidak boleh
  diam-diam mengubah harga yang sudah disetujui tamu — dan itu juga yang menutup
  "ubah jumlah" sebagai jalan menulis harga;
- katalog tidak punya harga untuk (menu, cabang) itu → **422**, bukan jatuh ke
  angka kiriman; katalog ambigu (>1 baris) → **422**;
- baris tanpa `ref_field` → **422** (baris yang tak bisa dialamati tak punya
  sumber harga).

Script hook sengaja **tidak** dipakai: ia akan menjadi tempat kedua yang tahu
cara menghargai satu baris (picker satu, script satu), dan keduanya bisa
menyimpang — persis kelas yang melahirkan 10.76.

## 3. 6.5.10 ✅ — pengalih konteks sesi di menu pengguna

**Backend:** `/_meta/me` kini membawa `context` (role + dimensi + nilai) dan
`context_choices` (semua konteks principal). Pemisahannya disengaja: pilihan
berasal dari record principal (otoritas atas apa yang BOLEH menjadi), sedangkan
konteks aktif dibaca dari identitas sesi (`role` + `attrs`) — itulah yang
dipakai otorisasi, jadi melaporkannya dari tempat lain bisa berbeda dari batas
yang ditegakkan (`sessionContextOf`, `internal/api/meta.go`).

**Frontend:** item "Switch context" di `UserMenu` (hanya muncul bila ada >1
pilihan — menu yang tak bisa berbuat apa-apa adalah kebisingan), membuka
`ContextPicker` yang sudah ada, lalu `POST /_ui/auth/switch` + reload permukaan.
Logika switch diekstrak ke `lib/api/switchContext.ts` sehingga layar 409
(`SwitchContextScreen`) dan pengalih menu memakai **satu** jalur — penting karena
switch me-revoke sesi lama, jadi urutan operasinya tidak boleh berbeda.

**OAuth:** `?assignment=` diteruskan lewat OAuth `state`, dan 409
`CONTEXT_REQUIRED` dari round-trip tidak lagi menjadi `oauth=error` generik.
Backend mengalihkan ke login App dengan pilihan di **fragment** (tak pernah
dikirim ke server, tak masuk access log; bukan rahasia — nilainya sama dengan
body 409) sebagai parameter `c` **berulang**, bukan string berdelimiter, karena
nilai konteks itu opaque. `OAuthCallback` menampilkan pemilih dan **melanjutkan
alur yang sama** dengan pilihan itu. Sign in ulang bukan opsi: akun OAuth murni
tidak punya password.

## Bukti

| Item                  | Bukti                                                                                                                                                                                                                                                                                           |
| --------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 10.76 baca            | `resource/price_scope_e2e_test.go` — anonim tanpa rujukan → **403**; dengan `session_id` → hanya cabang meja; `?branch_id=<cabang lain>` **tidak melebarkan** (dibuktikan ada harga cabang B di fixture sehingga asersinya tidak hampa)                                                         |
| 10.76 resolusi        | `internal/api/row_scope_via_test.go` — nilai = cabang SESI (bukan id sesi); gagal-tertutup untuk param hilang / rujukan menggantung / `via_field` kosong; `via` ditolak di luar `route`. `TestRowScope_MenuItemPriceIsScopedForStaff` — kasir dapat predikat cabang, pemegang `read_all` exempt |
| 10.76 derivasi        | `TestPublicGrantScope_KafeQR_PriceBranchFromTheSession`                                                                                                                                                                                                                                         |
| 10.79                 | `resource/picker_price_e2e_test.go` — tamu anonim mengirim harga `1` → tersimpan **45000**; harga **beku** saat katalog berubah (77777) dan kuantitas diedit; baris tanpa harga katalog → **422**                                                                                               |
| 6.5.10                | `internal/api/meta_context_test.go` (3) · `renderers/.../shell/UserMenu.test.tsx` (4) · `lib/oauthContext.test.ts` (12) · `internal/api/oauth_context_test.go` (2)                                                                                                                              |
| kalibrasi             | 10.76: hapus `price_scope` → test derivasi **dan** e2e merah; matikan `via` → merah. 10.79: matikan resolusi → **"stored line price = 1"** (lubang terbaca apa adanya). 6.5.10 tak perlu (tidak ada penegakan keamanan)                                                                         |
| suite                 | `go test ./...` hijau · `npx vitest run` **669 lulus / 56 file** · `golangci-lint` **0 issues** · kafe `validate` 88/0 · `check` 0 error                                                                                                                                                        |
| guard lama diperbarui | `TestKafeRowScopeSpec_ScopeAndSource` mem-pin aturan "sesi ATAU grant, tidak keduanya" — model yang justru melahirkan 10.76. Diganti invarian yang lebih kuat: entitas yang dibaca anonim butuh **kedua** sisi (sesi untuk staf **dan** `price_scope` di picker)                                |

**Catatan runner:** `make lint` gagal di container ini karena
`/home/vscode/.cache/golangci-lint` tidak writable — jalankan dengan
`GOLANGCI_LINT_CACHE=/tmp/gl-cache XDG_CACHE_HOME=/tmp/vscode-cache`.

## Sisa ⏸️

- **`via` untuk `row_scope` pada entity**: hanya dipakai di grant publik (kafe).
  Bentuk entity `row_scope from: route` + `via` sudah didukung runtime dan
  divalidasi, belum ada pemakainya — bukan celah, tapi belum terbukti dengan
  manifest nyata.
- **`price_scope` wajib bila `price_entity` dipakai**: saat ini derivasi membawa
  scope **hanya bila dideklarasikan**. Entitas harga yang boleh dibaca anonim
  tanpa deklarasi tetap tidak ter-scope — validator yang menuntutnya belum ada →
  item tersendiri (10.80).
- **Harga beku pada `resource.save()` (jalur script)**: resolusi berjalan di
  handler HTTP, jadi script yang menulis baris pesanan langsung ke store tidak
  melewatinya. Sama kelasnya dengan 10.46 (gate yang hanya hidup di jalur HTTP).
