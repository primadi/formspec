# 2026-10-07-006 — Intake challenge (proof-of-work) untuk permukaan anonim

Plan: `docs_internal/plan/intake-challenge-pow.md`. Todo: item 7.12 lanjutan
(`docs_internal/plan/todo.md`), kafe `gaps_found/TODO.md`.

## Apa yang diubah

Intake anonim (tamu memesan dari QR tanpa login) sebelumnya hanya dibatasi
`rate_limit: {max: 30, per: 1m, scope: ip}` — key yang sekaligus terlalu lemah
(IP bukan identitas) dan terlalu tumpul (semua tamu di balik satu NAT berbagi
satu kuota, dan saling memblokir). Ditambahkan **lapisan biaya kedua yang tidak
dibagi**: tamu harus memecahkan puzzle proof-of-work sebelum action yang
memintanya berjalan.

Deklarasinya dua tingkat, dengan aturan anti-divergensi yang tegas:

- `App.spec.intake.challenge` — milik App (unit yang di-review), berisi policy:
  `mode` (`escalate` default | `always`), `activate_at`, `global {max, per}`
  (sinyal tekanan), `difficulty {min, max}` (leading zero bit), `ttl`, `bind`.
- `Action.challenge: true` — **boolean** opt-in. Tuning tidak boleh ditulis di
  sini; menuliskannya di dua tempat membuat keduanya bisa menyimpang.

## Kenapa gate-nya di-key `(module, entity, action)`, bukan per-App

Phase 0 membuktikan App **tidak teresolusi pada request anonim**:
`AppFromContext` membaca `identity.App` (kosong tanpa identity), `_ui/entity`
tidak mengirim `?app=`, dan `publicGrants()` sudah me-merge semua App publik ke
key `module/entity`. Karena itu App tetap tempat DEKLARASI, sedangkan kebijakan
efektif dihitung sekali saat router build (`intakePolicies`, pola sama
`publicGrants`) — digabung **strictest-wins** supaya dua App yang berbagi route
tidak saling melemahkan. Mengirim `?app=` dari klien ditolak sebagai dasar gate:
klien bisa menghapusnya.

## Di mana gate diletakkan

Bukan middleware global (itu melihat static asset dan `_meta/*`; menggating
`_meta/apps` memutus render anonim). Gate masuk ke jalur yang sudah ada:
`intakeGateFor`/`intakeGateForAction` → `checkIntakeAction`
(`internal/api/intake.go`) = rate limit dulu (murah, menolak flood sebelum
challenge dibuat), lalu challenge. Satu implementasi; `checkRateLimit`/
`checkRateLimitAction` tetap sebagai primitif di dalamnya, dan seluruh call site
CRUD/custom/service dialihkan ke gate baru.

Cakupan: **hanya jalur anonim** (`isPublicGrantAuth` + identity nil).
Terautentikasi, klien ber-API-key, Service action `public: true` (dipakai klien
programatik), `_meta/*`, aset, dan `_auth/*` tidak pernah di-gate.

## Protokol

- Tolak: **403 `CHALLENGE_REQUIRED`** dengan challenge **di dalam error
  envelope** (`error.challenge {token, difficulty, alg, ttl_seconds}`) — bukan
  endpoint issue terpisah, jadi tidak ada endpoint publik tambahan yang perlu
  dilindungi dan klien bisa solve tanpa round-trip.
- Challenge stateless: `base64url(payload).base64url(HMAC-SHA256(payload))`,
  `payload = {module, entity, action, difficulty, expiry, nonce[, ip]}`.
  Verifikasi murah→mahal: bentuk → `hmac.Equal` → exp → binding → leading-zero
  bits `sha256(token + ":" + nonce)` ≥ difficulty.
- Klien mengirim `X-Forma-Intake: <token>:<solution>` dan mencoba ulang **sekali**.
- Tekanan: `ResourceRateLimiter.Observe` (bucket yang sama, satu implementasi)
  menghitung utilisasi **global se-aksi**. Sinyal per-IP sengaja tidak dipakai:
  serangan terdistribusi membuat tiap IP di bawah ambangnya sendiri, sehingga
  per-IP justru melaporkan "aman" tepat saat serangan berlangsung.

## Batasan yang diakui (bukan klaim berlebih)

- **Bukan anti-DDoS.** Ini pengurang L7 untuk permintaan yang dikarang; flood
  volumetrik tidak menjalankan JS dan harus diserap edge.
