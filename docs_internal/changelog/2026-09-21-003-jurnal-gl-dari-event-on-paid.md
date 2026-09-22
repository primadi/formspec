# Scenario 8 — jurnal GL otomatis dari event `on_paid` (dan 9 bug mesin yang muncul di jalurnya)

**Tanggal:** 2026-09-21 · **TODO:** kafe `gaps_found/TODO.md` skenario 8 ·
**Desain:** keputusan pemilik — order memancarkan event durable, GL mendengarkan
dan membangun jurnal; jurnal tidak seimbang = setting akun GL belum lengkap = tanggung jawab module `gl`, bukan module pemesanan

## Konteks

Skenario 8 ("order lunas → jurnal double-entry") sebelumnya ditandai ⛔ terhalang
item 6.3 (cross-app grant). Desain yang dipilih menggantinya: tidak ada integrator
dan tidak ada panggilan lintas-app — `cafe-order` hanya memancarkan event durable
`on_paid`/`on_cancel`, dan `gl` (yang memiliki jurnal, bagan akun, **dan setting
pemetaan akun**) mendengarkannya lewat `kind: Subscription` lalu membangun +
mem-posting jurnal. Jurnal yang tidak seimbang adalah sinyal setting akun GL
belum lengkap — errornya `FORMSPEC.GL.*`, bukan error module pemesanan.

Module `cafe-gl-integrator` dan app pendampingnya **dihapus**: keduanya akan
mem-posting ganda bersama subscription.

## Bug mesin yang ditemukan (semuanya diperbaiki, semuanya punya test)

Perjalanan skenario 8 membuka satu lapisan bug di bawah lapisan sebelumnya.
Masing-masing pernah menjadi satu-satunya alasan jurnal tidak terbentuk, jadi
urutannya adalah urutan perbaikan:

| # | Bug | Gejala di skenario 8 | Fix | Test |
| --- | --- | --- | --- | --- |
| 1 | `TransitionDecl.UnmarshalYAML` membuang `emit` | transisi `draft→paid` bertanda `emit: on_paid` tidak pernah memancarkan apa pun | field `Emit` ditambahkan ke struct raw | — |
| 2 | `resource.call` tidak bisa menargetkan satu record | `"gl.journal-entry.<uuid>"` dipecah sebagai module=`gl`, entity=`journal-entry.<uuid>` → *action not found* | `splitCallTarget` (module, entity, id); `callFn` menerima `id` | `TestSplitCallTarget`, `TestResourceAPI_Call_InstanceTarget` |
| 3 | `fail()` hanya **mengembalikan** nilai, tidak menghentikan script | guard `if bad: fail(...)` dilewati — script lanjut, record setengah jadi dibuat, lalu `ok()` dilaporkan | `fail()` mengembalikan `*FailError` (unwind stack); pesan penulis dilaporkan verbatim | `TestFailAbortsScript`, `TestFailAbortsFromNestedHelper` |
| 4 | `sum_line(field)` (bentuk yang didokumentasikan) tidak ada | guard keseimbangan jurnal gagal `undefined: sum_line` | builtin `sum_line` di env guard | `TestEvaluateGuard_SumLineBuiltin` |
| 5 | Env guard menolak `starlark.Value` dari env | builtin helper jadi string (`invalid call of non-function (string)`) | `toStarlark` meneruskan nilai yang sudah Starlark | (tercakup #4) |
| 6 | Guard hanya menerima `lines` bertipe `[]any` | child dari tabel (`[]map[string]any`) → sum = 0 → jurnal "tidak seimbang" padahal seimbang | `childRows` menormalkan kedua bentuk | `TestEvaluateGuard_TableStoredChildRows` |
| 7 | `event.payload` tidak pernah memuat `id` record | subscriber tidak tahu record mana yang berubah → `source_id` NULL, idempotensi mustahil | `withRecordID` menyuntik `id` di titik tulis outbox | `TestWithRecordID` |
| 8 | `ctx.config` dirutekan ke store KV kosong, **menutupi** nilai Config manifest | `gl_journal_account_kas` = None padahal manifest mendeklarasikan `"1-1000"` | `layeredConfigAPI`: override dulu, default manifest sebagai fallback | (verifikasi E2E) |
| 9 | `valuesEqual` memakai `==` pada slice → **panic** proses | outbox worker mati saat update jurnal (`comparing uncomparable type []map[string]interface {}`) | normalisasi slice + `comparableEqual` berbasis `reflect` | `TestValuesEqual_UncomparableShapes`, `TestComputeChanges_WithTableStoredChildren` |
| 10 | `ChildrenExtract` hanya menerima `[]any` | child tetap di JSONB parent, tabel child kosong | `asSlice` menormalkan kedua bentuk | (tercakup #9) |
| 11 | `emits` tidak pernah diresolusi di jalur `resource.call` | `journal-posted` tidak pernah terbit → proyeksi `gl-balance` tidak pernah terisi | `Dispatcher.EventEmitter` + `eventWiring` (tulis ke outbox) | (verifikasi E2E) |

Bug 3 adalah yang paling berbahaya kelasnya: `fail()` yang tidak menghentikan
script berarti **setiap guard bergaya `if bad: fail(...)` di seluruh ekosistem
adalah no-op** — script tetap berjalan dengan data yang belum lengkap dan
melaporkan sukses. Bug 9 mematikan proses, bukan sekadar gagal.

## Bukti E2E (SQLite, spec kafe)

| Tahap | Hasil |
| --- | --- |
| order lunas → event `on_paid` durable | outbox `('on_paid','completed',0)`, payload memuat `id` + money lengkap |
| dispatch subscription `gl/journalize` | berjalan **sebagai module `gl`** (own-module access bukan USES_VIOLATION) |
| idempotensi per `source_id` | tepat **1 jurnal** untuk `ORD-2026-00021` (retry outbox tidak menggandakan) |
| pembuatan jurnal | `JRN-2026-000055`, `source_id` terisi, `reference` = `ORD-2026-00021` |
| baris jurnal | 4 baris di tabel child (bukan JSONB parent) |
| keseimbangan | Kas debit 143750 = Omzet 125000 + Pajak 12500 + Service charge 6250 |
| posting otomatis | `status = posted` (guard `sum_line` lulus) |
| event `journal-posted` | outbox `('journal-posted','completed',0)` |

Setting akun GL yang tadinya kosong kini terisi dan **dinyatakan sebagai
tanggung jawab module `gl`**: `gl_journal_account_kas` `1-1000`,
`gl_journal_account_omzet` `4-1000`, `gl_journal_account_pajak` `2-2000`,
`gl_journal_account_service_charge` `2-1000`, `gl_journal_account_diskon`
`5-1000` — dengan akun pasangannya di bagan akun (`2-1000` Pendapatan Service
Charge, `5-1000` Diskon Penjualan).

## Sisa

> **Status 2026-09-21:** dua item pertama di bawah **sudah tertutup** oleh
> changelog `2026-09-21-004` — dibiarkan tertulis dengan penanda ini supaya
> tidak terbaca sebagai pekerjaan terbuka.

- ✅ **Tertutup oleh 004** — `deliver: target: { resource: gl.gl-balance, action: update }`
  (channel `reliable_event`) dulu hanya enqueue ke outbox, tidak memanggil action
  target. Kini sync call + retry, dan proyeksi saldo hidup.
- Item 6.3 (cross-app grant) tetap deferred, tetapi **skenario 8 tidak lagi
  bergantung padanya** — desain event-subscribe membuatnya tidak relevan.
