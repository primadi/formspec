# Spec Platform — Kontrak Lintas Sisi

Kontrak yang mengikat kedua sisi framework (visual dan penyimpanan) sekaligus.
Seorang shell author maupun persist-backend author wajib membaca section ini
sebelum masuk ke kontraknya masing-masing.

| Dokumen | Cakupan |
|---|---|
| [01-overview.md](01-overview.md) | Apa itu Forma, prinsip contract-vs-renderer, persona, batas scope |
| [02-workspace-app-module.md](02-workspace-app-module.md) | Model kepemilikan: workspace → app → module; App sebagai kurasi objek module; menu; qualifier referensi |
| [03-kind-system.md](03-kind-system.md) | Taksonomi kind, meta-kinds (KindDefinition, VisualSpecKind, Renderer, PersistBackend), pemetaan kind → plane |
| [04-control-plane.md](04-control-plane.md) | Kontrak control plane |
| [05-plane-protocol.md](05-plane-protocol.md) | Protokol antar plane |
| [06-datastore.md](06-datastore.md) | Kind Datastore dan lifecycle koneksinya |
| [07-marketplace.md](07-marketplace.md) | Distribusi module & renderer, trust tier |
| [08-project-layout.md](08-project-layout.md) | Struktur folder project aplikasi; runtime per Module untuk handler multi-bahasa |
| [09-observability.md](09-observability.md) | Kontrak observability engine Resource Plane: logging terstruktur, metrics, tracing, kosakata health, `forma logs` |
| [10-deployment-operations.md](10-deployment-operations.md) | Kontrak operasional: pipeline deployment & konvergensi, rollback, canary, promotion, DR/HA minimal |
