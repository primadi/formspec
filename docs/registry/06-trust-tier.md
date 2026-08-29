# Trust Tier

Sumber kontrak: [`../spec/platform/07-marketplace.md`](../spec/platform/07-marketplace.md) §2.

## Tiga Tier

| Tier        | Arti                                                        | Status             |
| ----------- | ----------------------------------------------------------- | ------------------ |
| `community` | Default semua module baru                                   | ✅ implemented     |
| `verified`  | Lolos review (manual/otomatis, kebijakan Platform Operator) | ⏸️ 13.3.5 deferred |
| `official`  | Ditandatangani FormSpec                                     | ⏸️ deferred        |

Tier disimpan di `Module.trust_tier` dan tercatat di `formspec.lock` konsumen
saat install.

## Verified Badge

Wajib untuk listing berbayar: signature chain ed25519 terverifikasi terhadap
public key vendor di registry + proses review. Implementasi review flow
direncanakan sebagai **Workflow kind** (approval engine Fase 7) atau native
service yang mengecek kriteria otomatis.

## Trust Tier Menggerbang Tipe `impl`

Pertahanan untuk handler biner adalah _provenance_, bukan sandbox sempurna.
Karena itu:

| Tier                                | impl yang boleh dijalankan                  |
| ----------------------------------- | ------------------------------------------- |
| community / unverified              | `script`, `script_ref`, WASM (sandbox) saja |
| verified                            | + `sidecar` (terisolasi via proxy)          |
| verified + security scan + approval | + `native`, `compiled`                      |

Konsekuensi: implementasi tier menyentuh enforcement di boot/install, bukan
sekadar field di entity.

## Proteksi Nilai Komersial

Manifest **tidak pernah dienkripsi** (keterbacaan adalah fitur). Proteksi IP:
(1) `impl` native/compiled boleh biner tanpa source; (2) signed provenance —
salinan yang di-rename tidak bisa memalsukan signature; (3) ekonomi
update+support tidak ikut berpindah; (4) lisensi module.
