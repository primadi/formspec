# 2026-08-31-007 — Vendor signup copy de-redundancy

## Apa

Menghilangkan redundansi penamaan antara **Sign up akun** (tombol kanan atas,
shell `chrome.auth: links` → `/register`, membuat akun auth username/password)
dan **Sign up Vendor** (halaman `/portal/vendor-signup`, membuat entitas
`registry.vendor` — identitas penerbit ed25519).

Keduanya memang konsep berbeda (akun ≠ identitas vendor) sehingga tidak
digabung, tapi copy-nya disamakan-seperti sehingga terlihat redundant. Fix:

- `pages/vendor-signup.yaml`: title "Sign up Vendor" → **"Daftarkan Vendor"**;
  hero menegaskan "Ini BUKAN sign-up akun"; langkah 1 merujuk tombol Sign up
  kanan atas.
- `forms/vendor-signup.yaml`: description + help field `owner_username`
  diperjelas; message submit menegaskan akun tidak berubah.
- `pages/home.yaml`: CTA "Sign up Vendor" → **"Daftarkan Vendor"**.

## Kenapa

User report kebingungan: dua tombol "sign up" yang terlihat sama padahal
beda domain (auth account vs vendor identity). Penamaan "Sign up Vendor"
menimbulkan kesan akun kedua.

## File terdampak

- `registry/spec/modules/portal/pages/vendor-signup.yaml`
- `registry/spec/modules/portal/forms/vendor-signup.yaml`
- `registry/spec/modules/portal/pages/home.yaml`

## Deferred (perlu dukungan renderer)

Auto-fill `owner_username` dari session (`_meta/me`) + gate "harus login
dulu" di halaman vendor-signup — belum ada mekanisme `default` dari identity
context pada FormField (`pkg/spec/frontend.go`), jadi field manual dipertahankan
dengan help text yang lebih jelas.
