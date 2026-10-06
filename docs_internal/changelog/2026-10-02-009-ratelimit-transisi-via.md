# 2026-10-02-009 — `rate_limit` pada transisi `via` akhirnya ditegakkan (kafe 10.60a)

**Apa yang diubah.** `HandleCustomAction` (`internal/api/handler.go`) kini
memeriksa rate limit lewat spec yang **sudah di-resolve** union, bukan
resolve ulang:

- `checkRateLimitAction(w, r, es, action, actionName)` baru di
  `internal/api/resource_ratelimit.go`; `checkRateLimit` lama menjadi
  pembungkusnya (`action = nil`) sehingga seluruh pemanggil lama **tidak
  berubah perilaku** — nilainya jatuh ke `resolveAction` lalu ke default
  resource, persis seperti sebelumnya.
- `rateLimitForAction(...)` baru di `handler.go` meneruskan `actionSpec` yang
  dipegang handler ke pemeriksaan itu; `HandleCustomAction` memakainya.
- `resolveAction` **tidak disentuh** — ia dipakai jalur `create`/`update`, dan
  tabrakan `via: update/delete` sengaja belum diputuskan
  (plan §Further Considerations #1).

**Kenapa.** Kafe 10.60a, ditemukan saat audit 10.60. `HandleCustomAction`
menerima spec dari registry union (`GetActionSpec` = `actions:` ∪ `via`), tetapi
pemeriksaan rate limit me-resolve ulang lewat `resolveAction` yang membaca
`es.Actions` saja. Sejak L4 menulis ulang `via` di `actions:` **ditolak**, jadi
via-only adalah bentuk yang diwajibkan — akibatnya `rate_limit` yang
dideklarasikan pada transisi **diam-diam tidak pernah berlaku**; hanya default
level resource yang jalan. Ini kelas yang sama dengan 10.59/10.60: kontrak
dideklarasikan, tidak ditegakkan.

**Dampak.** `internal/api/handler.go` · `internal/api/resource_ratelimit.go` ·
test baru `internal/api/ratelimit_transition_via_test.go`.

**Bukti (terukur, lewat handler nyata).** `TestRateLimit_TransitionViaEnforced
ThroughHandler`: entity dengan `rate_limit {max: 1, per: 1s}` **hanya** pada
transisi `via: confirm` (tanpa entri `actions:`), lalu route action dipanggil
dua kali melalui `HandleCustomAction` yang asli:

| Wiring handler                     | Panggilan ke-2            |
| ---------------------------------- | ------------------------- |
| `rateLimitFor` (regresi / sebelum) | **200** — limit diabaikan |
| `rateLimitForAction` (perbaikan)   | **429 `RATE_LIMITED`**    |

Guard-nya **terkalibrasi**: dengan wiring dikembalikan ke `rateLimitFor`, test
gagal `want 429, got 200`. Versi pertama test ini memanggil `checkRateLimitAction`
langsung dan tetap hijau saat wiring di-revert — **guard yang tidak menjaga**;
itu diperbaiki dengan menjalankan handler sungguhan, dan itulah bentuk yang
dipertahankan.

`go build ./...` hijau · `go test ./...` hijau (tak ada `FAIL`) ·
`formspec validate --spec examples/kafe/spec --schema schemas` → 89 manifest,
0 problem · `formspec check -f examples/kafe/spec` → 0 error, 0 warning.

Rujukan: kafe 10.60a, plan `docs_internal/plan/via-sebagai-action-penuh.md` L9.
