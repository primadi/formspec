# Plan — Penegakan scope permukaan publik (`scope.enforced` + `create_scope` + `via`)

**Status:** **SELESAI fase A–E** (2026-10-07). Changelog `2026-10-07-001` (A–B)
dan `2026-10-07-002` (C–E). Sisa tercatat sebagai item bernomor (10.80, 6.5.11).
**Konteks:** tindak lanjut pertanyaan "untuk page tanpa login (kafe-qr), kode cabang
harusnya ditentukan admin?" — jawabannya **tidak**: cabang sudah datang dari rantai
`qr_token → dining-table.branch_id → table-session.branch_id`. Yang kurang adalah
**penegakannya** di dua jalur (baca harga & tulis pesanan).

## Temuan yang memicu plan ini

1. **kafe 10.76 ⏸️** — grant anonim turunan kehilangan scope `branch_id` pada
   `cafe-master.menu-item-price`, sehingga anonim membaca harga **semua** cabang.
   Sebabnya: `picker.display.price_filter` = `{branch_id: "{session.branch_id}"}`,
   dan `price_filter` **hanya filter klien** (`PickerPanel.tsx` menyuntiknya ke query
   list); `from: session` ditolak untuk permukaan publik.
2. **Create anonim tanpa write scope** (temuan baru, belum terlacak) —
   `HandleCreate` (`internal/api/handler.go:825`) memanggil `store.Insert` **tanpa**
   row predicate (`db.InsertParams` tidak punya field itu). `denyForbiddenFieldWrites`
   hanya menjaga field ber-`required_permission`; `order.branch_id` tidak punya,
   jadi anonim dapat mengirim `branch_id` cabang lain. State machine menahan
   `status` (harus `initial`), bukan cabang.
3. **Harga baris pesanan tidak divalidasi server** — `order.lines[].unit_price_snapshot`
   diterima apa adanya dari klien, dan `line_total` adalah `computed`. Artinya
   menutup 10.76 saja belum menutup nilai pesanan yang dikarang.

## Model yang ditetapkan

Cabang **tidak** dipilih admin untuk permukaan tamu. Ia **tersirat** oleh token
perangkat fisik (stiker QR) dan ditegakkan server-side:

| Arah  | Deklarasi                                                                                                                            | Perilaku                                                                                                         |
| ----- | ------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------- |
| Baca  | `row_scope: [{field: branch_id, from: route, param: session_id, via: cafe-order.table-session, via_field: branch_id}]`               | nilai dimensi diambil dari record yang di-resolve `param`; tidak bisa dilebarkan klien                           |
| Tulis | `create_scope: [{field: branch_id, from: record, ref_field: table_session_id, via: cafe-order.table-session, via_field: branch_id}]` | nilai dimensi diambil dari record yang **dirujuk payload**; kiriman klien yang berbeda **ditolak** (fail closed) |

Aturan:

- `via` = **1 hop** (route payload → record → field). `via` pada sumber selain
  `route`/`record` ditolak validator.
- Mismatch pada `create_scope` ⇒ **403**, bukan ditimpa diam-diam.
- Sesi **tidak** diubah: satu sesi tetap satu `(role, dimension, value)`.
- `scope.enforced` menyatakan siapa yang menegakkan dimensi, sehingga "lupa
  menulis penegakan" menjadi terlihat:

  | nilai                     | arti                                                      | validator                                           |
  | ------------------------- | --------------------------------------------------------- | --------------------------------------------------- |
  | `session` (default/absen) | ditulis `row_scope from: session`                         | wajib ada `row_scope` pada field itu dari `session` |
  | `route`                   | token di route                                            | wajib ada `row_scope from: route`                   |
  | `none`                    | memang lintas cabang (mis. promo global)                  | pengecualian eksplisit                              |
  | `external`                | ditegakkan di luar entity (grant publik + `create_scope`) | pengecualian; lihat butir ⏸️                        |

## Fase

| #   | Isi                                                                                 | Ukuran | Status                                                                       |
| --- | ----------------------------------------------------------------------------------- | ------ | ---------------------------------------------------------------------------- |
| A   | `ScopeDecl.Enforced` + validator `ValidateScopeEnforcement` + migrasi 4 entity kafe | small  | ✅ 2026-10-07 (`2026-10-07-001`)                                             |
| B   | `create_scope` (spec + validator) + penegakan di `HandleCreate` + adopsi kafe       | medium | ✅ 2026-10-07 (`2026-10-07-001`)                                             |
| C   | `via` untuk `from: route` pada `row_scope` (menutup 10.76)                          | medium | ✅ 2026-10-07 (`2026-10-07-002`, nama konstruk dikoreksi `2026-10-07-003`)   |
| D   | Nilai baris dari entity `lookup` (menutup 10.79)                                    | medium | ✅ 2026-10-07 (`2026-10-07-002`/`003`)                                       |
| E   | Pengalih konteks di `UserMenu` + `/me` ekspos konteks + OAuth 409 → pemilih         | medium | ✅ 2026-10-07 (`2026-10-07-002`); sisa round-trip nyata → todo **6.5.11 ⏸️** |

