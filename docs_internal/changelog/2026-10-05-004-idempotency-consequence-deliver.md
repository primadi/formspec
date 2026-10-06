# 2026-10-05-004 — Idempotency consequence `deliver: target` ditegakkan (7.7.5)

Kontraknya sudah menuntut ini sejak awal — `01-core-basic.md` §7: _"Outbox
(normatif) … poll pending → **cek idempotency** → sync call ke target action →
delivered, atau backoff retry → dead-letter"_. Yang belum ada
implementasinya: `deliver[].idempotency_key` hanya dibaca jalur HTTP
(`resolveIdempotencyKey`, yang membaca request), sedangkan outbox worker tidak
punya request.

## Akibatnya terukur, bukan teoretis

Me-requeue satu event `journal-posted` yang sama — persis yang dilakukan retry —
menjalankan `gl-balance.update` dua kali dan mengakumulasi pergerakan saldo dua
kali: `debit_movement` 143750 → **287500**. Target-nya sudah
`idempotent: true` (7.7.3 mewajibkan itu), tetapi flag itu **tidak pernah
dibaca** di jalur consequence.

## Perbaikan

- **Entry utuh diteruskan ke dispatcher.** `db.ActionDispatch` menerima
  `spec.EventDeliveryDecl` (bukan hanya `*spec.DeliveryTarget`): entry itulah
  yang memuat `idempotency_key`. Sebelumnya key-nya dibuang di
  `DeliveryEventHandler`.
- **`runTargetOnce` menjaga panggilan target** (`resource/formspec.go`):
  - tanpa `idempotency_key` → jalan seperti semula (idempotensi alami target);
  - key **completed** → consequence **dilewati** (replay = tidak melakukan
    apa-apa; ini yang menutup kasus terukur);
  - key **pending/failed** → dijalankan, karena delivery at-least-once dan
    percobaan gagal harus bisa di-retry;
  - key dideklarasikan tetapi store tidak ter-wire → **gagal**, bukan jalan
    tanpa perlindungan;
  - template tidak bisa di-resolve → **gagal**, bukan jadi literal (literal
    `balance.{id}` memberi semua event tanpa `id` kunci yang SAMA, sehingga
    delivery kedua dilewati sebagai "sudah selesai" — konsekuensi hilang).
- **Scope key** `deliver:{resource}.{action}`, di tabel `formspec_idempotency_keys`
  yang sama dengan kunci HTTP, sehingga keduanya tidak bisa bertabrakan.
- **Store di-wire lebih awal** di `resource.New()` dan diteruskan juga di jalur
  reload (`a.idempotency`), jadi key yang diklaim sebelum reload tetap
  menekan retry sesudahnya.

## Batas yang dinyatakan (bukan disembunyikan)

Kunci diklaim (`pending`) **sebelum** target dipanggil dan ditandai `completed`
sesudahnya. Crash tepat di antara tulisan target dan penandaan menyisakan key
`pending` → retry mengklaim ulang dan menjalankan sekali lagi. Jendela itu hanya
bisa ditutup idempotensi alami target (kunci unik per `(source_id, account_id,
period)`), bukan oleh store ini. Ditulis di `01-core-basic.md` §7.

## Bukti

- `TestRunTargetOnce_RetryAfterSuccessDoesNotReRun` — **gagal sebelum, hijau
  sesudah**: dengan skip dihapus, consequence jalan **3×**; dengan skip, **1×**.
- `TestRunTargetOnce_FailureIsRetryable`, `_KeyWithoutStoreFailsLoudly`,
  `_UnresolvedKeyTemplateFails`, `_ScopedPerTarget`, `_NoKeyDeclaredRunsEveryTime`,
  `TestResolveDeliveryKey_NestedPathAndValues` (`resource/deliver_idempotency_test.go`).
- `TestDeliveryEventHandler_ReliableEventPassesTheWholeEntry` +
  `_ReliableEventWithoutTargetSkipsDispatch` — plumbing: key benar-benar sampai.
- `go build ./...` · `go vet ./...` · `gofmt` bersih · `go test ./...` hijau ·
  `formspec validate`: kafe 88/0, crc 33/0, service-demo 13/0 (tidak berubah —
  perbaikan ini runtime, bukan kontrak manifest).