- Botnet yang benar-benar menjalankan JS tetap bisa menyelesaikan puzzle.
- **Replay:** stateless ⇒ satu solusi tetap sah sampai `exp`. Jendela diperkecil
  (TTL 90s + binding action+ip + rate limit di bawahnya), tidak ditutup. Nonce
  single-use ditunda → item `⏸️` (butuh store, yang menjadi target DoS baru).
- **Multi-replica:** secret HMAC per-proses ⇒ replica lain menolak challenge
  replica ini (klien solve ulang). Sama dengan caveat in-memory limiter dan
  `wsTicketStore` yang sudah ada.
- **Adaptasi difficulty per perangkat (target waktu) belum ada** → item `⏸️`.

## Bug yang ditemukan verifikasi HTTP — dan perbaikannya

Test unit memanggil `checkChallenge` dengan konteks yang dibangun sendiri, jadi
ia **lolos** sementara gate sebenarnya tidak pernah menyala untuk tamu anonim.
Test HTTP di atas router nyata (`intake_http_test.go`) menangkap dua kesalahan
nyata:

1. **Gate mensyaratkan `isPublicGrantAuth`, padahal `RequirePermissionOrAnonymous`
   TIDAK menandai pemanggil anonim** — ia `return` lebih awal untuk identity
   `nil` dan hanya menandai cabang _signed-in tanpa permission_. Jadi penjaga
   itu mematikan gate tepat untuk kelas pemanggil yang menjadi sasarannya.
   Perbaikan: gate tidak lagi membutuhkan marker; keberadaan policy untuk
   `(module, entity, action)` sudah menyatakan action itu terbuka anonim (map
   dibangun dari derived public grants), dan identity non-nil tetap dilewati.
   **Terukur:** anonim `POST /kafe/_ui/entity/cafe-order/order` sebelum fix →
   tidak pernah `CHALLENGE_REQUIRED`; sesudah fix → `403 CHALLENGE_REQUIRED`
   berisi token, dan solusi yang benar menembus ke handler.
2. **Skill sukses tanpa challenge di mode dev karena tidak ada validator** —
   harness test tidak memasang token validator, sehingga `AuthMiddleware` tidak
   pernah menetapkan identity. Perbaikan: harness memasang `JWTValidator` nyata
   (seperti `resource/formspec.go` di dev maupun prod) dan menerbitkan token
   ber-workspace kafe, supaya kasus "staf tidak pernah di-challenge" benar-benar
   diuji, bukan lolos karena token ditolak.

Guard-nya kini ada di dua lapis: unit (bentuk request anonim diambil apa adanya
dari perilaku router, bukan marker buatan) dan HTTP (rantai middleware nyata).
Keduanya dikalibrasi — mengembalikan syarat `isPublicGrantAuth` membuat test
HTTP merah.

## File

- `pkg/spec/entity.go` — `Action.Challenge`
- `pkg/spec/resources.go` — `IntakeSpec`, `IntakeChallenge`, `PowDifficulty`,
  `validateIntake` (mode escalate tanpa `global` ditolak; `max` ≤ 32)
- `internal/api/intake.go` (baru) — policy efektif, HMAC lipat, gate
- `internal/api/resource_ratelimit.go` — `Observe` + refactor bucket bersama
- `internal/api/handler.go` — `intakeGateFor*`, `ErrorDetail.Challenge`, setter
- `internal/api/router.go` — `intakePolicies()` + `SetIntakePolicies` di BuildHTTP
- `cmd/formspec/validate_intake.go` (baru) — opt-in tanpa policy = error
- `renderers/react-shadcn/src/lib/intake/pow.ts` (baru), `stores/intake.ts`
  (baru), `shell/ChallengeScreen.tsx` (baru), `lib/api/authHooks.ts`,
  `lib/api/client.ts`, `stores/session.ts`, `App.tsx`, `types/manifest.ts`
- `examples/kafe/spec/apps/kafe-qr.yaml`, `.../cafe-order/order/entity.yaml`

## Verifikasi

- `go test ./internal/api/ ./cmd/formspec/ ./pkg/spec/` — round-trip HMAC,
  expired/binding/difficulty/replay-shape ditolak, eskalasi menyala lalu solusi
  diterima, gates kafe teresolusi dari manifest nyata, opt-in tanpa policy ditolak.
- `formspec validate --spec examples/kafe/spec --no-schema` → 88/0.
- `npx tsc -b` bersih; vitest: kontrak wire challenge + state `solving`.