### Fase C — bentuk yang akhirnya dipakai

`FilterSpec.Via`/`ViaField` membuat `from: route` menjadi **rujukan**: `param`
menyebut record (id atau natural key) dan nilai scope dibaca dari
`<via>.<via_field>`. Declarasi hidup di picker — sejak koreksi 2026-10-07 sebagai
`picker.lookup.scope` (generik; sebelumnya `display.price_scope`, kosakata yang
mengunci satu domain) — karena picker-lah yang menyebabkan fetch, sehingga grant
turunan mewarisinya dan deklarasi+penegakan tidak bisa berasal dari dua tempat.
Sisi staf ditutup `row_scope` entity (`from: session`) — keduanya hidup
berdampingan karena `row_scope` entity dilewati untuk pemanggil anonim ber-grant.

Satu implementasi melayani baca dan tulis (`resolveRouteScopeValue` →
`resolveViaValue`), karena `applyPublicScope` yang membaca param mentah adalah
jalan kedua yang memfilter dengan id sesi (nol baris) — kelas drift 10.46.

### Fase D — kenapa TANPA script

Picker sudah mendeklarasikan sumber nilai (`lookup`), jadi nilai baris adalah nilai
**turunan** — dan `computed` sudah memperlakukan nilai turunan sebagai
bukan-milik-pemanggil. `resolveLookupFields` menegakkan aturan yang sama: baris baru
dibaca dari entity `lookup`, baris lama mempertahankan snapshot (BEKU), pasangan
tidak ditemukan/ambigu → 422. Script hook ditolak sebagai desain: ia menjadi tempat
kedua yang tahu cara mengambil satu nilai per baris, dan dua tempat menyimpang.

## File yang disentuh

- `pkg/spec/entity.go` — `ScopeDecl.Enforced`, `ValidateEntitySpec`
- `pkg/spec/resources.go` — `PublicEntityDecl.CreateScope`
- `pkg/spec/rowscope.go` — `ValidatePublicCreateScope` (aturan sumber `record` + `via`)
- `internal/api/scope.go` — resolusi `via` (dipakai baca & tulis)
- `internal/api/handler.go` — gerbang `create_scope` di `HandleCreate`
- `internal/api/router.go` — `publicCreateScope` + lantai otorisasi create
- `internal/ui/surface.go` — derivasi `create_scope` untuk permukaan publik
- `examples/kafe/spec/modules/cafe-{master,order,stock}/**` — `enforced` + `create_scope`

## Bukti yang harus ada saat selesai

- `formspec validate` menolak `scope` tanpa `row_scope` (dibuktikan merah saat
  `enforced` dihapus dari satu entity kafe), dan **lolos** untuk 4 pengecualian
  eksplisit (`route`/`none`/`external`).
- `create` anonim yang mengirim `branch_id` berbeda dari `table_session_id` yang
  dirujuk ⇒ **403**; yang cocok ⇒ **201**.
- `TestKafeSeed_GrantsAllResolve` & `TestDerivePublicGrants_KafeQR` tetap hijau.
- `make lint` 0 issues; `go test ./...` hijau.

## Sisa (ditunjuk sebagai item bernomor)

- **10.80 ⏸️** — dua sisa satu kelas: (a) picker ber-`lookup.entity` **boleh**
  tanpa `scope`, dan derivasi hanya membawa scope bila ditulis → entitas
  berikutnya bisa terulang seperti 10.76 tanpa ada yang menahannya (yang hilang:
  validator, bukan mekanisme); (b) `resolveLookupFields` hidup di jalur HTTP, jadi
  script yang menulis baris langsung ke store tidak melewatinya
  (kelas **10.46**).
- **6.5.11 ⏸️** — round-trip OAuth untuk pemilihan konteks belum diamati
  end-to-end (tidak ada provider ter-konfigurasi di example); yang teruji baru
  penyusunan fragment (Go) dan parser/pembentuk URL (klien).
