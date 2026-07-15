# Forma Overview

**Version:** 0.1.0 · **Status:** Outline

> Dokumen berstatus Outline: heading di bawah menetapkan cakupan final; isi
> ditulis bertahap.

## 1. Apa itu Forma
Definisi platform spec-driven; posisi terhadap low-code/no-code; nilai jual
"tulis kontrak sekali, dapat implementasi di banyak platform".

## 2. Prinsip Inti: Spec = Kontrak, Renderer = Implementasi
Pernyataan prinsip dan konsekuensinya (satu spec banyak renderer; seam dirancang
sejak implementasi pertama; rendering = interpretasi runtime, bukan codegen).

## 3. Anatomi Sistem
Diagram besar: spec YAML → engine → Spec Resolution API → Shell; PersistBackend
di bawah engine; control plane di atas semuanya.

## 4. Persona dan Tier Developer
App developer (Layer 0/1 — tanpa dev environment lokal), Tier 2/3 developer
(handler native, frontend custom, konsumen codegen), renderer/shell author,
platform operator.

## 5. Batas Scope
Apa yang bukan urusan Forma (mis. Page standalone di luar App bebas stack apa
saja lewat API generik).

## 6. Peta Dokumen Spec
Rujukan ke platform/backend/frontend dan urutan baca.
