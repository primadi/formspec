# CLI Reference — Registry & Vendoring

Semua verb adalah subcommand dari satu binary `formspec`.

## `formspec sign`

```bash
formspec sign keygen --out <dir> --name <vendor>
formspec sign <module-dir> --key <private.key> [--out <sig-file>]
formspec sign verify <module-dir> --signature <b64|file> --public-key <pub.file>
```

- `keygen`: ed25519 keypair; private key ditulis mode `0600`.
- `sign <dir>`: hitung tree checksum lalu sign; output signature base64.
- `verify`: recompute checksum + verifikasi signature (tamper detection).

## `formspec module publish`

```bash
formspec module publish <module-dir> --vendor <name> --key <private.key>
                       --version <semver>
                       [--registry URL] [--api-key <key>] [--spec <path>]
```

| Flag         | Default                         | Keterangan                                  |
| ------------ | ------------------------------- | ------------------------------------------- |
| `--vendor`   | wajib                           | Nama vendor (pemilik identitas ed25519)     |
| `--key`      | wajib                           | File private key (base64)                   |
| `--version`  | wajib                           | Semver tag                                  |
| `--registry` | `https://registry.formspec.dev` | Override via env `FORMSPEC_MODULE_REGISTRY` |
| `--api-key`  | env `FORMSPEC_REGISTRY_API_KEY` | Publish ter-autentikasi                     |
| `--spec`     | `spec`                          | Fallback resolusi nama module               |

Nama module dibaca dari `module.yaml` di folder sumber. Versi immutable.

## `formspec module install`

```bash
# Dari registry (13.3.8):
formspec module install --from <registry> <module>[@<version>]
                        [--project <dir>] [--spec <path>] [--use]
                        [--registry URL]

# Offline (13.1.2): git URL / folder lokal / .tar.gz
formspec module install <source> [--use] [--version <tag>]
```

Tanpa `@version` → `latest`. Install selalu verifikasi signature sebelum
mempercayai tarball. Entri default nonaktif; `--use` langsung mengaktifkan.

## `formspec module list | uninstall`

```bash
formspec module list [--spec <path>] [--project <dir>]
formspec module uninstall <effective-name> [--spec <path>] [--project <dir>]
```

`list` menampilkan nama efektif, versi, trust tier, status aktif/nonaktif,
dan source. `uninstall` membersihkan `vendors/` + lock + marker.

## `formspec verify`

```bash
formspec verify [--project <dir>]
```

Recompute tree checksum `vendors/` vs `formspec.lock`. Modifikasi manual
terdeteksi → exit 1.

## `formspec override adopt | diff | list`

```bash
formspec override adopt <module> <kind> <name> [--spec <path>] [--project <dir>]
formspec override diff  <module> <kind> <name> [--project <dir>]
formspec override list  [--project <dir>]
```

Whitelist kind: `Form`, `VisualSpecKind` (presentation only). `diff` menampilkan
unified diff + flag drift bila upstream berubah sejak adopt.
