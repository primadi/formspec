# 2026-09-27-014 — L7 sebagian + pembalikan analisis L4: 20 permission melanggar §8.6

**Apa:** `docs/spec/backend/01-core-basic.md` §1.6 diperbarui (L7), dan
pemeriksaan §8.6 **membatalkan premis** analisis L4 sebelumnya.

**Bagian L7 yang selesai:** §1.6 kini menjelaskan kontrak `via`-adalah-action —
`via` muncul di bundle, dapat di-grant, dan mendapat route bila ada `impl`;
daftar lengkap field transisi; aturan turunan permission; kapan `via` boleh
diulang di `actions:` (dua bentuk saja); serta dua aturan yang lahir dari bug
sesi ini (satu nama = satu action walau beberapa transisi memakainya; nama
action grant harus sama persis dengan `via`/`actions[].name`). Contoh YAML di
§1.6 yang **mengajarkan pola duplikasi** sudah diganti dengan bentuk yang benar.

**Pembalikan analisis — bagian pentingnya.** Rencana L4 sebelumnya menyimpulkan
migrasi 36 Kategori B **butuh keputusan pemilik**: plural atau singular.
Pembacaan `01-core-basic.md` §8.6 membatalkannya — spec itu **normatif**:
bentuk kanonik satu-satunya `{module}.{plural}.{action}`, dan bentuk singular
"tidak pernah cocok ... hanya menghasilkan 403 yang sulit dilacak".

**Terukur:** dari 21 permission yang dideklarasikan, **20 berbentuk singular**
(melanggar §8.6); hanya 1 kanonik. Jadi tidak ada keputusan desain yang terbuka —
yang ada dua cacat:

**(a) Route action kustom tidak pernah mendaftarkan permission-nya** (registry
kaya, route miskin — terukur untuk `cafe-order.orders.submit-order`: ada di blob
grant, **tidak** ada di `authorized_actions` supervisor, **tidak** didaftarkan
`registerStandardPermissions` yang hardcoded).

**(b) 50 dari 77 duplikat tidak punya `impl`** → tidak ada route → maka
`required_permission`-nya terdaftar dan bisa di-grant tetapi **tidak pernah
dieksekusi**; hanya **27** yang punya route nyata.

**Akibat:** urutannya **kebalikan** dari tebakan awal — perbaiki generator agar
mendaftarkan bentuk kanonik (O1–O3) **dulu**; baru migrasi Kategori B berhenti
berbahaya, karena yang berubah hanyalah nama menuju bentuk yang spec sudah
nyatakan wajib.

**Bukti 10.47 (grant yang menguap diam-diam):** supervisor di-grant
`{page: order-page, actions: [..., {name: cancel}]}`. Materializer menghasilkan
`cafe-order.orders.cancel` (cocok dengan action lifecycle), sementara action
entitasnya bernama `cancel-order` → `authorized_actions` untuk `order` =
`[list, find, create, update]` — **tombol cancel tidak pernah muncul**, tanpa
satu pun peringatan. Akarnya: `Materialize` mencocokkan `fa.Action == ag.Name`
dan **membuang yang tidak cocok tanpa suara**.

**File:** `docs/spec/backend/01-core-basic.md`,
`docs_internal/plan/l4-validator-anti-duplikat.md` (bagian 4–6 + item O1–O8).

**Sisa:** kafe 10.51 (+O1–O8). O1–O3 small dan tidak butuh keputusan; O6
(50 action impl-less) butuh keputusan desain.
