# Fase 6 Dogfooding — RichText Sanitization (Fase J)

**Tanggal**: 2026-08-21 · **Sequence**: 012
**Plan**: `docs/plan/fase6-dogfooding-auth-module.md` (Fase J)

## Apa yang diubah

Server-side HTML sanitize untuk field `richtext` sebelum persist (todo 6.9.1).

### Fase J — selesai

- **J1** (6.9.1) `EntityStore.sanitizeRichText` + `sanitizeHTML` di
  `renderers/jsonb-persist/crud.go` — dipanggil di `Insert` dan `Update`
  (setelah `stripEnrichedRelations`, sebelum validasi). Sanitizer ringan
  (whitelist/blacklist):
  - Hapus blok `<script>`/`<style>` (termasuk isi)
  - Hapus tag berbahaya (`iframe`, `object`, `embed`, `form`, `input`,
    `button`, `link`, `meta`) + tag penutup
  - Hapus atribut event-handler (`on*`)
  - Hapus URL `javascript:` di `href`/`src`

> Sanitizer ringan cukup untuk vektor XSS umum; policy engine penuh
> (bluemonday) bisa menggantikan tanpa mengubah call site.

## Kenapa

Client HTML tidak pernah dipercaya mentah — field `richtext` harus di-sanitize
server-side sebelum disimpan untuk mencegah XSS.

## File yang terkena dampak

- `renderers/jsonb-persist/crud.go` — `sanitizeRichText` + `sanitizeHTML`,
  dipanggil di Insert/Update
- `renderers/jsonb-persist/richtext_test.go` (baru)

## Verifikasi

- `go build ./...` + `go test ./...` hijau.
- Test `sanitizeHTML` (script, style, iframe, onclick, javascript:) + test
  Insert menyimpan richtext ter-sanitize — hijau.
