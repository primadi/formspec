# Plan — Intake Challenge (Proof-of-Work) untuk permukaan anonim

Status: **Draft → dikerjakan 2026-10-07**
Terkait: kafe 10.36 (intake anonim per-IP), `docs/spec/backend/02-core-extended.md` §17 (rate limit),
`examples/kafe/spec/modules/cafe-order/transaction/order/entity.yaml` baris `rate_limit: {max: 30, per: 1m, scope: ip}`.

## 1. Masalah

Intake anonim (tamu memesan dari QR tanpa login) hanya dibatasi
`rate_limit: {scope: ip}`. Dua kelemahan nyata:

1. **Bukan identitas kuat** — IP dinamis/CGNAT mudah berganti; satu penyebar
   bisa memakai ribuan IP.
2. **Berkas bersama (shared NAT).** Semua tamu di balik satu IP publik
   (CGNAT kantor, WiFi kafe, operator seluler) berbagi satu kuota 30/menit →
   saling memblokir.

Catatan implementasi yang memperburuk: `clientIP` membaca `RemoteAddr` saja
(`internal/api/auth_handler.go:37`), tidak menghormati `X-Forwarded-For`;
jadi di balik reverse proxy seluruh trafik bisa terlihat sebagai satu IP.
Dan key limiter menggabungkan `ip` + `actionName` (bukan entity).

Menambah `max` tidak menyelesaikan keduanya: ia tetap berbagi dan tetap
mudah dilewati.

## 2. Keputusan desain (disetujui pemilik proyek)

| Aspek     | Keputusan                                                              |
| --------- | ---------------------------------------------------------------------- |
| Deklarasi | **Keduanya** — `App` = on/off + policy global; `Action` = opt-in       |
| Pemicu    | **Eskalasi** — aktif hanya saat tekanan tinggi (bukan always-on)       |
| Cakupan   | **Hanya jalur anonim** (public grant); terautentikasi + API-key bebas  |
| Mekanisme | **PoW self-hosted**, stateless HMAC + TTL (tanpa storage/pihak ketiga) |

### 2.1 Deviasi terdokumentasi: gate di-key `(module, entity, action)`, bukan per-App

Phase 0 (verifikasi) membuktikan **App TIDAK teresolusi pada request anonim**:

- `AppFromContext` (`internal/api/handler.go:1913`) membaca `identity.App`;
  untuk anonim identity `nil` → kosong.
- `WithApp` hanya di-set di `AuthMiddleware` (`internal/api/middleware.go:205`)
  dari klaim JWT.
- `_ui/entity` **tidak** mengirim `?app=` (hanya `_meta`/oauth yang memakainya).
- `publicGrants()` (`internal/api/router.go:283`) sudah me-_merge_ seluruh
  App publik ke key `module/entity` — asosiasi App memang hilang by design.

Mengirim `?app=` dari klien **ditolak** sebagai dasar gate: klien bisa
menghilangkannya, jadi ia bukan batas keamanan (hanya boleh untuk tuning).

**Konsekuensi:** `App` tetap tempat DEKLARASI; kebijakan efektif dihitung saat
router build untuk key `module/entity` — pola yang sama dengan `publicGrants()`.
Merge **strictest-wins**: jangan pernah melemahkan proteksi karena dua App
berbagi route.

## 3. Bentuk YAML

```yaml
# examples/kafe/spec/apps/kafe-qr.yaml
spec:
  intake:
    challenge:
      provider: pow # only "pow"
      mode: escalate # escalate (default) | always
      activate_at: 0.7 # fraksi budget `global` yang menyalakan gate
      global: { max: 300, per: 1m } # sinyal tekanan se-aksi (bukan per-IP)
      difficulty: { min: 16, max: 22 } # leading-zero bits
      ttl: 90s
      bind: [ip] # subset: ip  (action selalu terikat)
```

```yaml
# examples/kafe/spec/modules/cafe-order/transaction/order/entity.yaml
actions:
  - name: create
    challenge: true # opt-in: action ini ikut gate saat tekanan tinggi
```

Aturan anti-divergensi (pelajaran repo: gate di dua tempat ditolak validator):

- **App** hanya menulis enable + policy/tuning.
- **Action** hanya menulis **boolean** opt-in — tanpa parameter yang bisa
  menyimpang dari App.
- Opt-in tanpa policy App ⇒ **error validasi** (deklarasi inert harus berisik).

## 4. Protokol PoW

**Tolak** (saat gate menyala): `403 CHALLENGE_REQUIRED` dengan body:

```json
{
  "error": {
    "code": "CHALLENGE_REQUIRED",
    "challenge": {
      "token": "<b64url(payload)>.<b64url(hmac)>",
      "difficulty": 18,
      "alg": "sha256",
      "ttl": 90
    }
  }
}
```

Challenge dibawa **di body penolakan** (bukan endpoint issue terpisah) →
lebih sedikit endpoint publik yang perlu dilindungi, dan klien solve tanpa
round-trip tambahan.

- `payload = {v,m,e,a,d,x,n}` — module, entity, action, difficulty, expiry
  unix, nonce acak. `bind: [ip]` menambahkan `i` (IP klien).
- Verifikasi (murah → mahal): parse → `hmac.Equal` (constant-time, pola
  `internal/webhook/verify.go`) → exp → binding → leading-zero-bits `sha256(token + nonce)` ≥ `d`.
- Klien mengirim hasil di header `X-Forma-Intake: <token>:<nonce>`.

