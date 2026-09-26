# Plan — Perbaikan body respons error ky v2 + pemilih konteks sesi

**Status:** selesai 2026-09-25 (Phase A + B).
**Pemicu:** laporan pengguna — form login kafe menampilkan
`Failed to execute 'clone' on 'Response': Response body is already used`
sebagai ganti pesan server.

## Phase A — `Response body is already used`

### Akar masalah

`loginWithPassword` (`renderers/react-shadcn/src/lib/api/auth.ts`) membaca body
respons error lewat `err.response.clone().json()`. Sejak `ky` di-bump ke
`^2.0.2` (`package.json:24`, terpasang 2.0.2), ky **mengonsumsi** body itu
sebelum melempar error, untuk mengisi properti baru `HTTPError.data`:

```js
// node_modules/ky/distribution/core/Ky.js
httpError.data = await ky.#getResponseData(currentResponse) // :144
```

`#getResponseData` → `#readResponseText` memanggil `response.body.getReader()`
(`:519`), jadi stream terkunci. `clone()` berikutnya melempar `TypeError` —
dan karena dilempar **dari dalam blok catch**, ia menggantikan pesan server
yang seharusnya ditampilkan.

Didokumentasikan eksplisit di `errors/HTTPError.d.ts`:

> The response body is automatically consumed when populating `error.data`, so
> `error.response.json()` and other body methods will not work. Use `error.data`
> instead.

Artinya **setiap** login gagal (kredensial salah, `RATE_LIMITED`,
`INVALID_REQUEST`, `CONTEXT_REQUIRED`) menampilkan `TypeError` itu, bukan pesan
server. `authHooks.ts` memakai pola `.clone().json()` yang sama dan **tidak**
meledak hanya karena ky menyerahkan clone segar ke `afterResponse`
(`Ky.js:624`) — jadi ia aman secara kebetulan, bukan desain.

### Perubahan

| File                           | Isi                                                                                                                                                                                                      |
| ------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `src/lib/api/errors.ts` (baru) | Satu tempat membaca envelope: `errorEnvelope` (dari `err.data`), `buildFormaApiError`, `toFormaApiError`. Guard `typeof === "object"` karena ky mengisi `data` dengan **string** untuk respons non-JSON. |
| `src/types/manifest.ts`        | `FormaApiError` + `ErrorResponse.error` menerima `choices?: ContextChoice[]`; tipe `ContextChoice` baru (mirror `auth.ContextChoice`).                                                                   |
| `src/lib/api/auth.ts`          | Baca `err.data`; lempar `FormaApiError` typed (bukan `Error` polos), jadi `code`/`status`/`details`/`choices` tidak hilang.                                                                              |
| `src/lib/api/authHooks.ts`     | `afterResponse` pakai `response.json()` (clone segar dari ky) + helper yang sama.                                                                                                                        |

Batas yang disengaja: `authHooks` **tidak** bisa memakai `err.data` — hook
berjalan sebelum `HTTPError` dibangun, jadi tidak ada `data` saat itu.

### Bukti

- `src/lib/api/auth.test.ts` (8 test, env **jsdom**): pesan server muncul,
  `TypeError` tidak bocor, `FormaApiError` membawa `status`/`code`, `choices`
  diteruskan, `assignment` terkirim di body, body non-envelope jatuh ke
  `statusText`.
- Test **dibuktikan gagal** saat kode lama dikembalikan (5 failed / 3 passed).
- Test terakhir mem-`pin` akar masalah mentah: `err.response.bodyUsed === true`
  dan `clone()` melempar.
- jsdom dipilih (bukan node) karena `Request` node menolak URL relatif
  `/kafe/_ui/auth/login` yang dibangun kode produksi; jsdom + shim `Request`
  ber-base-URL meniru browser.

## Phase B — Pemilih konteks sesi (todo 6.5.9)

### Keputusan

`docs/spec/backend/01-core-basic.md` §8.7 melarang **server** memilih boundary:
principal dengan >1 assignment mendapat **409 `CONTEXT_REQUIRED` + `choices`**,
tanpa token. Plan `session-context-role-branch.md` menaruh pilihan terakhir
**di klien, per-device**.

