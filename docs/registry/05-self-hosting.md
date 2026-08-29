# Self-Hosting Registry

## Mode Dev (POC)

```bash
formspec dev --spec registry/spec
```

Registry adalah FormSpec app biasa (`registry/spec/`): App `registry` + Module
`registry` + entities `Vendor`/`Module`/`ModuleVersion`. Data di SQLite lokal,
tarball via file storage. Cocok untuk development dan smoke test.

## Mode Production

Prasyarat (Fase 8 production serve):

- **Postgres** — DSN `sqlite:` ditolak di production mode
- **JWT asimetris** — `--jwt-public-key` (RS256/ES256 PEM)
- **TLS** — `--tls-cert/--tls-key` (min TLS 1.2)
- **CORS allow-list** — wildcard `*` ditolak

```bash
formspec serve --mode=production --spec registry/spec \
  --dsn postgres://... --jwt-public-key keys/jwt.pub \
  --tls-cert cert.pem --tls-key key.pem \
  --cors-origin https://registry.formspec.dev
```

Observability bawaan: health `GET /health`, Prometheus di `--metrics-addr`
(default `:9102`), structured JSON-lines logging.

## Native Binary (Plan C — batch 1 ✅)

`cmd/formspec-registry` = wrapper tipis yang meng-embed engine + spec via
`//go:embed` (`registry/embed.go`) — single-file deployment: tanpa `--spec`,
spec diekstrak ke temp dir saat boot.

```bash
formspec-registry --dsn postgres://... --addr :8080 --prod \
  --jwt-public-key keys/jwt.pub --web-dir renderers/react-shadcn/dist
```

Native handler terdaftar di binary ini (tidak ada di `formspec dev`):

- **`registry.SignatureVerify`** — service `registry.signature-verify.verify`
  (`POST /{ws}/api/v1/registry/signature-verify/verify`): verifikasi ed25519
  server-side atas tree checksum. Publish CLI memanggilnya sebelum upload —
  signature invalid → publish ditolak; registry tanpa native handler (dev)
  → dilewati (client-side verify saat install tetap melindungi konsumen).

Deployment target (batch berikutnya): K8s 3 replica stateless + Postgres HA +
Redis cache (`ctx.cache`) untuk MRU modules — lihat `docs/plan/` untuk status.

## Catatan Operasional

- Tarball disimpan via `ctx.storage` — untuk production gunakan object store
  (MinIO/S3); tarball immutable sehingga aman di-depan CDN.
- Backup: `formspec backup create` (4.8.x); jadwal otomatis masih deferred (8.3).
- Skalabilitas: app stateless (state di Postgres + storage) — replikasi instance
  di belakang load balancer; audit multi-instance (rate limiter shared,
  outbox lease) bagian dari Plan C.
