# 2026-10-07-005 — `required_when` akhirnya ditegakkan server (bagian c dari kafe 10.81)

**Plan:** —
**Konteks:** jawaban pemilik proyek atas verifikasi `2026-10-07-004` — (3)
"`manual_discount_amount` wajib ada alasan". Dikerjakan; (1) dan (2) **terblokir**
oleh celah platform yang baru teridentifikasi (lihat bawah).

## Yang dikerjakan: `required_when` punya penegak

`required_when` ada di schema, di-type-check `formspec check`, dan dihormati
renderer — **dan tidak ditegakkan siapa pun di server**. `validateRequired` (store)
hanya mengenal `required: true` statis. Jadi manifest bisa menyatakan "diskon
manual wajib beralasan" dan pemanggil API langsung melewatinya begitu saja.

Ditutup di jalur tulis, tempat yang sudah memegang payload:

- `internal/api/conditional_required.go` — `enforceConditionalRequired`, dipanggil di
  `HandleCreate` dan `HandleUpdate` setelah `preparePickerRows`.
- `examples/kafe/.../order/entity.yaml` — `manual_discount_reason` mendapat
  `required_when: "fields.manual_discount_amount != null"`, jadi deklarasinya
  berlaku untuk SEMUA permukaan (bukan hanya form POS).
- Evaluasi memakai pandangan **GABUNGAN** untuk PATCH (record tersimpan + body),
  sehingga melewatkan field pemicu bukan cara lolos; dan hasil non-boolean
  dianggap **error**, bukan truthy — gerbang yang lolos karena typo lebih buruk
  daripada tidak ada gerbang.

## Celah platform yang ditemukan: FormSpecExpr bukan subset Starlark

Ini yang tadinya menghalangi penegakan apa pun: dokumentasi menyebut FormSpecExpr
sebagai subset Starlark, tetapi **tidak**. Tiga literalnya ditulis huruf kecil —
`true`, `false`, `null` — sedangkan Starlark menuntut `True`, `False`, `None`.
Semua manifest memakai bentuk huruf kecil (`required_when:
"fields.manual_discount_amount != null"`).

Akibatnya bukan hanya `required_when`: **empat** tempat yang sudah lebih dulu
mengevaluasi ekspresi ini di server (kondisi grant/ABAC, kondisi action,
transform subscription) memanggil `starlark.EvalExpr` langsung, sehingga ekspresi
yang sah bagi renderer gagal di sana dengan `undefined: true` — divergensi
kosa kata bersama yang tidak terlihat sampai ada pemanggil server-side yang
memerlukannya.

Ditutup dengan satu normalizer bersama: `internal/starlark/formspec_expr.go`
(`EvalFormSpecExpr` / `EvalFormSpecBool` / `FieldMapEnv`). Rewrite-nya
**token-aware** — penggantian buta akan merusak identifier yang kebetulan memuat
kata itu (`is_nullable`, `truthy`) dan menulis ulang teks di dalam string literal
(`fields.code == "null"`).

## Bukti

| Lapis      | Bukti                                                                                                                                                                                                                                                              |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Normalizer | `formspec_expr_test.go` — literal huruf kecil dievaluasi; identifier & string literal utuh; non-boolean ditolak                                                                                                                                                    |
| Unit + e2e | `TestKafe_ManualDiscountRequiresAReason` (tamu anonim, `manual_discount_amount` tanpa alasan → **422**, dengan alasan → **201**) · `TestKafe_ManualDiscountRequiresAReasonOnUpdate` (PATCH: tanpa alasan → 422; sesudah punya alasan, PATCH tak berhubungan → 200) |
| Kalibrasi  | Guard dimatikan → test gagal dengan **201** dan `manual_discount_amount: 5000` tersimpan tanpa alasan (lubangnya terbaca apa adanya)                                                                                                                               |
| Suite      | `go test ./...` hijau · `golangci-lint` **0 issues** · kafe `check` 0 error/0 warning                                                                                                                                                                              |

## Yang TERBLOKIR, dengan alasan konkret (bagian a & b dari 10.81)

**(a) Evaluasi promo "pilih yang terbaik" belum bisa ditulis.** Dua penghalang:

1. **Starlark tidak punya list/query.** `resource.*` yang tersedia hanya
   `id · field · set · save · call · fetch · find · upsert · create · new`
   (`internal/starlark/resource.go` `AttrNames`). `find` mengembalikan SATU
   record, jadi memilih kandidat terbaik di antara banyak promo — aturan
   `priority` + `min_purchase` + jendela waktu + `applies_to` — tidak bisa
   diungkapkan tanpa `ctx.db().query()` SQL mentah, yang dilarang konvensi
   proyek. Jadi "pilih terbaik" menuntut **primitive list/query untuk Starlark**,
   atau sebuah fitur nilai-turunan di platform.
2. **Tidak ada konstruk "field diturunkan oleh server".** `computed` dievaluasi
   saat BACA dan hanya boleh membaca field record itu sendiri (karena itu
   `tax_percent` harus di-`snapshot` ke pesanan). `lookup_field` menurunkan dari
   satu entity melalui picker. Tidak ada yang menyatakan "nilai field ini
   dihasilkan script yang boleh membaca entity lain dan Config, dan pemanggil
   tidak boleh mengarangnya" — padahal itu yang dibutuhkan `discount_amount`.

**(b) `points_value` dari tarif di Config** kena penghalang yang sama: tarifnya
tidak bisa masuk ke record (tidak ada `snapshot` dari Config) dan tidak ada
field-turunan untuk menghitungnya.

**Dua jalur yang bisa dipilih** (butuh keputusan, bukan tebakan saya):

- **Jalur A — tambah primitive:** beri Starlark kemampuan query list yang aman
  (mis. `resource.query(entity, filters, limit)` dalam bahasa ctx.\*, bukan SQL),
  lalu evaluasi promo jadi script module + field-turunan yang menyebutnya.
  Ini pekerjaan platform ukuran medium-large, dan membuka jalur untuk semua
  aplikasi (bukan hanya kafe).
- **Jalur B — jujur soal batas:** tegakkan `required_permission` pada
  `discount_amount`/`points_value` (pemanggil tanpa permission **tidak boleh
  menyetelnya** — mekanisme ini sudah ada dan sudah ditegakkan server), lalu
  perhitungannya dilakukan permukaan yang berwenang (kasir di POS, via
  action/script). Efeknya: celah anonim tertutup HARI INI, tapi "pilih yang
  terbaik" belum otomatis — promo dipilih, bukan dihitung sistem.

## Sisa ⏸️ (diperbarui)

- **kafe 10.81 (a) `discount_amount`**, **(b) `points_value`**, **(c) batas
  diskon manual** (angkanya, bukan alasannya — yang sudah ditutup di sini), dan
  **(d) `lines[].discount_amount`** (masih belum dijawab: masukkan ke
  `line_total`, atau hapus fieldnya).
- Test yang menuntut perilaku aman untuk (a)/(b)/(c-batas)/(d) tetap ada sebagai
  `t.Skip` di `resource/discount_authority_e2e_test.go` — menutup item berarti
  menghapus `t.Skip`-nya.
