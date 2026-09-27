# Meja: 5 status + temuan gerbang transisi per-role (10.44)

**Ledger:** **10.40** (diperluas ke 5 state) · **10.44 ⏸️** (baru) · 10.42, 10.43
**Plan:** `docs_internal/plan/kafe-qr-table-session-flow.md`

## Perubahan

`cafe-master.dining-table` naik dari 3 → **5 status** sesuai permintaan pemilik:

| State               | Arti                    | Transisi masuk                                  |
| ------------------- | ----------------------- | ----------------------------------------------- |
| `available`         | kosong                  | `release`, `unreserve`, `mark-available`        |
| `occupied`          | terisi                  | `occupy` (dari `available`/`reserved`/`served`) |
| `served`            | semua pesanan disajikan | `mark-table-served`                             |
| **`reserved`**      | dipesan                 | `reserve` (dari `available`)                    |
| **`not_available`** | tidak bisa dipakai      | `mark-not-available` (dari `available`)         |

Aksi bernama: `occupy`, `mark-table-served`, `release`, `reserve`, `unreserve`,
`mark-not-available`, `mark-available`. Kafe `validate` 85 manifest
**0 problem**.

## Temuan: pembedaan per-role TIDAK bisa dinyatakan hari ini (10.44)

Permintaan pemilik: "`available ↔ not_available` oleh **admin**;
`available ↔ reserved` oleh **kasir**". **Terukur, ini tidak ditegakkan:**

```
kasir: PATCH table_status=not_available  → 200   (DB jadi not_available)
```

Kasir — yang menurut permintaan hanya boleh `reserved` — **berhasil** melakukan
transisi khusus admin.

**Akar, tiga lapis dan semuanya terverifikasi:**

1. `TransitionDecl` (`pkg/spec/entity.go:1717`) = `{from, to, via, guard,
emit}` — **tanpa** `required_permission`.
2. Penerap transisi adalah `PATCH`, diperiksa `{module}.{plural}.update` —
   **satu** permission untuk semua transisi (10.43). `registerStandardPermissions`
   hanya mendaftarkan `list/view/create/update/delete` + action; tidak ada slot
   gerbang per-transisi.
3. `guard.expression` **bukan** pengganti: FormSpecExpr **melarang**
   identitas/permission (`docs/spec/frontend/08-formspec-expr.md` §69) —
   `user.roles`/`user.has(...)` tidak bisa dievaluasi.

**Satu-satunya jalan hari ini:** transisi ber-gerbang sendiri dijadikan **action
dengan `impl`** (dapat route + `required_permission` sendiri) — pola
`order.void-order` dan `gl.journal-entry.post`, di mana
`gl/scripts/journal_post.star` men-`set("status","posted")` lalu `save()`.
Konsekuensinya: tiap transisi jadi file script, dan di UI ia muncul sebagai
tombol action, bukan sebagai state machine murni.

**Jadi manifest saat ini menyatakan niat yang belum ditegakkan** —
`required_permission: dining-tables.not-available` pada action `mark-not-available`
adalah **klaim yang tidak berlaku** sampai 10.44 diputuskan. Yang benar-benar
menggerbangi `PATCH` adalah `dining-tables.update`, dan grant itu dimiliki
kasir.

## Sisa

- **10.44 ⏸️** — putuskan: `transition.required_permission` di engine (→ `PATCH`
  mengecek permission transisi, bukan `update`), atau tiap transisi jadi action
  `impl` + script.
- **10.42 ⏸️** (backfill DB lama), **10.43 ⏸️** (semantik `required_permission`
  pada action impl-less), **10.40b ⏸️** (pemicu otomatis saat bayar),
  **10.41 ⏸️** (agregat "semua disajikan").
- **10.34c / 10.36 / 10.37 / 10.38 / 10.39 / 10.20** — tidak berubah.
