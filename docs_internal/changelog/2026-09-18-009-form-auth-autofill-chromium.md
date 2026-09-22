# 009 — Form auth patuh guidance password manager Chromium

Plan: `docs_internal/plan/auth-form-autofill-chromium.md`. Acuan:
[Chromium — Create Amazing Password Forms](https://www.chromium.org/developers/design-documents/create-amazing-password-forms/),
lanjutan dari changelog `2026-09-09-007` (hapus hack `readOnly`-until-focus).
Audit menemukan tiga gap di `LoginScreen`/`AuthFormRenderer` plus sisa token
`autoComplete="nope"` di tiga komponen lain.

## Apa

- **`shell/LoginScreen.tsx`** — password memakai `autoComplete` kondisional
  (`new-password` untuk register, `current-password` untuk login); semua input
  diberi `name`; `<form key={mode}>` agar login dan register adalah form element
  berbeda; hidden `input[name=workspace]`/`[name=app]` saat workspace/app datang
  dari URL.
- **`kinds/form/AuthFormRenderer.tsx`** — tabel konvensional
  `AUTOCOMPLETE_BY_ACTION[auth_action][field]`, `<form autoComplete="on">`, dan
  `id`/`name`/`autoComplete` diteruskan ke `TextInput`/`PasswordInput`. Ini juga
  memperbaiki bug a11y: `<label htmlFor={field.name}>` sebelumnya menunjuk ke id
  yang tidak pernah dirender.
- **`widgets/PasswordInput.tsx` + `widgets/TextInput.tsx`** — prop `name` dan
  `autoComplete` diteruskan ke `Input`/`Textarea`; tombol reveal password dapat
  `aria-label` + `aria-pressed`.
- **`kinds/form/FormRenderer.tsx`** — field password entity dapat
  `autoComplete="new-password"` + `name` (form entity selalu *menetapkan*
  password, bukan login).
- **`components/ui/textarea.tsx`**, **`kinds/wizard/WizardFormStep.tsx`** (2×),
  **`kinds/wizard/WizardRenderer.tsx`** — token tidak valid `"nope"` dihapus /
  diganti `"off"`.
- **Docs** — `docs/kind/ui/Form.md` §Auth Forms mendokumentasikan pemetaan
  `autocomplete` konvensional; `.github/skills/formspec-frontend/SKILL.md`
  menambah satu design rule.
## Kenapa

Password manager hanya bisa memasangkan, mengisi, dan menyimpan kredensial bila
form menyatakan peran tiap field. Sebelumnya form register mengaku
`current-password` — Chrome menawarkan kredensial lama di halaman sign-up dan
menyimpan hal yang salah — dan saat workspace berasal dari URL field itu hilang
total dari DOM sehingga kredensial tersimpan kehilangan konteks akunnya.
`"nope"` bukan token `autocomplete` yang valid; itu persis "fool the browser"
yang diperingatkan guidance, dan sisa-sisanya belum ikut dibersihkan pada
changelog 2026-09-09-007.

## Lanjutan — layar auth lain (pola yang sama)

Tiga gap di atas diperbaiki untuk seluruh permukaan auth, bukan hanya login:
`SetupScreen`, `ResetPasswordScreen`, `ChangePasswordPage`, dan
`ChangePasswordDialog` mendapat `name` pada field kredensial; tiga layar yang
mengenal workspace dari URL (`setup`, `reset-password`, `change-password`)
mendapat hidden `input[name=workspace]`, dan reset juga menambahkan hidden
`input[name=token]` (single-use token dari `?reset_token`) — contoh persis
"hidden fields for implicit information".

## File terdampak

- `renderers/react-shadcn/src/shell/LoginScreen.tsx` (+ test baru
  `src/shell/LoginScreen.test.tsx`)
- `renderers/react-shadcn/src/shell/SetupScreen.tsx`
- `renderers/react-shadcn/src/shell/ResetPasswordScreen.tsx`
- `renderers/react-shadcn/src/shell/ChangePasswordPage.tsx`
- `renderers/react-shadcn/src/shell/ChangePasswordDialog.tsx`
  (+ test baru `src/shell/auth-screens.autofill.test.tsx`)
- `renderers/react-shadcn/src/kinds/form/AuthFormRenderer.tsx`
- `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx`
- `renderers/react-shadcn/src/widgets/PasswordInput.tsx`
- `renderers/react-shadcn/src/widgets/TextInput.tsx`
- `renderers/react-shadcn/src/components/ui/textarea.tsx`
- `renderers/react-shadcn/src/kinds/wizard/WizardFormStep.tsx`
- `renderers/react-shadcn/src/kinds/wizard/WizardRenderer.tsx`
- `docs/kind/ui/Form.md`, `.github/skills/formspec-frontend/SKILL.md`

## Verifikasi

- `npx tsc --noEmit`: 0 error.
- `npx vitest run`: 17 file / 273 test PASS — 5 test baru di
  `LoginScreen.test.tsx` (token `autocomplete`, `name`, hidden
  `workspace`/`app`, pemisahan form login↔forgot) dan 3 test baru di
  `auth-screens.autofill.test.tsx` (setup, reset, change-password).
