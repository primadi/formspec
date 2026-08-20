---
name: codebase-memory-id
description: "Gunakan knowledge graph kode untuk query struktural. Terpicu saat: jelajahi codebase, pahami arsitektur, fungsi apa saja yang ada, tunjukkan struktur, siapa yang memanggil fungsi ini, apa yang dipanggil oleh X, telusuri call chain, cari caller, tunjukkan dependensi, analisis dampak, dead code, fungsi tidak terpakai, high fan-out, kandidat refactor, audit kualitas kode, sintaks query graph, contoh Cypher, tipe edge, cara pakai search_graph. (Indonesian trigger for codebase-memory MCP)"
---

# Codebase Memory — Knowledge Graph (Bahasa Indonesia)

Skill ini memicu **MCP `codebase-memory`** untuk query struktural kode. Tool graph
memberikan hasil struktural yang presisi dalam ~500 token vs ~80K untuk grep.

> **Prasyarat**: MCP server `codebase-memory` harus **running** di VS Code
> (panel MCP → status "Running"). Jika tool `search_graph`/`trace_path`/dll tidak
> tersedia, fallback ke `grep_search`/`file_search`/`read_file`.

## Matriks Keputusan Cepat

| Pertanyaan                  | Panggilan tool                                          |
| --------------------------- | ------------------------------------------------------- |
| Siapa yang memanggil X?     | `trace_path(direction="inbound")`                       |
| Apa yang dipanggil X?       | `trace_path(direction="outbound")`                      |
| Konteks call lengkap        | `trace_path(direction="both")`                          |
| Cari berdasarkan pola nama  | `search_graph(name_pattern="...")`                      |
| Dead code                   | `search_graph(max_degree=0, exclude_entry_points=true)` |
| Edge lintas-service         | `query_graph` dengan Cypher                             |
| Dampak perubahan lokal      | `detect_changes()`                                      |
| Trace berklasifikasi risiko | `trace_path(risk_labels=true)`                          |
| Pencarian teks              | `search_code` atau Grep                                 |

## Alur Eksplorasi

1. `list_projects` — cek apakah project sudah di-index
2. `get_graph_schema` — pahami tipe node/edge
3. `search_graph(label="Function", name_pattern=".*Pola.*")` — cari kode
4. `get_code_snippet(qualified_name="project.path.FuncName")` — baca source

## Alur Tracing

1. `search_graph(name_pattern=".*NamaFungsi.*")` — temukan nama persis
2. `trace_path(function_name="NamaFungsi", direction="both", depth=3)` — trace
3. `detect_changes()` — petakan git diff ke simbol yang terdampak

## Tingkat Bukti

- **Scout (Tier 1):** lookup positif cepat dengan sedikit panggilan graph + cek
  source tertarget. Anggap hasil provisional; jangan pernah klaim absence,
  exhaustive, dead-code, atau complete-impact.
- **Verify (Tier 2, default):** pencarian terarah tugas, arah trace relevan,
  snippet persis untuk klaim material, dan semua halaman hasil relevan.
- **Auditor (Tier 3):** verifikasi penuh scope terbatas dengan generasi graph
  terkini, pagination lengkap, kedua arah call + relasi lebih luas bila material,
  plus batasan unresolved eksplisit.
- **Setiap tier:** setelah jalur kandidat diketahui, panggil `check_index_coverage`
  sekali dengan setiap path bukti. Untuk klaim negatif/exhaustive sertakan scope
  relevan. Hasil bersih = tidak ada gap tercatat, bukan bukti kelengkapan.

## 15 Tool MCP

`index_repository`, `index_status`, `list_projects`, `delete_project`,
`search_graph`, `search_code`, `trace_path`, `detect_changes`,
`query_graph`, `get_graph_schema`, `get_code_snippet`, `get_architecture`,
`check_index_coverage`, `manage_adr`, `ingest_traces`

## Tipe Edge

CALLS, HTTP_CALLS, ASYNC_CALLS, DATA_FLOWS, IMPORTS, DEFINES, DEFINES_METHOD,
HANDLES, IMPLEMENTS, OVERRIDE, USAGE, CALL_REFERENCE, CONFIGURES, FILE_CHANGES_WITH,
SIMILAR_TO, SEMANTICALLY_RELATED, CONTAINS_FILE, CONTAINS_FOLDER,
CONTAINS_PACKAGE

## Contoh Cypher (untuk query_graph)

```
MATCH (a)-[r:HTTP_CALLS]->(b) RETURN a.name, b.name, r.url_path, r.confidence LIMIT 20
MATCH (f:Function) WHERE f.name =~ '.*Handler.*' RETURN f.name, f.file_path
MATCH (a)-[r:CALLS]->(b) WHERE a.name = 'main' RETURN b.name
```

## Gotchas

1. `search_graph(relationship="HTTP_CALLS")` memfilter node berdasarkan degree —
   gunakan `query_graph` dengan Cypher untuk melihat edge sebenarnya.
2. `query_graph` punya batas 100k baris — tambahkan `LIMIT` Cypher untuk query
   luas atau gunakan pagination `search_graph`.
3. `trace_path` butuh nama persis — gunakan `search_graph(name_pattern=...)` dulu.
4. `direction="outbound"` melewatkan caller lintas-service — gunakan `direction="both"`.
5. `search_graph` default 50 hasil per halaman — cek `has_more` dan gunakan `offset`.

## Kapan Memakai (vs grep biasa)

| Jenis tugas                                        | codebase-memory?   |
| -------------------------------------------------- | ------------------ |
| Implementasi fitur baru, tulis kode, jalankan test | ❌ Cukup grep/read |
| Update docs, changelog, todo                       | ❌ Tidak wajib     |
| Siapa yang memanggil X? / trace call chain         | ✅ Sangat membantu |
| Dead code / fungsi tidak terpakai                  | ✅ Sangat membantu |
| Analisis dampak perubahan                          | ✅ Sangat membantu |
| High fan-out / kandidat refactor                   | ✅ Sangat membantu |
