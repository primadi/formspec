# 2026-09-14-007 — S3: allowlist akses anonim per entity+aksi (`public_entities`)

Item `TODO.md` 1.2 (S3, gap #6 dan sebagian #45).

**Masalahnya.** `access: public` memberi anonim `list`/`find`/`create` untuk
**seluruh entity** di module yang di-mount — granularitasnya module, bukan entity.
Aplikasi publik kafe karena itu ikut membuka `member` (nomor HP pelanggan),
`employee`, `shift`, dan `cash-movement` (data kas) hanya karena entity itu
berbagi module dengan menu/pesanan.

**Konstruksi baru.** `App.spec.public_entities`:

```yaml
access: public
public_entities:
  - { entity: cafe-master.menu-item, actions: [list, find] }
  - { entity: cafe-order.order,      actions: [create] }
```

Tiga keadaan — pointer `*[]PublicEntityDecl` membedakan **absen** dari **kosong**:

| Keadaan | Arti |
| --- | --- |
| absen | perilaku lama: list/find/create untuk seluruh module (kompatibel ke belakang) |
| `[]` | tidak ada yang anonim — seluruh permukaan App butuh autentikasi |
| daftar | hanya pasangan entity+aksi itu; sisanya di module yang sama butuh autentikasi |

Yang diubah:

- `pkg/spec/resources.go` — `PublicEntityDecl`, `PublicEntityActions` (closed set
  `list|find|create|update|delete`), `NormalizeEntityRef`, dan validasi di
  `ValidateAppSpec`: menolak `access` bukan `public`, module yang tidak di-mount by
  `spec.modules`, aksi di luar closed set, `actions` kosong, ref malformed, duplikat.
- `internal/api/router.go` — `publicGrants()` + `isPublicAction(module, entity, action)`
  menggantikan pengecekan per-module di `/_ui/entity/`. Grant module-wide (legacy)
  hanya berlaku untuk himpunan aksi lama, sehingga update/delete tetap tidak pernah anonim.
- `internal/genjsonschema/generator.go` — `PublicEntityDecl` masuk daftar shared type
  (tanpa ini schema App gagal dikompilasi: `json-pointer … not found`).
- `examples/kafe/spec/apps/kafe-qr.yaml` — memakai allowlist; komentar
  *"Bentuk IDEAL yang belum bisa dinyatakan"* dihapus karena sudah bisa.
- Schema diregenerasi (144 shared types).

`NormalizeEntityRef` menerima **dua** ejaan yang memang dipakai manifest
(`module.entity` seperti `relation.resource`, dan `module/entity` seperti Page ref),
memisah pada separator **terakhir** sehingga nama module bertitik
(`formspec.core.workspace`) tetap benar.

Verifikasi runtime (anonim, spec kafe):

| Entity/aksi | Sebelum | Sesudah |
| --- | --- | --- |
| `menu-category`/`menu-item`/`menu-item-price` list | 200 | **200** |
| `order` create | 201 | **201** |
| `member` list & create | 200/201 | **401** |
| `employee` list | 200 | **401** |
| `shift` list, `cash-movement` list | 200 | **401** |
| `menu-item-price` create (di luar izin) | 201 | **401** |
| `order` list (di luar izin) | 200 | **401** |

Dua baris terakhir memperlihatkan granularitas **per-aksi**, bukan hanya per-entity.
Test: `internal/api/public_entities_test.go` (4 case) +
`pkg/spec/public_entities_test.go` (4 case + `TestNormalizeEntityRef`).
`go test ./...` → 35 paket `ok`; spec kafe **0 problem**.

**#45 sebagian.** Izin per-entity ✅ dan penyelesaian oleh kasir ✅ (rute lifecycle
ada sejak #52, dan `order` lifecycle-free sejak #44 sehingga record anonim langsung
bisa direferensikan `payment`). **Sisa:** klaim kepemilikan record oleh tamu
(`table-session.guest_token`) — butuh scope **per-permukaan**, sebab `scope`
menyeluruh pada `table-session` akan menuntut `?token=` juga di POS kasir
(alasan yang sama dengan 1.1).

Referensi: `examples/kafe/gaps_found/TODO.md` (1.2), `README.md` (#6, #45).
