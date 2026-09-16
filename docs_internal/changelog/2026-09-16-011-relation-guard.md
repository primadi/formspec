# 2026-09-16-011 — Relation guard: resolusi target & lintas kategori tidak lagi senyap

Item `examples/kafe/gaps_found/TODO.md` **3.7** (gap **#11** + **#12**). Plan:
`docs_internal/plan/relation-guard.md`.

**Akar tunggalnya:** relasi yang tidak bisa diresolusi diperlakukan sebagai "tidak
ada yang perlu diperiksa", bukan sebagai cacat.

**(a) Runtime (#12).** `ValidateRelationTargets` menjawab "target not found atau
table doesn't exist" dengan `continue`: referensi menggantung **diterima**, dan
nama tabel yang salah (mis. `{module}_{plural}` naif untuk relasi lintas module)
membuat guard-nya **dilewati** alih-alih gagal. Kini tiga kasus dibedakan dan
diberi nama — target entity tidak teresolusi, baris target tidak ada (referensi
menggantung), tabel tidak terbaca — sementara relasi opsional yang tidak diisi
tetap lolos.

**(b) Statik (#11 + akar #12).** Gerbang cross-manifest baru
`cmd/formspec/validate_relations.go` menolak, dengan seluruh spec tree terlihat:
`relation.resource` yang tidak menunjuk entity terdaftar (bentuk dotted maupun
`module/entity`, termasuk field di dalam `child`) dan relasi yang melintasi
`persist.category` — yang di runtime hanya memblokir sambil menulis satu baris
log, sehingga list-nya "jalan" padahal relasinya tidak pernah resolve.

**Bukti.** `TestValidateRelations` (lintas module yang resolve → diterima; target
tak terdaftar → ditolak; lintas kategori → ditolak; entity tanpa kategori tidak
dianggap kategori lain) dan `TestValidateRelationTargets_RefusesDanglingAndUnresolvable`
(runtime: resolver gagal → error; baris target tidak ada → error; relasi tak
diisi → lolos). `go test ./...` hijau; kafe `validate` **0 problem** — seluruh
relasi lintas module kafe memang resolve dengan nama, bukan kebetulan cocok.

**Sisa.** Blokir cross-category di jalur baca (`resolveRelations`) masih log +
skip sebagai jaring pengaman; mengubahnya jadi hard error akan memutus list yang
sudah berjalan dan layak diputuskan tersendiri.
