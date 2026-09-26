# Natural-key routing

Menetapkan implementasi URL dan breadcrumb entity untuk memakai `natural_key` sebagai identifier publik utama, dengan fallback ke UUID primary key. `unique` biasa tidak dipilih otomatis karena satu entity dapat memiliki beberapa unique field.

Perubahan mencakup helper pemilihan identifier, link tabel/detail/action, route identity untuk breadcrumb, kontrak `natural_key` (implisit `unique` + `required`; `false` eksplisit ditolak lewat presence flag), dan relaksasi presence di form untuk key yang dibuat server. Test dan dokumentasi routing ikut diperbarui. Referensi plan: `docs_internal/plan/entity-natural-key-routing.md`.
