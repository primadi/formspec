# Config

<!-- generated:meta -->
| | |
|---|---|
| Grup | `data` |
| Plane | `resource` |
| Spec struct | `ConfigSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Config` adalah konfigurasi **level module** yang dibaca via `ctx.config` di script.

**Kapan memakai Config:**
- Parameter yang bisa diubah tanpa redeploy (tax rate, prefix, template)
- **Global settings** — currency, locale, timezone, format tanggal: hidup di workspace Config di bawah namespace `settings.*` (bukan ditebak per komponen)
- Secret (smtp_host, api_key) — ditandai `secret: true`, digovern Control Plane

**Kapan TIDAK pakai Config:**
- Bukan `formspec-app.yaml` — itu CLI dev/serve config, BUKAN `kind: Config` manifest
- Bukan dotenv — nilai di-resolve per environment, script baca via `ctx.config.get()`

**Sumber kontrak:** [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §10.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1alpha1
kind: Config
metadata:
  name: app
  module: core
spec:
  keys:
    invoice_due_days: { type: int, default: 30 }
    smtp_host:        { type: string, secret: true }

---
# Global settings — namespace settings.*
spec:
  keys:
    settings.default_currency:  { type: string, default: "USD" }
    settings.locale:            { type: string, default: "en-US" }
    settings.timezone:          { type: string, default: "UTC" }
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `keys` | map[string]`ConfigKey` | ✅ | {invoice_due_days: {type: int, default: 30}, smtp_host: {type: string, secret: true}} |  |

<!-- /generated:attributes -->

## Gotchas

- **Jangan pernah menebak setting global** — komponen baca `settings.*` atau deklarasi eksplisit; kalau tidak ada, itu error, bukan tebakan.
- **`formspec-app.yaml` ≠ `kind: Config`** — file itu CLI dev/serve config (DSN, runtime, themes), bukan manifest Config.
- **Secret & environment definition digovern Control Plane** — bukan env var mentah di script.
- **Cross-ref:** [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §10 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
