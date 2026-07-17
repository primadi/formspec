# Observability

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku.

## 1. Cakupan
Kontrak observability untuk **engine Resource Plane** (`forma serve` — proses
yang menjalankan handler bisnis tenant). Ia mendefinisikan apa yang **wajib**
di-*emit* setiap implementasi engine sehingga operator manapun (dan
reimplementasi engine yang konform) bisa dimonitor dengan cara yang sama:
logging terstruktur, metrics, tracing, dan kosakata health yang
machine-readable.

Ini **bukan** alerting engine dan **bukan** observability Control Plane.
Observability governance (transparency log, decision log, evidence) adalah
urusan [`04-control-plane.md`](04-control-plane.md) §7 dan
[`05-plane-protocol.md`](05-plane-protocol.md) §4.4; kosakata health yang
didefinisikan di sini (§5) adalah kontrak yang **dikonsumsi** oleh evidence
`health` Plane Protocol dan oleh forma/ops — tetapi engine tidak pernah
mendorong alert sendiri (§6).

**Larangan keras (normatif, tidak bisa dikonfigurasi lepas):** telemetry
**tidak boleh** membawa data bisnis tenant di luar level `debug` yang
di-gate eksplisit (§2). Ini perluasan langsung batas Control ↔ Resource
Plane ([`04-control-plane.md`](04-control-plane.md) §1): sebagaimana evidence
tidak pernah berisi data bisnis ([`05-plane-protocol.md`](05-plane-protocol.md)
§3.4), telemetry pun tidak.

## 2. Structured Logging
Engine **wajib** meng-emit log sebagai **JSON lines** (satu objek JSON per
baris) ke stdout — satu stream, di-collect oleh stack operator (K8s log
pipeline, Loki, dsb). Teks bebas non-JSON tidak konform di mode `prod`.

### 2.1 Field Wajib
Setiap record log level `info` ke atas **wajib** membawa field berikut (nilai
kosong ditulis `null`, bukan dihilangkan):

| Field | Isi |
|---|---|
| `timestamp` | RFC 3339 UTC, presisi milidetik |
| `level` | `debug` \| `info` \| `warn` \| `error` |
| `request_id` | ID korelasi request (§2.3); `null` untuk log di luar konteks request (boot, scheduler) |
| `workspace` | Workspace ID; identitas dari infrastruktur, bukan dari kode aplikasi |
| `module` | Module yang mengeksekusi |
| `entity` | Nama entity/Entity yang disentuh; `null` kalau tidak relevan |
| `action` | Action yang dieksekusi; `null` di luar konteks action |
| `actor` | Identitas pemanggil (user ID atau service identity); **tidak pernah** PII mentah (nama/email) — hanya ID |
| `duration_ms` | Durasi operasi, integer milidetik; hanya pada record penutup operasi |
| `error_code` | Kode error kontrak (mis. `DATASTORE_PERMISSION_DENIED`); `null` saat sukses |
| `trace_id` | Trace ID OpenTelemetry (§4) untuk korelasi log ↔ trace |
| `environment` | Nama environment; **hanya untuk atribusi log**, tidak pernah untuk percabangan bisnis ([`../backend/02-core-extended.md`](../backend/02-core-extended.md) §8) |

Field tambahan spesifik-engine **boleh** ditambahkan; consumer wajib
mengabaikan field yang tidak dikenal (forward-compatible).

### 2.2 PII & Data Discipline
`info`/`warn`/`error` **wajib** berisi hanya metadata (siapa, apa, berapa
lama, kode hasil) — **tidak pernah** payload field bisnis, nilai record,
argumen query, atau body request. Nilai bisnis yang membantu debugging hanya
boleh muncul di level `debug`, yang **wajib** off secara default di `prod`
dan hanya diaktifkan lewat kontrol operator yang tercatat (bukan flag
aplikasi). Pesan error yang dipropagasi ke log **wajib** sudah di-redact dari
nilai bisnis — kode error + identifier resource, bukan isinya.

