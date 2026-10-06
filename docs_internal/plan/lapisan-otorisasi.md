# Plan — Di lapisan mana otorisasi ditegakkan: HTTP atau store?

Menjawab satu paket: **10.46** (gerbang transisi tidak berlaku di
`resource.save()`), **10.67** (aturan #1 ditegakkan view), **10.71** (field-level
§5.3 tidak berlaku di jalur script).

Ketiganya pertanyaan yang sama: **di mana penegakan otorisasi diletakkan, dan
apakah ia berlaku untuk setiap jalur tulis?**

## Bukti yang sudah dibaca (bukan asumsi)

| Fakta | Sumber |
| --- | --- |
| `InsertParams.Permissions` / `UpdateParams.Permissions` **sudah ada** | `renderers/jsonb-persist/crud.go:566,959` |
| Jalur **HTTP** sudah mengoper permission pemanggil | `internal/api/handler.go:866,1186` |
| Jalur **script** (`resource.save()`) **juga sudah mengoper** permission pemanggil | `resource/formspec.go:1936,1953` (`auth.PermissionsFromContext(ctx)`) |
| Store hanya memakai `Permissions` untuk **override backdate/forward-date** | `renderers/jsonb-persist/transaction_date.go:157` |
| Gerbang transisi hidup **hanya** di HTTP | `internal/api/handler.go:1059-1067` |
| Field-level §5.3 hanya menjaga **baca**; tulis kini dijaga di HTTP saja | `internal/api/fieldsec.go` + changelog `2026-10-03-006` |
| `row_scope` hanya `from: session\|route` dan **per-entity** | `docs/spec/backend/01-core-basic.md` §1.7 |

**Kesimpulan bukti:** plumbing-nya **sudah ada** dan **kedua jalur ber-identitas
sudah mengopernya**. Yang hilang bukan pipa, melainkan **penegakan**.

## Rekomendasi

**Store = titik penegakan otoritatif (policy enforcement point). HTTP tetap ada
sebagai lapisan gagal-cepat. Bukan salah satu, melainkan keduanya dengan SATU
sumber kebenaran.**

### Kenapa store harus otoritatif

1. **Ia satu-satunya choke point.** Semua tulis lewat `Insert`/`Update`/
   `UpsertProjection`. 10.46 dan 10.71 ada **justru karena** gerbang
   ditempelkan pada sebuah **jalur**, bukan pada **invarian**.
2. **Jalur baru otomatis terlindungi.** Path tulis yang ditambahkan nanti
   (job, webhook keluar, operator, subscription) mewarisi penegakan alih-alih
   diam-diam melewatinya. Itu properti yang paling bernilai di sini.
3. **Pipanya sudah ada** — biaya marginalnya kecil, bukan refactor besar.
4. **Preseden internal sudah ada**: store **sudah** membaca `Permissions` untuk
   override kebijakan tanggal. Jadi store yang sadar-otorisasi bukan konsep baru.

### Kenapa HTTP tetap dipertahankan

1. **Pesan error yang benar sedini mungkin.** `403` + `details[].field` sebelum
   pekerjaan dilakukan, bukan error penyimpanan yang dipetakan belakangan.
2. **Baca tidak ikut pindah.** `row_scope`, `sanitizeData` (masking, strip
   field) sudah benar di HTTP dan menyangkut **bentuk respons** — itu urusan
   permukaan, bukan penyimpanan.
3. **Defense in depth.** Bila satu lapisan punya bug, lapisan lain menahan.

Aturan yang harus dijaga: **satu sumber kebenaran per aturan.**
`spec.TransitionPermission(trans)` dan `field.RequiredPermission` dibaca oleh
**keduanya**; tidak ada duplikasi deklarasi (ini yang dilarang validator:
gate di action *dan* transisi).

### Konsekuensi desain yang harus diterima

**Pemanggil sistem harus dinyatakan EKSPLISIT.** Hari ini "tanpa permission" =
"tanpa pemeriksaan" secara implisit — dan justru di situlah bypass bersembunyi.
Store butuh pembeda tegas:

- **pemanggil ber-identitas** → periksa (fail closed);
- **penulis sistem** (seed, migrasi, backup, auth internal, job tracker,
  escalation, subscription) → **menyatakan dirinya sistem**, bukan kebetulan
  tidak mengirim permission.

