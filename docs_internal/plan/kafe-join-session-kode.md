# Plan — Tamu kedua JOIN ke sesi meja (kode 4–6 digit)

**Tanggal:** 2026-09-28. **Status:** P1 ✅ · P2 ✅ · P3 ✅ · P4–P6 ⏸️ belum.
**Keputusan pemilik (pemicu):** _"tamu yg scan harus join ke session meja, caranya
harus input token (4-6 digit) yg bisa dilihat oleh pemilik session, atau tanya ke
kasir, yg bisa melihat token untuk meja tertentu."_
**Menutup:** kafe **10.57** (sesi ditinggalkan/meja terkunci) dan **10.38** +
**10.39** (yang selama ini jadi penghalangnya).

## Status per fase

| #      | Fase                                   | Status | Catatan                                          |
| ------ | -------------------------------------- | ------ | ------------------------------------------------ |
| **P1** | Service action publik (10.39)          | ✅     | `Action.Public` + generator + validator + 3 test |
| **P2** | `ctx.random_digits(n)` (10.38)         | ✅     | `crypto/rand` + rejection sampling + 4 test      |
| **P3** | Form → Service + redirect dari respons | ✅     | dipilih (a); alur tetap deklaratif               |
| **P4** | Kafe: `join_code` + Service            | ⏸️     | butuh P1 ✅ + P2 ✅ (keduanya siap)              |
| **P5** | Kafe: UI (form + kolom kasir)          | ⏸️     | butuh P3 + P4                                    |
| **P6** | Test + docs                            | ⏸️     | butuh P4 + P5                                    |

P1 dan P2 mendarat lebih kecil dari perkiraan, karena dua penemuan:

- **Jalur route anonim sudah lengkap** — `registerRouteWithPattern` sudah
  memasangkan `rd.Public` dengan `RequirePermissionOrAnonymous`, dan route
  Service sudah lewat jalur itu. P1 jadi "isi flag yang belum pernah diisi".
- **`RateLimitSpec` di Service sudah ditegakkan** — `HandleServiceAction`
  memanggil `rateLimitFor`, jadi tidak ada pekerjaan baru untuk rate limit.

## Koreksi yang ditemukan saat implementasi P1

