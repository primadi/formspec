# 2026-09-27-020 — Menutup sisa yang terbuka: 4 item kafe + 2 temuan baru

**Apa:** menindaklanjuti "perbaiki yang masih open" pada gelombang alur sesi meja.
Empat item ditutup, satu **ditarik** karena klaimnya salah, dan dua temuan baru
diberi nomor. Kafe: **89 manifest, 0 problem**; `go test ./...` dan
`make e2e-kafe` hijau.

## Ditutup

### 10.42 — baris lama membuat transisi state 500 (diperbaiki di engine)

`validateStateTransition` membaca `!oldExists` — nilai **LAMA** tidak ada — lalu
memperlakukannya sebagai "record baru" dan menuntut nilai baru **sama dengan**
initial state. Akibatnya `available → occupied` yang sah dijawab
**500 `initial state must be "available"`** pada baris yang dibuat sebelum field
`table_status` ada (terukur: 6 dari 7 meja di DB dev).

Cek itu **tidak pernah benar**: satu-satunya pemanggilnya adalah `Update`
(dikonfirmasi 1 call site), dan `Insert` selalu memberi initial state lewat
`applyDefaults` — jadi cabang "record baru" mustahil di jalur itu. Sekarang baris
tanpa state diperlakukan sebagai "berada di initial state", lalu transisinya
divalidasi normal.

Dipilih memperbaiki **sumbernya**, bukan menulis backfill: backfill menyembuhkan
satu database dan meninggalkan jebakan yang sama untuk entity berikutnya yang
menambah field status. Bukti: `TestEntityStore_StateMachineTransition_LegacyRowMissingField`
— sekaligus memastikan transisi ilegal tetap ditolak (fix tidak boleh meloloskan
apa pun).

### 10.35a — sesi tidak pernah ditutup (dan itu mengunci meja)

Entri ini bertanya _"ditutup otomatis saat meja di-`release`?"_. Jawabannya ya,
dan ternyata **bukan opsional**: 10.34c mengizinkan satu sesi TERBUKA per meja,
dan halaman masuk QR membuat sesi tiap kali kartu dipindai — jadi tanpa penutup,
meja hanya bisa dipakai **sekali seumur hidup**. Tamu kedua mendapat
**500 `UNIQUE constraint failed: cafe_order_table_sessions._dining_table_id`**.

Transisi `release` kini memancarkan `on_cleared` (durable), dan subscription
`cafe-order/table-cleared` + `close_session_on_clear.star` menutup sesi meja itu
(`closed_at` ikut diisi). Konsumennya di `cafe-order` karena itu pemilik
`table-session` — pola yang sama dengan `cafe-master/table-occupancy` arah
sebaliknya.

Bukti: `TestKafe_TableLifecycle_SessionClosesOnRelease` (meja sama, dua tamu
berturut-turut). Test ini **gagal lebih dulu** (`status="open"`) — reproduksi
sengaja sebelum perbaikan.

### 10.53 — satu grant jelek melemahkan seluruh role

`Materialize` kembali pada error pertama, jadi satu nama page yang salah
membatalkan **semua** grant role. Ditambahkan `MaterializePartial`: resolusi
per-grant, yang gagal dilewati **dan dinamai**. `Materialize` strict tetap ada
untuk validator/test.

**Koreksi yang perlu tercatat:** pengamatan awal saya ("`kasir` → 0 permission
karena grant") **salah** — penyebabnya `role.app` (login tanpa `app` menyaring
role per-App keluar). Jadi hazard itu **laten**, bukan aktif; ia dibuktikan
dengan test (`TestMaterializePartial_KeepsGoodGrants`), bukan dengan klaim.
Dua kelas masalah kini dilaporkan: page tidak dikenal, dan page yang resolve tapi
action-nya tidak ada di footprint-nya (kelas 10.47).

### 10.43 — SUPERSEDED (klaimnya sudah tidak benar)