Ukuran: ±10 pemanggil produksi non-HTTP/non-script perlu anotasi eksplisit
(`cmd/formspec/seed.go`, `backup.go`, `archive.go`, `cmd/formspec-registry`,
`internal/auth/{apikey,role,session,user,workspace}.go`, `internal/job`,
`internal/workflow/escalation.go`, `renderers/jsonb-persist/crud.go` internal).
Ini **fitur**, bukan beban: "tulisan ini tidak punya pengguna" menjadi pernyataan
yang bisa diaudit.

## Rancangan

1. **Principal pada param tulis.** Ganti `Permissions []string` menjadi principal
   yang bisa dinyatakan salah satu dari: ber-identitas (permissions) atau
   **sistem** (eksplisit). Bentuk minimal: pertahankan `Permissions` +
   tambahkan penanda sistem, sehingga pemanggil lama tetap kompilasi dan
   tinjauannya per-situs.
2. **Penegakan di store**, satu tempat, dibaca dari spec yang sudah dipegang
   store:
   - **field-level tulis** (§5.3) — `s.fields[].RequiredPermission`;
   - **gerbang transisi** (10.46) — `spec.TransitionPermission(trans)` pada
     `validateStateTransition`, yang sudah tahu `from`/`to`;
   - **(opsional, diputuskan terpisah)** `action.RequiredPermission` untuk action
     yang hanya punya jalur store — jangan digabung tanpa keputusan, sebab
     gerbang action-impl-less sudah sengaja hidup di transisi.
3. **Error bertipe + pemetaan.** `ErrForbidden` di store dipetakan ke `403`
   (seperti `ErrValidationRule` → 422), supaya jawaban jalur script sama
   kelasnya dengan jalur HTTP — bukan `500`.
4. **HTTP mempertahankan pemeriksaannya** (gagal-cepat, pesan ramah).
5. **Sisa yang tetap tidak tertutup oleh ini:** **filter baris per-peran**
   (10.67). Ia bukan soal "di lapisan mana", melainkan **model grant belum
   punya dimensi baris**. Itu perluasan tersendiri — lihat di bawah.

## Yang TIDAK diselesaikan paket ini

- **10.67 (batas status per-peran).** `row_scope` memfilter **siapa**, bukan
  **nilai kolom**, dan bersifat **per-entity** (filter status akan membutakan
  kasir terhadap draft-nya). Menutupnya = menambah **filter baris per-peran**
  pada model grant — keputusan produk + perluasan mesin, terpisah dari paket ini.
  Paket ini hanya menghapus *satu* alasan pembenar lama (bahwa lapisan tulis
  bocor).

## Urutan & risiko

1. Principal (mekanis, tanpa perubahan perilaku — semua situs tetap "sistem"
   sampai ditinjau).
2. Penegakan field-level tulis di store + pemetaan error. **Risiko rendah**:
   tidak ada field ber-`required_permission` di kafe (dampak nol, terverifikasi).
3. Penegakan gerbang transisi di store. **Risiko menengah**: penulis sistem pada
   entity ber-transisi bergerbang harus dinyatakan sistem — kafe punya
   `table_status_from_order.star` (subscription) yang menulis `dining-table`.
4. Meninjau ±10 situs sistem.
5. Test per jalur: HTTP (sudah ada) + **script** (baru) + **sistem** (baru,
   memastikan bypass eksplisit bekerja).

## Definisi selesai

- Menulis lewat HTTP **dan** `resource.save()` atas field/transisi bergerbang
  sama-sama ditolak tanpa permission; pesannya `403` di kedua jalur.
- Penulis sistem lolos **karena menyatakan dirinya sistem**, bukan karena lupa
  mengisi permission.
- Test menutup ketiga jalur; bila penegakan store dilepas, test **gagal**.
- Ledger: 10.46 ✅, 10.71 ✅; 10.67 tetap ⏸️ dengan alasan yang diperbarui
  (bukan lagi "lapisan", melainkan "model grant belum punya dimensi baris").

## Referensi

- `docs/spec/backend/01-core-basic.md` §1.7 (scope/row_scope), §5 (action)
- `docs/spec/backend/05-field-types.md` §5.1–§5.3
- kafe 10.46 / 10.67 / 10.71; changelog `2026-10-03-006`
