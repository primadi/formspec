# Plan — Guard relasi tidak boleh lolos senyap (kafe TODO 3.7 / gap #11 + #12)

Sumber: `examples/kafe/gaps_found/TODO.md` 3.7; `06-engine-kontrak.md` gap #11 dan
#12.

## Akar tunggal

Dua gap, satu kebiasaan: **relasi yang tidak bisa diresolusi diperlakukan sebagai
"tidak ada yang perlu diperiksa"**.

| Gap | Gejala                                                                                                                                                                                                |
| --- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| #12 | `ValidateRelationTargets` → `continue` saat "target not found **atau** table doesn't exist". Nama tabel naif (`{module}_{plural}`) membuat guard-nya **dilewati**; referensi menggantung **diterima** |
| #11 | Relasi lintas `persist.category` diblokir di jalur baca dengan **satu baris log** + alias tidak dipopulasi — list "jalan", relasinya tidak pernah resolve                                             |

## Yang ditetapkan

| Aturan                                                   | Alasan                                                         |
| -------------------------------------------------------- | -------------------------------------------------------------- |
| Target entity tak teresolusi → **error**                 | ini cacat spec; melewatinya membuat FK tak pernah divalidasi   |
| Baris target tidak ada → **error** "does not exist"      | referensi menggantung adalah yang ingin dicegah                |
| Tabel target tak terbaca → **error** menyebut nama tabel | nama tabel salah = bug resolusi, bukan alasan melewati guard   |
| Relasi opsional yang tidak diisi → lolos                 | guard tentang referensi yang ada, bukan semua field            |
| `relation.resource` tak terdaftar → **ditolak statis**   | menjawab akar #12 sebelum deploy, dengan seluruh tree terlihat |
| Relasi lintas kategori → **ditolak statis**              | cacatnya tertangkap di validasi, bukan jadi log saat runtime   |

## File

- `renderers/jsonb-persist/crud.go` — tiga kasus error di `ValidateRelationTargets`.
- `cmd/formspec/validate_relations.go` (baru) — gerbang cross-manifest; diperiksa
  juga field di dalam `child`.
- `cmd/formspec/validate.go` — laporan `relation:` per manifest.
- `cmd/formspec/validate_relations_test.go` + `renderers/jsonb-persist/relation_guard_test.go`.

## Bukti

4 kasus validator (lintas module yang resolve diterima; target tak terdaftar
ditolak; lintas kategori ditolak; entity tanpa kategori tidak dianggap kategori
lain) + 3 kasus runtime. `go test ./...` hijau · kafe `validate` **0 problem** —
seluruh relasi lintas module kafe resolve dengan nama.

## Sisa

Blokir cross-category di jalur baca masih log + skip sebagai jaring pengaman;
mengubahnya menjadi hard error akan memutus list yang sudah berjalan.

## Estimasi: **small-medium**