Entri itu menyatakan `PATCH` diperiksa `{plural}.update`, **bukan**
`require_permission` transisi. Itu sudah tidak benar sejak changelog `-005`/`-006`/`-007`
(hari yang sama, setelah entri ini ditulis): `HandleUpdate` membaca
`spec.TransitionPermission(*trans)` dan menolak 403
(`internal/api/handler.go:1047`).

Buktinya dari harness, bukan pembacaan kode: `dapur` (punya `orders.update`,
tanpa `orders.mark-served`) → `PATCH status=served` → **403** dengan
`(required for transition ready -> served)`; `pelayan` → **200**.

Dibiarkan terbaca sebagai "gate bukan batas keamanan" itu **berbahaya**: ia
membuat satu-satunya pembeda `mark-served`/`mark-ready` dianggap dekorasi.
Yang benar-benar masih terbuka sudah punya entri: jalur **script** (10.46) dan
`PATCH` UI yang mengirim seluruh field (10.46).

## Ditarik karena SALAH (bukan diperbaiki)

### 10.54 — `LIMIT 1` yang saya klaim tidak ada, ternyata ada

Saya menulis item ini dari **penalaran**, bukan dari membaca kode: "fallback
`GetByID` memakai `WHERE _<natural_key> = ?`, dan tidak ada `LIMIT 1` yang bisa
dinyatakan". `FindByField` justru membangun
`... WHERE _<field> = ? AND tenant_id = ? [AND deleted_at IS NULL] LIMIT 1`, dan
`FindByFields` sama untuk match multi-kolom. Tidak ada pekerjaan di sini.

Dicatat sebagai **RETIRED**, bukan dihapus, dan alasannya ditulis di entri itu —
supaya pola kesalahannya terlihat: dua item lain di gelombang yang sama
(10.42 benar, klaim "kasir 0 permission" di 10.53 salah) lahir dari kebiasaan
yang sama, menyimpulkan dari inferensi alih-alih membuka tempat yang dimaksud.

## Temuan baru bernomor

- **10.56 — handler subscription tidak bisa mendeklarasikan `uses`.** Dispatcher
  membuat `spec.Action` sintetis yang hanya mengisi `Name` + `Impl`, dan
  `SubscriptionSpec` tidak punya field `Uses`. Di ProdMode, `ctx.db` lalu gagal
  dengan `USES_VIOLATION` yang **menyuruh operator menambahkan
  `uses.primitives: [db]`** — sesuatu yang tidak mungkin dituruti pada
  subscription. Belum menggigit (handler kafe memakai `resource.*`, bukan
  `ctx.*`), tetapi `ctx.now()` yang saya pakai **lolos** hanya karena
  `builtinNow` tidak memanggil `checkPrimitive` — jadi pemeriksaannya tidak
  konsisten antar-primitive, bukan "aman".
- **10.57 — sesi yang dibuat lalu ditinggalkan mengunci mejanya.** Celah yang
  **terbuka karena memperbaiki 10.35a**: penutupnya adalah `release`, dan
  `release` hanya bisa dari `occupied`/`served` — sedangkan meja baru `occupied`
  setelah BAYAR. Tamu yang memindai lalu pergi meninggalkan sesi `open` tanpa
  jalur keluar, dan (sejak 10.34c) itu memblokir tamu berikutnya dengan 500.
  Jadi 10.35a selesai untuk jalur normal, belum untuk jalur ditinggalkan.

## File

`renderers/jsonb-persist/crud.go` (+`crud_test.go`),
`internal/auth/materialize.go` (+`materialize_test.go`, `resolver.go`),
`examples/kafe/spec/modules/cafe-master/master/dining-table/entity.yaml`,
`examples/kafe/spec/modules/cafe-order/{subscriptions/table-cleared.yaml,
scripts/close_session_on_clear.star}`, `resource/kafe_table_lifecycle_e2e_test.go`,
`examples/kafe/gaps_found/TODO.md`
