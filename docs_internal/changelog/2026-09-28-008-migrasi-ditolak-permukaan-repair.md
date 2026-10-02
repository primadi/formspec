# 2026-09-28-008 — Migrasi yang ditolak: permukaan repair bisa dibuka (+2 bug di jalur itu)

**Plan**: `docs_internal/plan/migrasi-ditolak-permukaan-repair.md`
**Todo**: 4.2.7 ✅ (ditutup), 4.2.8 ⏸️ + 5.24.7 ⏸️ (dibuka)

## Apa yang diubah

**Konteks.** `formspec dev` kafe gagal boot: `1 destructive change(s) refused` —
partial unique index `(dining_table_id) WHERE status = 'open'` (kafe 10.34c) tidak
bisa dibuat karena DB dev memuat **19 sesi `open` pada satu meja** (data menumpuk
sebelum index + subscription penutup sesi ada). Penolakan itu **benar**
(`01-core-basic.md` §4.4), tetapi jalur perbaikannya tidak bisa dijalankan:
`formspec repl -f repair.star` — perintah yang ditunjuk pesan `Remedy` —
memanggil `formspec.New` → `SyncSchema` → **penolakan yang sama** (`exit 1`).

1. **`Config.SkipSchemaSync`** (`resource/formspec.go`): `New` melewati
   `reg.SyncSchema` bila diberi. Hanya satu pemanggil yang memakainya.
2. **`formspec repl --no-sync`** (`cmd/formspec/repl.go`): membuka datastore
   **tanpa** menyentuh schema. Bukan "melewati gerbang": boot normal (`dev`,
   `serve`) tetap menolak. Saat penolakan terjadi, console juga mencetak hint
   `re-run with --no-sync`.
3. **Pesan `Remedy` menunjuk perintah yang bisa dijalankan**
   (`renderers/jsonb-persist/diff.go`, `alter.go`):
   `formspec repl --no-sync -f repair.star` + catatan menulis lewat `ctx.db()`
   (karena `resource.*` di console belum ter-wire ke datastore).
4. **Bug: `-f` mengeksekusi (hampir) tidak apa-apa.** `-f` melewati
   `replEval` = `syntax.ParseCompoundStmt` (parser **modal** milik REPL: berhenti
   di akhir compound statement pertama, dan baris kosong mengakhiri input).
   Terukur: file dua baris `print("FIRST") / print("SECOND")` hanya mencetak
   `FIRST`; file yang diawali komentar mencetak **nol** — sementara exit status
   **0** dan console mencetak `Ran <file>.`. Jadi permukaan repair resmi
   melaporkan sukses sambil tidak mengubah data. Kini `replExecFile` memakai
   `starlark.ExecFileOptions` (seluruh file), dengan test yang dibuktikan gagal
   saat di-inject kembali.
5. **`formspec help` exit 0** (`cmd/formspec/main.go`): `help`/`--help`/`-h`
   jatuh ke jalur "unknown command" yang `exit 1` — permintaan yang baru saja
   dijawab dilaporkan sebagai kegagalan. Daftar usage kini juga menyebut `help`.
6. **DB dev kafe dirapikan sekali** (operasional, bukan code): 18 sesi `open`
   ganda → `closed` lewat `formspec repl --no-sync -f`, lalu
   `migrate apply` → `Applied 1 migration(s)`; `formspec dev --dev-ui` boot
   (`engine loaded: 178 routes`).

## Dokumen

`docs/cli-tools/02-formspec-cli.md` (`repl`: tabel flag + `--no-sync` + subset
bahasa script; `migrate`: `--no-sync` pada alur §4.4), `docs/spec/backend/01-core-basic.md`
§4.4, `docs/renderers/jsonb-persist/03-migration-engine.md`,
`docs/spec/backend/04-persist-backend.md`. Contoh console lama
(`>>> invoice.load("inv-001")`) dihapus: `invoice` bukan global console dan
`load` adalah reserved word Starlark.

## Sisa (item bernomor)

- **4.2.8 ⏸️** — `formspec validate`/`check` **tidak** mem-parse `.star`:
  script yang rusak sintaksis lolos keduanya (terukur: `def broken_repair(`
  disisipkan ke `close_session_on_clear.star` → `89 manifest(s) validated,
0 problem(s)` + `0 error(s), 0 warning(s)`), dan baru gagal saat action-nya
  dipanggil. Repair manual ikut terdampak: v1 skrip repair di sesi ini gagal
  karena memakai implicit string concatenation.
- **5.24.7 ⏸️** — `resource.*` di console belum ter-wire (`find`/`fetch`/`save`),
  sehingga repair harus menulis SQL mentah lewat `ctx.db()` — jalur yang konvensi
  repo ini larang untuk business logic, dan tidak menghormati event/hook
  (mis. transisi `status` tidak memancarkan `emit`).
