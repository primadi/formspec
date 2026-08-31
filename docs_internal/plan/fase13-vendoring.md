# Fase 13.1: Local Vendoring & Activation (offline-first)

> Referensi: `docs/spec/platform/08-project-layout.md` §6,
> `docs/technical-notes/Forma-Technical-Note-Module-Vendoring-Aktivasi.md`
> (keputusan D-e–D-g), `docs/cli-tools/02-formspec-cli.md` §9.
> Batch ini: 13.1.1–13.1.6. 13.2 (overrides) dan 13.3 (registry) menyusul.

## Keputusan Teknis

1. **`internal/vendor/`** package baru — lockfile types, tree checksum,
   marker parser/writer, alias resolver, install flow. Semua logika vendoring
   di satu package; CLI (`cmd/formspec/module*.go`) hanya presentasi.
2. **`formspec.lock`** (YAML, project root) — entri per module: `source`,
   `name` (metadata.name), `alias`, `version`, `checksum` (sha256 tree),
   `signature`, `trust_tier`, `installed_at`. Re-install mempertahankan
   `signature`/`trust_tier` (diisi 13.3.6).
3. **Marker di App manifest** — blok terstruktur:
   ```yaml
   modules:
     - billing
     # >>> formspec:vendor github.com/acme/billing-module @1.0.0
     # - acme-billing
     # <<< formspec:vendor
   ```
   **Adaptasi dari tech note**: tech note menggambar entri `- source: … as: …`
   di bawah `uses:`; realita kode hari ini adalah `AppSpec.Modules []string`
   — jadi marker memuat **nama efektif** (alias) sebagai string, metadata
   source@version di header marker. Keputusan dicatat; tech note tidak diubah
   (arsip).
4. **Aktivasi** — entri ter-comment = nonaktif (default), uncomment = aktif;
   `--use` langsung aktif. Re-install **mempertahankan** status aktif (D-g) —
   hanya versi di header marker + lock yang di-update.
5. **Alias Opsi B (D-e)** — dihitung saat install, terhadap semua module yang
   pernah ter-install (lock) + module lokal (`spec/modules/*`). Derivasi:
   `{org}-{name}` dari source (mis. `github.com/acme/billing-module` →
   `acme-billing`); fallback `{name}-2`, `-3`.
6. **Boot-time enforcement (13.1.4)** — hanya module vendor AKTIF yang
   di-register: `AddManifestRoot(vendors/{dir})` per module aktif; module
   nonaktif diam di disk. Alias di-rewrite saat load via seam baru
   `Loader.SetAliases` (original → effective, diterapkan ke
   `metadata.module` semua manifest dari root vendor). Bentrok nama efektif
   di set aktif → refuse boot.
7. **`formspec verify`** — recompute tree checksum vs lock; mismatch = exit 1
   (modifikasi manual vendors/ terdeteksi).

## File

| File                          | Isi                                                                                                               |
| ----------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `internal/vendor/lock.go`     | `Lock`/`LockEntry`, Load/Save, `TreeChecksum`                                                                     |
| `internal/vendor/marker.go`   | Parse/write marker block di App manifest (idempotent, preserve active)                                            |
| `internal/vendor/alias.go`    | `ResolveAlias` (Opsi B)                                                                                           |
| `internal/vendor/install.go`  | `Install`: fetch (folder/tarball/git) → stage → validate → copy → checksum → lock → marker; `Uninstall`; `Verify` |
| `internal/manifest/loader.go` | Seam `SetAliases` (rewrite metadata.module)                                                                       |
| `resource/formspec.go`        | Boot wiring: deteksi lock, active set, roots + alias + conflict refusal                                           |
| `cmd/formspec/module.go`      | Dispatch `module install/list/uninstall`                                                                          |
| `cmd/formspec/verify.go`      | `formspec verify`                                                                                                 |

## Out of scope (batch ini)

- 13.2 overrides (shadow copy) — batch berikutnya.
- 13.3 registry app + sign/publish — butuh 13.2 dulu untuk alur lengkap.
