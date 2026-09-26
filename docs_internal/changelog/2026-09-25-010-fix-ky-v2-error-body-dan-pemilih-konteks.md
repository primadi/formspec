# 2026-09-25-010 — Fix `Response body is already used` + pemilih konteks sesi

## Apa yang diubah

**1. Fix `Failed to execute 'clone' on 'Response': Response body is already
used` di form login.** `loginWithPassword`
(`renderers/react-shadcn/src/lib/api/auth.ts`) membaca body respons error lewat
`err.response.clone().json()`. Sejak `ky` di-bump ke `^2.0.2`, ky **mengonsumsi**
body itu sebelum melempar, untuk mengisi properti baru `HTTPError.data`
(`node_modules/ky/distribution/core/Ky.js:144` →
`#readResponseText` memakai `response.body.getReader()`, `:519`). `clone()`
berikutnya melempar `TypeError` — dan karena dilempar dari dalam blok `catch`,
ia **menggantikan** pesan server, sehingga setiap login gagal menampilkan
TypeError itu alih-alih `invalid username or password` / `RATE_LIMITED` /
`CONTEXT_REQUIRED`.

**2. Satu helper pembaca envelope** — `src/lib/api/errors.ts` (baru):
`errorEnvelope` (baca `err.data`), `buildFormaApiError`, `toFormaApiError`.
`auth.ts` memakainya dan melempar `FormaApiError` **typed** (bukan `Error`
polos), jadi `status`/`code`/`details`/`choices` tidak hilang. `authHooks.ts`
memakai helper yang sama tapi membaca dari `response.json()` — hook berjalan
**sebelum** `HTTPError` dibangun, jadi tidak ada `err.data` di sana. Pola
`.clone().json()` lama di hook aman hanya karena ky menyerahkan clone segar
(`Ky.js:624`) — sekarang tidak lagi bergantung pada kebetulan itu.

**3. Pemilih konteks sesi (todo 6.5.9, tahap 4 plan
`session-context-role-branch.md`).** Principal dengan >1 assignment (role ×
cabang) dijawab **409 `CONTEXT_REQUIRED` + `choices`** oleh server (§8.7), dan
sebelumnya respons itu tidak punya UI sama sekali. Kini: `ContextPicker`
(radiogroup, label `role · value`) tampil di `LoginScreen`, dengan prefill dari
pilihan terakhir (`lib/session-context.ts`, localStorage
`formspec-context:<ws>:<app>`) dan fallback ke pilihan pertama. Prefill **tidak**
auto-submit — boundary tetap dinyatakan pemanggil, sehingga jawaban audit
"sebagai role apa, di cabang mana" tetap sah.

**4. 409 saat refresh tidak lagi meng-expire sesi.** `Service.Refresh`
(`internal/auth/service.go:1143`) mengembalikan `ContextRequiredError`
**sebelum** `s.session.Delete`, jadi refresh token masih valid saat 409 —
meng-expire sesi di titik itu membuang kredensial yang masih bekerja.
`refreshSession` (`stores/session.ts`) yang dulu `catch { return false }` kini
menyimpan `pendingContext`, dan hook `authHooks` mendapat opsi `needsContext`
supaya tidak memanggil `notifySessionExpired()`. `SwitchContextScreen` (baru)
memakai `POST /_ui/auth/switch` untuk memilih konteks baru.

**5. `assignment` diteruskan di semua jalur login** — `loginWithPassword`
(parameter baru), `FormspecAuth.login` (`formspec-client.ts`), dan
`AuthFormRenderer` (form `auth_action` kustom) yang juga menampilkan picker.

## Kenapa

- Bug login: bump `ky` v2 tidak tercatat di changelog, dan `error.data`
  menggantikan cara membaca body error yang lama — regresi luput.
- Pemilih konteks: spec §8.7 sudah normatif sejak 2026-09-16 tapi tidak punya
  UI, sehingga 409 hanya bisa ditangani lewat curl (tercatat di todo 6.5.9).

## File terdampak

- `renderers/react-shadcn/src/lib/api/errors.ts` — **baru**, helper envelope
- `renderers/react-shadcn/src/lib/api/auth.ts` — `err.data`, `FormaApiError`
- `renderers/react-shadcn/src/lib/api/authHooks.ts` — `response.json()`, `needsContext`
- `renderers/react-shadcn/src/lib/api/client.ts`, `src/lib/api/meta.ts` — teruskan `needsContext`
- `renderers/react-shadcn/src/lib/session-context.ts` — **baru**, preferensi konteks
- `renderers/react-shadcn/src/lib/formspec-client.ts` — `assignment` di `FormspecAuth.login`
- `renderers/react-shadcn/src/shell/ContextPicker.tsx` — **baru**
- `renderers/react-shadcn/src/shell/SwitchContextScreen.tsx` — **baru**
- `renderers/react-shadcn/src/shell/LoginScreen.tsx` — picker + simpan pilihan
- `renderers/react-shadcn/src/stores/session.ts` — `pendingContext`
- `renderers/react-shadcn/src/stores/meta.ts` — teruskan `needsContext`
- `renderers/react-shadcn/src/kinds/form/AuthFormRenderer.tsx` — picker + `assignment`
- `renderers/react-shadcn/src/types/manifest.ts` — `ContextChoice`, `FormaApiError.choices`
- `renderers/react-shadcn/src/lib/api/auth.test.ts` — **baru** (8 test)
- `renderers/react-shadcn/src/shell/ContextPicker.test.tsx` — **baru** (11 test)
- `src/App.tsx` — `SwitchContextScreen` saat `pendingContext`

## Bukti

- `vitest` **473 lulus** / 31 file (baseline 462 / 30 — +11 test baru), stabil di
  dua run berturut-turut.
- `auth.test.ts` **dibuktikan gagal** saat kode lama dikembalikan
  (5 failed / 3 passed, pesan `Response.clone: Body has already been consumed`).
- Test mem-`pin` akar masalah mentah: `err.response.bodyUsed === true` dan
  `clone()` melempar; `err.data` tetap memuat envelope.
- `tsc --noEmit -p tsconfig.app.json` bersih untuk seluruh file yang disentuh.
- Test di jsdom, bukan node: `Request` node menolak URL relatif
  `/kafe/_ui/auth/login` yang dibangun kode produksi.
- **Browser `:8099` (SPA hasil `npx vite build`):** login salah →
  `invalid username or password`; 8 login gagal → 429 `too many login
attempts, try again later`; `kasir.dua` (2 assignment) → picker tanpa
  auto-submit; preference cabang BDG → radio BDG ter-`checked`; klaim token
  `role: "kasir"` + `attrs.branch_id` = cabang terpilih; data ter-scope
  (BDG **2** order / JKT **21** order, tiap cabang hanya barisnya sendiri).

## Sisa

- Pengalih konteks di header/`UserMenu` (tahap 4 plan) belum dikerjakan —
  `SwitchContextScreen` hanya muncul saat refresh 409. → todo 6.5.10 ⏸️
- `sdk/browser` tidak terkena (fetch native, tanpa ky); `next-app/` tidak
  memakai ky.

## Referensi

- Plan `docs_internal/plan/fix-ky-v2-error-body.md`
- `docs/spec/backend/01-core-basic.md` §8.7 · `docs_internal/plan/session-context-role-branch.md`
- Todo 6.5.9
