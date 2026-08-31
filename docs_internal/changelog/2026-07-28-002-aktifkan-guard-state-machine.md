# Aktifkan State Machine Guard Expression

Handler `HandleCustomAction` sebelumnya hanya cek `transitions[].from` match
secara inline. Guard expression (`transitions[].guard`) sudah didefinisikan di
engine `state_machine.go` tapi tidak dipanggil.

## Perubahan

- **`internal/api/handler.go`**:
  - Ganti inline state machine from-check dengan `CanTransition()` dari
    `entityengine` package
  - Sekarang guard expression dievaluasi bersama validasi `from` match
  - Jika guard gagal → `INVALID_TRANSITION` (422) dengan `ste.Reason`
  - Jika evaluasi guard error → `GUARD_ERROR` (500)

## Dampak

Guard `"diagnosis != None and len(diagnosis) > 0"` pada transisi
`complete` di visit entity sekarang **ditegakkan** — pasien tidak bisa
complete tanpa diagnosis.

## Files
- `internal/api/handler.go` (edit)
