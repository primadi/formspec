# 2026-10-03-008 — Gerbang transisi ditegakkan di store; principal sistem eksplisit (kafe 10.46)

**Yang dikerjakan.** Langkah 3–4 dari plan `docs_internal/plan/lapisan-otorisasi.md`:
gerbang transisi pindah ke **store**, dan jalur script/worker mendapat **principal
sistem yang eksplisit**. Ini menutup **10.46**.

## Cacat yang diukur

`require_permission` pada transisi hanya diperiksa di **handler HTTP**, sehingga
`resource.save()` dari sebuah script **melewati transisi apa pun** tanpa
pemeriksaan. Terukur (kalibrasi di bawah): dengan gerbang store dimatikan, sebuah
script yang menyeberangi transisi bergerbang menjawab **200** — bukan 403.

## Penghalang yang harus dipecahkan lebih dulu

`SetSaveHandler` dipakai **bersama** oleh (a) script aksi yang dipicu HTTP dan
(b) script yang dijalankan subscription/worker. Menandai `SystemCaller: true`
untuk keduanya akan **membuka** gerbang di jalur (a) — memperburuk keadaan.

Jalan keluarnya bukan menebak, melainkan **membedakan konteks secara eksplisit**:

- **`action.ExecuteParams.SystemCaller`** — lapisan dispatch menyatakan asal
  eksekusi. **Hanya** `subscription.Dispatcher` yang menyetelnya (event terjadi
  tanpa pengguna).
- Marker itu ikut ke dalam **context** (`action.WithSystemCaller`) lewat
  `ScriptExecutor`, karena handler tulis hanya menerima context.
- Handler tulis (`resource.save`/`resource.create`) meneruskan
  `SystemCaller: action.IsSystemCaller(ctx)` **beserta** permissions pemanggil.

**Kenapa tidak boleh disimpulkan dari "tidak ada identitas".** Pemanggil
**anonim** juga tidak punya identitas. Menyimpulkan sistem dari ketiadaan
identitas akan **menaikkan** request anonim ke hak istimewa sistem. Karena itu
penanda wajib eksplisit, dan **absen penanda = bukan sistem** (fail-closed).

## Perubahan

1. `validateStateTransition(old, new, permissions, systemCaller)` — memeriksa
   `spec.TransitionPermission(t)` pada transisi yang cocok, **kecuali**
   `systemCaller`.
2. `action.ExecuteParams.SystemCaller` + `action.WithSystemCaller/IsSystemCaller`.
3. `subscription.Dispatcher` menyetel `SystemCaller: true`.
4. Handler tulis script meneruskan principal ke store (permissions + system).
5. **`spec.QualifyPermission`** — aturan auto-prefix **dipindah ke satu tempat
   bersama**. Store harus mengkualifikasi persis seperti handler HTTP, kalau tidak
   gerbangnya **tak pernah bisa dibuka** (`orders.confirm-payment` tidak akan
   pernah cocok dengan `cafe-order.orders.confirm-payment` yang termaterialisasi)
   — mode kegagalan yang sudah tercatat sebagai kafe 10.47.
   `internal/permission.AutoPrefixPermission` kini **mendelegasikan** ke helper itu
   (aturan disalin dari sana, kini tinggal satu implementasi).

## Bukti

- **Store (unit)** `renderers/jsonb-persist/transition_permission_test.go` — 5
  subtest: transisi bergerbang ditolak tanpa permission · **lolos dengan
  permission TERKUALIFIKASI** (kalau gagal, gerbangnya tak bisa dibuka) ·
  deklarasi tanpa prefix tetap cocok dengan pemegang permission ber-prefix ·
  transisi tanpa gerbang tak terpengaruh · **`SystemCaller` mem-bypass, dan
  payload yang sama TANPA penanda tetap ditolak** (anonim HTTP tampak persis
  seperti ini).
- **Script (e2e)** `resource/transition_permission_script_e2e_test.go` — aksi
  Starlark nyata atas `resource.save()`: transisi bergerbang **403** tanpa
  permission, sukses dengan, dan transisi tak bergerbang tak terpengaruh.
- **Kafe (in-process)** `TestKafe_TableLifecycle_FullScenario` dkk. tetap hijau —
  subscription yang menulis `dining-table` melewati transisi bergerbang
  (`mark-table-served`) **sebagai sistem**.
- **Dev server (live, DB bersih):** `in_kitchen` oleh **kasir → 403** (bukan
  haknya), oleh **barista → 200**; `ready` → 200; `served` oleh pelayan → 200;
  dan **meja berakhir `served`** — artinya subscription benar-benar menyeberangi
  transisi **bergerbang** `available/occupied → served` sebagai sistem.
- **Kalibrasi:** gerbang store dimatikan → test store gagal, dan e2e script
  mengembalikan **200** (transisi bergerbang tertembus) — tepat bug 10.46.
  Dipulihkan dari backup `/tmp`.
- `go test -count=1 ./...` hijau · `gofmt` bersih · `formspec validate` 89/0.

## Sisa yang dicatat (bukan diklaim selesai)

**Gerbang sekarang berlaku di HTTP + `resource.save()`/`resource.create`. Yang
belum:** apakah **Insert** boleh lahir langsung di luar state awal. Perlu diukur
dan diputuskan tersendiri → kafe **10.72 ⏸️** (bukan bagian 10.46).

**Dampak.** `renderers/jsonb-persist/crud.go` · `internal/action/dispatcher.go`,
`internal/action/script.go`, `internal/action/systemcaller.go` (baru) ·
`internal/subscription/dispatch.go` · `resource/formspec.go` ·
`internal/permission/permission.go` · `pkg/spec/entity.go` · test baru
`renderers/jsonb-persist/transition_permission_test.go`,
`resource/transition_permission_script_e2e_test.go` · ledger kafe.
