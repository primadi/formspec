# 2026-08-29-001 — Plan C batch 1: native binary formspec-registry + server-side signature verify

## Apa yang diubah

Batch pertama Plan C (todo 13.5.6) — registry prod-ready:

- **`cmd/formspec-registry`** — binary native production: wrapper tipis yang
  meng-embed engine + spec registry. Spec di-embed via `registry/embed.go`
  (`//go:embed spec`); tanpa `--spec`, diekstrak ke temp dir saat boot
  (single-file deployment). Flag: `--dsn/--addr/--prod/--jwt-public-key/
--jwt-secret/--jwt-issuer/--strict/--web-dir` (pola `examples/reference-app`).
- **Service `signature-verify`** (13.3.3) — `registry/spec/modules/registry/
services/signature-verify.yaml`: action `verify` dengan `impl: { type:
native, ref: registry.SignatureVerify }`; endpoint `POST /{ws}/api/v1/
registry/signature-verify/verify` (auto-generate dari service registry).
- **Native handler `registry.SignatureVerify`** — ed25519 verify
  (`vendor.VerifyChecksum`); input checksum/signature/public_key, output
  `{valid, error}`. Hanya tersedia di binary ini (bukan `formspec dev`).
- **Publish CLI server-side verify** — `internal/vendor/registry.go`:
  `VerifySignatureServer` + dipanggil di `PublishModule` sebelum upload
  tarball. Registry native → signature invalid = publish REFUSED; registry
  dev (tanpa native handler, 404) → dilewati gracefully (client-side verify
  saat install tetap melindungi konsumen).

## Verifikasi

- `go build ./...` + `go test ./internal/vendor/... ./cmd/formspec/...` — 94 pass.
- E2E native binary: boot dengan embedded spec (tanpa --spec) → endpoint
  verify hidup; signature asli → `valid:true`; checksum tampered →
  `valid:false` + pesan refusal.
- E2E publish penuh: `formspec module publish` ke native registry →
  `published: billing@1.0.0` (server-side verify lulus).

## Sisa Plan C (batch berikutnya)

- Driver Redis/Valkey untuk `ctx.cache` (cloud driver saat ini fail loudly di
  `resource/datastoreregistry.go`) + cache-aside metadata module.
- Deploy artifacts K8s: Dockerfile, Deployment 3 replica, Service, Ingress,
  Secret, Datastore manifest valkey (`registry/deploy/`).

## Referensi

- Docs: `docs/registry/05-self-hosting.md` (section Native Binary)
- Plan: session plan "FormSpec Registry — 3 Plan Terpisah" (Plan C)
