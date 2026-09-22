# Skenario 8 lanjutan — proyeksi saldo GL hidup, 5 bug di jalur consequence

**Tanggal:** 2026-09-21 · **TODO:** `docs_internal/plan/todo.md` 15.12 (sisa dicatat) ·
**Konteks:** changelog `2026-09-21-003` mencatat "`deliver: target` hanya enqueue,
proyeksi `gl-balance` belum terisi". Entry ini menutup sisa itu — dan menemukan
lima bug di jalur consequence-nya.

## Yang diperbaiki

**1. `deliver: channel: reliable_event` dengan `target` tidak pernah memanggil
action-nya.** Case-nya di `event_handler.go` hanya `break` dengan komentar "the
outbox entry *is* the guarantee" — padahal kontraknya normatif lain
(`docs_old/spec/02-core-basic.md` §12.2): *"poll pending → idempotency check →
**sync call** ke target action → delivered, atau backoff retry → dead-letter"*.
Sekarang `DeliveryEventHandler.Actions` memanggil target lewat dispatcher action
yang sama dengan `resource.call`; error dari target mengembalikan error sehingga
outbox worker meng-retry — itulah jalur retry-nya.

**2. `payload.fields: [id]` menghasilkan id null.** `id` adalah kolom tabel, bukan
field, jadi `data["id"]` selalu kosong — dan `projectFields` menyalin nilai kosong
itu apa adanya. Konsumen yang meng-address record lewat id mati:
`resource.fetch("gl.journal-entry", params.id)` → *"got NoneType, want string"*.
`ResolveEmission` sekarang menerima `recordID` terpisah; `projectFields` mengisi
`id` darinya (dan field yang tidak ada tetap hadir sebagai null — publisher sudah
menjanjikannya), `withRecordID` menyuntiknya saat tidak ada `payload.fields`.
Sebelas call-site di `internal/api/handler.go` + jalur `resource.call` diperbarui
dengan id yang relevan di scope masing-masing.

**3. `journal-reversed` menargetkan action yang tidak ada.** Manifest menunjuk
`gl.gl-balance.reverse`, sementara `gl-balance` hanya punya `update`. Akibatnya
outbox meng-retry sampai habis lalu dead-letter, dan proyeksi tidak pernah
ter-update — dengan `formspec validate` melaporkan **0 problem**. Kedua event
kini menargetkan `update`; arah pembalikan dibaca script dari
`journal.status == "reversed"`. Diperbaiki juga di `verticals/gl` dan
`verticals/reference-app`.

**4. `gl_balance_update.star` — saldo closing salah tanda untuk akun
kredit-normal.** Cabang insert memakai `debit - credit` mentah (mengabaikan arah
saldo normal), dan cabang update mengabaikannya juga. Omzet 125.000 tercatat
**−125.000**. Akun sekarang dibaca di luar kedua cabang, dan closing selalu
`opening ± movement` pada arah saldo normal akun. Hasil: Kas +143.750,
Omzet +125.000, Pajak +12.500, Service charge +6.250.

**5. `condition: resource.status == 'posted'` gagal dievaluasi.**
`evaluateCondition` menyuntik map mentah sebagai `resource`, dan map Go tidak
punya atribut → *"dict has no .status field or method"*; action `reverse` tidak
pernah bisa jalan. Guard state machine sudah memakai `FieldMap` justru untuk ini;
conditions sekarang selaras (`resource`/`data` = `FieldMap`), jadi dot-notation
dan bracket-notation dua-duanya bekerja.

## Validasi baru (menutup kelas bug #3)

`cmd/formspec/validate_events.go` — layer cross-manifest yang memeriksa setiap
`deliver: channel: reliable_event` target: action-nya harus ada (entity **atau**
service), dan harus `idempotent: true` karena outbox meng-retry-nya (aturan yang
sama dengan 7.7.3 untuk Integrator). Target di luar manifest set di-skip, bukan
ditolak. Dibuktikan: mengembalikan `action: reverse` ke manifest membuat
validator menolak dengan pesan yang menyebut action yang hilang.

Sebelum check ini: `formspec validate` hijau untuk manifest yang consequence-nya
dijamin tidak akan pernah terjadi.

## Bukti E2E (SQLite, spec kafe)

| Tahap | Hasil |
| --- | --- |
| `on_paid` → jurnal | 1 jurnal `posted`, idempoten, 4 baris seimbang |
| `journal-posted` → `gl-balance.update` | **4 baris proyeksi terisi** (sebelumnya 0) |
| saldo setelah POST | Kas 143750, Omzet 125000, Pajak 12500, Service charge 6250 — semua positif di sisi normalnya |
| `POST /journal-entry/{id}/reverse` | 200 → `journal-reversed` terbit → proyeksi kembali **0** di keempat akun |
| outbox | ketiganya `completed`, **0 deliver failure** |

`go test ./...` hijau · `make lint` 0 issues · `formspec validate` 78 manifest 0
problem · tidak ada regresi di verticals/examples lain (dibandingkan dengan
binary pra-perubahan: jumlah problem identik).

## Sisa

> Dua-duanya kini **item terlacak** di `docs_internal/plan/todo.md` — bukan prosa
> di bawah item `[x]`, supaya bisa di-grep sebagai pekerjaan terbuka.

- **7.7.5 ⏸️ — Idempotency retry belum ditegakkan di jalur consequence.**
  `deliver.target` punya `idempotency_key: "balance.{id}"`, tetapi enforcement-nya
  hanya hidup di jalur HTTP (`resolveIdempotencyKey` membaca request). **Terbukti
  bukan teoretis (2026-09-21, kafe/SQLite):** me-requeue event `journal-posted`
  yang sama menjalankan `gl-balance.update` dua kali → `debit_movement`
  143750 → **287500**. Retry outbox yang mengulang target yang sudah menulis
  sebagian mengorupsi proyeksi, bukan memulihkannya.
- **7.7.6 ⏸️ — `webhook`/`notification` channel belum ada.** Enumerasi kode:
  tidak ada `case "webhook"`/`case "notification"` mana pun; keduanya jatuh ke
  `default:` dan hanya menghasilkan warning. Klaim `todo.md` 7.3.5 bahwa
  keduanya selesai **dikoreksi**.

> Catatan koreksi diri: saat pertama kali menjawab pertanyaan pemilik soal
> changelog, saya menduga `journal_reverse.star` tanpa guard status membuat
> jurnal yang sudah `reversed` bisa dibalik dua kali. **Dugaan itu salah** —
> `action.condition` (`resource.status == 'posted'`) dievaluasi di jalur custom
> action (bukti: reverse kedua → `422 CONDITION_FAILED`), dan guard state machine
> menolak `reversed → reversed`. Yang benar-benar terbuka adalah 7.7.5 di atas.
