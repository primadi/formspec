# 8.6 — Regenerasi artefak (schema, kind docs, generate)

**Tanggal:** 2026-09-20 · **TODO:** `examples/kafe/gaps_found/TODO.md` 8.6 · **Plan:** `docs_internal/plan/kafe-sisa-gap.md`

## Apa yang dijalankan

| Perintah | Hasil |
| --- | --- |
| `make generate-schema` | 162 shared type definitions → `schemas/` |
| `make generate` | **no-op** — target ini masih stub ("`formspec generate` is not implemented yet", `Makefile:102`) |
| `make generate-kind-docs` | 33 kind docs ditulis/diperbarui di `docs/kind/` (narrative dipertahankan, hanya region generated yang di-refresh) |

`make generate` belum menghasilkan apa pun; item TODO-nya menyebut langkah itu —
yang benar-benar ada hanyalah schema + kind docs. Dicatat di sini supaya tidak
dianggap "sudah dijalankan dan bersih".

## Churn yang muncul (dan kenapa itu benar)

Working tree sudah menumpuk perubahan fitur yang belum pernah diregenerasi.
Setelah regenerasi, diff-nya **tepat** perubahan-perubahan itu:

| Artefak | Isi diff |
| --- | --- |
| `schemas/formspec.schema.json` (+303/−54) | `$defs` baru dari S10 lanjutan: `ReportAggregate`, `ReportFormat`, `ReportParamType`, `EventChannel`, `PrintFormat`, `WorkflowStepMode` (7.2, 8.5) |
| `schemas/kinds/Timeline.schema.json` (+4) | properti `realtime` (7.4) |
| `docs/kind/ui/Timeline.md` (+1) | baris atribut `realtime` yang sama |
| `docs/kind/ui/Form.md` (+10) | paragraf + gotcha `autocomplete` form auth (Fase 17) |

Tidak ada churn di luar itu (tidak ada berkas tak terkait yang ikut berubah).

**Regenerasi konvergen:** dijalankan dua kali berturut-turut dan diverifikasi
dengan `md5sum -c` pada keempat berkas → keempatnya **OK** (tidak ada perubahan
pada run kedua). Jadi tidak ada drift tersisa antara `pkg/spec` dan artefak.

## Satu cacat generator ikut ditutup

`Timeline.realtime` muncul di schema dengan deskripsi **terpotong** di tengah
kalimat: `"Realtime subscribes the timeline to its entity's mutation events, so a"`.
Penyebabnya: generator mengambil **baris pertama** komentar Go sebagai
description, sementara komentarnya mengalir ke baris kedua. Komentar di
`pkg/spec/frontend.go` ditulis ulang agar baris pertamanya kalimat utuh →
`"Realtime refetches the timeline on its entity's mutation events."`

Audit lanjutan untuk kelas cacat yang sama: `grep` seluruh description di
`schemas/formspec.schema.json` yang berakhir dengan kata sambung
(`so|and|the|a|or|to|with|for|that|is|are|of|in|on|by|as`) → **0 temuan**.

## Bukti

| Perintah | Hasil |
| --- | --- |
| `make generate-schema && make generate-kind-docs && md5sum -c` | keempat berkas **OK** (idempotent) |
| `go build ./...` | exit 0 |
| `go test ./...` | hijau |
| `make build` | binary `formspec`, `formspec-ctl`, `formspec-registry`, `formspec-operator` ter-build |
| `cd examples/kafe && ../../bin/formspec validate --schema ../../schemas` | **0 problem** (69 manifest) |
| `grep -cE ' (so\|and\|the\|a\|…)$'` atas description schema | 0 |
