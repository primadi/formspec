# Perluas guard recall untuk cek treatments

## Perubahan

- **File**: `examples/Clinic-UI-Showcase/spec/modules/clinic/transaction/visit/entity.yaml`
- Guard recall sebelumnya hanya cek `diagnosis == None`
- Sekarang juga cek `treatments` kosong — recall hanya diizinkan jika **belum ada pemeriksaan sama sekali** (tanpa diagnosis dan tanpa tindakan/obat)

## Guard baru
```yaml
guard: "data.get(\"diagnosis\", None) == None and (data.get(\"treatments\", None) == None or len(data.get(\"treatments\")) == 0)"
```

## Verifikasi
- Diagnosis + treatments terisi → guard block
- Diagnosis null + treatments ada → guard block
- Diagnosis null + treatments kosong → recall berhasil
