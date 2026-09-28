# 2026-09-28-006 — Kafe void end-to-end: 2 bug ditemukan & ditutup (5.24.4)

**Plan:** `docs_internal/plan/action-input-contract.md`
**Menutup:** todo `5.24.4` ⏸️, `5.24.5` ✅ (baru, ditemukan di sini)

## Kenapa dikerjakan

`TestKafe_TableLifecycle_CancelReturnsToAvailable` sengaja menghindari
`void-order` — approval-gated, dan alasannya tidak bisa dikumpulkan. Kontrak
input menutup paruh kedua itu, jadi rantai penuhnya sudah bisa dijalankan. Ini
satu-satunya cara mengetahui tiga perbaikan (`2026-09-28-004`) benar-benar
menyatu di aplikasi nyata, bukan hanya di fixture.

Hasil: **dua bug**, keduanya tidak terlihat oleh test yang ada. Yang pertama ada
di kode saya sendiri.

## Bug 1 — transisi kehilangan `params` saat ada entri `actions:` (kode saya)

`EffectiveActionSpec` mengembalikan action dari `ActionSources()`, dan
`ActionSources()` membiarkan **action yang dideklarasikan menang**. Itu benar
untuk "entri mana yang tampil di `actions[]`", tetapi salah untuk "kontrak apa
yang berlaku bagi transisi ini". Entri `actions:` hanya bertahan melewati
`ValidateActionTransitionDuplication` bila ia menambah sesuatu yang **tidak**
dimiliki transisi — jadi keduanya memang sah-sah saja mendeskripsikan paruh yang
berbeda dari satu action.

Kafe `void-order`: entri `actions:` menyumbang `description` + `audit` +
`conditions`; **transisi** menyumbang `params.inputs`. Membaca entri saja
membuang input-nya → nama input tidak teresolusi → `TransitionInputParams`
mengembalikan `nil` → `params.get('void_reason')` tidak melihat apa pun.

**Terukur:** `PATCH {"status":"cancelled","void_reason":"tamu komplain"}`
→ **422 `CONDITION_FAILED: Alasan void wajib diisi`**, padahal body-nya jelas
memuat alasannya. Setiap void ditolak, selamanya.

Ditutup dengan overlay per-field di `EffectiveActionSpec`: `Params` milik
transisi menang bila ada, dan `Conditions` **digabung** (milik transisi lebih
dulu) — bukan diganti, karena mengganti berarti gate yang dideklarasikan bisa
berhenti ditegakkan tanpa error di mana pun.

## Bug 2 — transisi ber-approval tidak pernah memancarkan `emit`

`HandleUpdate` `return` **sebelum** blok resolusi emisi begitu approval
diperlukan, dan `executeWorkflowTransition` tidak meresolusinya sendiri — jadi
transisi yang approval-gated **tidak memancarkan apa pun**.

**Terukur:** kafe `void-order` (`emit: on_cancel`) → order menjadi `cancelled`,
meja tetap `occupied` selamanya (`waitForTableStatus` timeout pada
`"available"`, terakhir terlihat `"occupied"`). Jembatan meja-lah yang
mendengarkan `on_cancel`, dan indeks unik parsial menolak sesi OPEN kedua pada
satu meja — jadi **tamu berikutnya tidak bisa check-in**. Menjalankan transisi
yang sama **tanpa** workflow memancarkan event-nya (dibuktikan
`TestKafe_TableLifecycle_CancelReturnsToAvailable`), dan itulah yang membuat
asimetri ini tidak terlihat.

Ditutup di `executeWorkflowTransition`: resolusi `ResolveTransitionEmission`
dengan pasangan `(fromState, toState)` — `fromState` datang dari baris approval,
jadi pasangan itu tetap mengidentifikasi transisinya meski panggilan approval
adalah request yang berbeda — lalu `PendingEvents` ikut ke `store.Update` yang
sama, sehingga state + event **atomik**.

## File yang terdampak

- `pkg/spec/action_input.go` — overlay `Params`/`Conditions` di `EffectiveActionSpec`
- `pkg/spec/action_input_params_test.go` — 3 test overlay (termasuk anti-aliasing)
- `internal/api/handler.go` — `executeWorkflowTransition` menerima `fromState`,
  meresolusikan emisi, dan mengirim `PendingEvents`; 2 call site diperbarui
- `resource/kafe_void_approval_e2e_test.go` (baru) — 2 test e2e kafe
- `docs/spec/backend/01-core-basic.md` — `emit` juga berlaku pada jalur approval
- `docs/spec/backend/02-core-extended.md` — approval menunda, tidak mengubah

## Bukti

E2E kafe, pada spec kafe asli (`resource/kafe_void_approval_e2e_test.go`):

| Langkah                    | Hasil                                                                         |
| -------------------------- | ----------------------------------------------------------------------------- |
| void **tanpa** alasan      | **422** (dulu lolos: `conditions` tak dievaluasi di PATCH)                    |
| void **dengan** alasan     | **202**, tidak ada yang tertulis (status tetap `paid`, `void_reason` kosong)  |
| pemohon menyetujui sendiri | **403** (requester exclusion, 7.4.5)                                          |
| supervisor menyetujui      | **200**                                                                       |
| setelah approval           | status `cancelled` **dan** `void_reason` = alasan pemohon — tersimpan bersama |
| meja setelah void          | kembali **`available`** (dulu tetap `occupied`)                               |

Kalibrasi: test **gagal** dulu dengan pesan yang tepat (`void dengan alasan =
422`; timeout pada `available`), lalu hijau setelah masing-masing fix. Fix 1
menutup perbaikan `2026-09-28-004` yang ternyata belum bekerja pada manifest
kafe — jadi ini bukan temuan teoretis.

`go test ./...` hijau. Tiga test overlay baru juga dibuktikan gagal tanpa
overlay (entri `actions:` tidak punya `Params`, sehingga `got.Params == nil`).

## Catatan

Dua bug ini **tidak** ditemukan unit test, dan keduanya hanya muncul karena
deklarasi kontrak dan deklarasi action berdiri di dua tempat berbeda pada
manifest yang sama. Pola yang layak diingat: `ActionSources()` menang-menangan
adalah keputusan tentang **daftar**, bukan tentang **kontrak** — dan setiap
konsumen yang membaca kontrak harus lewat satu fungsi yang menggabungkan
keduanya, seperti `src/lib/actionParams.ts` sudah melakukannya di renderer
(`transition.params ?? action.params`).

## Sisa

Tidak ada sisa baru. `5.24.2` ⏸️ (Tier 2 `params.form.ref`) tetap terbuka dan
tidak tersentuh.
