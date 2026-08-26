# 2026-08-26-002 — Fix freeze login + pindah auth ke /\_ui/auth

## Apa yang diubah

**1. Fix freeze (deadlock) di test `TestAuthAuthz_E2E`.** `validatePeriodGuard`
(period closing, 7.11) dipanggil **di dalam transaksi Insert** (`runTx`) dan
membaca entity store lain (`formspec.core.period-closing`) lewat koneksi
terpisah → deadlock pada SQLite single-connection. Dipindah ke **sebelum**
transaksi dibuka (Insert), sehingga guard baca lewat koneksi normal.

**2. Fix login gagal "invalid credentials".** `period.Guard.IsClosed`
memperlakukan `ErrNotFound` (tidak ada record period-closing) sebagai **error**,
padahal seharusnya "tidak ada record = periode terbuka". Karena entity
`formspec.core.session` adalah `characteristic: transaction` + punya
`transaction_date`, setiap insert session memanggil guard → error "not found" →
login gagal. Kini `IsClosed` mengembalikan `false, nil` saat record tidak ada.

**3. Pindah auth ke `/_ui/auth/*`.** Login/refresh kini di-mount di
`/{ws}/_ui/auth/login` + `/{ws}/_ui/auth/refresh` — surface UI yang **selalu
tersedia** (login adalah kebutuhan UI, session-based). `/api/v1/auth/*` dihapus
dari default; hanya di-mount jika `EnableAPIAuth` (Config / env
`FORMSPEC_ENABLE_API_AUTH`) — konsisten dengan prinsip "api/v1 = external yang
di-expose, deny-by-default" (`01-core-basic.md` §8.2).

**4. Frontend** — `loginWithPassword` (`lib/api/auth.ts`) dan `refreshSession`
(`stores/session.ts`) kini memanggil `/{ws}/_ui/auth/login` + `/_ui/auth/refresh`.

## Kenapa

- Freeze: guard period baca store lain di dalam transaksi → deadlock SQLite.
- Login gagal: guard salah memperlakukan "tidak ada record" sebagai error.
- Pindah auth: login adalah concern UI (harus selalu ada, session-based),
  bukan external API (deny-by-default, API key).

## File terdampak

- `renderers/jsonb-persist/crud.go` — `validatePeriodGuard` sebelum transaksi (Insert)
- `internal/period/calendar.go` — `IsClosed` treat `ErrNotFound` sebagai open
- `internal/api/router.go` — `/_ui/auth/*` selalu; `/api/v1/auth/*` opt-in (`EnableAPIAuth`)
- `internal/api/auth_handler.go` — doc comment
- `resource/formspec.go` — `Config.EnableAPIAuth` + wiring (boot + reload)
- `renderers/react-shadcn/src/lib/api/auth.ts`, `stores/session.ts` — jalur `/_ui/auth/*`
- `internal/api/auth_handler_test.go`, `auth_pipeline_test.go`, `resource/auth_e2e_test.go` — path test
- `renderers/jsonb-persist/period_test.go` — test period guard

## Referensi

- Todo: `docs/plan/todo.md` §7.11.5 (period guard), §6.1 (auth)
- Spec: `docs/spec/backend/01-core-basic.md` §8 (surface), `02-core-extended.md` §9.3 (period closing)