### 2.3 Korelasi via `request_id`
Engine **wajib** menerbitkan `request_id` di boundary masuk (HTTP request,
pesan queue, tick scheduler) kalau belum ada, atau meneruskan yang datang
dari upstream. `request_id` yang sama **wajib** dipropagasi ke: script
Starlark (terbaca sebagai `ctx.request_id`), call ke sidecar (lewat header
transport — bagian dari kontrak wire sidecar), dan operasi `ctx.*` turunan.
Ini yang membuat satu request bisa dijejak lintas engine → script/sidecar →
persist dalam log, dan berpasangan dengan `trace_id` untuk tracing (§4).

## 3. Metrics
Engine **wajib** meng-expose endpoint metrics format **Prometheus** (text
exposition, `GET /metrics`) pada listener administratif yang terpisah dari
traffic bisnis. Endpoint ini tidak pernah membawa data bisnis — hanya counter,
gauge, dan histogram beragregat.

### 3.1 Set Metric Minimal Wajib
Setiap engine konform **wajib** meng-emit minimal metric berikut (nama boleh
di-prefix implementasi, semantik wajib identik):

| Metric | Tipe | Makna |
|---|---|---|
| `http_requests_total` | counter | Jumlah request, per `route_class` + `method` + `status_class` |
| `http_request_duration_seconds` | histogram | Latensi request per `route_class` |
| `http_request_errors_total` | counter | Request gagal per `route_class` + `error_code` |
| `action_duration_seconds` | histogram | Durasi eksekusi action per `module` + `action` |
| `action_errors_total` | counter | Action gagal per `module` + `action` + `error_code` |
| `outbox_pending` | gauge | Kedalaman antrian outbox yang belum ter-flush |
| `outbox_lag_seconds` | gauge | Umur entri outbox tertua yang belum terkirim |
| `ws_connections` | gauge | Koneksi websocket aktif |
| `db_pool_open` / `db_pool_idle` / `db_pool_wait_total` | gauge / counter | Statistik `ConnectionPool` per Datastore ([`06-datastore.md`](06-datastore.md) §7) |
| `snapshot_age_seconds` | gauge | Umur snapshot Plane Protocol terakhir (dasar degradasi [`05-plane-protocol.md`](05-plane-protocol.md) §5) |

### 3.2 Label & Kardinalitas
Label wajib dibatasi ke dimensi ber-**kardinalitas terbatas** yang diketahui
dari manifest: `workspace` (dalam model 1 workspace = 1 Deployment praktis
konstan per pod — [`../../architecture/05-failover.md`](../../architecture/05-failover.md)
§3), `module`, `action`, `route_class`, `error_code`, `status_class`
(`2xx`/`4xx`/`5xx`, bukan status mentah).

**Dilarang jadi label** (normatif — sumber ledakan kardinalitas): `entity`
instance ID, `request_id`, `actor`, path URL mentah, nilai field bisnis,
atau nilai bebas apapun yang berasal dari input tenant. `route_class`
mengelompokkan rute menurut kelas (entity CRUD, action invoke, admin panel,
websocket, health), **bukan** path per-record, justru supaya kardinalitas
tetap terbatas.

## 4. Tracing
Engine **wajib** kompatibel OpenTelemetry: satu request menghasilkan span
tree `HTTP → action → script/sidecar → persist`, dengan span persist
menandai operasi `ctx.*` (query, lock, queue). Atribut span tunduk pada
disiplin data yang sama dengan log (§2.2) — tidak ada nilai bisnis di
atribut span pada level default.

**Propagasi konteks trace adalah bagian kontrak wire.** Engine **wajib**
meng-inject dan menerima trace context format **W3C Trace Context**
(`traceparent`/`tracestate`) pada: request HTTP masuk, dan setiap panggilan
ke sidecar lewat transport SDK (`lib-forma-*`). Ini menjadikan trace utuh
lintas proses engine ↔ sidecar sebuah kewajiban interoperabilitas, bukan
opsi implementasi — SDK sidecar konform **wajib** meneruskan header ini ke
span-nya sendiri. Export trace (endpoint OTLP, sampling rate) adalah
konfigurasi operator, bukan bagian kontrak.

## 5. Kosakata Health (Machine-Readable)
Engine **wajib** meng-expose `GET /health` yang mengembalikan status
machine-readable memakai kosakata tunggal yang sama dengan
[`../../architecture/05-failover.md`](../../architecture/05-failover.md) §7:

