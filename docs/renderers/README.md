# Renderers — Implementasi Resmi

Dokumentasi **implementasi** kontrak Forma. Berbeda dari [`../spec/`](../spec/README.md)
yang normatif, isi section ini deskriptif: menjelaskan bagaimana implementasi
resmi memenuhi kontraknya hari ini, dan boleh berubah tanpa mengubah kontrak.

| Seam | Kontrak | Implementasi resmi |
|---|---|---|
| Shell (visual) | [`spec/frontend/`](../spec/frontend/) | [`shadcn-shell/`](shadcn-shell/) — React + shadcn/ui, interpreter runtime |
| PersistBackend (penyimpanan) | [`spec/backend/04-persist-backend.md`](../spec/backend/04-persist-backend.md) | [`jsonb-persist/`](jsonb-persist/) — hybrid JSONB, jalan di atas Postgres maupun SQLite |

Topik lintas (server + client) yang diimplementasikan di atas satu seam:

- **[Realtime WebSocket](realtime.md)** — transport realtime (`/_ui/_ws`):
  subscription server-side, hook client `useRealtime`, optimasi, lifecycle
  navigasi/refresh/reconnect.

Satu implementasi resmi per seam hari ini. Tiap subfolder di sini adalah satu
implementasi terdaftar untuk satu seam — tempat implementasi kedua nanti sudah
tersedia tanpa membongkar kode inti maupun implementasi resmi yang ada, tinggal
ditambah folder baru sejajar: shell kedua (mis. Flutter) di
`renderers/<nama-shell>/`, PersistBackend dengan strategi skema berbeda (mis.
fully-relational — tiap field jadi kolom nyata, bukan JSONB) di
`renderers/<nama-backend>/`. Panduan menulis implementasi baru ada di
[`../guides/`](../guides/).
