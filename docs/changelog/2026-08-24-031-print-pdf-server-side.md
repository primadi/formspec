# 2026-08-24-031 — Print PDF Server-Side (5.13.2)

## Apa yang diubah

`kind: Print` kini mendukung **PDF server-side generation** — endpoint
`GET /{ws}/_ui/print/{module}/{name}/{id}` merender Print manifest + record ke PDF tanpa
browser, menggunakan library `go-pdf/fpdf` (dependency baru).

`internal/api/print.go`:

- `HandlePrint` (method `RouterBuilder`) — resolve Print manifest dari UI registry, load
  record via `EntityStore.GetByID`, render PDF, serve `application/pdf` + `Content-Disposition`.
- `renderPrintPDF` — header (title/subtitle), body fields (label/value rows), child_table
  (header + rows), footer; `{path}` interpolation + dot-path resolution.
- Route didaftarkan di `internal/api/router.go` di bawah `/_ui/`.

`format: html` via `window.print()` (frontend) tetap ada.

## File terdampak

- `internal/api/print.go` (baru), `internal/api/print_test.go` (baru)
- `internal/api/router.go`
- `go.mod`/`go.sum` (+ `github.com/go-pdf/fpdf`)

## Referensi

- Plan: `docs/plan/fase5-completion.md` (WS-H)
- Todo: `docs/plan/todo.md` §5.13.2
