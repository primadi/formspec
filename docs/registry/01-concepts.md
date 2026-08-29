# Konsep Registry

**Status:** Implemented (Fase 13, 2026-08-28)

## Model Data

Registry menyimpan tiga entity (didefinisikan sebagai manifest FormSpec biasa
di `registry/spec/modules/registry/entities/`):

| Entity          | Peran                               | Field kunci                                                                       |
| --------------- | ----------------------------------- | --------------------------------------------------------------------------------- |
| `Vendor`        | Penerbit module — identitas ed25519 | `name`, `public_key`                                                              |
| `Module`        | Katalog module                      | `name` (unique), `vendor_id`, `trust_tier`, `description`                         |
| `ModuleVersion` | Satu rilis                          | `semver` (unique per module), `checksum`, `signature`, `tarball` (file), `status` |

`ModuleVersion` punya state machine: `draft → published → deprecated`.

## Signing (ed25519)

- Publisher membuat keypair via `formspec sign keygen` — private key setara
  password vendor, tidak pernah dikirim ke registry.
- Yang ditandatangani adalah **tree checksum** module (`sha256:` atas seluruh
  isi folder, path terurut) — bukan tarball mentah. Tarball di-download ulang,
  diekstrak, checksum-nya dihitung ulang, lalu signature diverifikasi.
- Public key didaftarkan ke entity `Vendor` saat publish pertama; setiap
  `module install --from` memverifikasi signature **terhadap public key vendor
  terdaftar SEBELUM tarball dipercaya**. Checksum mismatch → install REFUSED.

## Immutability Versi

Versi bersifat immutable: re-publish semver yang sama dengan konten berbeda
ditolak. Re-publish semver sama dengan konten identik bersifat idempotent.
Untuk perubahan apa pun, naikkan versi.

## Install ≠ Aktivasi

`formspec module install` hanya **men-fetch** artifact:

1. Copy ke `vendors/{effective-name}/` (read-only, checksum terkunci)
2. Catat provenance di `formspec.lock` (source, version, checksum, signature, trust_tier)
3. Tulis entri **ter-comment (nonaktif)** di `App.spec.modules` dalam marker blok

Module nonaktif tidak diregister saat boot. Aktivasi = uncomment entri, atau
`--use` saat install. Re-install mempertahankan status aktif.

## Alias Saat Konflik Nama

Jika nama module bentrok dengan module lokal atau vendor lain, alias efektif
`{org}-{name}` dihitung **saat install** dan dicatat di lock + marker;
`module.yaml` vendor dinormalisasi ke nama efektif sehingga boot mendaftar
entity di bawah nama tersebut.

## Shadow Copy (Kustomisasi)

`formspec override adopt` menyalin manifest presentation (whitelist: `Form`,
`VisualSpecKind`) ke `overrides/{module}/` yang **replace-total** upstream saat
boot. Checksum "asal fork" dicatat di lock; vendor update yang mengubah upstream
memicu drift warning (bukan hard-fail). Entity/Service/Workflow tidak bisa
di-shadow-copy — gunakan Entity Extension / Integrator pattern.
