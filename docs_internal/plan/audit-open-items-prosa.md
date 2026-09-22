# Audit: pekerjaan terbuka yang belum jadi item todo

**Tanggal:** 2026-09-22 · **Pemicu:** permintaan pemilik — "apakah ada open item yang masih berupa prosa dan belum dicatat sebagai open item todo?"

**Metode:** scan mekanis seluruh `docs_internal/plan/todo.md` (2033 baris):
setiap item `[x]` yang **teksnya sendiri** mengakui ada yang kurang
(`belum …`, `Sisa`, `deferred`, `ditunda`, `sebagian:`, `belum (gap)`),
lalu setiap klaim dicek: (a) apakah ada item bernomor yang melacaknya, dan
(b) apakah prasyarat yang disebut masih benar-benar belum ada.

## Ringkasan

| Kategori | Jumlah |
| --- | --- |
| Item `[x]` yang mengakui ada gap | **20** |
| Gap yang punya item terlacak (aman) | 8 |
| Gap **tanpa** item terlacak (temuan) | **12** |
| Gap yang redaksinya **menyesatkan** (prasyarat sudah landing) | **3** |

## A. Gap tanpa item terlacak — 11

Semuanya bentuk yang sama: flag/fitur disebut "belum (gap)" di dalam item `[x]`,
tidak ada item bernomor mana pun yang melacaknya. Di-grep dengan nama flag pun
0 hasil.

