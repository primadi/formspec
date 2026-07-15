# Renderers — Implementasi Resmi

Dokumentasi **implementasi** kontrak Forma. Berbeda dari [`../spec/`](../spec/README.md)
yang normatif, isi section ini deskriptif: menjelaskan bagaimana implementasi
resmi memenuhi kontraknya hari ini, dan boleh berubah tanpa mengubah kontrak.

| Seam | Kontrak | Implementasi resmi |
|---|---|---|
| Shell (visual) | [`spec/frontend/`](../spec/frontend/) | [`shadcn-shell/`](shadcn-shell/) — React + shadcn/ui, interpreter runtime |
| PersistBackend (penyimpanan) | [`spec/backend/04-persist-backend.md`](../spec/backend/04-persist-backend.md) | [`persist-postgres/`](persist-postgres/) — Postgres hybrid JSONB |

Satu implementasi resmi per seam hari ini; kontraknya dirancang agar implementasi
kedua (shell Flutter, backend fully-relational/SQLite) bisa ditambah tanpa
membongkar kode inti. Panduan menulis implementasi baru ada di
[`../guides/`](../guides/).
