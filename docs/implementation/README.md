# Forma Framework — Implementation Documentation

> Dokumentasi teknis implementasi Forma. Untuk spesifikasi (kontrak), lihat [`docs/spec/`](../spec/README.md).

---

## Document Map

```
docs/
├── spec/                          # Spesifikasi (kontrak, CC0)
│   ├── 01-overview.md
│   ├── 02-core-basic.md
│   ├── ...
│   └── 11-reference.md
│
└── implementation/                # Dokumentasi implementasi (internal)
    ├── README.md                  # ← Anda di sini
    └── database-layer.md          # internal/db
```

---

## Index

| Dokumen | Package | Deskripsi | Status |
|---|---|---|---|
| [database-layer.md](./database-layer.md) | `internal/db` | DB abstraction, DSN parsing, SQLite/PostgreSQL drivers, DDL generation, schema migration, CRUD query builder, child storage, natural key counter, idempotency store, outbox table | ✅ Fase 1.1 selesai |

---

## Konvensi

1. **Satu file per package** — nama file = nama package (tanpa `internal/`)
2. **Referensi ke spec** — setiap dokumen harus menyebut spec section mana yang diimplementasikan
3. **File reference table** — daftar semua file dalam package dengan line count & purpose
4. **Updated date** — cantumkan tanggal terakhir update di footer

---

> **Catatan:** Folder ini untuk developer yang berkontribusi ke codebase Forma. Untuk membangun aplikasi di atas Forma, lihat [`docs/spec/`](../spec/README.md).
