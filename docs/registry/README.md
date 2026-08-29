# FormSpec Registry

> Dokumentasi resmi **FormSpec Module Registry** — ekosistem distribusi module
> npm-like untuk FormSpec: publish, sign, install, dan trust tier.
> Registry berjalan sebagai **FormSpec app** (dogfooding) di `registry.formspec.dev`.

## Peta Dokumen

| Dokumen                                      | Isi                                                               | Untuk siapa               |
| -------------------------------------------- | ----------------------------------------------------------------- | ------------------------- |
| [`01-concepts.md`](01-concepts.md)           | Vendor, Module, ModuleVersion, trust tier, signing, immutability  | Semua                     |
| [`02-quickstart.md`](02-quickstart.md)       | E2E: keygen → sign → publish → install → boot                     | Developer (mulai di sini) |
| [`03-cli-reference.md`](03-cli-reference.md) | Referensi lengkap `formspec sign`, `module`, `override`, `verify` | Developer                 |
| [`04-rest-api.md`](04-rest-api.md)           | Endpoint REST registry + kontrak `spec.expose`                    | Integrator                |
| [`05-self-hosting.md`](05-self-hosting.md)   | Menjalankan registry sendiri (dev + production)                   | Operator                  |
| [`06-trust-tier.md`](06-trust-tier.md)       | Review flow, gerbang impl type per tier                           | Vendor & operator         |

## Jalur Baca

**Publisher** (ingin mempublikasikan module):
`02-quickstart` → `03-cli-reference` → `06-trust-tier`.

**Consumer** (ingin memakai module vendor):
`02-quickstart` (bagian install) → `01-concepts` (aktivasi & shadow copy).

**Operator** (menjalankan instance registry):
`05-self-hosting` → `04-rest-api`.

## Sumber Kontrak

- Kontrak normatif: [`../spec/platform/07-marketplace.md`](../spec/platform/07-marketplace.md), [`../spec/platform/08-project-layout.md`](../spec/platform/08-project-layout.md) §6
- Implementasi: `internal/vendor/`, `cmd/formspec/{sign,module,publish,override,verify}.go`, `registry/` (app spec)
