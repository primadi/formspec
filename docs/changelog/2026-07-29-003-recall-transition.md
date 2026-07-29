# Tambah state machine transisi `recall` — in_consultation → waiting

**Tanggal:** 2026-07-29  
**File:** 
- `examples/Clinic-UI-Showcase/spec/modules/clinic/transaction/visit/entity.yaml`
- `examples/Clinic-UI-Showcase/spec/modules/clinic/transaction/visit/scripts/recall.star`

## Perubahan

### State Machine
Tambah transisi `in_consultation → waiting` via action `recall`:
- Guard: `diagnosis == None or len(diagnosis) == 0` — hanya bisa recall jika belum ada diagnosa
- Mencegah penyalahgunaan pada konsultasi yang sudah berjalan

### Action `recall`
```yaml
- name: recall
  required_permission: visits.recall
  audit: true
  ui:
    confirm: "Kembalikan pasien ke antrian? Pastikan belum ada pemeriksaan yang dilakukan."
```

### Script `recall.star`
- status → `waiting`
- started_at → `None` (reset timestamp)

**Use case:** Petugas salah memilih pasien → bisa langsung tarik kembali ke antrian tanpa harus cancel + daftar ulang.
