# 2026-09-15-007 — `formspec summary rebuild` + channel `reliable_event` yang akhirnya durabel

Menutup sisi runtime dari kontrak summary rebuild (`2026-09-15-006`): summary
Entity yang tidak lagi bisa direbuild hanya karena tidak ada alat untuk
melakukannya. Todo item **3.6.4**.

**Yang ditambahkan.**

- `internal/summary/` — resolusi kontrak jadi rencana rebuild. `Resolve`
  menerima `module/entity`, `module.entity`, atau nama entity telanjang (hanya
  di antara entity `characteristic: summary`, jadi rebuild tidak bisa
  dibelokkan ke entity biasa). `PlanRebuild` memetakan `sources` ke stream
  durabel yang benar-benar mendengarkan event-nya, dan melaporkan sumber yang
  tidak punya subscriber durabel sebagai **orphaned** — bukan menghilangkannya.
  Entity ber-`rebuild.strategy: none`, atau summary tanpa `sources`, atau
  entity non-summary **ditolak** dengan pesan yang menyebut alasannya.
- `internal/subscription/replay.go` — `StreamingWorker.ReplaySummaryProjection`
  me-replay stream durabel dari awal lewat jalur yang sama dengan delivery live
  (filter → transform → dispatchOne). Setiap run memakai consumer group sendiri
  (`formspec-summary-rebuild:<runID>:<module>/<name>`), sehingga cursor dan
  pending entry worker live tidak tersentuh — rebuild aman dijalankan terhadap
  server yang melayani. Kegagalan handler di-ack di group run itu (supaya loop
  tidak menggantung karena at-least-once) dan **dilaporkan**, bukan di-retry:
  retry tetap milik worker live, dan run yang tidak lengkap keluar dengan
  status ≠ 0.
- `cmd/formspec/summary.go` — verb `formspec summary list|rebuild` (Fase 3.6.4):
  `--spec`, `--dsn`, `--workspace`, `--subscriber`, `--reset`, `--dry-run`,
  `--json`. `--reset` menolak berjalan pada `strategy: partial` (baris di luar
  jendela harus tetap ada) dan menolak membangun SQL dari nama tabel yang tidak
  lolos `^[A-Za-z_][A-Za-z0-9_]*$`. Backend stream in-memory (default dev)
  memunculkan peringatan eksplisit — proses CLI terpisah tidak punya riwayat
  untuk di-replay, dan itu dikatakan alih-alih melaporkan sukses kosong.
- `resource.App`: akses `Subscriptions()`/`StreamingWorker()`/`Stream()` plus
  field `subReg`/`subDispatch` (di-rebuild juga saat `ReloadSpec`), supaya
  rebuild memakai wiring yang sama dengan server.

**Bug nyata yang ikut ditutup: channel `reliable_event` adalah no-op.**

`ValidateEventDurability` sudah _mewajibkan_ `publish.durable: true` untuk
`deliver: [{channel: reliable_event}]`, dan `docs/guides/order-to-cash-tutorial.md`
menjanjikan event itu lewat outbox — tetapi di runtime channel ini jatuh ke
cabang `default:` dan hanya menulis warning `event.channel_not_implemented`.
Artinya manifest lolos validasi sementara **tidak ada event yang terkirim sama
sekali**: proyeksi yang digerakkan event durabel (`kafe` `order.on_paid` →
stok & jurnal) diam-diam kosong. Kini `reliable_event` masuk ke outbox
(`internal/action/deliver.go`), dengan `event_handler.go` memberi kasus eksplisit
supaya tidak lagi menyamar sebagai "belum diimplementasikan".

**Adopsi di `examples/kafe`.** Tiga proyeksi (`cafe-stock/stock-level`,
`cafe-stock/menu-cost`, `cafe-loyalty/member-point`) kini mendeklarasikan
`sources`/`join_key`/`rebuild` sesuai §6. `rebuild.strategy: full` dipakai
dengan sengaja: biaya rata-rata bergerak dan saldo poin bergantung pada seluruh
riwayat, bukan satu jendela. `formspec summary list` melaporkan ketiganya
sebagai **orphaned** — sumbernya belum punya subscriber durabel (kafe memakai
`kind: Integrator`, yang tidak melalui stream dan karena itu tidak punya jalur
replay). Gap itu sekarang terlihat, bukan diasumsikan beres.

**Guard baru di generator schema.** `internal/genjsonschema/schema_refs_test.go`
menguji dua hal: (a) setiap `$ref` di setiap kind schema punya entri `$defs` di
root, dan (b) `schemas/` yang di-commit sama persis dengan hasil generate
sekarang. Yang kedua menangkap kelas kegagalan yang benar-benar terjadi: `pkg/spec`
berubah sementara `schemas/` tidak di-regenerate → `formspec validate --schema
schemas` gagal compile untuk **semua** Entity (`$defs/RebuildSpec` tidak ada),
termasuk Entity yang tidak berhubungan. Guard itu sudah diverifikasi bisa gagal
(dengan sengaja men-stale-kan satu file), lalu hijau setelah di-restore.

**Verifikasi.** `go build ./...` hijau; `go test ./internal/summary/...
./internal/subscription/... ./internal/action/... ./internal/genjsonschema/...`
hijau; `./bin/formspec summary list --spec examples/kafe/spec` menampilkan tiga
proyeksi + orphaned-nya; `./bin/formspec validate --schema schemas --spec
examples/kafe/spec` → `72 manifest(s) validated, 0 problem(s) found`;
`./bin/formspec check -f examples/kafe/spec` → `0 error(s), 0 warning(s)`.

**Sisa (dicatat, belum dikerjakan).** `formspec validate` tanpa `--schema`
masih membaca schema registry yang belum memuat `sources`/`join_key`/`rebuild`
(3 problem) — tiket yang sama dengan `scope`/`public_entities`: registry perlu
di-refresh lebih dulu (todo 3.6.7). Proyeksi yang digerakkan `kind: Integrator`
(semua proyeksi `kafe` hari ini) belum punya jalur replay — integrator tidak lewat
stream durabel, jadi butuh kontrak replay sendiri atau migrasi ke Subscription
durabel (todo 3.6.6). Dan `rebuild.strategy: partial` **belum benar-benar
parsial**: `RebuildSpec.Window`/`Since` sudah dideklarasikan dan dicetak di
rencana, tetapi tidak pernah dikonsumsi (`subscription.ReplayOptions` tidak punya
field window/since; replay selalu dari `earliest`), sementara `ValidateEntitySpec`
hanya memvalidasi `strategy` sehingga format `window` tidak dijaga — docs §6
memakai `"7d"` dan fixture memakai `"month"`. Perlu keputusan format + semantik
`since` dulu (todo 3.6.8).
