# 2026-10-07-003 — Kosakata picker digeneralisasi: `price_*` → `lookup` (+ `lookup.scope`, `map.lookup_field`)

**Plan:** `docs_internal/plan/public-scope-enforcement.md`
**Konteks:** arahan pemilik proyek — `price_scope` "tidak generic, hanya khusus kafe,
tidak boleh ada. kafe hanya sekedar contoh kasus aplikasi bisnis". Setelah ditelusuri,
yang tidak generik bukan hanya nama `price_scope`: seluruh kosakata `price_*` pada
picker mengunci satu domain ke dalam `pkg/spec`. Semuanya diganti.

## Kenapa ini bukan sekadar rename

`price_entity`/`price_match_field`/`price_field` menyatakan **satu pola** dengan
kosakata satu contoh. Pola itu sendiri umum: _"baca SATU nilai per baris yang dipilih
dari tabel yang di-key oleh baris itu"_. Tukar kata bendanya dan ia menjadi tarif pajak
per region, stok per gudang, tarif per klinik, kuota per tenant, atau label terlokalisasi
— semuanya bentuk yang sama. Menaruhnya sebagai `price_*` berarti setiap pola berikutnya
menambah kosakata baru ke spec, dan spec berhenti bisa dipakai lintas skenario.

## Yang berubah

**Picker (`pkg/spec/picker.go`).** Blok baru `lookup` (opsional), dengan kosakata netral:

```yaml
picker:
  entity: <sumber baris>
  filter: { ... }
  lookup:
    entity: <entity terkait> # dari mana nilainya dibaca
    key: <field> # field di `entity` yang dicocokkan ke record sumber
    field: <field> # field di `entity` yang nilainya dipakai
    filter: { ... } # narrowing KLIEN (template {token})
    scope: [FilterSpec] # narrowing yang DITEGAKKAN SERVER
  display:
    {
      name_field,
      image_field,
      description_field,
      category_field,
      columns,
      search,
      empty_text,
    }
  map:
    ref_field: ...
    name_field: ...
    lookup_field: ... # menerima `lookup.field`  (dulu `map.price_field`)
```

- `display` kembali menjadi **presentasi saja** — autorisasi tidak lagi menempel di
  struktur display, yang memang bukan tempatnya.
- `lookup.scope` memakai `FilterSpec` yang sama dengan `row_scope`, jadi `from: route`
  - `via`/`via_field` (mekanisme yang sudah ada) langsung berlaku: nilainya **diturunkan
    dari record yang dirujuk**, bukan diambil dari request.
- **Decoder JSON tidak lagi menerima `price_*`** — sengaja tanpa jalur kompatibilitas.
  Spec ini pra-rilis, kafe satu-satunya pemakai, dan mempertahankan dua kosakata untuk
  satu makna adalah persis yang diminta dihapus. Migrasi: kafe (satu picker) + test.

**Resolusi nilai di server (`internal/api/pickerlookup.go`, dulu `pickerprice.go`).**
`resolveLookupFields` menggantikan `resolvePickerPrices` dan tidak lagi tahu apa-apa soal
harga — hanya "nilai ini diturunkan, bukan milik pemanggil". Dua aturan yang perlu
dinyatakan karena keduanya pilihan:

- **Beku, bukan dihitung ulang.** Baris yang rujukannya sudah ada di record tersimpan
  mempertahankan nilainya; hanya baris yang BARU masuk yang diresolusi. Mengedit jumlah
  tidak boleh menulis ulang nilai yang sudah disetujui — dan itu juga menutup "ubah
  jumlah" sebagai jalan menulis nilai.
- **Narrowing diambil dari RECORD, bukan dari request.** `lookup.scope` yang bersumber
  (`from: session|route`) dicocokkan dengan nilai field itu pada record yang sedang
  ditulis (payload, lalu baris tersimpan). Nilai itulah yang akan tersimpan, dan untuk
  tulis yang dimensinya dijepit (`create_scope`) ia sudah tervalidasi terhadap record
  yang dirujuk. Field yang tidak ada → **422**, bukan pencocokan longgar.

