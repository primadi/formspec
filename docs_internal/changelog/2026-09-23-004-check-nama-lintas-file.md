# 2026-09-23-004 — `formspec check` memeriksa NAMA lintas-file (10.12)

**Konteks.** Item 10.12: `formspec validate` menerima manifest yang menunjuk view,
entity, atau widget yang **tidak pernah dideklarasikan**. Setiap pemeriksaan
`validate` bersifat per-manifest, jadi nama yang tidak ada di mana pun tidak
pernah dibandingkan dengan apa pun — kegagalannya muncul saat runtime sebagai
route 404 atau placeholder, jauh setelah penulisan.

Terukur pada kafe sebelum perbaikannya: `formspec validate` **hijau** pada report
ber-`entity: ledger` (tidak pernah ada), Table yang menunjuk `journal_entry`
(manifest-nya `journal-entry`), dan dashboard yang menempatkan widget
`recent-journals` yang tidak pernah ditulis. Ketiganya sudah diperbaiki manual di
10.10; yang belum ada adalah pemeriksanya.

## Yang ditambahkan

`checkReferences` (`cmd/formspec/check.go`) — pemeriksaan **NAMA**, bukan field:

- **`spec.entity`** pada Table, Form, Listing, Kanban, Timeline, Calendar, Print,
  Report (menghormati `source.entity`), Widget, Wizard. Ref boleh
  `module.entity` atau telanjang (= modul manifest sendiri). Form dengan
  `auth_action` dikecualikan — memang tanpa entity.
- **Rujukan view** pada Page (`blocks` dan `tabs`): `form`, `table`, `component`,
  `widget`. Ref kosong atau ber-`asset` dianggap sah (bukan nama menggantung).
- **Rujukan widget** pada Dashboard (`widgets` **dan** `defaults` — `defaults`
  adalah ejaan lama, entri basi di sana adalah nama menggantung yang sama).

**Field entity sengaja TIDAK diperiksa**: sudah ditangani
`checkForms`/`checkKanban`/`checkWizard`/`checkAggregates`, dan jalur kolom
ber-titik seperti `customer.name` butuh penelusuran relasi yang tidak dilakukan
check ini.

## Satu false positive ditemukan & dihapus

Versi pertama hanya menerima ref telanjang sebagai nama modul-lokal, dan itu
**melaporkan dashboard yang bekerja sebagai rusak**: di `Clinic-UI-Showcase`,
dashboard modul `clinic` menempatkan `pharmacy-queue-count` yang dideklarasikan
modul `pharmacy` (ada komentarnya di manifest). Ref telanjang kini juga diterima
bila **modul mana pun** mendeklarasikan nama itu; ref yang **menyebut modul**
tetap ketat, karena di sana penulis sudah menamai modulnya.

Checker yang menuduh manifest benar sebagai rusak lebih buruk daripada tidak ada
checker — penulis akan belajar mengabaikannya. Jadi itu diperbaiki sebelum
di-commit, bukan dicatat sebagai keterbatasan.

## Hasil pada contoh yang dikirim

| contoh | sebelum | sesudah | selisih |
| --- | --- | --- | --- |
| kafe, cafe, crc-management, service-demo, storefront | 0 | 0 | — |
| arisan | 4 | 4 | 0 (semua dari `checkUses`, sudah ada) |
| Clinic-UI-Showcase | 4 | 4 | 0 (setelah false positive dihapus) |
| **Midtrans-Payment-Gateway** | 0 | **2** | **2 bug nyata** |

## Bug nyata yang ditemukan: contoh Midtrans

```
ERROR spec/pages/midtrans-config.yaml#0:
      Page "midtrans-config-page" block form references unknown form "midtrans-config-form"
ERROR spec/pages/midtrans-webhook-log.yaml#0:
      Page "midtrans-webhook-log" block table references unknown table "midtrans-webhook-table"
```

Diperiksa: contoh itu **tidak punya satupun manifest `Form` atau `Table`**
(kind yang ada hanya Config, Mockup, Module, Page×2, Service, Subscription,
Webhook) — jadi kedua Page merujuk sesuatu yang tidak pernah ada, dan keduanya
tidak bisa merender apa pun.

**Ini tidak bisa ditambal tanpa keputusan desain**: tidak ada kind UI yang bisa
mengedit `kind: Config` atau menampilkan log `kind: Webhook`.

- `FormSpec` hanya bisa mengikat `entity` atau `auth_action` — **tidak ada** yang
  mengikat Config.
- `WebhookSpec` tidak punya hook query/list (hanya `for`/`method`/`path`/`auth`/
  idempotency).

Jadi dicatat sebagai celah engine (**7.8.17 ⏸️**), bukan diperbaiki dengan
menambah manifest palsu yang hanya menyembunyikan errornya. Contoh itu tidak
dijalankan gate mana pun (tidak ada target Makefile yang menjalankan `check`
pada seluruh contoh), jadi tidak ada CI yang memerah — tetapi contoh yang
dikirim sekarang jujur tentang keadaannya.

## Test pengunci

`cmd/formspec/check_references_test.go`:

- `TestCheckReferences_DanglingNames` — ketiga bentuk yang dulu hijau
  (`entity`, ref table, ref widget) dilaporkan.
- `TestCheckReferences_NoFalsePositives` — widget lintas-modul nama telanjang dan
  Page ber-block Form yang sah **tidak** dilaporkan.
- `TestCheckReferences_CleanSpecIsSilent` — spec bersih (termasuk Form ber-
  `auth_action`) menghasilkan nol temuan.

Dibuktikan **gagal** saat lookup-nya dinonaktifkan (`missing finding "Report
\"trial-balance\" references unknown entity \"ledger\""`).

## Dampak

- `cmd/formspec/check.go` — `checkReferences` + `kindIndex` + `splitKindRef`,
  didaftarkan di pipeline setelah `checkUses`; header perintah diperbarui.
- Gate: `gofmt`/`go vet`/`go test ./...` bersih.

Referensi plan: `docs_internal/plan/todo.md` 7.8.16/7.8.17; item kafe `10.12`.
