# Keamanan & Permission — Aplikasi Arisan

## Prinsip

**`permission = resource + action`** — permission selalu berbentuk
`<module>.<entity>.<action>`. Tidak ada hardcode nama role di dalam YAML.
Pemetaan role → permission dilakukan di lapisan auth/control plane (di luar
scope aplikasi ini).

## Deklarasi Permission di Aplikasi

### Aksi custom (Entity)

| Entity | Aksi | Permission | Audit | Efek |
|--------|------|------------|-------|------|
| `arisan-group` | `complete` | `arisan-master.arisan-group.complete` | ✅ | `active → completed` |
| `contribution` | `validate` | `arisan-field.contribution.validate` | ✅ | `pending → validated`, cocokkan mutasi |
| `contribution` | `reject` | `arisan-field.contribution.reject` | ✅ | `pending → rejected` |
| `arisan-period` | `run-lottery` | `arisan-field.arisan-period.run-lottery` | ✅ | `open → closed`, buat draw |
| `draw` | `mark-paid` | `arisan-field.draw.mark-paid` | ✅ | `drawn → paid_out` |

### Report

| Report | Permission |
|--------|------------|
| `payment-recap-report` | `arisan-report.payment-recap-report.view` |

## Audit & Event

- Semua aksi kritis diberi `audit: true` → dicatat ke **audit log**.
- Event async di-declare dan di-deliver ke channel `audit_log`:
  - `contribution.validated` (dari aksi `validate`)
  - `contribution.rejected` (dari aksi `reject`)
  - `period.drawn` (dari aksi `run-lottery`)

```yaml
actions:
  - name: validate
    required_permission: arisan-field.contribution.validate
    audit: true
    emits: validated
    uses:
      resources: [bank-mutation.find, bank-mutation.update]
    impl: { type: script_ref, ref: validate }
```

## Batasan Expose (Serangan Permukaan API)

`expose` adalah array `{type, actions}`. Entity tertentu sengaja tidak
mengekspos semua CRUD:

| Entity | Expose REST | Catatan |
|--------|-------------|---------|
| `contribution` | `list, find, create` | status **tidak** bisa diubah via update/delete — hanya lewat aksi `validate`/`reject` |
| lainnya | `list, find, create, update, delete` | CRUD penuh |

> ⚠️ **Guard absolut engine**: `delete` tidak bisa dilewati permission mana pun
> (setara `ON DELETE RESTRICT`). Tidak ada `override_permission` untuk delete.

## kind: Policy (Governance Control Plane)

**`Policy` adalah kind Control Plane** (bukan per-entity). Dipakai untuk
governance tingkat platform: aturan keamanan, compliance, dan limit resource.
Di-manage oleh **Platform Operator** dan dievaluasi oleh control plane
(`platform/04-control-plane.md`).

Skema (`schemas/kinds/Policy.schema.json`):

```yaml
apiVersion: formspec.dev/v1alpha1
kind: Policy
metadata:
  name: ...
  module: ...
spec:
  blocked: []                 # aturan yang tidak bisa dikonfigurasi
  require_approval: []        # transisi yang butuh approval
  require_signing: false      # wajib tanda tangan
  require_staging_first: false# wajib lewat staging dulu
  rego: ""                    # escape hatch OPA (full body policy)
```

**Status di proyek ini**: belum ada manifest `Policy` — aplikasi berjalan
lokal tanpa control plane. Saat deploy ke produksi, `Policy` ditambahkan di
lapisan platform (bukan di `spec/modules/`).

## Arah Selanjutnya (TODO Keamanan)

- [ ] Manifes `kind: Policy` untuk env produksi (approval transisi, dsb.)
- [ ] Pemetaan role → permission di layer auth (di luar scope FormSpec spec)
- [ ] Verifikasi `required_permission` benar-benar di-enforce oleh engine di
      produksi (pada dev, endpoint tidak memerlukan token — lihat
      `development.md`)
