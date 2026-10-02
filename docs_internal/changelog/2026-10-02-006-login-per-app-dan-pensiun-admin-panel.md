# 2026-10-02-006 — Login per-App & pensiun panel admin `_admin`

Plan: `docs_internal/plan/app-scoped-login.md` (D1–D8).

## Apa yang diubah

**App menjadi batas sesi.** `POST /{ws}/_ui/auth/login` kini **wajib** membawa
`app`; App divalidasi (tak dikenal → 400 `UNKNOWN_APP`, tanpa entry point auth →
400 `APP_PUBLIC_NO_LOGIN`), dan login yang lolos kredensial tetapi tidak punya
satu pun permission di App itu dijawab **403 `NO_APP_ACCESS`** — dulu 200 dengan
sesi hampa (kafe 10.21/10.22). Rute `/{ws}/_admin/login` dan `/login` top-level
dihapus. Token **mengikat** `_meta/ui`: sesi App A yang meminta `?app=B` → 403
`APP_MISMATCH`. Sesi disimpan **per `(workspace, App)`** di klien, jadi pindah App
memakai sesi App itu (tanpa ketik password ulang) dan limit sesi konkuen dihitung
per App — login App B tidak mengeluarkan sesi App A.

**Panel entity `_admin` dihapus.** Varían bundle unscoped `?admin=true` +
gate biner `_admin.access` dicabut (kini 400 `ADMIN_BUNDLE_REMOVED`); gate-nya
terlalu kasar (satu permission membuka **semua** module — kafe 5.22.7). Rute
**framework** di `_admin/*` dipertahankan — `setup`, `oauth/callback`,
`oauth/link-callback`, `change-password` — karena itu bootstrap, bukan panel.
Tidak ada App baru bernama `_admin`. Fallback "tidak ada App → ke `_admin`"
diganti 404 jujur.

OAuth membawa `app` lewat `state`; callback meng-issue sesi App itu dan mendarat
di surface App. `accepts_login` ditambahkan ke `/_meta/apps` sebagai satu sumber
kebenaran (uji yang sama dengan endpoint login), dan invarian baru di
`internal/app/resolve.go` menolak App `private` tanpa entry point auth.

## File terdampak

Backend: `internal/auth/service.go`, `internal/auth/session.go`,
`internal/api/{auth_handler,meta,oauth_handler}.go`, `internal/app/resolve.go`,
`internal/ui/meta.go` (`ChromeAcceptsLogin`), `internal/vendor/registry.go`,
`resource/formspec.go`.
Frontend: `src/App.tsx`, `src/stores/{session,meta}.ts`,
`src/lib/api/meta.ts`, `src/hooks/{useSurface,useResolvedMenu}.ts`,
`src/shell/{LoginScreen,LoginPage,SetupScreen,ChangePasswordPage,OAuthCallback,OAuthLinkCallback,SwitchContextScreen}.tsx`,
`src/kinds/wizard/WizardRenderer.tsx`, `src/types/manifest.ts`.
Test: seluruh fixture login diperbarui + test baru untuk gate App, session-limit
per App, `ADMIN_BUNDLE_REMOVED`, `APP_MISMATCH`.

## Kenapa

Pemilik proyek: _"user bukan login ke workspace, tapi login ke App"_ dan panel
`_admin` dikhawatirkan jadi attack surface. Identitas tetap level workspace
(`spec/platform/02` §8) — yang di-scope per App adalah **sesi** dan permission.

## Dua bug ditemukan saat verifikasi browser (ikut diperbaiki)

1. **`boot()` positional — argumen tertukar diam-diam.** Semua parameter `boot`
   bertipe `string`, jadi pemanggil gaya lama (`boot(workspace, token, refresh,
app)`) tetap kompilasi: token masuk ke slot `app` dan sesi dipulihkan di bawah
   App palsu, sementara bundle diambil memakai refresh token sebagai bearer →
   permukaan kosong. **Terukur:** login `kasir` → mendarat di `/kafe/app/pos`
   dengan "Nothing here for your account" (bundle anonim). Diperbaiki dengan
   menjadikan `boot` menerima **objek opsional** (`{workspace, app, token?,
refreshToken?}`) sehingga urutan salah menjadi error tipe, bukan bug senyap.
2. **Boot effect memakai `loaded` global, bukan per App.** `loaded` di store
   sesi berlaku untuk seluruh SPA, jadi berpindah App tidak mem-boot App tujuan
   dan malah membawa token App sebelumnya → server menjawab **403
   `APP_MISMATCH`** (terukur: `/kafe` publik kosong). Diperbaiki: efek memakai
   `sessionReady = loaded && session.app === app.name`, dan single-flight `boot`
   di-key per App (boot App A tidak pernah dipakai ulang untuk App B).

## Verifikasi terukur (dev server :8080, SPA :5174, spec kafe)

| Kasus                            | Hasil                                                                               |
| -------------------------------- | ----------------------------------------------------------------------------------- |
| login tanpa `app`                | 400 `APP_REQUIRED`                                                                  |
| `app: app-ngawur`                | 400 `UNKNOWN_APP`                                                                   |
| `app: kafe-qr` (publik, no auth) | 400 `APP_PUBLIC_NO_LOGIN`                                                           |
| `kasir` → `kafe-pos`             | 200 + sesi 12 entity / 6 menu                                                       |
| `barista` → `kafe-pos`           | **403 `NO_APP_ACCESS`** (dulu 200 + 0 entity)                                       |
| `barista` → `kafe-kds`           | 200 (per-App access benar dua arah)                                                 |
| token pos → `?app=kafe-kds`      | 403 `APP_MISMATCH`                                                                  |
| `?admin=true`                    | 400 `ADMIN_BUNDLE_REMOVED`                                                          |
| `/kafe/_admin` (browser)         | "Page not found" (panel hilang)                                                     |
| `/kafe/_admin/change-password`   | form Change Password (rute framework tetap)                                         |
| `/kafe` publik (browser)         | tanpa form login, anonim, `NoAccessState`                                           |
| pindah pos → KDS → pos           | slot `formspec-session:kafe:kafe-pos` bertahan; POS dipulihkan tanpa ketik password |

Catatan: `/kafe` yang mendarat di `NoAccessState` **bukan regresi** — seluruh
entity `kafe-qr` bertanda `routable: false` dan tidak ada page ber-route `/`,
jadi itu memang gap kafe **10.20 ⏸️** (bukan hasil perubahan ini).

## Sisa

- 10.18/10.20 (kafe, App publik) → item tersendiri di `examples/kafe/gaps_found/TODO.md`.
- 14.c.4 (gerbang App `access_permission`) **tidak** diambil; digantikan gate
  saat login. Item ditutup dengan rujukan plan.
