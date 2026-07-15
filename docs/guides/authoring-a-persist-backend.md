# Menulis PersistBackend Baru

**Status:** Outline

> Outline: heading menetapkan cakupan; isi ditulis setelah kontrak
> PersistBackend berstatus Draft.

## 1. Prasyarat
Kontrak PersistBackend (spec backend 04) dan bagian storage-agnostic core basic.

## 2. Interface Wajib
Structural diff apply, query resolution (seluruh filter operator), next_key
gap-free, index generation, uninstall extension bersih.

## 3. Menjaga Jaminan
Transaksionalitas, idempotensi, konsistensi event.

## 4. Yang Tidak Perlu Dipenuhi
Dialek `ctx.db` milik backend lain; detail strategi skema backend resmi.

## 5. Konformansi dan Registrasi
Test konformansi; deklarasi kind PersistBackend.