| # | Sumber | Gap | Bukti tidak terlacak |
| --- | --- | --- | --- |
| 1 | 3.7.1 (L1013) | `backup create --incremental` / `--filter` | grep `--incremental`, `--filter` → 0 item |
| 2 | 3.7.3 (L1015) | `restore --map-resource` / `remap` | grep `map-resource` → 0 item |
| 3 | 3.7.4 (L1016) | `logs --action/--level/--since/--until/--request-id/--follow` | grep `--follow`, `--request-id` → 0 item |
| 4 | 4.8.1 (L1085) | `--incremental` (sama dengan #1, jalur archive) | idem |
| 5 | 4.9.5 (L1097) | `archive restore-batch` | grep `restore-batch` → 0 item |
| 6 | 2.1.1 (L833) | SDK sidecar tidak mengirim `X-FormSpec-Scope-Id` | tidak ada item SDK-sidecar |
| 7 | 2.6.4 (L887) | module auto-suspend + incident audit (bagian `USES_VIOLATION`) | tidak ada item; stub middleware tetap dead code |
| 8 | 3.2.2 (L978) | SDK `dotnet` belum tersedia (auto-detect) | tidak ada item |
| 9 | 6.6.1 (L1349) | cookie auth untuk `/_ui` (API key saja) | grep `cookie` → 0 item |
| 10 | 5.10.14 (L1247) | opsi array (backend) untuk comma-separated field | teks hanya "Opsi array (backend) ditunda" |
| 11 | 13.5.6 (L1873) | cache-aside wiring registry + shared rate limiter antar-pod | grep `cache-aside`, `rate limiter` → 0 item |
| 12 | 2.1.4 (L836) | `core.idempotency_retention` → `IdempotencyTTL` (pemetaan key Config) | grep `idempotency_retention` di Go → 0 pembacaan, hanya komentar |

## B. Redaksi menyesatkan — 3 (prasyarat sudah landing)

Tiga item `[x]` menyatakan sesuatu "menunggu X" padahal X **sudah selesai**.
Pembaca yang menemui kalimatnya akan mengira masih terblokir.

| Lokasi | Kalimat | Kenyataan |
| --- | --- | --- |
| 2.9.1 (L903) | "Primitif lain + named datastore masih error jelas … **menunggu** 2.9.2–2.9.4" | 2.9.2 + 2.9.4 **landing**; `ResolveNamed` hidup (`resource/datastoreregistry.go:761`) dan ada test E2E (`ctx_db_module_scoped_e2e_test.go`) |
| 2.9.3 (L905) | "named datastore selain `'default'` → error jelas (**menunggu 2.9.4**)" | idem — sudah tersedia |
| 6.8.1 (L1365) | "populasi store **menunggu** Config runtime 7.2" | 7.2.1–7.2.4 landing 2026-08-25, dan `SetSecretsStore(cfgReg.Secrets())` memang sudah di-wire (`resource/formspec.go:1851`) |

**Dikoreksi saat audit ini:** 2.1.4 (L836) tadinya saya tandai menyesatkan juga,
ternyata **tidak**. Kalimatnya ("resolusi dari manifest `kind: Config`
(`core.idempotency_retention`) menunggu runtime Config-kind") masih **akurat**:
grep `idempotency_retention` di seluruh Go menemukan **0 pembacaan** — hanya
komentar. Yang landing adalah *runtime* Config (7.2), bukan *pemetaan* key
`core.idempotency_retention` ke `IdempotencyTTL`. Gag ini masuk kategori A
sebagai item baru, bukan koreksi redaksi.

## C. Bukti bahwa masalah ini bukan teoretis

Kasus terbaru (2026-09-21): item `[x] 15.12` memuat prosa sisa `deliver: target`
yang **sudah ditutup** oleh changelog `004`. Prosa lama tetap mengumumkan
`gl-balance` belum terisi padahal sudah jalan — misinformasi aktif, bukan
sekadar catatan basi. Ditutup dengan memindahkannya ke item `7.7.5 ⏸️` / `7.7.6 ⏸️`.

## Tindakan

1. **12 gap tanpa item → sudah dibuatkan item `[⏸️]` bernomor** (dengan bukti
   pengamatan + effort):

   | Item baru | Gap |
   | --- | --- |
   | 2.1.5 | SDK sidecar belum kirim `X-FormSpec-Scope-Id` |
   | 2.1.6 | `core.idempotency_retention` belum dipetakan ke `IdempotencyTTL` |
   | 2.6.5 | Module auto-suspend + incident audit pada `USES_VIOLATION` |
   | 3.2.6 | SDK `dotnet` belum tersedia |
   | 3.7.5 | `backup create --incremental` |
   | 3.7.6 | `backup create --filter` |
   | 3.7.7 | `restore --map-resource` / `remap` |
   | 3.7.8 | `logs` filter lanjutan |
   | 3.7.9 | `archive restore-batch` |
   | 5.10.16 | Opsi array (backend) untuk field comma-separated |
   | 6.6.5 | Auth via cookie untuk `/_ui` |
   | 13.5.7 | cache-aside registry + shared rate limiter |

   Item `[⏸️]` di todo naik dari **4 → 18**.

2. **3 kalimat menyesatkan dikoreksi** (2.9.1, 2.9.3, 6.8.1).
3. **Aturan** ditulis ke `AGENTS.md` §3, `ai_skills/formspec-app-workflow/SKILL.md`
   (Phase 4), dan `.github/agents/todo-runner.agent.md` (plus field baru di
   format laporannya), supaya tidak terulang.

## Temuan tambahan: ID duplikat (pra-eksisting, di luar cakupan)

Saat memverifikasi ID baru, ditemukan **dua blok bernomor sama** di Fase 5.2:
`5.2.21`, `5.2.22`, `5.2.23` masing-masing muncul **dua kali** (baris 1165–1167
dan 1168–1170). Efeknya sama kelasnya dengan masalah prosa: nomor item tidak lagi
menunjuk satu pekerjaan, jadi rujukan `→ 5.2.22` ambigu. **Tidak diperbaiki di
sini** (di luar permintaan, dan penomoran ulang menyentuh rujukan di dokumen
lain) — dicatat supaya tidak hilang.

Juga: `3.1.1` dan `5.13.1` muncul dua kali karena judul dan sub-item; perlu
diperiksa terpisah apakah itu duplikasi nyata atau turunan yang sah.
