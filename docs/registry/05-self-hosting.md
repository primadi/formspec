# Self-Hosting Registry

## Mode Dev (POC)

```bash
make registry-dev          # = scripts/run-registry.sh
```

Script menjalankan `go run ./cmd/formspec-registry` dengan config
`cmd/formspec-registry/formspec-app.yaml`. Semua permukaan di-prefix slug
workspace (D50), jadi UI dibuka di **`http://localhost:8080/default/`** —
`/` sendiri bukan route (URL yang dicetak saat boot adalah yang benar).

Sumber SPA yang disajikan, berurutan:

| #   | Sumber                           | Kapan                                                                                               |
| --- | -------------------------------- | --------------------------------------------------------------------------------------------------- |
| 1   | `--web-dir <dir>`                | eksplisit; menang atas semuanya                                                                     |
| 2   | `renderers/react-shadcn/dist`    | di-_auto-detect_ dari CWD ke atas — sumber renderer, paling segar (`npm run build` menulis ke sini) |
| 3   | `cmd/formspec-registry/web/dist` | salinan tersinkron untuk `//go:embed` (`make build-registry`); dipakai bila (2) tidak ada           |
| 4   | embedded `web/dist`              | deploy: `make build-registry` (SPA di-embed via `-tags formspec_spa`) — bekerja tanpa checkout repo |

Auto-detect (2 dan 3) **dilewati** bila embed sudah berisi dist asli, dan sebuah
direktori hanya dianggap bundle bila memuat `index.html`: kedua produsennya tidak
atomik (`rm -rf` lalu `cp -r`), dan menyajikan dist setengah jadi memberi halaman
kosong tanpa sebab yang terlihat dari browser.

Kenapa auto-detect ada: `go run` dikompilasi **tanpa** `-tags formspec_spa`,
sehingga `web/embed.go` — file embed SPA asli — tidak ikut build dan
`embed_stub.go` yang menjawab dengan halaman placeholder. "Ada `embed.go`"
tidak berarti embed-nya dipakai: **build tag** yang menentukan. Bila hasilnya
tetap placeholder (mis. binary `go install` dijalankan di luar checkout),
banner menyatakannya eksplisit beserta cara memperbaikinya.

Dengan satu App ber-`root_url`, boot non-produksi juga mengalihkan `GET /` →
`/{ws}/` supaya nama host polos tidak menjawab `404 page not found`. Di
production redirect itu tidak dipasang — segmen root milik edge (subdomain per
workspace / aturan ingress).

Registry adalah FormSpec app biasa (`cmd/formspec-registry/app-spec/spec/`): App `registry` + Module
`registry` + entities `Vendor`/`Module`/`ModuleVersion`. Data di SQLite lokal,
tarball via file storage. Cocok untuk development dan smoke test.

## Mode Production

Prasyarat (Fase 8 production serve):

- **Postgres** — DSN `sqlite:` ditolak di production mode
- **JWT asimetris** — `--jwt-public-key` (RS256/ES256 PEM)
- **TLS** — `--tls-cert/--tls-key` (min TLS 1.2)
- **CORS allow-list** — wildcard `*` ditolak

```bash
formspec serve --mode=production --spec cmd/formspec-registry/app-spec/spec \
  --dsn postgres://... --jwt-public-key keys/jwt.pub \
  --tls-cert cert.pem --tls-key key.pem \
  --cors-origin https://registry.formspec.dev
```

Observability bawaan: health `GET /health`, Prometheus di `--metrics-addr`
(default `:9102`), structured JSON-lines logging.

## Native Binary (Plan C — batch 1 ✅)

`cmd/formspec-registry` = wrapper tipis yang meng-embed engine + spec via
`//go:embed` (`app-spec/embed.go`) — single-file deployment: tanpa `--spec`,
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
- **`registry.vendor.approve`** — action `approve` pada entity `vendor`
  (vendor upgrade flow): admin menyetujui aplikasi vendor pending → status
  vendor menjadi `active` + owner user diberi role `vendor` dan permissions
  `registry.vendor.*`/`registry.module.*`.

Deployment target (batch berikutnya): K8s 3 replica stateless + Postgres HA +
Redis cache (`ctx.cache`) untuk MRU modules — lihat `docs_internal/plan/` untuk status.

## Catatan Operasional

- Tarball disimpan via `ctx.storage` — untuk production gunakan object store
  (Garage/S3); tarball immutable sehingga aman di-depan CDN.
- Backup: `formspec backup create` (4.8.x); jadwal otomatis masih deferred (8.3).
- Skalabilitas: app stateless (state di Postgres + storage) — replikasi instance
  di belakang load balancer; audit multi-instance (rate limiter shared,
  outbox lease) bagian dari Plan C.