| Status | Arti | Konsekuensi |
|---|---|---|
| `healthy` | Semua dependency terjangkau, dalam ambang | Terima traffic |
| `degraded` | Masih melayani, tapi ada dependency menurun (lihat `reasons`) | Tetap terima traffic; sinyal ke ops |
| `unhealthy` | Tidak bisa melayani dengan benar | Keluar dari endpoint Service |

Response **wajib** berbentuk `{ "status": "...", "reasons": [...],
"checked_at": "..." }`. `reasons[]` memakai kode terkontrol, minimal:
`snapshot_stale` (umur snapshot ≥ ambang policy, [`05-plane-protocol.md`](05-plane-protocol.md)
§4), `datastore_unreachable`, `db_pool_exhausted`, `outbox_backlog`,
`control_plane_unreachable`. `status: dead` **tidak** self-reported — itu
turunan observasi Control Plane (3× missed heartbeat) dan tetap didefinisikan
di failover §7, bukan di sini.

Endpoint yang sama melayani liveness dan readiness probe K8s
([`../../architecture/05-failover.md`](../../architecture/05-failover.md) §2):
liveness lulus selama proses hidup dan event-loop responsif; readiness lulus
hanya saat `status ∈ {healthy, degraded}`. Ringkasan health ini adalah isi
evidence `health` yang dikirim ke Control Plane
([`05-plane-protocol.md`](05-plane-protocol.md) §4.4) — **hitungan/status
saja, tidak pernah data bisnis**.

## 6. Alerting — Stance
Forma **tidak** membangun alerting engine sendiri. Kontrak ini berhenti pada
**mengekspos** metrics (§3) dan health (§5) dalam format standar; **rule
alerting, threshold, routing, dan on-call adalah tanggung jawab stack
operator** (Prometheus Alertmanager, Grafana, dsb). Alasan sama dengan
larangan write-back Plane Protocol: engine melaporkan keadaan, keputusan
(termasuk "ini layak dibangunkan tengah malam") ada di lapisan governance/ops
operator. forma/ops (aplikasi Forma first-party di atas Control Plane, bukan
bagian engine) **boleh** membangun surface alerting di atas kosakata health
ini — tetapi itu aplikasi, bukan kewajiban engine.

## 7. `forma logs`
Verb CLI untuk membaca stream log terstruktur (§2) dari engine — tail dan
filter tanpa harus menyaring JSON manual. Normatif untuk perilaku;
implementasi CLI mengikuti dokumen ini.

```bash
forma logs --workspace corp-456 --follow            # tail live
forma logs --module billing --entity invoice        # filter per module/entity
forma logs --level error --since 1h                 # hanya error, jendela waktu
forma logs --request-id req-abc123                   # satu request, lintas komponen
```

| Flag | Fungsi |
|---|---|
| `--follow` / `-f` | Tail berkelanjutan |
| `--workspace` | Filter workspace |
| `--module` / `--entity` / `--action` | Filter dimensi eksekusi |
| `--level` | Level minimum (`debug`/`info`/`warn`/`error`) |
| `--request-id` | Ambil semua record satu request (korelasi §2.3) |
| `--since` / `--until` | Jendela waktu |
| `--output` | `pretty` (default TTY) \| `json` (raw JSON lines) |

`forma logs` **tidak pernah** menembus disiplin PII (§2.2): kalau `debug`
tidak diaktifkan operator, nilai bisnis tetap tidak ada di stream, dan
`forma logs` tidak bisa memunculkannya. Verb ini **wajib** ditambahkan ke
referensi CLI ([`../../cli-tools/02-forma-cli.md`](../../cli-tools/02-forma-cli.md)).

## 8. Kode Error
| Kode | Kondisi |
|---|---|
| `OBSERVABILITY_METRICS_DISABLED` | Endpoint `/metrics` diminta tapi dimatikan konfigurasi |
| `OBSERVABILITY_DEBUG_FORBIDDEN` | Aktivasi log `debug` dicoba di `prod` tanpa otorisasi operator |
| `LOGS_FILTER_INVALID` | Kombinasi filter `forma logs` tidak valid |