1. **Komentar di `registerRouteWithPattern` bertentangan dengan
   `RequirePermissionOrAnonymous`.** Komentar bilang _"a signed-in caller on the
   same route still needs the permission (#45)"_; dokumen middleware-nya bilang
   sebaliknya — **public grant adalah LANTAI, bukan jalur khusus anonim**, jadi
   pemanggil yang sudah login **tanpa** permission juga diizinkan (itu sengaja,
   supaya login tidak membuat orang lebih buruk daripada tamu). Komentar lama
   itu yang menyesatkan saya menulis assertion `403` yang salah. Sudah
   diperbaiki di kedua tempat, dan perilakunya sekarang dikunci test.
2. **Script action harus `return ok({...})`, bukan dict biasa.** `echo.star`
   pertama saya mengembalikan `{"echo": ...}` dan hasilnya `data: nil` dengan
   **200 OK** — sukses yang tidak membawa apa-apa. Ini kelas yang mudah
   terlewat: tidak ada error, hanya nilai yang hilang.
3. **`Action.Public` tidak boleh berlaku untuk entity.** Entity sudah punya
   jalur anonim sendiri (`public_entities` di App), dan itu satu-satunya tempat
   operator meninjau keputusan tersebut. Validator menolak `public: true` pada
   entity action dan mengarahkannya ke `public_entities`.
4. **`public: true` WAJIB punya `rate_limit`.** Sebuah endpoint anonim tanpa
   rate limit adalah vektor penyalahgunaan, dan hanya manifest yang bisa
   menyatakannya. Ditolak saat validasi, bukan diperingatkan.

## Kebutuhan, dinyatakan tegas

1. Meja **tanpa** sesi terbuka → tamu membuat sesi (perilaku hari ini).
2. Meja **dengan** sesi terbuka → tamu **JOIN ke sesi itu**, membuktikan
   kehadiran dengan kode 4–6 digit.
3. Kode **terlihat oleh pemilik sesi** (tamu pertama).
4. **Kasir bisa melihat kode untuk meja tertentu** (kalau tamu bertanya).
5. Salah kode → ditolak, tidak boleh bisa ditebak paksa.

**Konsekuensi model yang harus eksplisit:** "join ke sesi" berarti **satu sesi
per meja** (aturan 10.34c dipertahankan), dan tamu kedua **berbagi bill** —
pesanannya membawa `guest_token` sesi itu, bukan token sendiri. Ini yang membuat
tamu kedua bisa melihat pesanan meja yang sama, dan itu memang yang diminta.

## P3 — yang mendarat

Jalan **(a)** dipilih (deklaratif), bukan asset, jadi alur kafe tetap murni YAML.

| Bagian                              | Isi                                                                                                                                                                               |
| ----------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `FormSubmit.Call`                   | `"module.service.action"`. Bila diset, submit form **memanggil Service**, bukan menulis entity — service yang memiliki mutasi                                                     |
| `{response.*}` di `submit.redirect` | Satu-satunya cara menjangkau nilai yang **diputuskan server**: setelah join, token yang harus dibawa tamu adalah milik sesi yang **sudah ada**, yang tidak pernah diketahui klien |
| `lib/serviceCall.ts`                | `serviceCallPath()` — pemilik aritmetika prefix `../service/...`, diuji terhadap server HTTP nyata                                                                                |
| `lib/submitRedirect.ts`             | `resolveSubmitRedirect()` — resolusi token + **menolak navigate** pada token yang tak terpecahkan                                                                                 |
| Validasi                            | `submit.call` harus 3 segmen (`internal/ui/validate.go`)                                                                                                                          |

**Dua keputusan yang perlu dibaca sebelum mengubahnya:**

1. **Tombol submit TIDAK digating permission entity saat `submit.call` diset.**
   Gate lama memeriksa `create`/`update` entity, dan form seperti ini **tidak
   menulis entity itu** — memeriksanya berarti memeriksa hal yang salah, dan ia
   gagal tepat pada alur yang dimotori fitur ini: check-in tamu berjalan
   **anonim**, dan begitu service yang menulis, grant `create` anonim pada
   `table-session` justru **dicabut** (grant itu adalah keluhan asli 10.34 —
   create anonim tanpa bukti kehadiran). Klien tidak punya metadata service
   untuk menggantikannya, jadi posisi jujurnya: render, biarkan gate milik
   service yang memutuskan. **Biarannya nyata dan diterima:** pada form privat
   yang memanggil service terbatas, tombolnya tampil dan server menjawab 403.
   **Sisa → 10.58**: kirim metadata service (nama + permission + `public`) di
   meta bundle supaya klien bisa pra-cek.
2. **`{response.*}` TIDAK di-spread ke context.** Ia di-namespace di bawah
   `response` supaya sebuah service tidak bisa **menutupi** slot framework
   (`route`, `user`, `session`, `fields`) dan mengalihkan pemanggil ke tempat
   yang ia pilih. Dikunci test `does not let the response SHADOW a framework
context slot`.

**Kegagalan yang dicegah, bukan diperbaiki:** `interpolateTokens` sengaja
membiarkan token yang tak dikenal **verbatim** — benar untuk nilai field
(placeholder terlihat lebih baik daripada kosong misterius), dan **salah untuk
alamat**. Navigasi ke `/menu/{response.guest_token}` tampak berhasil, mendaratkan
tamu di halaman kosong, dan tidak melaporkan apa pun. Karena itu resolusi redirect
memiliki tipe hasil eksplisit dan **tidak** menavigasi saat token tak terpecahkan.

## Temuan riset: yang sudah ada vs yang harus dibangun

Ini yang mengubah ukuran pekerjaannya — sebagian besar mesinnya **sudah ada**.

| Temuan                                                    | Bukti                                                                                                                                                              | Arti                                                                       |
| --------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------- |
| Jalur route anonim sudah lengkap                          | `registerRouteWithPattern`: `case rd.Public && rd.RequiredPermission != ""` → `RequirePermissionOrAnonymous` (`internal/api/router.go`)                            | Tinggal **mengisi** `rd.Public`                                            |
| Route Service sudah lewat jalur itu                       | `GenerateUIServiceRoutes` (`internal/api/generator.go:560`) mengeset `RequiredPermission` (default `{module}.{service}.{action}`) tetapi **tidak pernah** `Public` | 10.39 = **kecil**, bukan medium seperti tertulis di ledger                 |
| Service sudah punya rate limit                            | `HandleServiceAction` memanggil `f.rateLimitFor(w, r, module, serviceName, actionName)` (`handler.go:2658`)                                                        | `Action.RateLimit` di service langsung bekerja                             |
| Keamanan per-field sudah ada                              | `internal/api/fieldsec.go`: `exclude` (per surface), `required_permission` (per identitas → field **dihapus**), `masked` (nilai disamarkan)                        | Kode bisa disembunyikan dari anonim **tanpa mekanisme baru**               |
| Role hanya bisa di-grant page+action                      | entity `formspec.core.role` hanya punya `{name, app, module, description, grants}`; tidak ada `permissions` mentah                                                 | Permission field harus memakai permission yang **sudah** materializable    |
| `guest_token` = natural key                               | `table-session/entity.yaml` (`natural_key: true`)                                                                                                                  | `/menu/{guest_token}` resolve; kode bisa dipakai di redirect               |
| Native impl **tidak bisa** dimuat `formspec dev`          | `app.RegisterNative(ref, handler)` dipanggil dari Go (`resource/formspec.go:953`), bukan dari spec dir                                                             | Module native untuk kafe akan **merusak** jalur run/test yang ada → jangan |
| Starlark punya `ctx.now`/`ctx.next_key`, **tanpa** random | `internal/starlark/context.go` (`builtinNow`, `builtinNextKey`)                                                                                                    | Sumber acak harus ditambah di engine                                       |

**Satu dokumen basi yang ikut ketemu** (bukan bagian fitur, tapi salah):
`formspec.core/seeds/roles.yaml` masih menulis _"resolver **melewati role itu
tanpa suara** (`continue // malformed grant — skip role`)"_. Itu tidak lagi benar
sejak `2026-09-27-020`: resolver memakai `MaterializePartial` dan **melaporkan**
grant yang gagal. Diperbaiki di Fase 5.

## Desain

### Penyimpanan kode

- Field baru pada `table-session`: `join_code` (string, 6 digit), **digating**
  `required_permission: cafe-order.table-sessions.view`.
  - Anonim (tidak punya permission) → field **dihapus** dari respons (`fieldsec.go`).
  - Kasir/staf (punya `view`) → melihat kode di daftar sesi. **Ini yang menjawab
    kebutuhan (4)** tanpa UI baru — cukup kolom di table sesi untuk staf.
  - Pemilik sesi (anonim) → tidak bisa membaca lewat entity; ia menerima kodenya
    **dari respons Service saat sesi dibuat** (kebutuhan 3).

  **Kenapa TIDAK di-hash** (membalik rekomendasi 10.38): kebutuhan (4) menyatakan
  kasir harus **melihat** kode. Yang tidak bisa dibaca tidak bisa ditunjukkan.
  Jadi kode disimpan plaintext dan pertahanannya bukan one-way hash, melainkan:
  (a) tidak pernah terkirim ke anonim, (b) rate limit per-IP, (c) **kunci setelah
  N percobaan gagal** per sesi. Ini harus ditulis jujur — kode ini **bukan
  kredensial kuat**, ia bukti kehadiran fisik; 10.000–1.000.000 kemungkinan hanya
  bermakna bersama (b) dan (c).

  **Kenapa 6, bukan 4:** 4 digit = 10.000 kemungkinan. Dengan lockout 5 percobaan
  per sesi, dua angka itu sama-sama memadai — tetapi 6 memberi ruang jauh lebih
  besar bila lockout salah konfigurasi. Rentang 4–6 tetap didukung (dikonfigurasi),
  default **6**.

- Field pendukung: `join_attempts` (integer, default 0) — penghitung kegagalan
  untuk lockout. Ditulis hanya oleh Service (server-side), tidak pernah dipapar.

### Operasi server: `kind: Service` `cafe-order/table-access`

Satu action yang menyelesaikan seluruh keputusan, karena klien tidak boleh tahu
apakah meja terisi tanpa membocorkan sesuatu:

```yaml
kind: Service
metadata: { name: table-access, module: cafe-order }
spec:
  version: v1
  actions:
    - name: open
      public: true # ← field baru (Fase 1)
      rate_limit: { max: 10, per: 1m, scope: ip }
      params:
        validate:
          - { field: qr_token, rule: required }
      uses: { resources: [cafe-order.table-session, cafe-master.dining-table] }
      impl: { type: script_ref, ref: cafe-order/open_table_session }
```

Perilaku `open({qr_token, join_code?, guest_token, guest_name?, guest_count?})`:

| Keadaan                         | Hasil                                                                                                                 |
| ------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| Meja bebas                      | buat sesi; `join_code` di-generate; `{mode: "created", guest_token, join_code}`                                       |
| Meja terisi, `join_code` kosong | **422** `JOIN_CODE_REQUIRED` — pesan: _"meja A-01 sedang terisi. Masukkan kode dari pemilik sesi, atau tanya kasir."_ |
| Meja terisi, kode **salah**     | **403** `JOIN_CODE_INVALID` + `join_attempts` naik; setelah 5 → **429/423** `JOIN_LOCKED`                             |
| Meja terisi, kode **benar**     | `{mode: "joined", guest_token: <token sesi yang ada>}` — **tamu kedua memakai token pemilik**, jadi bill-nya satu     |

Catatan penting: pada `joined`, `guest_token` yang dikembalikan adalah milik
**sesi yang sudah ada**, bukan token yang dikirim tamu kedua. Itu yang membuat
dua tamu berbagi satu bill.

### Bagaimana tamu mendarat di menu setelah JOIN

Ini masalah nyata, bukan detail: setelah `joined`, token yang harus dipakai ada di
**respons**, sedangkan `submit.redirect` hari ini hanya bisa menginterpolasi
**render context** (`interpolateTokens(redirect, ctx)`), bukan respons.

Dua jalan:

- **(a) Deklaratif — dua tambahan engine kecil:** Form boleh menargetkan Service
  (`submit.call: {call: "cafe-order.table-access.open"}`) dan `submit.redirect`
  boleh memakai `{response.guest_token}`. Keduanya berguna umum, bukan khusus
  kafe: "panggil operasi server, lalu mendarat di hasilnya".
- **(b) Custom Page + asset** (`mode: custom`, komponen JS memanggil Service lewat
  `formspec.api` lalu `formspec.navigate`). Didukung hari ini tanpa perubahan
  engine, tetapi menyimpang dari sifat kafe yang deklaratif dan menambah JS ke
  contoh yang selama ini murni YAML.

**Rekomendasi: (a).** Ia menjaga kafe tetap deklaratif, dan kedua kemampuan itu
memang lubang umum (form tidak bisa memanggil operasi server lalu memakai
hasilnya). (b) tetap dicatat sebagai jalan mundur bila (a) ternyata lebih besar
dari perkiraan.

## Fase

| #      | Fase                                           | Isi                                                                                                                                                    | Effort       | Depends |
| ------ | ---------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------ | ------- |
| **P1** | Engine: Service action publik (10.39)          | `Action.Public bool`; `GenerateUIServiceRoutes` → `Public: true`; validator (Public hanya untuk Service; entity tetap lewat `public_entities`); test   | small        | —       |
| **P2** | Engine: `ctx.random_digits(n)` (10.38)         | helper `crypto/rand` di `internal/starlark/context.go`, batas n (4–12), test; dokumentasi di `docs/reference/primitives.md`                            | small        | —       |
| **P3** | Engine: form → Service + redirect dari respons | `FormSubmit.Call` + `{response.*}` di `submit.redirect`; validasi + test                                                                               | medium       | —       |
| **P4** | Kafe: kode + Service                           | `join_code`/`join_attempts` pada `table-session`; Service `table-access` + `open_table_session.star`; lockout                                          | medium       | P1,P2   |
| **P5** | Kafe: UI                                       | check-in form memakai Service + redirect; kolom kode di table sesi staf; perbaiki doc basi di `roles.yaml`                                             | small–medium | P3,P4   |
| **P6** | Test + docs                                    | Go e2e (create/join/salah-kode/lockout/anonim-tidak-bisa-baca-kode), Playwright (dua konteks tamu), ledger 10.38/10.39/10.57, changelog, `overview.md` | medium       | P4,P5   |

P1, P2, P3 saling bebas → boleh paralel. P4 butuh P1+P2; P5 butuh P3+P4.

## Verifikasi yang harus lulus

- **Anonim tidak bisa membaca kode:** `GET table-session/{token}` anonim → field
  `join_code` **tidak ada** di JSON; kasir (punya `table-sessions.view`) → ada.
  Ini test terpenting; kalau gagal, seluruh fitur tidak aman.
- Join sukses: tamu kedua memakai `guest_token` **sama** dengan pemilik; `list
order` keduanya melihat pesanan yang sama.
- Salah kode 5× → terkunci; kode benar setelah terkunci → tetap ditolak.
- Rate limit: > 10 permintaan/menit dari satu IP → 429.
- Meja bebas → sesi dibuat, kode dikembalikan **sekali** di respons.
- `formspec validate` kafe 0 problem; `go test ./...`; `make e2e-kafe`; vitest;
  `gofmt`; oxlint tetap 41.

## Sisa yang akan tetap terbuka (dicatat, bukan disembunyikan)

- **Kode plaintext** (konsekuensi kebutuhan 4). Dicatat di ledger dengan alasan.
- **Lockout per sesi** tidak menghalangi penyerang mencoba di **banyak meja**
  (satu percobaan per meja). Rate limit per-IP menutup sebagian; sisa dicatat.
- **Sesi yang dibuat lalu ditinggalkan** (10.57) belum sepenuhnya tertutup: kode
  menyelesaikan kasus "tamu kedua mau masuk", **bukan** kasus "tamu pertama pergi
  tanpa bayar" — meja tetap terkunci sampai kasir menutup. Perlu keputusan
  terpisah (kedaluwarsa/`expires_at`).
- **(a) vs (b)** di atas: kalau P3 membengkak, pindah ke asset.

## Yang TIDAK dikerjakan

- Modul native Go untuk kafe (akan merusak `formspec dev` + harness E2E).
- Hashing kode (bertentangan dengan kebutuhan 4).
- Mengubah sumbu "terisi" meja ke sesi terbuka — keputusan pemilik 2026-09-27
  (terisi = dibayar) tetap berlaku.
