# 2026-09-11-002 — Pindahkan kode operator ke cmd/formspec-operator

## Apa

Seluruh kode yang hanya dipakai oleh binary `formspec-operator` dipindahkan dari
`internal/operator/` ke dalam `cmd/formspec-operator/`:

- `internal/operator/api/v1alpha1` → `cmd/formspec-operator/api/v1alpha1`
- `internal/operator/controller` → `cmd/formspec-operator/controller`
- `internal/operator/report` → `cmd/formspec-operator/report`

Semua import path diperbarui
(`github.com/primadi/formspec/internal/operator/...` →
`github.com/primadi/formspec/cmd/formspec-operator/...`).

## Kenapa

Operator adalah komponen closed source (D-ARCH-15) yang berdiri sendiri —
tidak ada package lain di repo yang mengimpornya. Menempatkannya penuh di
`cmd/formspec-operator/` membuat seluruh kodenya terisolasi dalam satu folder
sehingga mudah diekstrak ke repo terpisah, dan mengikuti pola yang sudah
dipakai `internal/sidecar` → `cmd/formspec-sidecar`.

## File terdampak

- `cmd/formspec-operator/main.go` (import path)
- `cmd/formspec-operator/api/v1alpha1/*` (dipindah)
- `cmd/formspec-operator/controller/*` (dipindah, import path)
- `cmd/formspec-operator/report/reporter.go` (dipindah, import path)
- `docs/runtimes/03-formspec-operator.md`, `docs/runtimes/README.md` (referensi path)

## Verifikasi

- `go build ./...` ✅
- `go vet ./cmd/formspec-operator/...` ✅
- `go test ./cmd/formspec-operator/...` ✅ (controller tests pass)
