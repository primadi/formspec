# Kind Reference — FormSpec Resource Kinds

Referensi per-kind yang lengkap: **satu file per kind**, dipecah dalam 4 grup
yang mencerminkan taksonomi kind (`docs/spec/platform/03-kind-system.md` §1).

| Grup | Isi | Jumlah |
|---|---|---|
| [`curation/`](curation/) | Struktur workspace — App, Module | 2 |
| [`data/`](data/) | Model domain & behavior — Entity, Service, Config, dst. | 11 |
| [`ui/`](ui/) | Presentasi visual — Page, Form, Table, Dashboard, dst. | 15 |
| [`infra/`](infra/) | Renderer, storage, control plane — Renderer, Datastore, Policy, dst. | 5 |

**Total: 33 kind.** Meta-kind (`VisualSpecKind`) tidak punya halaman sendiri —
didefinisikan di `docs/spec/frontend/02-visual-spec-kind.md`.

## Cara Baca

Tiap file kind punya struktur konsisten:

| Section | Sumber | Boleh diedit? |
|---|---|---|
| `Kapan Memakai` | Manual | ✅ Ya — narasi author |
| `Contoh Manifest` | Manual | ✅ Ya — contoh YAML |
| `Atribut` | **Generated** dari `pkg/spec` | ❌ Jangan edit — ditimpa saat regenerate |
| `Gotchas` | Manual | ✅ Ya — narasi author |

> ⚠️ **Aturan emas:** jangan pernah mengedit konten **di antara** marker
> `<!-- generated:... -->`. Ubah atribut di `pkg/spec` (Go struct + godoc +
> `// @schema {...}` annotations), lalu regenerate. Konten di luar marker aman.

**Notasi tipe:** `enum (a · b · c)` di kolom `Tipe` berarti himpunan nilai
tertutup — nilai field harus salah satu dari daftar itu. Notasi ini dipakai
seragam untuk named enum type (mis. `characteristic`) maupun field `string`
dengan annotation `@schema {enum}` (mis. `lifecycle`) — bagi author YAML
keduanya berarti hal yang sama dan divalidasi identik.

## Cara Regenerate

```bash
make generate-kind-docs   # regenerate semua 33 file
```

Idempotent: setelah regenerate, `git diff` pada `docs/kind/` hanya menunjukkan
perubahan atribut yang benar-benar berubah di `pkg/spec` — narasi manual tidak
pernah disentuh.

Sumber kebenaran atribut: `pkg/spec/*.go` + `schemas/kinds/*.schema.json`
(JSON Schema di-generate dari sumber yang sama via `make generate-schema`).

## Marker Contract

Region generated ditandai pasangan komentar HTML:

```markdown
<!-- generated:meta --> ... <!-- /generated:meta -->      # profil kind (grup, plane, struct)
<!-- generated:attributes --> ... <!-- /generated:attributes -->  # tabel atribut
```

Generator hanya mengganti isi di antara marker. Jika marker hilang (file diedit
tangan dan marker dihapus), generator meregenerasi file utuh dengan warning —
narasi manual yang dihapus tidak bisa dikembalikan.

## Enrichment Atribut

Deskripsi, contoh, dan enum di tabel atribut berasal dari annotation
`// @schema {...}` di Go struct — lihat `pkg/spec/entity.go` untuk contoh.
Untuk memperkaya tampilan tabel (mis. menambah `example`):

```go
// @schema {description: "Kode unik grup", example: "AR-001"}
Code string `json:"code" yaml:"code"`
```

Lalu regenerate. JSON Schema (`schemas/`) ikut mendapat `example` juga.