Keputusan pemilik proyek (2026-09-25): **picker ter-prefill, tanpa
auto-submit**. Prefill dari pilihan terakhir; belum pernah login → pilihan
pertama. Boundary tetap dinyatakan pemanggil, jadi jawaban audit ("sebagai role
apa, di cabang mana") tetap sah.

### Perubahan

| File                                                                | Isi                                                                                                                                                                                                  |
| ------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `src/lib/session-context.ts` (baru)                                 | `readContextPreference`/`writeContextPreference` (localStorage, key `formspec-context:<ws>:<app>`) + `defaultContextChoice` (pilihan terakhir bila masih ada di daftar, jika tidak pilihan pertama). |
| `src/shell/ContextPicker.tsx` (baru)                                | Radiogroup + label `role · value`; prefill; tidak pernah auto-submit.                                                                                                                                |
| `src/shell/LoginScreen.tsx`                                         | 409 `CONTEXT_REQUIRED` → tampilkan picker (bukan banner error); sukses → simpan pilihan.                                                                                                             |
| `src/shell/SwitchContextScreen.tsx` (baru)                          | Picker untuk 409 yang datang dari **refresh**; memakai `POST /_ui/auth/switch`.                                                                                                                      |
| `src/lib/api/authHooks.ts`                                          | Opsi `needsContext` — 409 saat refresh **tidak** meng-expire sesi.                                                                                                                                   |
| `src/stores/session.ts`                                             | `pendingContext` + `clearPendingContext`; `refreshSession` menangkap 409 alih-alih menelannya (`catch { return false }` lama).                                                                       |
| `src/lib/api/{client,meta}.ts`, `src/stores/meta.ts`                | Meneruskan `needsContext`.                                                                                                                                                                           |
| `src/lib/formspec-client.ts`, `src/kinds/form/AuthFormRenderer.tsx` | ikut mengirim `assignment` + menampilkan picker.                                                                                                                                                     |

### Catatan penting soal refresh

`Service.Refresh` (`internal/auth/service.go:1143`) mengembalikan
`ContextRequiredError` **sebelum** `s.session.Delete` — jadi refresh token
**masih valid** saat 409. Karena itu meng-expire sesi di titik itu membuang
kredensial yang masih bekerja, dan satu-satunya jalan memperbaiki konteks
adalah `/_ui/auth/switch` (bukan refresh ulang). Itulah alasan
`SwitchContextScreen` ada.

### Bukti

- `src/shell/ContextPicker.test.tsx` (11 test): round-trip preferensi
  per-workspace/App, fallback ke pilihan pertama, **tidak** menpreselect id yang
  sudah dicabut, tidak auto-submit, submit mengirim id terpilih.
- `vitest` **473 lulus** (baseline 462, +11) · `tsc` bersih untuk file yang
  disentuh.

## Verifikasi browser (selesai 2026-09-25)

`npx vite build` berhasil setelah workstream paralel selesai (bundel lama yang
memuat `.clone().json()` hilang; `formspec-context` hadir). Server:
`./bin/formspec dev --spec examples/kafe/spec --dsn sqlite:.formspec/kafe.db
--workspace-id kafe --addr :8099 --web-dir renderers/react-shadcn/dist`.

| Uji                              | Hasil terukur                                                                                            |
| -------------------------------- | -------------------------------------------------------------------------------------------------------- |
| Login `manajer` password salah   | `invalid username or password` (bukan `Response body is already used`)                                   |
| 8 login gagal berturut           | 5× `invalid username or password` → 3× `too many login attempts, try again later` (jalur 429 juga benar) |
| Login `kasir.dua` (2 assignment) | picker muncul, **tidak** auto-submit, pilihan pertama ter-prefill                                        |
| Prefill pilihan terakhir         | preference di-set ke cabang BDG → picker membuka dengan radio BDG `[checked]`                            |
| Klaim token                      | `role: "kasir"` (tunggal) + `attrs.branch_id` = cabang terpilih                                          |
| Scoping data                     | cabang BDG → **2** order (semua BDG); cabang JKT → **21** order (semua JKT)                              |
| `localStorage`                   | key `formspec-context:kafe:kafe-pos` = `<role>@<branch-id>`                                              |

Kedua cabang diuji berurutan dengan login ulang, dan tiap kali `branch_id` pada
hasil query cocok dengan yang dipilih — jadi prefill tidak "menempel" ke satu
cabang:

## Referensi

- `package.json` — `"ky": "^2.0.2"`
- `docs/spec/backend/01-core-basic.md` §8.7 (konteks sesi)
- `docs_internal/plan/session-context-role-branch.md` (tahap 4 = UI pemilih)
- `internal/api/auth_handler.go` — `writeContextRequired`, `HandleSwitchContext`
