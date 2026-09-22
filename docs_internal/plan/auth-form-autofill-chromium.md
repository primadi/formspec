# Plan: Auth Form Autofill Compliance (Chromium "Create Amazing Password Forms")

**Status**: ✅ selesai (2026-09-18) — changelog
`docs_internal/changelog/2026-09-18-009-form-auth-autofill-chromium.md`
**Referensi**: <https://www.chromium.org/developers/design-documents/create-amazing-password-forms/>
**Terkait**: `docs_internal/changelog/2026-09-09-007-remove-input-blockautofill-hack.md`
(dihapusnya hack `readOnly`-until-focus + default `autoComplete="nope"` di `Input`)

## Goal

Selaraskan form autentikasi FormSpec dengan tujuh butir guidance Chromium agar
password manager (Chrome, 1Password, Bitwarden, dsb.) bisa menebak, mengisi,
menyimpan, dan memperbarui kredensial dengan benar. Titik berat: **form login
bawaan** (`LoginScreen`) dan **auth form spec-driven** (`AuthFormRenderer`),
plus sisa hack "fool the browser" di komponen lain.

## Audit — kondisi sebelum perubahan

| # | Guidance | LoginScreen | AuthFormRenderer |
| - | -------- | ----------- | ---------------- |
| 1 | Group fields dalam satu `<form>`, jangan gabung proses | 1 form untuk login **dan** register (dibedakan `mode`) | 1 form per `auth_action` ✅ |
| 2 | `autocomplete` attributes | login: `username` + `current-password` ✅ · **register: tetap `current-password`** ❌ | **tidak ada sama sekali** ❌ |
| 3 | Submission jelas (navigasi/replaceState + form dihapus) | ✅ `navigate(..., {replace:true})`, form unmount | ✅ `redirectAfterLogin()` |
| 4 | Hidden field untuk info implisit | ❌ — workspace hilang total dari DOM saat datang dari URL | ❌ |
| 5 | Jangan fool the browser | ✅ bersih | ❌ `<form autoComplete="off">` |
| 6 | Ikuti konvensi (login gagal tidak pindah halaman) | ✅ error inline | ✅ error inline |
| 7 | HTML guidelines (id unik, label↔input, submit button) | ✅ id unik + label terpasang · tapi **tanpa `name`** | ❌ `htmlFor={field.name}` **tanpa `id`** di input → label tidak terhubung |

Temuan tambahan di luar dua file itu:

- `components/ui/textarea.tsx` masih hardcode `autoComplete="nope"` — sisa dari
  changelog 2026-09-09-007 yang hanya membersihkan `input.tsx`.
- `kinds/wizard/WizardFormStep.tsx` (2×) dan `kinds/wizard/WizardRenderer.tsx` (1×)
  juga memakai nilai token tidak valid `"nope"`.
- `kinds/form/FormRenderer.tsx` — field password entity tidak punya
  `autocomplete`, jadi Chrome memperlakukannya sebagai password login.

## Perubahan

Semua perubahan bersifat additive/korektif pada atribut HTML; tidak ada
perubahan kontrak manifest, API, atau backend.

### A. `renderers/react-shadcn/src/shell/LoginScreen.tsx` (LoE: small)

1. `autoComplete` password **kondisional**: `mode === "register"` →
   `new-password`, selain itu `current-password`.
2. Tambah `name` pada semua input (`workspace`, `display_name`, `email`,
   `username`, `password`) — password manager memakai `name`+`id` untuk heuristik.
3. Hidden field untuk info implisit: `<input type="hidden" name="workspace">`
   saat workspace berasal dari URL, dan `name="app"` saat login app-scoped
   (role/permission per-App). Ditempatkan di dalam `<form>` yang bersangkutan.
4. `<form key={mode}>` agar login dan register adalah **form element berbeda**
   (guidance #1) walau mode berganti in-place lewat `?mode=register`.

### B. `widgets/PasswordInput.tsx` + `widgets/TextInput.tsx` (LoE: small)

Tambah prop `name` dan `autoComplete` dan teruskan ke `Input` (dan `Textarea`
untuk cabang `maxLength > 120`). Tambah `aria-label` + `aria-pressed` pada
tombol reveal password (guidance #7: aksesibilitas).

### C. `kinds/form/AuthFormRenderer.tsx` (LoE: medium)

1. Tabel konvensional `AUTOCOMPLETE_BY_ACTION[auth_action][field]` →
   `login: username/current-password`, `register: username+email+name/new-password`,
   `change_password: current-password/new-password`,
   `forgot_password: email`, `reset_password: new-password`. Field yang tidak
   dikenal → `off` (cegah Chrome menebak salah, mis. alamat/kartu).
   Konvensional, bukan field YAML baru — sejalan dengan pemetaan nama field
   auth yang sudah konvensional di renderer ini.
2. `<form autoComplete="on">` (bukan `off`).
3. Teruskan `id={field.name}`, `name={field.name}`, `autoComplete={...}` ke
   `PasswordInput`/`TextInput` → **memperbaiki label yang tidak terhubung**.

### D. Bersihkan sisa "fool the browser" (LoE: small)

- `components/ui/textarea.tsx`: hapus `autoComplete="nope"`.
- `kinds/wizard/WizardFormStep.tsx`, `kinds/wizard/WizardRenderer.tsx`:
  `"nope"` → `"off"` (token valid).
- `kinds/form/FormRenderer.tsx`: field password entity → `autoComplete="new-password"`
  (form entity selalu "menetapkan password", bukan login).

### E. Layar auth lain — pola yang sama (LoE: small)

`SetupScreen`, `ResetPasswordScreen`, `ChangePasswordPage`,
`ChangePasswordDialog`: `name` pada field kredensial; hidden
`input[name=workspace]` untuk tiga layar yang mengenal workspace dari URL, dan
reset juga hidden `input[name=token]` (single-use token dari `?reset_token`).

### F. Dokumentasi (LoE: small)

- `docs/kind/ui/Form.md` §Auth Forms: catat pemetaan `autocomplete` konvensional.
- `.github/skills/formspec-frontend/SKILL.md` design rules: satu baris aturan
  "form auth wajib pakai autocomplete token standar; jangan `nope`/`off`".

## Dependensi

A → B (widget harus menerima prop lebih dulu). C bergantung B. D dan E independen.

## Verifikasi

- `cd renderers/react-shadcn && npx tsc --noEmit` → 0 error.
- `cd renderers/react-shadcn && npx vitest run` → 17 file / 273 test PASS.
- Test regresi baru: `shell/LoginScreen.test.tsx` (5) dan
  `shell/auth-screens.autofill.test.tsx` (3).
- Inspeksi DOM `/login` & `/register`: form login punya
  `autocomplete=current-password`, form register `new-password`, kedua form
  punya input `name` dan hidden `workspace` saat workspace dari URL.
- **Sisa (todo 17.7)**: verifikasi autofill Chrome sungguhan di browser belum
  dilakukan — assertion di atas hanya level DOM (vitest/jsdom).
