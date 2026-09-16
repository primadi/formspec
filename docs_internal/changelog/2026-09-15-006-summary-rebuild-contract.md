# 2026-09-15-006 — summary rebuild contract di-spec-kan

Kontrak summary Entity untuk `sources`/`join_key`/`rebuild` sekarang ditetapkan di canonical Go model `pkg/spec.EntitySpec` dan didokumentasikan di `docs/spec/backend/02-core-extended.md` §6. Keputusan ini menutup gap yang sebelumnya muncul di todo item 3.6.4: summary tak lagi mengandalkan implicit metadata yang tidak ada di schema, melainkan definisi eksplisit untuk replay source dan strategi rebuild.

Dampak perubahan: validasi summary entity dapat menolak konfigurasi yang mengacau (`rebuild.strategy` tidak valid, `join_key` tanpa sumber, atau metadata summary dipakai di entity non-summary). Dokumen backlog tetap mencatat bahwa CLI `formspec summary rebuild <entity>` dan projection runner replay event durable masih belum diimplementasikan; ini adalah task engine yang terpisah dari kontrak spesifikasi.