**Derivasi grant (`internal/ui/surface.go`).** `visitField` membaca `lookup.entity` +
`lookup.scope`, bukan `display.price_*`.

**Validator baru (`pkg/spec`).** `lookup` wajib punya `entity`+`key`+`field`;
`map.lookup_field` tanpa `lookup` **ditolak** (field penerima tanpa pengisi tidak akan
pernah berisi, dan karena server yang mengisi, submit akan gagal tanpa petunjuk);
`lookup.scope` divalidasi dengan `ValidateRowScopeFilters` (termasuk pasangan
`via`/`via_field`). `PickerLookup` ditambahkan ke `sharedTypes` generator schema — tanpa
itu setiap Entity schema menunjuk definisi yang tidak ada (guard
`TestGeneratedKindSchemas_HaveNoDanglingRefs` yang menangkapnya lebih dulu).

## Bukti

- **Guard baru:** `TestValidateEntitySpec_PickerLookupContract` (entity/key/field wajib,
  lookup opsional), `TestValidateEntitySpec_PickerLookupFieldNeedsLookup`,
  `TestValidateEntitySpec_PickerLookupScopeShape` (scope divalidasi sebagai row scope).
- **Test klien:** `lib/picker.test.ts` — `lookupIndex`, `pickerTiles` (`lookup`/
  `valueField`), `lookupScopeParams` (beberapa entri, hanya `from: route` yang dikirim),
  dan parity deklarasi kafe.
- **Kalibrasi (dua lapis, keduanya MERAH lebih dulu):** cabut blok `lookup` dari
  `order/entity.yaml` → `TestPublicGrantScope_KafeQR_PriceBranchFromTheSession` gagal
  (grant anonim kehilangan scope); matikan `resolveLookupFields` → test e2e gagal dengan
  `stored line price = 1` (lubangnya terbaca apa adanya).
- **Suite:** `go test ./...` hijau · `npx vitest run` **669 lulus / 56 file** ·
  `golangci-lint` **0 issues** · kafe `validate` 88/0 · `check` 0 error/0 warning ·
  `make generate-schema` (174 shared defs) + `make generate-kind-docs`.
- **Dokumentasi kontrak diperbarui**, bukan hanya kode: `docs/spec/backend/01-core-basic.md`
  §1.3 (contoh + tabel aturan, termasuk baris `lookup`/`lookup_field`/fail-closed),
  `docs/spec/frontend/05-app-kinds.md` (tabel derivasi → `picker.lookup.entity`),
  `docs/spec/frontend/06-page-kinds.md` (contoh), `ai_skills/entity-authoring/SKILL.md`.

## Koreksi atas entry sebelumnya

`2026-10-07-002` menyatakan penegakan baca memakai `picker.display.price_scope`. Field itu
**tidak ada lagi**; yang berlaku sekarang `picker.lookup.scope` (perilaku identik).
Bagian lain dari 002 (aturan `via`, `resolveRouteScopeValue` sebagai satu implementasi,
kondisi fail-closed, harga bekupa, pengalih konteks 6.5.10) **tidak berubah**.

## Sisa

- **10.80 ⏸️** (sudah tercatat): (a) `lookup.entity` sebaiknya diwajibkan punya `scope`
  bila target-nya entitas berdimensi — derivasi hanya membawa scope bila DITULIS;
  (b) `resolveLookupFields` hidup di jalur HTTP, jadi script yang menulis baris langsung
  ke store tidak melewatinya (kelas 10.46).
- Nama file/istilah historis di `docs_internal/changelog/` dan
  `examples/kafe/gaps_found/` yang menyebut `price_*` **tidak** ditulis ulang — itu
  catatan sejarah; item 10.80 sudah diberi catatan penunjuk ke entry ini.
