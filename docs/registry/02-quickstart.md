# Quickstart — Publish & Install Module

Alur end-to-end yang terverifikasi (smoke test Fase 13.3).

## 0. Daftar sebagai vendor (via portal web)

Alternatif tanpa CLI: buka portal registry → **Sign up** (buat akun) →
**Sign up Vendor** (`/portal/vendor-signup`) — isi nama vendor, tempel isi
file `.pub`, dan tulis username akun Anda. Vendor tercatat dengan public key
tersebut; publish CLI berikutnya otomatis find-or-create ke record yang sama
(berdasarkan nama vendor).

## 1. Buat keypair vendor

```bash
formspec sign keygen --out ~/.formspec/keys --name acme
# private: ~/.formspec/keys/acme.key  (JANGAN di-commit)
# public:  ~/.formspec/keys/acme.pub  (didaftarkan ke registry)
```

## 2. Sign module

```bash
formspec sign ./my-module --key ~/.formspec/keys/acme.key
# stdout: signature base64; stderr: checksum tree
```

## 3. Publish ke registry

```bash
formspec module publish ./my-module \
  --vendor acme --key ~/.formspec/keys/acme.key --version 1.0.0 \
  --registry https://registry.formspec.dev
```

Yang terjadi: tar folder → checksum → sign → find-or-create vendor & module
(auto-submit) → create version → upload tarball. Versi immutable — re-publish
semver sama dengan konten berbeda ditolak.

Env alternatif: `FORMSPEC_MODULE_REGISTRY` (URL default) dan
`FORMSPEC_REGISTRY_API_KEY` (API key publish).

## 4. Install dari registry (project konsumen)

```bash
formspec module install --from https://registry.formspec.dev acme-billing@1.0.0
# atau langsung aktif:
formspec module install --from registry.formspec.dev acme-billing@1.0.0 --use
```

Client melakukan: lookup version → download tarball → **verifikasi signature
terhadap public key vendor terdaftar** (checksum mismatch → REFUSED) → copy ke
`vendors/` → tulis `formspec.lock` → tulis marker di App manifest.

## 5. Aktivasi & boot

Tanpa `--use`, aktifkan manual dengan meng-uncomment entri di App manifest:

```yaml
spec:
  modules:
    - my-app
    # >>> formspec:vendor registry.formspec.dev/acme-billing@1.0.0
    - acme-billing # uncomment = aktif
    # <<< formspec:vendor
```

Boot app — module vendor ter-register, API-nya reachable.

## 6. Integritas & maintenance

```bash
formspec verify                    # checksum vendors/ vs lock (tamper detection)
formspec module list               # status aktif/nonaktif + trust tier
formspec module uninstall acme-billing
formspec override adopt acme-billing Form checkout-form   # kustomisasi aman
formspec override diff acme-billing Form checkout-form    # cek drift
```
