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
    ├── database-layer.md          # internal/db
    ├── api-layer.md               # internal/api
    └── frontend-renderer.md       # web/ + internal/ui (Meta API)
```

---

## Index

| Dokumen | Package | Deskripsi | Status |
|---|---|---|---|
| [database-layer.md](./database-layer.md) | `internal/db` | DB abstraction, DSN parsing, SQLite/PostgreSQL drivers, DDL generation, schema migration, CRUD query builder, child storage, natural key counter, idempotency store, outbox table | ✅ Fase 1.1 |
| [api-layer.md](./api-layer.md) | `internal/api` | Multi-protocol router (chi radix-tree), deny-by-default exposure, workspace-prefixed routes, smart internal dispatch, auto-generated REST handlers, response envelopes | 📋 Planned — Fase 1.3 |
| [frontend-renderer.md](./frontend-renderer.md) | `web/`, `internal/ui` | Manifest-driven renderer: Meta API, derivation engine (D17 — auto Table/Form/Page/Menu dari Entity), 12 kind renderers, FormaExpr interpreter, permission-driven UI, modal/drawer/page containers | 📋 Design — Fase 4 |

## Konvensi

1. **Satu file per package** — nama file = nama package (tanpa `internal/`)
2. **Referensi ke spec** — setiap dokumen harus menyebut spec section mana yang diimplementasikan
3. **File reference table** — daftar semua file dalam package dengan line count & purpose
4. **Updated date** — cantumkan tanggal terakhir update di footer

---

> **Catatan:** Folder ini untuk developer yang berkontribusi ke codebase Forma. Untuk membangun aplikasi di atas Forma, lihat [`docs/spec/`](../spec/README.md).
