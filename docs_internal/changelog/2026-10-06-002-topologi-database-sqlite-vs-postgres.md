# 2026-10-06-002 — Topologi database: kapan SQLite, kapan PostgreSQL (D-ARCH-32)

Menambahkan `docs/architecture/10-database-topology.md` sebagai keputusan arsitektur
eksplisit atas pertanyaan "SQLite atau PostgreSQL untuk produksi?" — pertanyaan yang
sebelumnya hanya dijawab sepotong: `03-deployment-flow.md` §4 punya tabel Dev (SQLite) vs
Production (Postgres), dan `05-failover.md` §3.5 punya baris "SQLite lokal ❌ auto-failover",
tetapi tidak ada dokumen yang menyatakan **aturan penentunya** dan batas teknis di baliknya.

**Keputusan:** satu backend (jsonb-persist) di atas dua engine. SQLite sah sebagai engine
**development dan embedding**, tetapi bukan sebagai engine **production** — dan aturan itu
sudah ditegakkan kode, bukan sekadar rekomendasi: `cmd/formspec/serve.go` (`--mode=production`)
menolak DSN berawalan `sqlite:` dan mewajibkan `--dsn postgres://…` (gate Fase 8 / todo 8.1.4).
Di luar itu: tier shared economy memakai PostgreSQL; enterprise memakai Postgres dedicated via
`kind: Datastore` + `access.filter.workspaces`. Aturan ini kini `D-ARCH-32` di
`docs/architecture/01-architecture-overview.md` §12.

**Temuan saat menulis (mengoreksi draf sendiri).** Draf pertama dokumen ini menyatakan
"SQLite atau Postgres — keduanya sah" untuk tier standalone. Verifikasi kode membantahnya:
`serve.go:` menolak SQLite di production mode. §1 dan §6 dokumen dikoreksi menjadi
"SQLite = engine dev/embedding", dengan §1.1 menjelaskan bahwa gate-nya sudah ada sejak
Fase 8 dan dokumen ini menyediakan **alasan teknis** di baliknya (supaya gate itu tidak
dilonggarkan tanpa memahami biayanya). Ini memperkuat tesis dokumen, bukan melemahkannya.

**Temuan teknis yang jadi dasar (verifikasi, bukan asumsi).** Pembatas SQLite di FormSpec
adalah **satu koneksi**, bukan "satu proses": `OpenSQLite` (`renderers/jsonb-persist/sqlite_db.go`)
memaksa `SetMaxOpenConns(1)`/`SetMaxIdleConns(1)` dengan pragma WAL, `foreign_keys(ON)`,
`busy_timeout(5000)`, `cache_size(-32000)`; PostgreSQL memakai 25/10. Karena itu kebuntuan
bisa terjadi **di dalam satu proses** — persis kelas bug yang tercatat di
`examples/arisan/docs/engine-sqlite-deadlock.md` (baca lewat pool dasar saat `TxScope`
memegang satu-satunya koneksi). Bug itu **sudah tertutup** di repo ini: `crud.go` memakai
`txReadDB(ctx, s.db)` pada jalur resolusi relasi, dan `grep 's\.db\.\(QueryContext\|ExecContext\)'`
atas `crud.go` kembali nol. Dokumen §4 mencatat invarian `txReadDB`/`writeDB`/`runTx` beserta
resep verifikasinya.

**Konsekuensi turunan yang kini tertulis.** Semua worker FormSpec adalah goroutine in-process
yang berbagi handle `DB` yang sama dengan jalur HTTP (`resource/formspec.go` untuk outbox,
job tracker, link sweeper, streaming; `internal/approval/escalation.go`;
`internal/subscription/{dynamic,stream}.go`) — tidak ada binary worker di `cmd/`. Ini bukan
kebetulan melainkan syarat yang mengikat selama SQLite didukung: memindahkan worker ke proses
lain mengubah _latency cliff_ menjadi kegagalan tulis rutin, tanpa perbaikan yang mungkin di
level aplikasi.

**Alternatif yang ditolak (sengaja tidak masuk `docs/`).** (a) Satu database embedded per
workspace — ditolak: `txscope.go` hanya mengizinkan transaksi lintas-Module dalam satu
Datastore fisik (`ErrCrossStoreTx`); worker/poller/backup menjadi N kali; kontrak backup
`filterable` (§3 `04-persist-backend.md`) tidak terpenuhi. (b) PGlite (Postgres WASM) sebagai
engine ketiga — ditolak: ia library TypeScript/JS, status Alpha, dan batasan resminya
"single user/connection"; `config.go` menerima closed set `sqlite`|`postgres`. (c)
Menaikkan `SetMaxOpenConns` pada SQLite untuk multi-writer — ditolak: menuntut perubahan
jaminan `TxScope` dengan nilai kecil dibanding memindahkan tier economy ke PostgreSQL; dicatat
sebagai batas scope di §8 dokumen.

**File yang tersentuh.** `docs/architecture/10-database-topology.md` (baru);
`docs/architecture/01-architecture-overview.md` (D-ARCH-32 + rentang di header);
`docs/architecture/README.md` (Document Map, jalur baca Platform Operator, indeks keputusan,
tabel relasi ke spec); `docs-site/.vitepress/config.mts` (entri sidebar). Tidak ada perubahan
kode.

**Referensi.** Keputusan ini tidak berasal dari plan terpisah — diminta langsung sebagai
dokumen keputusan arsitektur. Follow-up yang berbentuk **pekerjaan terbuka** (routing koneksi
per-workspace di Resource Plane + worker per-store) diberi rumah yang bisa ditunjuk sebagai
item `8.3.3 ⏸️` di `docs_internal/plan/todo.md`; sisanya adalah batas desain (SQLite
multi-writer, RLS sebagai lapis kedua, metering berbasis ukuran DB) dan dinyatakan sebagai
**ditolak secara desain** di §8, bukan sebagai backlog tersembunyi.