**Replay (jujur, terdokumentasi):** stateless ⇒ satu solve berlaku sampai
`exp`. Jendela ditutup sebagian oleh TTL pendek (≈90s) + binding
(`action`, `ip`) + rate limit yang tetap berlaku. Single-use nonce
(cache gaya `wsticket.go`) **ditunda** — ukur dulu jendela nyatanya.

**Sinyal tekanan:** memakai limiter yang sudah ada. Tambah
`ResourceRateLimiter.Utilization(rs, key)`. Aktivasi memakai sinyal
**global se-aksi** (`intake.global`), bukan per-IP — sebab serangan
terdistribusi membuat tiap IP di bawah ambang. Difficulty naik
proporsional `min` → `max` mengikuti tekanan.

## 5. Penempatan gate (4 axis)

1. **Lifecycle HTTP (server): handler, bukan middleware global.**
   Middleware global (`BuildHTTP`) melihat static asset + `_meta/*`;
   menggating `_meta/apps` memutus render anonim (chicken-and-egg).
   Gate masuk ke jalur yang **sudah ada**: `checkRateLimitAction`
   (`internal/api/resource_ratelimit.go:186`), dipanggil dari setiap aksi
   (`handler.go`: list:452, find:690, create:833, update:1016, delete:1392,
   custom:2355, service:3133). Urutan: rate limit (murah) → challenge.
   Satu implementasi `checkIntakeAction`; dua wrapper lama jadi delegasi tipis
   (hindari dua implementasi yang drift).
2. **Deklarasi YAML:** `pkg/spec` (`Action.Challenge`, `AppSpec.Intake`).
3. **Sinyal tekanan:** `Utilization` + kebijakan efektif di-hitung saat
   router build.
4. **Klien (solve):** header via live getter di `authHooks.ts`; intercept
   `403 CHALLENGE_REQUIRED`; layar blocking meniru pola
   `pendingContext` → `SwitchContextScreen` di `App.tsx`; PoW di Web Worker
   sebagai aset bawaan framework (`shell/authAssets.ts`).

## 6. Cakupan & pengecualian

- Gate aktif hanya bila `isPublicGrantAuth(ctx)` **dan** identity `nil`.
- Exempt: `_meta/*`, `/_ui/assets/*`, `/_ui/auth/*`, `/_ui/_ws/*`.
- Service action `public: true` **tidak** di-gate — ia dipakai klien
  programatik (`curl`/SDK). Konsekuensinya: akses anonim programatik ke
  entity memang tak didukung by design (sudah begitu: grant anonim berasal
  dari surface browser).

## 7. Batasan yang diakui

- **Bukan anti-DDoS.** Ini pengurang L7 untuk permintaan yang dikarang.
  DDoS volumetrik (L3/L4) harus diserap edge/upstream.
- PoW **memindahkan** beban ke perangkat: botnet dengan JS nyata tetap bisa
  menyelesaikannya.
- **Multi-replica:** secret HMAC per-proses (acak saat build) ⇒ replica lain
  tak bisa memverifikasi challenge replica ini. Sama dengan caveat in-memory
  limiter yang sudah ada (`resource_ratelimit.go`); jalur bersama (Redis)
  adalah pekerjaan lapisan Control Plane.
- **Adaptasi perangkat (target waktu) BELUM diimplementasikan** → deferred
  `⏸️`; difficulty dipilih server dari tekanan, bukan dari kecepatan HP.

## 8. Fase & dependency

| Fase | Isi                                              | Effort | Dep   |
| ---- | ------------------------------------------------ | ------ | ----- |
| 0    | Verifikasi App pada request anonim               | small  | —     |
| 1    | `pkg/spec`: tipe + validasi                      | medium | 0     |
| 2    | Backend: HMAC issue/verify + `checkIntakeAction` | large  | 1     |
| 3    | `Utilization` + aktivasi eskalasi                | medium | 2     |
| 4    | Frontend: attach/intercept/layar/worker          | large  | 2     |
| 5    | Wiring kafe + changelog + todo                   | small  | semua |

## 9. Verifikasi

- Go unit: HMAC round-trip, expired, binding mismatch, difficulty, aktivasi
  ambang, exempt (`_meta`, terautentikasi, non-opt-in, Service public).
- `go build ./... && go test ./internal/api/ ./pkg/spec/`.
- `bin/formspec validate --spec examples/kafe/spec --schema schemas`
  (opt-in tanpa policy harus merah).
- Vitest: cabang hook challenge→solve→retry, state store, render layar.
- E2E kafe `activate_at: 0`: anon POST order → 403 → solve → 201; staf
  terautentikasi tak terpengaruh.

## 10. File yang tersentuh

- `pkg/spec/entity.go` — `Action.Challenge`
- `pkg/spec/resources.go` — `AppSpec.Intake`, `IntakeSpec`, `IntakeChallenge`,
  `PowDifficulty`, validasi di `ValidateAppSpec`
- `internal/api/intake.go` (baru) — kebijakan efektif, HMAC, `checkIntakeAction`
- `internal/api/resource_ratelimit.go` — `Utilization`, delegasi
- `internal/api/handler.go` — call site
- `internal/api/router.go` — hitung & suntik kebijakan intake
- `cmd/formspec/validate.go` (+ helper) — cross-check opt-in ↔ policy
- `renderers/react-shadcn/src/lib/api/authHooks.ts`, `stores/session.ts`,
  `src/App.tsx`, `src/shell/ChallengeScreen.tsx`, `src/lib/intake/pow.ts`
- `examples/kafe/spec/apps/kafe-qr.yaml`,
  `examples/kafe/spec/modules/cafe-order/transaction/order/entity.yaml`
