**Last Updated**: 2026-10-06 (**Akar masalah 3.6.7 ketemu: `dist/` di `.gitignore` juga mencocoki `schemas/dist/`, jadi file kind BARU dilewati senyap oleh `git add schemas/dist` → `Seed` 404 di registry.**
File-nya ada di disk sejak 2026-09-25, tetapi `git ls-tree -r HEAD` hanya memuat `schemas/kinds/Seed.schema.json` (34 file di `dist/v1/kinds`, 33 tracked). Karena file `dist/` lama sudah tracked, `git add schemas/dist` — instruksi di `schemas/README.md` — terlihat sukses padahal tiap kind baru terlewat, dan `index.json` (tracked) menyebut kind yang skemanya tak pernah ter-commit. Ditutup: negasi `!schemas/dist/` di `.gitignore`, dua file `Seed` di-stage, **guard baru di `scripts/publish-schemas.sh`** (`git check-ignore --no-index` atas tiap file `dist/` → `exit 1` bila ada yang ter-ignore), catatan di `schemas/README.md` (+ `git status --short schemas/dist` pada alur publish, + per-kind schema bukan schema mandiri — `$ref`-nya di-resolve dari `$defs` root). **Bukti:** `git check-ignore --no-index` pada `Seed.schema.json` → tidak match sementara delapan `dist/` lain tetap IGNORED · registry-mirror lokal (`python3 -m http.server` di `schemas/dist`, tanpa `--schema`) → service-demo **13/0**, kafe **88/0** (sebelumnya `404 .../kinds/Seed.schema.json`, exit 2) · `make publish-schemas` guard hijau; dengan negasi dihapus sementara → merah, 74 file terdeteksi. Sisa = push + verifikasi live → tetap **3.6.7 ⏸️**. Plan `docs_internal/plan/schema-registry-sync.md`, changelog `2026-10-06-005`.
**Last Updated (sebelumnya)**: 2026-10-06 (**Shorthand `guard` ditutup di schema: `GuardDecl` kini `oneOf: [string, object]`.**
Sisa terakhir dari divergensi loader↔schema yang dicatat di 3.1.1 (kelas sama dengan `FormRenderDecl`/**5.4**): bentuk kanonis `guard: "len(resource.items) > 0"` — persis yang dipakai `examples/kafe` dan `verticals/billing` — ditolak lapis schema (`Incorrect type. Expected "GuardDecl"`) padahal `GuardDecl.UnmarshalYAML` menerimanya, jadi editor menyatakan spec salah sementara engine hijau. Kini generator meng-emit `oneOf: [string, object]` seperti `FormRenderDecl`; berlaku di `$defs` **dan** alias `definitions`. **Bukti:** `verticals/billing` `entities/order.yaml#0` → `[OK]` (problem 20 → 19, nol baru) · guard `TestGuardDecl_AcceptsShorthandAndObject` + `TestGeneratedSchemas_MatchOnDisk` hijau · `schemas/formspec.schema.json` diregenerasi (36+/20−). Plan `docs_internal/plan/schema-guard-shorthand.md`. Changelog `2026-10-06-004`.
**Last Updated (sebelumnya)**: 2026-10-06 (**Topologi database jadi keputusan arsitektur: `D-ARCH-32` + `docs/architecture/10-database-topology.md`.**
Latar: pertanyaan "SQLite atau Postgres untuk produksi?" sebelumnya hanya dijawab sepotong (`03-deployment-flow.md` §4 Dev=SQLite/Prod=Postgres; `05-failover.md` §3.5 baris "SQLite lokal ❌ failover") tanpa dokumen yang menyatakan **aturan penentunya**. Temuan saat menulis: gate-nya **sudah ada** — `cmd/formspec/serve.go` (`--mode=production`) menolak DSN `sqlite:` (8.1.4), jadi dokumen baru ini menyediakan alasan teknis di balik gate itu. Batas sebenarnya bukan "1 proses" melainkan **1 koneksi** (`SetMaxOpenConns(1)` di `sqlite_db.go`) → kebuntuan bisa terjadi DI DALAM satu proses (kelas bug arisan, kini tertutup: `crud.go` memakai `txReadDB`, grep `s\.db\.(QueryContext|ExecContext)` = 0 hasil) dan **semua worker wajib in-process** (§3 dokumen). Item **8.3.3 ⏸️** dibuka: routing koneksi per-workspace (= prasyarat "dedicated per workspace" tanpa satu deployment per tenant). Changelog `2026-10-06-002`.
**Last Updated (sebelumnya)**: 2026-10-06 (**7.7.6 ditutup: `notification` + `webhook` (keluar) kini benar-benar terkirim — `unsupportedChannels` kosong.**
Increment 3 dari 7.7.6. **`notification`:** module resmi `internal/notify` memberi entity `formspec.core.notification` (`row_scope {recipient_id == user_id}`) yang diisi framework dengan `SystemCaller: true`; `recipient` **wajib dan harus resolve** karena baris tanpa penerima = baris yang tidak bisa dibaca siapa pun — ditolak, bukan ditulis tak terlihat. Sekaligus menghidupkan `kind: NotificationCenter`, yang selama ini mencari entity yang tidak ada (kelas kegagalan sama dengan `ApprovalInbox` sebelum 5.13.6). **`webhook` (keluar):** `internal/webhookout`, tepat satu dari `url:` / `url_from: {config:}`, non-2xx = error (retry → dead-letter), dan **UNSIGNED** — batas itu ditulis di `02-core-extended.md` §3, bukan disembunyikan. **Bug yang ditemukan test suite:** interpolasi token yang tidak resolve menulis ulang potongan **no-op**, jadi `{` yang sama ditemukan selamanya — `go test ./internal/notify/` menggantung >180 s. **Bukti:** 41 ok / 0 FAIL, test baru 18 (10 notify + 8 webhookout), kafe 88/0 · crc 33/0 · service-demo 13/0 · arisan 17/0 · verticals/notifications 3/0 (billing 35/20 pre-existing, nol baru). Sisa dipisah ke item bernomor: signature webhook → **7.7.7 ⏸️**, blok `delivery:` Tier-2 inert → **7.7.8 ⏸️**. Changelog `2026-10-06-003`.
**Last Updated (sebelumnya)**: 2026-10-06 (**5.13.12 ditutup: `roles`-saja di-deprecate sebagai PERINGATAN, `sequential` dikecualikan.**
Step roles-only bekerja, jadi memfailkan run akan mematahkan manifest yang sah — yang disampaikan adalah **apa yang dirugikan** (nama role di manifest: mengganti nama role diam-diam mematikan approval; duty `resource + action` tidak begitu). Peringatan menyertakan jalan keluarnya (duty + grant persisnya). **`mode: sequential` dikecualikan** dan itu load-bearing: rantai diurutkan oleh `roles` dan `validateApprovalStepMode` menolak `sequential`+`permission`, jadi di rantai `roles` satu-satunya bentuk yang bisa menyatakan urutan. **Bukti live:** step roles-only → `[WARN]` dengan grant konkret (1 warning / 0 problem); rantai sequential → 0 peringatan; blast radius nol di repo. Changelog `2026-10-06-001`.
**Last Updated (sebelumnya)**: 2026-10-05 (**Tiga vertical diperbaiki + cek baru `events:` Subscription resolve.**
`verticals/notifications` + dua integrator memakai bentuk Subscription lama (`on:` + `deliver:`) yang **bukan** field `SubscriptionSpec` — skemanya `additionalProperties: false`, jadi ketiganya **gagal validasi sejak sebelum sesi ini** (1 problem masing-masing → **0**). Bentuk itu juga salah konsep: `deliver:` milik publisher; handler Subscription sudah berjalan di worker outbox. Sekalian **cek baru** `validateSubscriptionEvents`: sebelumnya tidak ada yang memeriksa nama event di `events:`, jadi typo/rename = subscription yang **tidak pernah menyala** dengan validate hijau. Dua penjaga false-positive (event reserved implied; entity luar tree di-skip), blast radius **0 di 10 tree**. Changelog `2026-10-05-007`.
**Last Updated (sebelumnya)**: 2026-10-05 (**`queue` terkirim: `job:` menamai Service action, dijalankan worker outbox.**
Increment 2 dari 7.7.6. `deliver: {channel: queue, job: <ref>}` kini benar-benar berjalan: `<ref>` = `service.action` (module publisher) atau `module.service.action`, di-resolve ke Service action dan dipanggil worker outbox (channel `queue` sudah mewajibkan `publish.durable: true`, jadi outbox **adalah** antreannya — retry + dead-letter ikut). Rumahnya Service karena job = komputasi tanpa state, dan Service sudah membawa `impl`/`uses`/dispatcher — tidak ada registry kedua. Nama polos ditolak (ambiguous). Batas yang dinyatakan di spec: **tidak ada worker paralel**, throughput terikat poll interval outbox (~1s).
**Terukur:** `verticals/billing` 20 problem **sebelum & sesudah** (34→35 manifest, nol baru) dan warning kanal **2 → 0**; kafe 88/0 · crc 33/0 · service-demo 13/0. **Temuan yang diperbaiki sekalian:** tutorial Order-to-Cash mengajarkan Subscription dengan `on:`/`deliver:` — bukan bentuk yang spec punya (ditolak skema); contohnya diperbaiki + batasnya ditulis. Sisa 7.7.6: `webhook` keluar + `notification`. Changelog `2026-10-05-006`.
**Last Updated (sebelumnya)**: 2026-10-05 (**7.7.6 dijujurkan: kanal delivery yang belum terkirim kini dilaporkan validator dan GAGAL di runtime, bukan diam-diam "delivered".**
Empat kanal dideklarasikan + diterima skema tetapi tidak dikirim: `queue`/`webhook (keluar)`/`notification` pada `events[].deliver[]`, dan seluruh blok `delivery:` Tier-2 pada `kind: Subscription` (field-nya tidak dibaca siapa pun). Sebelumnya: `default:` → log warning + **outbox menandai `completed`**, jadi konsekuensi yang tidak pernah terjadi **tidak terlihat di data mana pun**. Kini: satu sumber kebenaran `pkg/spec/delivery_channels.go` dipakai validator + runtime; `formspec validate` memberi `[WARN]` per kanal tak terkirim dengan alasannya (warning, bukan gagal — `queue` masih dipakai `verticals/*`); runtime **error** → outbox retry → **dead-letter** (`status='failed'`, terlihat & bisa di-query), dan jalur non-durable menaikkan log ke **error**. **Bukti:** 3 file test baru + satu test lama DIBALIK (`TestDeliveryEventHandler_UnimplementedChannel_ReturnsNilNotError` → `_FailsNotSilentlyDelivered`, karena dulu memakukan perilaku diam) · `go test ./...` hijau · kafe 88/0 · crc 33/0 · service-demo 13/0 · `verticals/billing` melaporkan 2 `queue` tak terkirim. Implementasi ketiga kanal tetap terbuka (butuh registry job handler / subscriber registry / module `formspec/notify`) → **7.7.6 ⏸️**. Changelog `2026-10-05-005`.
**Last Updated (sebelumnya)**: 2026-10-05 (**7.7.5 ditutup: idempotency `deliver: target` ditegakkan di jalur outbox.**
`deliver[].idempotency_key` kini benar-benar diperiksa worker sebelum sync call — langkah **"cek idempotency"** yang sudah normatif di `01-core-basic.md` §7 tapi belum terpasang. Key **completed → consequence dilewati** (retry = replay = tidak melakukan apa-apa); **pending/failed → dijalankan** (at-least-once); key tanpa store / template tak bisa di-resolve → **gagal**, bukan jalan tanpa perlindungan (literal `balance.{id}` akan membagi satu kunci ke semua event tanpa `id`, sehingga delivery kedua dilewati sebagai "selesai" — konsekuensi hilang). `ActionDispatch` kini menerima entry utuh supaya key-nya sampai; scope `deliver:{resource}.{action}` di tabel idempotency yang sama dengan kunci HTTP. **Bukti:** `TestRunTargetOnce_RetryAfterSuccessDoesNotReRun` gagal sebelum / hijau sesudah (skip dihapus → jalan **3×**; dengan skip → **1×**) · 8 test baru · `go test ./...` hijau · kafe 88/0 · crc 33/0 · service-demo 13/0. Batas yang dinyatakan: crash di antara tulisan target dan penandaan `completed` masih bisa mengulang sekali — perlu idempotensi alami target. Changelog `2026-10-05-004`.
**Last Updated (sebelumnya)**: 2026-10-05 (**Tiga problem contoh ditutup (crc 2 → 0, service-demo 1 → 0) + rename runtime 7.4.9: paket `internal/approval`, tipe `ApprovalStep`.**
**Tiga problem itu ternyata dua kelas berbeda.** (a) **Manifest salah:** report crc memakai `type: date_range` untuk parameter report, padahal `date_range` adalah kosa kata **FilterSpec** (terimplementasi di TableRenderer), bukan `ReportParamType` (`text|date|datetime|select|relation`) — diperbaiki ke bentuk dua tanggal `date_from`/`date_to` yang memang pola normatif `06-page-kinds.md` §8. (b) **Drift kode-vs-implementasi:** `EventChannel` tidak memuat `pubsub` padahal `renderers/jsonb-persist/event_handler.go` **mengimplementasikannya** (`case "pubsub"`, channel dari `target.scope`) — enum-nya sendiri mengklaim "every name is implemented", jadi schema menolak manifest yang bisa dilayani engine. `pubsub` ditambahkan ke `EventChannel` (+ union TS, + test). (c) **Aturan 7.7.2 dilanggar:** integrator `sharepoint-archiver` membuat efek samping pada `completed` tanpa penangan pembatalan — dilengkapi: event `on_cancel` pada entity + `emit: on_cancel` pada transisi `cancel`, action pembalik `remove` (idempoten, audit) pada Service `sharepoint-upload`, dan integrator `sharepoint-archive-cancel`. **Bukti:** kafe 88/0 · crc **33/0** · service-demo **13/0** · `go test ./...` hijau · `npx tsc -b` bersih (satu error TS pra-ada di `formatApprovalField.test.ts` ikut diperbaiki).
**Rename 7.4.9 selesai:** paket `internal/workflow` → `internal/approval`; `spec.WorkflowStep`/`WorkflowReject` → `ApprovalStep`/`ApprovalReject`; `db.WorkflowApproval*` → `ApprovalRequest*`; `WorkflowName`/`WorkflowModule` → `GateName`/`GateModule`; `handleWorkflowApproval`/`executeWorkflowTransition`/`recordWorkflowAudit` → `handleApproval`/`executeApprovalTransition`/`recordApprovalAudit`; 7 file + 4 nama test. **Ikut ditemukan:** entri kind `"Workflow"` masih tersisa di `internal/genkinddocs/markdown.go` dan union `ResourceKind` TS — dua sisa penghapusan kind yang lolos karena bukan `KnownKinds`. **Sengaja tetap:** tabel `formspec_workflow_approval` + kolom/route/wire `workflow*` (butuh migrasi DB + kompatibilitas klien; Go field sudah `Gate`/`GateModule`). Changelog `2026-10-05-003`.
**Last Updated (sebelumnya)**: 2026-10-05 (**Eskalasi jadi satu kosa kata: `escalation: { after, reassign }` dengan `reassign` berupa DUTY (permission), `notify_roles` dihapus — 5.13.15 ditutup.**
`notify_roles` bukan cuma salah kosa kata, ia **tidak punya pembaca** sejak awal (tidak ada kanal notifikasi), jadi mengganti namanya jadi permission tetap tidak memberi tahu siapa pun — maka ia **dibuang**, dan akan kembali sebagai `notify` hanya saat kanalnya benar-benar ada. Yang diperbaiki adalah `reassign`, yang memang melanggar aturan 6 (dulu `reassign_roles: [nama-role]`): kini duty yang di-qualify seperti duty step (`workflow.{module}.{entity}.{transition}.{reassign}`) dan di-grant lewat `{ page: "workflow:{entity}.{transition}", actions: [{name: {reassign}}] }`.
**Tiga bentuk ditolak** `formspec validate` karena tidak bisa berbuat apa-apa: `after` tanpa `reassign`, `reassign` tanpa `after`, dan `reassign` yang menunjuk duty step itu sendiri (eskalasi ke orang yang **sudah** boleh menyetujui = no-op; ini persis yang terjadi pada contoh lama `escalation: supervisor-check`).
**Ditemukan saat implementasi:** duty takeover adalah permission di gate `{entity}.{transition}` yang SAMA dengan step, jadi materializer kini mengembalikan `[]spec.DutyRef` (duty step **dan** duty eskalasi) — tanpa itu, grant tak bisa menyebut takeover dan eskalasinya menunjuk permission yang tak dipegang siapa pun. Baris `escalated_steps` lama berisi nama role tetap dibaca `CanApprove` (dual-read), jadi approval yang sedang berjalan tidak kehilangan reassignment-nya.
**Terukur:** 5 eskalasi contoh dimigrasikan (kafe `manager-check`, crc `foreman-sign-off`/`foreman-supervisor-sign-off`/`cap-supervisor-sign-off`, service-demo `head-review`) + grant takeover ditambahkan ke 3 seed; `go test ./...` hijau; kafe 88/0; crc 2 & service-demo 1 (pra-ada). Changelog `2026-10-05-002`.
**Last Updated (sebelumnya)**: 2026-10-05 (**Approval inline: `kind: Workflow` dihapus; gate menyatu pada `state_machine.transitions[].approval`.**
Approval tidak lagi manifest terpisah. Gate dideklarasikan DI transisi yang di-gate, jadi (1) tidak bisa menyimpang dari yang di-gate, (2) transisi multi-origin tercover **by construction** (lubang `from: paid` yang hanya mengawal 1 dari 4 state asal tidak bisa diekspresikan lagi), (3) satu manifest lebih sedikit. Identitas runtime = `{entity}.{transition}`; duty = `workflow.{module}.{entity}.{transition}.{step}`; grant seed = `{ page: "workflow:{entity}.{transition}", actions: [{name: {step}}] }`. Step identity/kuorum/eskalasi/requester-exclusion dipin apa adanya — hanya sumber deklarasi yang berubah.
**Terukur:** 4 manifest Workflow (kafe/crc×3/service-demo) → `approval:` inline; `kind: Workflow` dihapus dari `pkg/spec`, `KnownKinds`, `AllKinds()`, skema, dan kind-doc. `go test ./...` hijau; `formspec validate` kafe **88/0**, crc **32/2**, service-demo **13/1** (keduanya pre-existing, file lain). Plan `docs_internal/plan/approval-inline-di-transisi.md`; changelog `2026-10-05-001`.
**Last Updated (sebelumnya)**: 2026-10-04 (**5.13.14 ditutup: `escalation` level workflow ditolak validator (tidak bisa berbuat apa-apa), eskalasi crc jadi nyata di step. 5.13.15 ⏸️ dibuka: `notify_roles` belum dikirim.**)
`WorkflowEscalation` hanya punya `after` + `notify_roles` — **tanpa `reassign_roles`** — sedangkan satu-satunya efek eskalasi yang diimplementasikan adalah reassignment, dan notifikasi belum ada. Jadi bentuk itu tidak bisa menghasilkan apa pun; ketiga approval crc memakainya di level workflow, dan eskalasi 48h/72h mereka tidak pernah berjalan. **Keputusannya: tolak, bukan implementasikan** — mengimplementasikannya pun tetap tidak menghasilkan apa-apa. Keempat contoh di dokumentasi (spec §2, kind ref, skill) memakai bentuk mati itu dan kini diperbaiki; ketiga workflow crc pindah ke eskalasi step-level dengan reassign nyata ke role supervisor.
**`notify_roles` tetap tidak dikirim** — kini **dinyatakan** di spec §2.1 dan kind ref, bukan dibiarkan terbaca sebagai fitur → **5.13.15 ⏸️**. ✅ **Ditutup 2026-10-05**: field-nya **dihapus** (tanpa pembaca, jadi mengganti kosa katanya tak menolong) dan `reassign` jadi duty — lihat changelog `2026-10-05-002`.
**Bukti:** test baru (level workflow ditolak, timeout di step diterima) · nama role `escalation` divalidasi, dibuktikan dengan mengganti `reassign_roles` ke role tak ada → merah · seed crc 5 role · `go test ./...` hijau · `make lint` 0 issues · kafe 89/0.
**Last Updated (sebelumnya)**: 2026-10-04 (**5.13.9 ditutup: 4 workflow yang tak bisa disetujui kini punya role + duty, dan gate grant kini berlaku untuk SEMUA example.**
`crc-management` (3 workflow) dan `service-demo` (1) menamai role yang **tidak pernah dideklarasikan di tree-nya sendiri** → approval 403 selamanya dengan manifest yang terlihat benar. Kini keduanya punya seed role (5 + 2) dan keempat workflow memakai **DUTY** (`permission:` + grant `workflow:{name}`) — seluruh workflow di repo kini memakai duty. crc **5 → 2 problem**, service-demo **2 → 1** (sisanya sudah ada sebelumnya); seed dijalankan sungguhan (5 dan 2 inserted).
**Gate baru:** pemeriksaan "setiap grant seed resolve" yang tadinya hanya menjaga kafe kini menjaga **setiap** example (menemukan tree-nya sendiri via glob), plus pemeriksaan bahwa setiap step workflow **bisa dipenuhi** — `roles`-nya dideklarasikan atau duty-nya **di-grant** (deklarasi tanpa grant = separuh kontrak). Dibuktikan nyata: mencabut satu grant duty → merah menyebut permission persisnya.
**Temuan:** `escalation` level workflow (`after` + `notify_roles`) adalah **deklarasi mati** — tidak dibaca siapa pun; crc memakainya untuk ketiga approval-nya, jadi eskalasi 48h/72h tidak pernah terjadi → **5.13.14 ⏸️**. Dan **5.13.12 dikoreksi**: `roles` tidak bisa dihapus karena `mode: sequential` mewajibkannya.
**Last Updated (sebelumnya)**: 2026-10-04 (**5.13.13 ditutup: riwayat approval di-key NAMA step, dan jalur approve berhenti membaca posisi.**
Dua cacat dari satu akar — step dirujuk lewat **posisi**, bukan identitas: (1) jalur approve mengambil langkah dari `ApplicableSteps` tetapi `CanApprove` meng-index `wf.Steps`, jadi begitu satu step di-skip `when` step yang benar-benar menunggu **tidak pernah diperiksa** — kemunculan **keempat** bug yang sama di satu alur, dan tiga perbaikan sebelumnya hanya menambal satu pembaca; (2) `Approvals`/`EscalatedSteps` ber-key `{"0": …}` sehingga menyisipkan step di depan mengalihkan tanda tangan kemarin ke step lain.
Kini satu aturan `ActiveIndex(steps)` (nama dulu, index fallback) dipakai semua pembaca, dan riwayat di-key `StepKey` (nama; `#{index}` bila tanpa nama). **Baris lama tetap terbaca tanpa migrasi data**, dan `mergeLegacyBucket` memindahkannya saat ditulis — tanpa itu tanda tangan lama hilang dari hitungan kuorum.
**Terbukti live:** baris gaya lama (`active_step_name` kosong, `{"0":[…manajer]}`) di-approve → **200** dan tersimpan `{"supervisor-check":[…manajer,…supervisor]}`; baris baru tersimpan ber-key nama. **Bukti:** 5 test `step_key_test.go` + 1 test API · satu asersi test lama ditulis ulang karena perilaku benarnya berubah · `go test ./...` hijau · `make lint` 0 issues · kafe 89/0. Changelog `2026-10-04-009`.
**Last Updated (sebelumnya)**: 2026-10-04 (**Kuorum kini dari manifest — 5.13.11 ditutup; `mode: all` ditolak validator, `sequential` benar-benar berurutan.**
Tiga cacat sekelas dari satu baris (`eligibleCount := len(step.Roles)`): kuorum menghitung **nama role** bukan orang (bisa menuntut dua tanda tangan dari satu orang → step mustahil disetujui), `mode: sequential` **tidak pernah mengurutkan apa pun** (identik `any` sementara spec menjanjikan rantai), dan jalur approve memeriksa step dari daftar **authored** padahal langkahnya dari daftar **terfilter** — kemunculan ketiga bug daftar yang sama, dan yang paling panas.
`Quorum(step)` selalu dari manifest; **`mode: all` ditolak** dengan alasan + jalan keluar; rantai sequential **ditegakkan** (giliran per `roles`, eskalasi boleh membuka); kombinasi tak terdefinisi ditolak (`sequential`+`permission`, +`approvers`, tanpa `roles`); `CanApprove(a, steps, approver)` menjadikan daftar step **parameter** sehingga pemanggil harus menyatakan daftar mana yang ia maksud.
**Jebakan ikut tertutup:** `NewApproval` mengisi `WorkflowName` dengan **alamat pointer** — selamat selama nama hanya label, tetapi duty **diturunkan** dari nama itu, jadi nama placeholder = semua duty tak bisa dipegang dengan gejala **403 senyap**; kini nama wajib sebagai parameter.
**Bukti:** live (202 · approve 200 · record `cancelled` + `void_reason`) · 5 test baru kuorum/rantai · 5 sub-test validator · `go test ./...` hijau · `make lint` 0 issues · kafe 89/0 · skema di-regenerate. Changelog `2026-10-04-008`.
**Last Updated (sebelumnya)**: 2026-10-04 (**Fase 4a landing: grant duty hidup, dan kafe kini `roles:`-free.**)
Grant `{ page: "workflow:order-void-approval", actions: [{ name: supervisor-check }] }` dulu **mematerialisasi ke nol** (`navigationFootprint` hanya mengenal 6 kind; terukur: supervisor punya 44 permission, tidak satu pun `workflow.*`). Kini ada `case "workflow"` (permission **diturunkan** dari `spec.StepPermission`, di-key nama step), `Materializer.SetWorkflowDuties` (lookup di-wire; `nil` → dilaporkan, **bukan dibuang**), dan `workflow.Registry.GetByName`. **Kafe dibuat duty-only**: `roles:` dihapus dari step `order-void-approval`.
**Terbukti live:** grant dipasang → 45 permission termasuk duty · alur penuh hijau (202 → `can_decide: true` → approve 200 → `cancelled` + `void_reason`) · **grant dicabut** (DB saja, manifest workflow **tidak disentuh**) → perm 44, inbox **0 task**, approve **403**, record tetap `paid`. Itu klaim intinya: mengubah siapa yang boleh menyetujui tidak lagi menyentuh manifest workflow.
`TestKafeSeed_GrantsAllResolve` (gate lama) **MERAH** begitu grant dipasang dan diperbaiki dengan mewire lookup seperti produksi — bukan dengan melonggarkan asersi. Aturan baru: step tanpa `roles` **dan** tanpa `permission` ditolak validator (mustahil mencapai kuorum, dan hanya terlihat "menunggu").
**Bukti:** `go test ./...` hijau · `make lint` 0 issues · kafe 89/0 · `vitest` 636/636 · `tsc` bersih. Changelog `2026-10-04-007`.
**Last Updated (sebelumnya)**: 2026-10-04 (**Fase 2–3 `approval-duty-permission` landing: step punya identitas (`name`, kolom `active_step_name`) + duty sebagai permission; Fase 4 → 5.13.10 ⏸️, kuorum → 5.13.11 ⏸️.**
Fase 0–1 (`2026-10-04-005`) menutup **5.13.8**: `steps[].roles` kini divalidasi, dan kafe memakai nama role yang benar.
Fase 2–3 (`2026-10-04-006`) memberi step **identitas** dan menggantikan gagasan "gate = nama role" dengan **duty permission** (`WorkflowStep.permission`, `workflow.{module}.{workflow}.{step}`, 4 segmen sehingga tak bertabrakan dengan permission entity dan tak terjangkau wildcard module). `CanApprove` menerima **duty ATAU role** selama migrasi, jadi tidak ada flag day.
**Bug laten yang tertutup:** worker escalasi membaca `wf.Steps[ActiveStep]` (list _authored_) sementara jalur approve meng-index list **terfilter** `ApplicableSteps` — begitu ada step di-skip `when`, escalasi memberi `reassign_roles` milik step lain. Terukur: `escalated roles = [gl.wrong-head]` alih-alih `[gl.finance-head]` sebelum perbaikan; kini step aktif disimpan sebagai **nama** (kolom `active_step_name`, migrasi aman untuk baris lama).
**Keputusan yang saya batalkan sendiri:** `can_decide` sempat menuntut duty secara **AND** — salah, karena halaman record dan inbox adalah dua pintu ke satu alur dan `handleWorkflowApproval` memakai OR; kini keduanya memakai `CanApprove` yang sama.
**Batas Fase 4 terukur:** grant `workflow:{name}` **mematerialisasi ke nol** (`navigationFootprint` belum mengenal kind `workflow`) — kafe `/ _meta/me` untuk supervisor: 44 permission, tidak satu pun `workflow.*`. Karena itu grant duty **tidak ditempel**; `roles: [supervisor]` tetap yang menegakkan, dan step sudah mendeklarasikan `permission:` sebagai bentuk yang dituju.
**Bukti:** live canonical (`formspec dev --dev-ui`) — 202 → task dengan `can_decide: true` → approve 200 `transition_completed` → record `cancelled` + `void_reason`; `go test ./...` hijau · `make lint` 0 issues · kafe 89/0 · `vitest` 636/636 · `tsc` bersih.
**Last Updated (sebelumnya)**: 2026-10-04 (**5.13.6 ditutup — `ApprovalInbox` akhirnya punya sumber; 5.13.7 ⏸️ (realtime) dan 5.13.8 ✅ (role step) tercatat.**
Renderer `ApprovalInbox` merender **"No approval source configured"** secara
permanen karena ia mencari entity konvensional (`formspec.core.approval` dkk)
yang tidak ada di repo ini — dan akar masalahnya satu lapis lebih dalam: baris
approval hidup di tabel framework `formspec_workflow_approval` yang **bukan
Entity**, jadi tidak ada route yang mengeksposnya. Dari tiga jalur yang dicatat
item itu, **(b) endpoint khusus** adalah satu-satunya yang bisa
diimplementasikan: `GET /{ws}/_ui/workflow/approvals?app=` +
`POST …/approvals/{id}` dengan `{"decision":"approve"|"reject"}`, yang
**mendelegasikan ke `handleWorkflowApproval`** (bukan implementasi kedua, supaya
kuorum/7.4.5/audit/emit-event tidak bercabang dua). Empat aturan: `ListPending`
yang dipakai worker eskalasi sengaja **tenant-blind** → dipakai
`ListPendingForTenant` baru; eligibility memakai `CanApprove` yang sama dengan
jalur approve; `can_decide` memisahkan **terdaftar** (role step) dari **boleh
dijalankan** (permission route transisi); `display_fields` dipagari
`{module}.{plural}.view` karena pembacaan store di handler melewati pemeriksaan
permission HTTP. **Bug ditemukan jalan ini:** approval yang selesai tidak pernah
mengubah `Status` dari `pending`, sehingga request void kedua membalas **403
"workflow step out of range"** alih-alih approval baru — `Status = approved` kini
diset saat `AllStepsApproved`. **Bukti:** `go test ./...` hijau · `make lint` **0
issues** · `vitest` **629/629** · `tsc --noEmit` bersih · dua test **dibuktikan
gagal** saat perbaikan itu dihapus. Sisa: `realtime: true` belum dihormati →
**5.13.7 ⏸️** (hub WS per entity, approval bukan entity); role step kafe tidak
cocok dengan nama role IAM → **5.13.8 ⏸️**; grant per-inbox tetap
**5.25.10 ⏸️**. **Live terbukti di browser**
(`/kafe/app/pos/approval-inbox/supervisor-inbox`): task tampil lengkap
(Nomor Pesanan / Total `Rp125.000` / Alasan Void), tombol Approve bekerja →
"0 pending approvals", record `cancelled` + `void_reason` tersimpan. Changelog
`2026-10-04-004`.)
**Last Updated (sebelumnya)**: 2026-10-04 (**Sebagian 5.25.9 ditutup: `supervisor-inbox` kini punya entri menu; sisa `Print` + grant inbox → 5.25.9/5.25.10 ⏸️.**
`kafe-pos` mendapat grup authored **"Persetujuan" → "Antrean Void"**
(`module: cafe-order`, `view: supervisor-inbox`), dan entri itu **dihapus** dari
`registered_views` (menu = deklarasi permukaan; mendaftarkannya di dua tempat
hanya duplikasi). Sebelumnya route-nya **ada** tetapi tidak **terjangkau** —
supervisor harus mengetik `/kafe/app/pos/approval-inbox/supervisor-inbox`.
Menulis entri menu itu ternyata **gagal validasi**, dan pesannya berbohong:
`viewKinds` (`cmd/formspec/validate_dangling.go`) hanya memuat 10 kind, padahal
`ResolveViewRoute` + `buildRoutes` mengenal 14 — `Widget`, `Print`,
`ApprovalInbox`, `NotificationCenter` absen dari satu salinan konvensi, jadi
`view: approval-inbox:supervisor-inbox` ditolak dengan "would navigate nowhere".
Keempatnya ditambahkan, dan daftar kind di pesan error kini **diturunkan dari
`viewKinds`** (`viewKindNames()`) sehingga tidak bisa lagi berbohong.
**Bukti:** `go test ./...` hijau · `make lint` **0 issues** ·
`formspec validate --schema schemas` di kafe **89/0** (sebelumnya 1 problem:
entri menu di atas) · regresi baru `TestValidateDanglingRefs` sub-test
_"every kind with a client route is accepted as a menu view"_.
Sisa yang **belum** tertutup: `Print` masih tanpa pemicu UI (5.25.9 ⏸️), grant
per-inbox belum bisa (5.25.10 ⏸️), dan `realtime` inbox belum dihormati
(5.13.7 ⏸️). **Sumber data inbox yang dulu kosong kini SUDAH ADA** — 5.13.6
ditutup 2026-10-04 (`/_ui/workflow/approvals`, changelog `2026-10-04-004`).
Changelog `2026-10-04-003`.)
**Last Updated (sebelumnya)**: 2026-10-04 (**Grant anonim diturunkan dari permukaan view ✅ —
`public_entities` dihapus; kafe 10.13 ditutup, kafe 10.76 dibuka.**
Field `App.spec.public_entities` **dihapus** dari kontrak manifest. Allowlist
anonim App `access: public` kini diturunkan sekali per App dari permukaan view
(menu ∪ `registered_views`) × `public` per-view, lewat
`(*ui.Registry).DerivePublicGrants` (`internal/ui/surface.go`). Aturannya graf
**FETCH klien**: Table → `list`, Form `mode` → `create`/`update`+`find`/`find`,
`context.entity` → `find`, `picker.entity`/`price_entity` → `list`,
`relation` ter-render → `list`+`find`; `delete` tidak pernah implisit; scope
baris dari `param` route. Efeknya **memperketat**: `menu-category` dan
`menu-item.find` dari daftar lama tidak pernah di-fetch klien, jadi tidak lagi
di-grant. `BuildBundle` dan `publicGrants()` kini memakai helper reachability
yang **sama**, jadi bundle dan endpoint tidak bisa lagi berbeda — itulah yang
**menutup kafe 10.13** (insiden 2026-09-22: bundle **13** vs endpoint **4**;
kini **5 = 5**, dikunci `TestKafeQR_BundleAndEndpointAgree`).
**Delta yang sengaja dibuka → kafe 10.76 ⏸️:** `menu-item-price` kehilangan scope
`branch_id` (sumbernya `session.branch_id`, bukan parameter route) → anonim
membaca harga semua cabang; tanpa PII, tetapi nyata.
**Bukti:** `TestDerivePublicGrants_KafeQR` (pohon kafe nyata) ·
`TestKafeQR_BundleAndEndpointAgree` · `go test ./...` hijau · `make lint`
**0 issues** · `formspec validate --schema schemas` **89/0**.
Changelog `2026-10-04-002`; plan `docs_internal/plan/implicit-public-grants.md`.)
**Last Updated (sebelumnya)**: 2026-10-03 (**Batas baris per PERAN pada grant ✅ — kafe 10.67 + GAP-08 ditutup;
6.2.6 bergerak dari "inert" ke "sebagian ditegakkan".**
`ActionGrant.RowScope` (baru) + `FilterSpec.value` (literal) memberi aturan yang
**tidak bisa diungkapkan** `row_scope` entity: batas yang berlaku untuk satu
**peran**, bukan untuk setiap pemanggil. Konstruk lama bersifat per-entity dan
memfilter berdasarkan _siapa_, sehingga menaruh filter status di sana akan
membutakan **kasir** terhadap draft yang sedang ia susun — itulah sebabnya aturan
bisnis #1 kafe ("hanya pesanan LUNAS yang masuk dapur") selama ini hanya hidup
sebagai **kolom Kanban**: barista membaca order `draft` → **200**, lengkap dengan
`guest_token` milik tamu. Predikat grant di-AND **di SQL** bersama `row_scope`
entity dan filter klien (bukan digabung ke map filter, yang akan saling
menimpa), ditegakkan di **store** untuk baca **dan** tulis; baris di luar batas
dibaca sebagai **404** (bukan oracle keberadaan), 403 disimpan untuk batas yang
tak bisa diselesaikan. `SoftDelete` diperluas menjadi `DeleteParams` — tanpa
struct param tidak ada tempat menaruh principal, dan itulah sebabnya `delete`
adalah jalur tulis tanpa identitas. Adopsi kafe menutup **GAP-08** sekaligus
(penyaring cabang KDS kini server-side, bukan `fixed_filters` klien).
**Bukti:** kalibrasi 4 guard (matikan cabang literal / predikat SQL / wiring
`GrantScope` / kunci seed ⇒ test gagal; wiring dimatikan pernah memunculkan
_"a DRAFT order reached the kitchen"_ — reproduksi 10.67) · e2e kafe: barista
3 → **2** baris, `GET`/`PATCH` draft → **404** dengan status DB tak berubah,
kasir **tetap** melihat draft · `go test ./...` 39 paket ok · `golangci-lint`
**0 issues** (6 situs `AutoPrefixPermission` deprecated + 2 fungsi mati + 1
`ineffassign` ikut dibersihkan) · `vitest` **626** · validate 89/0 · check 0/0.
Changelog `2026-10-03-011`.)
**Last Updated (sebelumnya)**: 2026-10-03 (**Create wajib lahir di state awal ✅ — kafe 10.72.**
`POST {status:"paid"}` dulu **diterima** dan tersimpan `paid`; kini **422**.
Sebuah record yang lahir di tengah siklus menembus **dua** hal sekaligus dan
keduanya senyap: gerbang permission per-transisi (hanya di `Update`) **dan**
`emit:` milik transisi (emission diselesaikan dari **perubahan state** pada jalur
update) — jadi order `paid` tidak menerbitkan `on_paid`: tanpa jurnal, tanpa
okupansi meja, padahal record-nya tampak sah. Aturan di `Insert`, setelah
`applyDefaults` (membuat "field absen" = "state awal"); `SystemCaller` boleh state
apa pun (seed/restore/migrasi mereproduksi baris apa adanya). **Bukti:** unit
store 6 subtest · e2e kafe (penolakan **sebelum** tulis, dibuktikan tidak ada
order bukan-draft) · kalibrasi → di-`if false`-kan, e2e kembali **201** dengan
record lengkap · live: `paid`/`ready`/`ngawur` → 422, tanpa status → `draft`.
**Dampak test yang diinginkan:** 5 fixture `internal/api` yang menyemai
`{"status":"posted"}` ditandai sistem. **Dokumen:** aturan ditambahkan ke
`01-core-basic.md` §1.6 (spec mendefinisikan `state_machine` tetapi tak pernah
menyatakan create mulai dari `initial`). `go test -count=1 ./...` 39 paket ok ·
validate 89/0. Changelog `2026-10-03-009`.)
**Last Updated (sebelumnya)**: 2026-10-03 (**Gerbang transisi ditegakkan di STORE ✅ — kafe
10.46 ditutup; 10.72 ⏸️ baru.** Langkah 3–4 plan `lapisan-otorisasi.md`.
Penghalangnya dipecahkan dengan **principal sistem eksplisit**, bukan tebakan:
`action.ExecuteParams.SystemCaller` (hanya `subscription.Dispatcher` menyetelnya)
→ `action.WithSystemCaller(ctx)` → handler tulis script meneruskannya bersama
permissions pemanggil. **Kenapa tidak boleh disimpulkan dari "tidak ada
identitas":** pemanggil **anonim** juga tanpa identitas — menyimpulkannya akan
menaikkan request anonim ke hak istimewa sistem; absen penanda = bukan sistem.
**Ikut diperbaiki:** `spec.QualifyPermission` (aturan auto-prefix disatukan;
`internal/permission` mendelegasikan) — tanpa itu gerbangnya **tak pernah bisa
dibuka** (`orders.confirm-payment` ≠ `cafe-order.orders.confirm-payment`). **Bukti:**
unit store 5 subtest · e2e script → 403 · kafe in-process hijau (subscription
melewati transisi bergerbang) · **live:** kasir `in_kitchen` **403**, barista 200,
meja berakhir **`served`**; kalibrasi → gerbang dimatikan, script tembus **200**.
**Temuan baru (terukur): 10.72 ⏸️** — `POST {status:"paid"}` **diterima**, sehingga
create bisa lahir di luar state awal: melewati gerbang transisi **dan** `emit`
(→ tanpa jurnal), sementara `ngawur` ditolak hanya oleh CHECK enum.
`go test -count=1 ./...` hijau · validate 89/0. Changelog `2026-10-03-008`.)
**Last Updated (sebelumnya)**: 2026-10-03 (**Penegakan otorisasi pindah ke store ✅ — kafe
10.71 ditutup; keputusan lapisan diambil.** Pemilik memutuskan: **store =
lapisan penegakan otoritatif, HTTP tetap lapisan gagal-cepat** (plan
`lapisan-otorisasi.md`). Pipa ternyata sudah ada — `InsertParams/UpdateParams.
Permissions` sudah dioper **kedua** jalur ber-identitas (HTTP + `resource/
formspec.go`). Yang ditambahkan: **`SystemCaller bool`** (eksplisit, tidak
disimpulkan — menyimpulkannya dari "permissions kosong" akan menjadikan setiap
situs yang lupa sebagai bypass tanpa suara) + **guard field-level §5.3 di
`Insert`/`Update`**. Seluruh penulis produksi kini menyatakan identitasnya
(diverifikasi skrip audit: nol situs tak bertanda). **Dua cacat ikut terangkat:**
`resource.create` satu-satunya jalur tulis script **tanpa identitas** (field
bergerbang akan ditolak salah bagi yang memegang permission), dan **sentinel
tidak selamat melintasi batas script** → penolakan dijawab 500, bukan 403
(diperbaiki dengan marker pesan bersama `db.ForbiddenMarker`). **Bukti:** unit
store 7 subtest · e2e script (save **dan** create) → 403 · e2e HTTP tetap hijau ·
kalibrasi: hapus guard → test gagal (script tembus **200**). `go test -count=1
./...` hijau · dampak kafe **nol** (semua `required_permission` kafe di action).
**Sisa ⏸️:** 10.46 (gerbang transisi — penghalangnya kini spesifik:
`SetSaveHandler` dipakai bersama jalur aksi & worker, jadi konteks eksekutor
harus dibedakan dulu), 10.67, 10.68/10.69. Changelog `2026-10-03-007`.)
**Last Updated (sebelumnya)**: 2026-10-03 (**Field-level `required_permission` kini menjaga
TULIS juga ✅ — jalur ke kafe 10.67; 10.71 ⏸️ baru.** Saat menelusuri 10.67
("aturan #1 ditegakkan kolom Kanban, bukan API") saya memetakan batas yang
tersedia: opsi "batasi KDS ke pesanan lunas" **tidak bisa dinyatakan** —
`row_scope` hanya `from: session|route` (filter **siapa**, bukan nilai status)
dan bersifat **per-entity**, sehingga filter status akan membutakan kasir
terhadap draft-nya. Yang ditemukan justru cacat di jalur yang sama:
field-level `required_permission` (§5.3) hanya menjaga arah **baca**; arah
**tulis** tidak ditegakkan, jadi pemanggil yang tidak boleh MELIHAT sebuah field
tetap bisa **MENYETELNYA** (terukur: `limited` menyimpan `salary: 999999`).
Kini `403 FORBIDDEN` di create+update (`forbiddenFieldWrites`, memeriksa BODY
saat update — bukan hasil merge). **Dampak ke kafe nol** (semua
`required_permission` kafe ada di action, bukan field) — diverifikasi ulang di
dev server DB bersih. **Jebakan yang tertangkap:** versi pertama subtest update
**lulus tanpa perbaikan** karena `If-Match` wajib di ProdMode → PATCH dijawab
409 dan assertion-nya vakum; test kini mengirim header itu dan menolak 409.
**Sisa ⏸️:** 10.67 (batas status per-peran belum ada; perlu keputusan),
10.71 (guard tidak berlaku di jalur script), 10.68/10.69. Changelog
`2026-10-03-006`.)
**Last Updated (sebelumnya)**: 2026-10-03 (**Kontrak §5.1 `computed` ditegakkan ✅ — kafe
10.70 (dan 10.68 sebagian besar).** Ternyata **tidak perlu keputusan desain**:
`05-field-types.md` §5.1 sudah normatif ("Never client-writable" + "Recomputed on
save … sebelum persist"), dan implementasinya melanggar ketiganya. Terukur:
CREATE dengan `total_amount: 999999` → **tersimpan 999999** sementara respons API
`1155` (storage ≠ API, dan kolom turunan `_total_amount` — dasar sortir/filter/
report SQL — memuat angka palsu); PATCH dengan `777777` juga tersimpan; CREATE
tidak pernah menulis nilai computed (hanya muncul sebagai efek samping PATCH).
**Perbaikan:** `stripComputedValues` (induk + baris child) di Insert/Update/
UpsertProjection, dan `evaluateComputed` **di jalur tulis** sebelum persist
(di Insert setelah `applyFinancialSnapshot` — `tax_amount` membaca `tax_percent`
hasil snapshot); `resource`/`data` (FieldMap) kini juga di env formula **child**.
**Bukti:** test baru membaca **payload mentah dari tabel** (bukan respons), 4
subtest gagal-lebih-dulu; dev server DB bersih: create+forge → `34650`
(payload = kolom turunan), create tanpa uang → `11550` tersimpan, `?sort=`
monoton. `go test ./...` hijau · validate 89/0 · check 0/0 · vitest 616.
**Sisa ⏸️:** 10.67 (aturan #1 ditegakkan view), 10.68 (baris lama + nilai basi),
10.69 (backfill). Changelog `2026-10-03-005`.)
**Last Updated (sebelumnya)**: 2026-10-03 (**`default` field CHILD kini diterapkan ✅ — kafe
10.66; basis pajak diselaraskan.** `applyDefaults` hanya mengunjungi field
**induk**, jadi `default` pada field child tak pernah diterapkan di jalur mana
pun (`order.lines[].line_status` tersimpan sebagai key yang **absen**). Helper
baru `applyChildDefaults` mengunjungi `Child.Fields` tiap baris dan dipanggil di
Insert/Update/UpsertProjection; test membaca **baris MENTAH tersimpan** (jsonb +
table) dan gagal lebih dulu. **Dua koreksi diagnosis saya sendiri:** (i) "computed
diterapkan saat tulis" **salah** — `evaluateComputed` juga jalan di jalur baca;
yang membuat `line_total` tampak tertulis adalah **efek samping PATCH**
(fetch→compute→merge→save), kini **10.70 ⏸️**; (ii) klaim 10.68 "kolom turunan
`_total_amount` tetap NULL" **salah** — terisi setelah PATCH (terukur 25000/28875/
1155), jadi 10.68 dikoreksi lebih sempit dan ditautkan ke 10.70. **Basis pajak
dikonfirmasi pemilik** = `tax_percent × (subtotal + service charge)`; angka contoh
`o2c_e2e_test.go` diselaraskan (143750→144375, pajak 12500→13125) sehingga repo
memuat **satu** basis. Changelog `2026-10-03-004`.)
**Last Updated (sebelumnya)**: 2026-10-03 (**`order.total_amount` diturunkan sebagai `computed`
✅ — kafe 10.65 (major).** Pemilik memilih opsi (a). `branch_id` mendapat
`snapshot:` (`tax_percent`, `service_charge_percent`, `apply_service_charge`) dan
`service_charge_amount`/`tax_amount`/`total_amount` kini `computed`. **Kenapa
snapshot:** `computed` tidak bisa membaca entity lain (`evaluateComputed` jalan
SEBELUM `resolveRelations`), jadi tarif cabang harus ada di record. **Dua celah
engine ikut ditutup:** `resource`/`data` sebagai `FieldMap` di env `computed`
(tanpa itu, field opsional yang absen = error compile yang **ditelan** → hasil
absen senyap — akar 10.65), dan builtin `money_zero(x)` (cabang "tanpa diskon"
butuh nol bermata uang; `money ± number` dilarang §2.1). **Bukti:** e2e dengan
payload form QR **nyata** (tanpa angka uang) → `total 51975` → jurnal `posted`
seimbang; dev server `ORD-2026-00004` → `TOTAL 28875`, `on_paid` completed, jurnal
`JRN-2026-000002` (sebelumnya `failed` 6 retry, **nol jurnal**). Guard
terkalibrasi (blok dihapus → `total_amount is absent`). **Sisa ⏸️:** 10.68
(computed tidak dipersist → kolom `_total_amount` NULL → sortir Total), 10.69
(baris lama tidak di-backfill). **Perlu keputusan pemilik:** basis pajak — manifest
kini `(subtotal + service charge)`, verifikasi lama 9.4 memakai subtotal saja.
Changelog `2026-10-03-003`, plan `total-amount-computed.md`.)
**Last Updated (sebelumnya)**: 2026-10-03 (**Tipe detail error klien disamakan dengan wire ✅ —
kafe 10.64.** `renderers/react-shadcn` memakai satu tipe untuk dua peran, dan
tipe entri itu (`{field?, code, message}`) tidak pernah cocok dengan wire
`{level, field?, message}` — `code` diwajibkan padahal tak pernah dikirim,
`level` selalu dikirim padahal tak ada di tipe. Kini dipisah: `ErrorDetail` =
envelope, `ErrorDetailItem` = entri (sepakat field-demi-field dengan
`sdk/browser`). Laten sampai ada konsumen `details`; diperbaiki selagi murah.
`tsc -b` bersih · oxlint 0 error · vitest 616 lulus (+5 test, terkalibrasi
gagal 6 error TS saat tipe dikembalikan). Changelog `2026-10-03-001`.)
**Last Updated (sebelumnya)**: 2026-10-02 (**Triase ulang + sinkronisasi angka todo ✅ —
changelog `2026-10-02-010`.** Marker deferred dinormalisasi ke satu bentuk
(`- [⏸️]`) sehingga hitungan bisa direproduksi dengan satu perintah; `3.7.5`
dan baris pengalih `3.7.6` ditutup sebagai pelacakan ganda `backup
--incremental` (pelacak yang berlaku: **4.8.6 ⏸️**). Plan
`docs_internal/plan/triase-todo-2026-10-02.md`.
**`rate_limit` transisi `via` ditegakkan ✅ —
kafe 10.60a.** Ditemukan saat audit 10.60. `HandleCustomAction` menerima spec
dari registry union, tetapi pemeriksaan rate limit me-resolve **ulang** lewat
`resolveAction` (baca `es.Actions` saja) → override `rate_limit` pada transisi
via-only diabaikan, hanya default resource yang berlaku. Sejak L4 via-only
adalah bentuk yang diwajibkan, jadi kontraknya dideklarasikan tetapi tak
ditegakkan. **Perbaikan tidak menyentuh `resolveAction`:** `checkRateLimitAction`
menerima action hasil-resolve (`checkRateLimit` lama jadi pembungkus → pemanggil
lama netral) + `rateLimitForAction` dipakai `HandleCustomAction`.
**Terukur lewat handler nyata:** `rate_limit {max:1,per:1s}` hanya pada
`via: confirm` → panggilan ke-2 **200** sebelum · **429** sesudah; guard
terkalibrasi gagal saat wiring di-revert. Jebakan yang tercatat: versi pertama
test memanggil helper langsung dan **tetap hijau** saat wiring di-revert —
guard yang tidak menjaga; test kini melewati handler sungguhan. Changelog
`2026-10-02-009`, plan `via-sebagai-action-penuh.md` L9.
**Dua pembaca action terakhir membaca union ✅ —
kafe 10.60 / L9.** Sisa dari L8: `ValidateEntitySpec` memanggil
`ValidateHooks(d.Hooks, d.Actions)`, sehingga hook `on: before|after|on_error`
yang menyebut `via` transisi ditolak `hook action "…" does not match any
declared action` — menolak bentuk yang L4 wajibkan; dan `entityFootprint`
(`internal/auth/materialize.go`) punya satu pembaca `es.Actions` terakhir (peta
`disabled`), kini `ActionSources()` dihitung sekali untuk `disabled` + loop
kustom. Pengunci `pkg/spec/entity_hooks_union_test.go` +
`internal/auth/materialize_union_test.go`, keduanya terkalibrasi gagal saat
regresi disuntikkan. Celah kelas yang sama **ditemukan saat audit**: `rate_limit`
transisi via-only tak ditegakkan di route kustom → kafe **10.60a** — **ditutup
2026-10-02** (changelog `2026-10-02-009`; ternyata tidak butuh keputusan
`via: update/delete` — handler sudah memegang spec union). Changelog
`2026-10-02-008`, plan `via-sebagai-action-penuh.md` L9.)
(**Validator UI membaca union action ✅ — L8.**
`actionExists` (`internal/ui/validate.go`) menelusuri `es.Actions` langsung,
sehingga `formspec dev examples/kafe` mencetak **8 warning palsu** saat boot:
`Form "order-form-pos": action "start-preparing" not on entity` (+ tiga nama
lain) dan empat yang sama untuk `order-table-pos`. Keempat nama itu hidup hanya
sebagai `via:` transisi — bentuk yang sejak L4 justru **diwajibkan** — sementara
`confirm-payment`/`void-order` di manifest yang sama, satu-satunya yang masih
punya entri `actions:` (load-bearing), dilaporkan bersih; jadi yang diperingatkan
justru manifest yang mengikuti kontrak baru. Ini pembaca union **kelima** yang
terlewat (L3 ×2 · L5 ×2 · 5.24.3 · ini). Ditutup ke `ActionSources()` + pesan
validator menyebut union; **probe yang meniru jalur `resource/formspec.go` atas
spec kafe: 8 → 0 masalah**. Pengunci
`internal/ui/validate_transition_action_test.go`, terkalibrasi gagal dengan pesan
identik saat regresi disuntikkan. Kafe 10.59 ✅; sisa audit dua pembaca lain
(`internal/auth/materialize.go:242`, `pkg/spec/entity.go:1597`) dibuka sebagai
**kafe 10.60 ⏸️** — **ditutup 2026-10-02** (changelog `2026-10-02-008`, plan
L9). Changelog `2026-10-02-007`, plan
`docs_internal/plan/via-sebagai-action-penuh.md` L8.
**Login per-App & pensiun panel `_admin` ✅ —
D1–D8.** `POST /_ui/auth/login` wajib ber-`app`; App tak dikenal → 400
`UNKNOWN_APP`, App tanpa entry point auth → 400 `APP_PUBLIC_NO_LOGIN`, 0
permission di App itu → 403 `NO_APP_ACCESS` (kafe 10.21/10.22 ✅); sesi mengikat
`_meta/ui` (403 `APP_MISMATCH`) dan disimpan per `(workspace, App)`. Panel entity
`?admin=true` + gerbang biner `_admin.access` **dihapus** (5.22.7 ✅, 14.c.4
ditutup sebagai digantikan); rute framework `_admin/{setup,oauth/*,change-password}`
dipertahankan. Invarian "App privat wajib punya jalan masuk" (14.c.1 ✅) ditegakkan
saat resolve App. Plan `docs_internal/plan/app-scoped-login.md`, changelog
`2026-10-02-006`.)
**Change Password di permukaan App ✅ — 14.c.**
route `change-password` kini didaftarkan per-permukaan di `SurfaceShell`
(`surfacePath − mountPrefix`), jadi user menu tidak lagi menabrak catch-all di
App dengan `root_url` bebas (`/kafe/app/pos/change-password` dulu "Page not
found"); `_admin` tak berubah. Changelog `2026-10-02-005`.)
**Permukaan App = menu ∪ `registered_views` ✅ — 5.25.**
`App.spec.registered_views` menutup celah "entity yang tidak dipakai App tetap
bisa dibuka lewat URL langsung": route hanya dibuat untuk target menu ∪
`registered_views`; entity non-routable ditandai `routable: false` (tetap
dikirim untuk relasi/picker). Contoh kafe dimigrasikan. Sisa: 5.25.2/5.25.3/5.25.6.
**Wizard: peluncur + commit + warisan reachability ✅ — 5.25.4/5.25.5/5.25.7.**
Tombol transisi membuka wizard bila ada wizard ber-`action` yang sama; wizard
bisa commit transisi `via` lewat PATCH (terukur: `counted_cash`/`note` ikut
tersimpan); wizard **mewarisi** reachability entity-nya sehingga tak perlu lagi
didaftarkan di `registered_views`; dan **tutup shift kini 1 klik dari daftar**
(row action "Tutup Shift", 5.25.8). **Sisa gerbang App 14.c.4.**)
**Chrome jadi model REGION ✅ — 14.c.** `App.spec.
chrome.regions` (topbar/sidebar/rightbar/bottombar/footer) menggantikan model
boolean; archetype = preset (`sidebar-nav` = `no-nav` + sidebar). Dua bug kafe
tertutup: landing `/kafe` tak lagi ke rute yang tak terdaftar, dan kontrol sesi
selalu punya jalan keluar. Sisa: invarian validator + gerbang App. **Command CLI kini memakai default project yang sama
dengan `formspec dev` ✅ — 3.10.** Pertanyaan pemilik "apa tidak sebaiknya `repl`
disamakan dengan `dev`, termasuk membaca `formspec-app.yaml` yang berisi dsn?"
jawabannya **ya, dan bug-nya lebih luas dari `repl`**: setiap command membawa
literal sendiri (`spec`, `sqlite:.formspec/data.db`, workspace `"demo"`) — nilai
yang `dev` pakai justru saat config file **tidak ada**. **Terukur di
`examples/kafe`:** binary lama `repl` membuka `.formspec/data.db` dengan workspace
`demo`, sementara `dev` menyajikan `.formspec/kafe.db` sebagai `kafe`. Akibat
nyata: alur repair yang baru saja diperbaiki (4.2.7) mengerjakan DB yang tidak
dibaca `dev`, sehingga terbaca "repair tidak berefek"; dan `formspec seed` tanpa
flag membuat tenant **ketiga** (`[default:1, demo:2, kafe:2]`) yang tak pernah
dibaca App — `make seed-kafe` menutupinya hanya karena eksplisit `--workspace`.
Ditutup dengan satu helper (`loadProjectDefaults`/`finishProjectDefaults`)
menerapkan urutan `dev` di 12 command, plus **guard kelasnya** yang langsung
menemukan 3 tersangka tambahan (`get`/`describe`/`delete`). Dibuka **3.10.1 ⏸️**:
config hanya dicari di CWD, jadi menjalankan command dari luar project tetap jatuh
ke fallback.) (**Migrasi yang ditolak: permukaan repair akhirnya
bisa dibuka ✅ — 4.2.7.** `formspec dev` kafe gagal boot dengan
`1 destructive change(s) refused` (partial unique index `table-session` atas
`dining_table_id WHERE status='open'`, sementara DB dev memuat **19 sesi `open`
pada satu meja** — data menumpuk sebelum 10.34c + subscription penutup sesi ada).
Penolakannya **benar**, tetapi `Remedy`-nya menunjuk perintah yang tidak bisa
start: `formspec repl` juga menyelaraskan schema, jadi ia mati dengan pesan yang
identik — deadlock, bukan sekadar pesan kurang jelas. Ditutup dengan
`Config.SkipSchemaSync` + `formspec repl --no-sync` (boot normal tetap
menolak), `Remedy` menyebut perintah yang bisa dijalankan, dan hint saat
penolakan. **Dua bug permukaan ditemukan tepat karena akhirnya dipakai:** (a)
`-f` melewati `syntax.ParseCompoundStmt` — parser **modal** REPL — sehingga
mengeksekusi paling banyak statement pertama; file yang diawali komentar
mengeksekusi **nol** sementara exit **0** dan mencetak `Ran <file>.`, jadi
permukaan repair resmi melaporkan sukses tanpa mengubah data (kini
`starlark.ExecFileOptions`, dengan test yang dibuktikan gagal saat di-inject);
(b) `formspec help` **exit 1** karena berbagi jalur "unknown command".
**Bukti E2E:** DB pra-repair menolak + hint → `--no-sync` membuka → repair 18 sesi
→ `migrate apply` → `Applied 1 migration(s)`, index ada & menegakkan →
`formspec dev --dev-ui` `engine loaded: 178 routes`. Dibuka **4.2.8 ⏸️** (script
`.star` tidak diperiksa statis: `def broken_repair(` di dalam script kafe → hidup
`0 problem(s)` di validate **dan** check).) (**Tier 2 ditutup sebagai tidak diperlukan + 1 sisa
nyata menggantikannya.** Pertanyaan pengguna "kenapa 5.24.2 masih terbuka?"
jawabannya: **karena saya salah membiarkannya terbuka.** Alasan deferralnya
melingkar — "keputusan D7" adalah rekomendasi saya sendiri di percakapan yang
sama — sehingga tidak lolos aturan repo ini bahwa pekerjaan tertunda harus punya
alasan yang bisa diperiksa. Dua pengukuran: (1) klaim teknisnya **benar** —
`FormRenderer.doSubmit` punya empat cabang dan tidak satu pun mem-POST ke
`{id}/{action}`, jadi `kind: Form` tidak bisa menyasar action entity; (2)
permintaan **nol** — input terdeklarasi terbesar di seluruh repo = **2**
(`service-demo/tax-calculator`), kafe `void-order` = 1, sedangkan justifikasi
Tier 2 adalah ">12 input". Ditutup dengan trigger eksplisit. Sebagai gantinya
dibuka **5.24.6 ⏸️** yang nyata dan terukur: identitas approver tidak pernah
masuk ke field record — `void_approved_by` muncul 3× di seluruh repo (deklarasi,
entri `read_only` di form, baris domain-model) dan **nol penulis**, jadi field
yang sengaja di-`read_only` tetap kosong selamanya meski approval supervisor
berhasil.) (**Kafe void end-to-end ✅ — 2 bug ditemukan,
keduanya ditutup; satu di antaranya di kode saya sendiri.** Menjalankan
`void-order` pada spec kafe asli (`resource/kafe_void_approval_e2e_test.go`)
memperlihatkan bahwa perbaikan `2026-09-28-004` **belum bekerja** di sana:
`EffectiveActionSpec` membiarkan entri `actions:` menang, padahal entri itu dan
transisi mendeskripsikan paruh yang berbeda — kafe `void-order` menyumbang
`description`+`audit`+`conditions` di `actions:`, sementara `params.inputs` ada di
TRANSISI. Akibatnya nama input tidak teresolusi dan
`PATCH {status, void_reason}` → **422 "Alasan void wajib diisi"** padahal body
memuat alasannya — setiap void ditolak selamanya. Ditutup dengan overlay
per-field (5.24.4). **Bug kedua:** transisi ber-approval **tidak pernah
memancarkan `emit`** — `HandleUpdate` `return` sebelum resolusi emisi dan
`executeWorkflowTransition` tidak meresolusinya sendiri; terukur order
`cancelled` sementara mejanya tetap `occupied` (timeout), sehingga tamu
berikutnya tak bisa check-in karena sesi OPEN kedua ditolak indeks unik parsial.
Ditutup atomik bersama penulisan state (5.24.5). `go test ./...` hijau. Changelog
`2026-09-28-006`.) (**Route REST transisi `via`+`impl` ✅ — 5.24.3
selesai, plus satu bug duplikat-route laten ditemukan & ditutup.** Item 5.24.3
yang saya buka beberapa jam sebelumnya ternyata lebih dalam dari teksnya:
`GenerateCustomActionRoutes` memang masih memindai `es.Actions` (klaim L3
2026-09-27 menyebutnya sudah memakai union — **prosa itu dikoreksi**), sehingga
transisi `via`+`impl` hanya punya route `/_ui/entity/…` dan `formspec generate`
tidak punya method untuknya. **Terukur:** sebelum → `[]`; sesudah →
`POST /api/v1/cafe-order/orders/{id}/post`, dan spec `via`-only menghasilkan
`GlJournalEntryPostParams` + method `post(...)` yang sebelumnya tidak ada.
**Sekaligus ditemukan:** satu aksi bisa dapat DUA descriptor `(Method, Path)` —
generator generik memutuskan "punya `impl` sendiri?" dari `es.Actions` sementara
generator custom dari union, dan karena `mergeRoutes` menyimpan descriptor
PERTAMA, handler generik menang sehingga `impl` yang dideklarasikan **diam-diam
tidak pernah berjalan** (laten; contoh yang ada memakai entri `actions:` yang
terbaca). Ditutup di helper `generateRESTRoutes` (param `customHandled`) +
invariant `(Method, Path)` unik di kedua surface. `go test ./internal/api/`,
`./cmd/formspec/`, `./resource/` (kafe E2E), `./internal/manifest/` hijau.
Changelog `2026-09-28-005`.) (**Kontrak input transisi & action ✅ — kontrak baru
`params.inputs`, 2 item ⏸️ ditutup, 3 sisa diberi nomor.** Sebelumnya transisi,
action, dan keputusan approval **tidak punya kontrak input yang bisa dirender**:
`params` hanya berisi rule validasi (`{field, rules}`) tanpa tipe, jadi tidak ada
yang bisa _membuat_ body — hanya menolaknya. Tiga cacat nyata yang lahir dari
satu akar itu: (1) **jalur PATCH tidak menegakkan apa pun** — ia satu-satunya
jalur bagi transisi tanpa `impl` dan hanya mengevaluasi `guard`, bukan
`conditions` transisi maupun `params.validate`-nya, jadi kafe `void-order` yang
mendeklarasikan `len(params.get('void_reason','')) > 0` meloloskan
`{"status":"cancelled"}` polos; (2) **tidak ada permukaan yang bisa
mengumpulkan input** — tombol transisi POST tanpa body, dan `void_reason` field
entity tak pernah terisi di jalur itu; (3) **input pemohon hilang saat approval**
— 202 sebelum write dan `handleWorkflowApproval` hanya membaca verb `decision`.
Kini `ParamInput` (merujuk field entity → tipe/enum diwarisi + nilai disimpan;
ad-hoc wajib `type`), `inputs_from` (set bernama di Entity), `render.mode`;
penegakan di **3 jalur tulis**; kolom `params` di `formspec_workflow_approval`
dengan `seedStoredApprovalParams`/`mergeApprovalParams`; `ActionSummary.params` +
`input_sets` di bundle; `formspec generate` bertipe dari `inputs`; satu
`ActionInputDialog` generik dipakai DetailPage/Table (baris + massal)/Kanban.
**Terukur:** PATCH `{"status":"voided"}` → **422** (dulu lolos), dengan nilai →
**200 + nilai tersimpan**, approval → nilai pemohon bertahan sampai eksekusi.
`go test ./...` hijau · vitest **580** hijau · kafe 89 manifest 0 problem.
Changelog `2026-09-28-004`, plan `action-input-contract.md`. **Ditutup:** 5.24.1 ✅,
5.12.9 ✅ (bulk action akhirnya bisa mengumpulkan parameter — menutup 5.12.9 yang
dibuka saat 5.12.8), 7.4.8 ✅ (test PATCH approval). **Tier 2 DITUTUP sebagai
tidak diperlukan** (5.24.2 — alasan deferralnya melingkar, dan permintaan
terukurnya nol: input terdeklarasi terbesar di seluruh repo = **2**) lalu diganti
**5.24.6 ⏸️** yang nyata: identitas approver tidak pernah masuk ke field record
(`void_approved_by` punya 3 kemunculan di seluruh repo, **nol penulis**). (5.24.3 ✅,
5.24.4 ✅ lewat e2e kafe, 5.24.5 ✅ — dua bug ditemukan e2e itu; lihat header.)) (**Bug yang timbul dari alur sesi meja: 2 diperbaiki,
1 menunggu keputusan, 1 tidak terbukti ✅.** **(1) Pelanggaran constraint dijawab
`500 INTERNAL_ERROR` di SELURUH platform** — terukur 3 bentuk (partial unique
index, composite unique, field `unique: true`) semuanya 500, karena
`jsonb-persist` tidak punya sentinel constraint dan `isConflictError` hanya
mengenali "version conflict"/"not found". Sekarang **409 `CONFLICT`** dengan nama
field (`a record with this value already exists: dining_table_id`); klasifikasi
dipasang di **titik tulis**, jadi script/seed ikut dapat kelas yang sama.
**(2) Handler subscription tidak bisa mendeklarasikan `uses`** (kafe 10.56 ✅) —
`ctx.db` di ProdMode gagal `USES_VIOLATION` dengan instruksi yang manifest-nya
tidak punya tempat untuk dituruti; `SubscriptionSpec.Uses` ditambahkan (saudara
`handler`, bukan anaknya) + test. Asimetri primitive (hanya 7 datastore yang
diperiksa; `config`/`log`/`now`/`today`/`next_key`/`unit` tidak) kini
**didokumentasikan** di `docs/reference/primitives.md`, bukan diam-diam.
**(3) kafe 10.57** (sesi ditinggalkan mengunci meja) **butuh keputusan pemilik** —
gejalanya kini 409 yang jujur, tetapi tamu tetap tidak bisa memesan.
**(4) vitest flaky** (3 gagal di 1 dari 10 run) tidak dikejar tanpa bukti.
Changelog `2026-09-28-001`, plan `perbaikan-bug-timbul-2026-09-28.md`.** (**Sisa terbuka kafe ditindaklanjuti: 4 item
ditutup, 1 klaim saya sendiri DITARIK, 2 temuan diberi nomor ✅.** 10.42
(baris lama → transisi state 500) diperbaiki **di engine** — `validateStateTransition`
salah membaca `!oldExists` sebagai "record baru"; 10.35a (sesi tidak pernah
ditutup) ternyata **wajib**, bukan pilihan: bersama 10.34c ia membuat meja hanya
bisa dipakai sekali, jadi `release` sekarang memancarkan `on_cleared` dan
`cafe-order` menutup sesinya; 10.53 (satu grant jelek melemahkan seluruh role)
diperbaiki dengan `MaterializePartial` + pelaporan per-grant; 10.43 **SUPERSEDED**
— `PATCH` memang menghormati `require_permission` transisi (`handler.go:1047`,
terbukti 403 `dapur` vs 200 `pelayan`). **10.54 DITARIK:** saya mengklaim
`LIMIT 1` tidak ada, padahal `FindByField`/`FindByFields` memilikinya — kesalahan
dari inferensi, bukan pembacaan. Temuan baru: **10.56** (handler subscription
tidak bisa mendeklarasikan `uses` → `USES_VIOLATION` di ProdMode dengan
instruksi yang mustahil dituruti) dan **10.57** (sesi dibuat lalu ditinggalkan
mengunci meja). Changelog `2026-09-27-020`.** (**Alur sesi meja kafe ✅ mendarat + harness E2E
browser PERTAMA di repo ✅.** Skenario pemilik — tamu scan QR → pesan → bayar
QRIS → dapur → meja `served` → tambah pesanan → bayar → `served` → kasir
`release` — kini **dijalankan**, bukan dibaca: `make e2e-kafe` (Playwright,
Chromium nyata) + 3 test Go in-process. Enam gap kafe tertutup: 10.34b
(`qr_token` natural key), 10.34c (satu sesi terbuka per meja), 10.35/2.15
(halaman masuk token → sesi), 10.36 (rate limit intake anonim), 10.37
(`submit.redirect` + token `{uuid}`), 10.40b (`on_paid` → `occupied`, plus
`on_served` → `served` dan `on_cancel` → `available`). **Dua bug renderer nyata
ikut tertangkap** dan diperbaiki — tombol Create hilang di permukaan publik
(`canDoEntityAction` memeriksa identitas sebelum `authorized_actions`), dan
`default_from` mengirim placeholder mentah saat `spec.context` async; keduanya
membuat alur tamu **tidak mungkin diklik di browser** sebelum ini. Kafe: 88
manifest / 0 problem. **Sisa yang bernomor** (kafe ledger): 10.38 (PIN tamu
kedua), 10.39 (Service publik), 10.20 (landing), 10.41 🟡 (`semua pesanan
disajikan` — agregat lintas-record), 10.52 (pembatalan mengosongkan meja berisi
pesanan lain), 10.53 (`Materialize` menolak seluruh role bila satu grant gagal —
senyap, blast radius sistemik), 10.54 (lookup natural key tanpa `LIMIT 1`),
10.55 (tanpa kompensasi bila subscription occupancy gagal). Changelog
`2026-09-27-017` · `-018` · `-019`.** (**L3 ✅ · L1 ✅ · L5 ✅ · validator L4 ✅ — dan
temuan bahwa migrasi L4 TIDAK boleh buta.** Tiga hal. **(1) L5 lebih luas dari
rencana:** dua pembaca tambahan (`buildEntitySchema`, `entityFootprint`) juga
membaca `es.Actions`, sehingga menghapus entri `actions:` membuat
`schema.actions` **kosong** dan `authorized_actions` kehilangan
`release`/`reserve` — regresi 10.49 dari arah lain, dan **prasyarat keras
migrasi L4**. **(2) Bug dedup ditemukan bukti bundle, bukan test:**
`ActionSources()` mensintesis satu action **per transisi**, jadi `dining-table`
melaporkan `occupy` **3×** (tiga transisi memakai `via: occupy`) → React key
ganda di daftar tombol. Diperbaiki: satu action per nama, transisi pertama
menang. **(3) Validator L4 mendarat dengan dua pengecualian wajib**, karena
**36 dari 77 duplikat mengubah otorisasi** bila dihapus — termasuk satu
permission dipakai dua action (`start-compounding` + `mark-ready`), dan
`cancel` yang mempersempit route lifecycle generik (6 entitas). **Jebakan
metodologis saya sendiri:** alat ukur meniru logika prefix alih-alih memanggil
`AutoPrefixPermission` → laporan "76 dari 77 berubah" yang **salah**;
sebenarnya 36. **Bukti:** blok `actions:` kafe `dining-table` dihapus
seluruhnya, tombol "Tandai meja dipesan" tetap tampil, PATCH
`table_status: reserved` → **200**. **Butuh keputusan pemilik sebelum migrasi 36
Kategori B → kafe 10.51.** Changelog `2026-09-27-013`, plan
`docs_internal/plan/l4-validator-anti-duplikat.md`.)
(**Sebelumnya:** 2026-09-27 **Lanjutan plan `via-sebagai-action-penuh`: L1 ✅ +
L3 ✅ tuntas.** Dua bug nyata ditutup dengan bukti terukur. **L3**: generator
sudah lama menghasilkan `RouteDescriptor` untuk transisi ber-`impl`, tetapi
**registrasi handler**-nya belum ikut — `registerRouteWithPattern` dan
`generatePrepareRoutes` masih memindai `EntitySpec.Actions` langsung, sementara
`ActionSources()` **mensintesis** `via` tanpa menuliskannya kembali. Akibatnya
route dilewati dan permintaan jatuh ke handler file: terukur `POST
/kafe/_ui/entity/gl/journal-entry/1/post` → **404 `no such file field or action:
post`**, kini **403 `missing permission: gl.journal-entrys.post`** (setara
kontrol `submit`). **L1**: pengukuran menunjukkan migrasi L4 akan **lossy** —
dari 83 deklarasi ganda, 11 membawa `uses` dan 2 membawa `params`, yang tanpa
field baru akan **hilang** (footprint consent menyempit diam-diam). Guard baru
terkalibrasi gagal saat penyalinan field dihapus. **Item terbuka baru: kafe
10.51** (L4/L5/L7 — lihat `examples/kafe/gaps_found/TODO.md`). Changelog
`2026-09-27-011`, `-012`. Plan `docs_internal/plan/via-sebagai-action-penuh.md`.)
(**Sebelumnya:** 2026-09-26 **Sesi "kerjakan semua todo terbuka"** — 83 item
`[ ]`/`[⏸️]` ditriase; **19 ditutup\*\*, 64 tersisa dengan alasan yang
diverifikasi). Plan induk: `docs_internal/plan/close-open-items-2026-09-26.md`.
Changelog: `2026-09-26-001`–`017`. (\*\*Tambahan 2026-09-26:\*\* fix `--dev-ui` Vite
proxy multi-workspace dari laporan pemilik `localhost:5174/kafe` — mengoreksi
klaim 2.11.7 (hanya sisi Go) + item baru 2.11.7a; changelog `2026-09-26-017`,
plan `docs_internal/plan/dev-ui-vite-proxy-multi-workspace.md`.)

**Ditutup dengan kode (12 perubahan, semuanya dengan test yang dibuktikan gagal
lebih dulu):** 5.10.24 (flaky kafe E2E — akarnya **dua**: balapan `draft` vs
`posted` **dan** tanggal hardcoded yang melewati `BackdatePolicy` sehingga 6 test
gagal **permanen**; `go test ./resource/` 2/8 → **5/5 hijau**) · 2.1.6
(`core.idempotency_retention` dibaca dari manifest, boot + reload) · 5.10.22
(`generate` mengemit literal union dari `options`) · 3.7.7 (`restore
--map-resource`) · 4.8.7 (**bug senyap**: `backup`/`restore` membaca workspace
`"demo"` hardcoded — terukur `0 → 160 record` pada DB kafe nyata) · 5.22.6
(`routeExists` mengikuti `bundle.pages`; dua assertion lama **mem-pin bug-nya**) ·
5.23.3 + 5.10.17 (`help`/kosakata widget seragam di `SearchSelect` & wizard) ·
5.12.8 (bulk action dieksekusi) · 5.21.2 (dialog gambar di sel + kartu katalog) ·
5.23.2 (18 deskripsi kafe jadi teks pengguna + guard) · 5.18.5 (kolom relasi tidak
menawarkan sortir yang ditolak server).

**Ditutup sebagai koreksi (klaim tidak lagi benar):** 3.6.3a, 9.4.1a, 3.7.6,
5.10.18, 7.8.18 (=5.10.24), **2.1.4/4.2.4/4.2.5/4.2.6** (`kind: Migration` +
`DataMigration` **dicabut** 2026-09-16-012 — item 4.2.6 difile satu hari
sebelumnya), **4.4.4** (diputuskan tetap log+skip: validator menolak lebih dulu,
jadi hard error menambah risiko tanpa cakupan), **5.22.8** (enam klaim usang di
`shadcn-shell/01-architecture.md` §5 — bertentangan dengan dokumennya sendiri),
**6.5.9** (tiga dari empat bagian sudah landing di `2026-09-25-010` → hanya
pengalih header yang tersisa, kini 6.5.10).

**Diperjelas tanpa ditutup (7.8.11):** premis itemnya **salah** — "`emit:` tanpa
`emits:` = event tidak pernah dikirim" bergantung pada jalur HTTP (PATCH
menerbitkan lewat transisi, POST action lewat `emits:`, script via if/else);
`order.confirm-payment` di kafe menerbitkan lewat PATCH tetapi **diam** lewat
POST.

**71 item deferred** dari **84 item terbuka** (terukur 2026-10-02). Alasan yang
bisa diperiksa: butuh keputusan kontrak (7.8.11, 5.13.6, 5.18.7, **5.24.6**,
4.8.6, 7.9.1–7.9.4), SDK baru (3.2.6 / 2.1.5), cloud/control plane
(2.11.9–2.12.8, 8.x, 13.x), atau verifikasi browser manusia (5.21.3).

> **Catatan 2026-10-04:** dari daftar itu, **5.13.6 sudah ditutup**
> (`/_ui/workflow/approvals`, changelog `2026-10-04-004`) — angka 71 di atas
> adalah snapshot 2026-10-02 dan tidak diubah.
> Sisanya **13 item `[ ]`** = pekerjaan belum dimulai dan belum dideferred:
> `1.4.12`, `3.6.7`, `3.6.8`, `7.18.1`–`7.18.3`, `7.19.1`, `9.1.1`,
> `10.5.1`–`10.5.4`, `17.7`.

> **Angka "37" lama adalah batas bawah dan kini diganti angka terukur —
> sinkronisasi 2026-10-02, changelog `2026-10-02-010`.** Akarnya bentuk
> penandaan yang tidak seragam: deferred hidup di `- [⏸️] <id>`,
> `- [ ] <id> ⏸️`, dan `- [ ] ⏸️` sekaligus, sedangkan recipe lama hanya membaca
> bentuk pertama dan ketiga. Sesudah normalisasi **marker selalu berada di
> checkbox**, jadi satu perintah otoritatif:
>
> ```bash
> grep -cE '^\s*- \[⏸️\]' docs_internal/plan/todo.md   # 71  deferred
> grep -cE '^\s*- \[ \]'   docs_internal/plan/todo.md   # 13  belum deferred
> grep -cE '^\s*- \[x\]'   docs_internal/plan/todo.md   # 585 selesai
> ```
>
> Total terbuka turun 86 → 84 karena **3.7.5** dan baris pengalih **3.7.6**
> ditutup sebagai pelacakan ganda `backup create --incremental` (pelacak yang
> berlaku: **4.8.6 ⏸️**). Tidak ada item yang diklaim selesai tanpa kode —
> triase ini hanya menyatukan penandaan dan menghapus duplikasi.
>
> **Aturan penandaan (berlaku sejak 2026-10-02):** item yang teksnya menyatakan
> deferred (`⏸️`, `defer`, `ditunda`, `menunggu`, `perlu keputusan`) **wajib**
> memakai `- [⏸️]`. Menulis `- [ ] <id> ⏸️` kembali berarti angka deferred
> tidak bisa direproduksi — kelas kesalahan yang persis dikoreksi di sini.
> Sebelumnya: (**Fase 5.10.19** Cardinality `options` di Entity SELESAI
> (5.10.19) — plus 5.10.18 tertutup, dengan 5 sisa ⏸️ (5.10.20–5.10.24).
> Pertanyaan pengguna pada `promo.days_of_week`: "field type `json` ada `options`,
> tidak jelas single-select atau multi-select; di `promo-form.yaml` dipakai
> `widget: select-multi-tag` — seharusnya penentuan single/multi ada di level Entity,
> level Form hanya mengikuti." Benar, dan akarnya asimetri dua jalur: jalur
> **derivasi** sudah mengikuti Entity (`formWidget()`), jalur **authored** tidak
> (`FormRenderer` hanya memetakan `money`/`time`), sehingga `promo-form.yaml`
> **wajib** menulis `widget:` dan keputusan single/multi hidup di Form. Ditutup
> dengan **`Field.multiple`** (pointer bool; wajib di `json`/`string` karena
> keduanya bisa satu nilai **atau** daftar) + `options` kini sah di field skalar
> non-enum (single-select ber-caption — celah "known, separately tracked gap" yang
> selama ini hanya prosa di godoc, kini punya kontrak). **Form mengikuti Entity**
> dua lapis: `deriveFormWidget` menjadi satu sumber untuk Form turunan **dan** Form
> yang ditulis, dan `formspec check` menolak widget yang bertentangan dua arah.
> Konsumen ikut: sel tabel + halaman detail (nilai tunggal = badge ber-caption),
> filter `select` (**5.10.18 tertutup**), derivasi kolom Table/Listing/Kanban.
> **Terukur** (browser `:8099`, App `kafe-pos`, form promo **tanpa** `widget:` di
> YAML): chip hari urut deklarasi ("Senin, Jumat" walau klik Jumat lebih dulu) ·
> DB `days_of_week=[5,1]` **int** · `channel='qris'` skalar · detail page chip +
> caption `QRIS`. Uji negatif: hapus `multiple` / `multiple: true` pada `integer` /
> `options` pada `enum` → **validate** tolak; `widget: select` pada himpunan +
> `select-multi-tag` pada nilai tunggal → **check** tolak. `go test ./...` 39 paket
> · vitest **454** · `tsc -b` bersih · kafe `validate` 85 manifest 0 problem ·
> `check` 0/0. Plan `docs_internal/plan/options-cardinality-entity.md`, changelog
> `2026-09-25-007`. Ditemukan sekaligus: `go test ./resource/` **flaky
> pra-eksisting** (`journal status = "draft", want posted`; HEAD bersih 2/8 run,
> working tree 1/8 — setara) → **5.10.24 ⏸️**. Sebelumnya: (**Fase 5.23** Field `help` — warisan `description`
> entity + situs bolong SELESAI (5.23.1), dengan 2 sisa ⏸️ (5.23.2–5.23.3).
> Pertanyaan pengguna pada `promo-form`: "di entity field ada `description`, di form
> field ada `help`, apa yg ditampilkan di ui?" Jawabannya hanya `help`, di tiga
> situs, dan satu di antaranya bolong di 5 dari 6 cabang. Akarnya dua kosakata yang
> tidak terhubung: hanya jalur **derivasi** membaca `field.description`
> (`formField()`), sedangkan `resolveForm()` mengembalikan manifest authored apa
> adanya — jadi `kind: Form` (biasanya demi section/`visible_when`) menghapus setiap
> description, dan penulis menyalin manual (`promo-form.yaml` + `promo/entity.yaml`
> memuat kalimat identik). `withEntityFieldDefaults()` kini mengisi `label` +
> `help` + `sections[0].description`; `WizardFormStep` merender help di keenam
> cabang (dulu hanya `relation`) dan ikut memanggil resolver; `WizardRenderer` jalur
> `steps[].fields` inline (0 help) ikut; `OverlayHost` subtitle berhenti jatuh ke
> `"Fill in the details for this promo."`. Kontrak `description` ditegaskan sebagai
> **teks pengguna**, bukan catatan desain. **Terukur**: vitest **444 lulus** (+11,
> **3 dibuktikan gagal** saat warisan dinetralkan), `tsc -b` bersih, `go test ./...`
> hijau, `formspec check` kafe 0 error, drawer promo + wizard close-shift
> diverifikasi browser. Changelog `2026-09-25-006`. Sebelumnya: (**Fase 5.22** Routing — dokumen otoritatif +
> visibilitas menu SELESAI (5.22.1–5.22.5), dengan 3 sisa ⏸️ (5.22.6–5.22.8).
> Empat cacat nyata ditutup: (a) `ResolveViewRoute` kehilangan cabang `Listing`
> sehingga `formspec validate` menerima `view: <listing>` lalu `app.Resolve` gagal
> dan App **tidak mount sama sekali**; (b) `MenuItem.When` tidak dievaluasi
> sekaligus tidak divalidasi — contoh clinic memakai `when:
"user.has('clinic.settings.update')"`, bentuk yang tidak bisa dievaluasi evaluator
> mana pun dan melanggar `08-formspec-expr.md` §3, sehingga item tampil untuk
> **semua** orang sambil terlihat dijaga; (c) `MenuItem.Permissions` diperiksa klien
> padahal field-nya tidak pernah ada (cek mati, di tempat yang bisa di-bypass) — kini
> RBAC disaring **server**, klien tidak lagi punya pendapat; (d) gate FormSpecExpr
> kehilangan himpunan callable tertutup, sehingga `user.has(...)`, `session.x()`,
> dan salah ketik (`leng`) lolos validasi lalu mati sebagai warning runtime. Dua bug
> tambahan ketemu saat menulis test: `today()` **tidak bisa diparse** klien dan
> perbandingan `>=` menolak string — keduanya kelas "gate terima, runtime tolak".
> Dokumen baru `docs/renderers/shadcn-shell/05-routing.md` (empat jenis route, cara
> membedakan, cara resolve, lapisan menu, resep diagnosis). **Terukur**: test Go
> `internal/ui` +4 (empat nya **dibuktikan gagal** saat cabang `Listings` dihapus),
> `cmd/formspec` +4, vitest **444 lulus** (+41), `tsc -b` bersih, `go test ./...`
> hijau, kafe 85 manifest 0 problem; uji negatif menegaskan `formspec check`
> menolak `user.has(...)` pada contoh nyata. Changelog `2026-09-25-001..005`.
> Sebelumnya: **todo 5.10.15** SELESAI — field
> `promo.days_of_week` dirender editor JSON mentah, dan `widget: tags` yang ada
> menerima ketikan bebas sehingga himpunan `1..7` tidak bisa ditegakkan (`9`
> tersimpan). Ditutup dengan **dua** kontrak: atribut `Field.options`
> (`{value,label}` di Entity — `enum_values` tak punya caption) + widget
> **`select-multi-tag`** (katalog 24 → 25) yang sumbernya deklarasi: opsi terpilih
> tidak ditawarkan lagi, chip urut deklarasi, nilai di luar deklarasi tetap tampil
> (tidak dibuang saat save), nilai non-daftar → error terlihat, bentuk nilai
> dipertahankan. Satu resolver `lib/field-options.ts` dipakai Form, DetailPage,
> dan sel tabel/listing. **Terukur** (browser): pilih Senin → daftar tinggal
> `[Selasa…Minggu]`; detail `preCount: 0`; simpan UI → API `[1, 5, 2]`
> `types: [int,int,int]`. `go test ./...` 39 paket hijau · vitest **403** (+20) ·
> `tsc -b` bersih · kafe `validate` 85 manifest 0 problem. Changelog
> `2026-09-24-010`; sisa → 5.10.17 ⏸️ (wizard tidak memakai kosakata widget) +
> 5.10.18 ⏸️ (filter `select` belum baca `options`). Sebelumnya: **todo 5.21.1** SELESAI secara sebagian — klik FOTO
> MENU di halaman detail kafe membuka JPEG telanjang di **tab peramban baru**,
> sehingga App, sidebar, dan record yang sedang dibuka hilang; di permukaan kasir
> tab nyasar mudah terlupakan. Cacat yang sama ada di tiga situs (`DetailPage`
> field file, `FileInput` pratinjau readonly, `FileInput` thumbnail mode edit),
> jadi diperbaiki lewat **satu komponen bersama** `ImageLightbox`
> (`components/ui/image-lightbox.tsx`) alih-alih tiga patch. **Terukur** (browser,
> viewport 923×560): `openTabs` 2 → **1**, URL record tidak berubah, sesudah Close
> dialog 0; panel 831×489, foto 655×439 (rasio asli) — `sm:max-w-sm` default
> dialog hanya memberi 423px dan `w-auto` lebih buruk lagi (462px = `viewport −
50%` karena elemen `fixed` shrink-to-fit). vitest **383 lulus** (+6), `tsc -b`
> bersih, `web-build` sukses; test **dibuktikan gagal** saat `DetailPage`
> dikembalikan ke anchor `target="_blank"`. Changelog `2026-09-24-009`; sisa →
> 5.21.2 ⏸️ (sel tabel & kartu katalog belum). Sebelumnya: **todo 5.14.5** SELESAI
> — caption field di form
> authored tampil sebagai nama mentah (`min_purchase`) padahal entity sudah punya
> `title: "Minimum Belanja"`. Akarnya dua jalur tidak konsisten: `deriveForm()`
> membaca `field.title`, sedangkan `resolveForm()` mengembalikan manifest authored
> apa adanya sehingga renderer jatuh ke `field.label ?? field.name` — jadi
> mendeklarasikan `kind: Form` (biasanya demi urutan/section/`visible_when`,
> bukan label) menurunkan seluruh caption. Diperbaiki di **fungsi resolusi**
> (`entityFieldLabel` + `withEntityFieldLabels`/`withEntityColumnLabels`), bukan
> di 8 renderer satu per satu, karena lima ejaan berbeda sudah hidup berdampingan
> (WizardFormStep bahkan jatuh ke `description`; DetailPage mengabaikan `title`
> sepenuhnya). **Terukur:** caption drawer promo = `Kode Promo`/`Minimum Belanja`/
> `Menu Spesifik`/`Batas per Member`, regex nama mentah → false; header tabel &
> detail cabang ikut benar; vitest **377 lulus** (+12), `tsc -b` bersih,
> `go test ./...` hijau. Changelog `2026-09-24-008`; sisa → 5.14.6 ⏸️ (presedensi
> hanya di satu shell). Sebelumnya: **kafe 10.18** / todo **5.11.6** SELESAI —
> empat banner "Expression error: unexpected token: '" di form promo create.
> `FormSpecExpr` adalah subset Starlark (`08-formspec-expr.md` §2), jadi kutip
> tunggal sah — server menerimanya (`TestEvaluateGuard_SumLineBuiltin` pakai
> `sum_line('debit')`) — tetapi lexer klien hanya punya `case '"'`, sehingga `'`
> jadi token `ILLEGAL` dan keenam field kondisional `promo-form`
> (`fields.type == 'percentage'`) gagal dievaluasi. Diperbaiki di **interpreter**,
> bukan manifest: memperbaiki `promo-form.yaml` saja akan membuat halaman jalan
> sambil meninggalkan 15 situs lain rusak (kafe 11, Clinic 5). **Gap kedua ikut
> ketemu:** `formspec check` melaporkan `0 error(s)` untuk ekspresi yang mustahil
> diparse klien, jadi gate kini memindai token (string opaque; operator/karakter
> asing & string tak tertutup ditolak) — janji §4 "lolos apply ⇒ bisa dievaluasi"
> baru benar sekarang. **Terukur:** vitest **365 lulus** (+15), `tsc -b` bersih,
> `go test ./...` hijau, `gofmt` bersih; browser tanpa banner + ketiga kondisi
> field bekerja. Changelog `2026-09-24-007`; sisa → 5.11.7 ⏸️ (aturan lexer
> diduplikasi tanpa fixture bersama) & 5.11.8 ⏸️ (identifier tanpa kutip lolos
> sebagai `null` tanpa warning). Sebelumnya: (**kafe 10.31** / todo **5.20.1** SELESAI —
> dilaporkan menu sidebar tidak bisa di-scroll, jadi item di bawah viewport tak
> terjangkau. Akarnya bukan `overflow` yang lupa diset: `ScrollArea` sudah
> dipasang (`flex-1 py-2`) dan Viewport sudah `overflow: scroll`, tetapi
> `min-height: auto` pada flex item membuat Root **tumbuh mengikuti isi** (1354px
> di viewport 560px), sehingga `flex-1` tidak meng-clamp apa pun, `aside` meluber,
> dan Scrollbar base-ui tidak di-mount (`hasOverflowY` false → `shouldRender`
> null). Wilayah 1338px menjadi tak bisa disentuh. Diperbaiki di **primitif**
> (`components/ui/scroll-area.tsx` → `min-h-0`), bukan di dua call-site — bentuk
> 10.25/10.26 (satu kosakata, dua tempat, satu bolong) akan terulang begitu ada
> pemakaian ketiga. **Terukur:** root 1354 → **504**, viewport 1338/1338 → **488/1338
> scrollable**, scrollbar **10×504**, `aside.scrollHeight` 1383 → **560**, item
> terbawah bottom 1398 → **548**, wheel → `scrollTop` **850**. Test
> `scroll-area.test.tsx` (4 case, kedua varian sidebar) gagal 2 bila `min-h-0`
> dilepas. Changelog `2026-09-24-006`. Sebelumnya: (**lima gap mekanis SELESAI** — scope disepakati
> pengguna: hanya yang tidak butuh keputusan kontrak): **kafe 10.30**/
> todo **5.18.6** (`TableColumn.format` kini himpunan tertutup `TableCellFormat`
> — `format: currncy` ditolak, bukan mencetak nilai mentah), **5.19.1**
> (`useSelectFilterOptions` diangkat ke atas `switch` — error `rules-of-hooks`
> hilang, filter relasi tetap memuat opsi), **kafe 10.15** (`docs/kind/data/Seed.md`
> ada; total halaman kind **33 → 34**; taksonomi spec yang menulis "33 kind" dan
> tanpa `Seed`/`Workspace` diselaraskan), **kafe 10.17** (`StateDirFor` meng-anchor
> state dir ke project root; A/B binary membuktikan `.formspec/dev-jwt-secret`
> pindah dari CWD ke project root), **kafe 10.16** (`backup`/`restore` lewat
> storage **service**; sisi restore yang **hilang sama sekali** ditambahkan —
> test gagal sebelum patch `got 0`). Dua gap baru: **4.8.6 ⏸️** (`--incremental`
> belum ada) + **4.8.7 ⏸️** (`backup`/`restore` menulis ke workspace `"demo"`
> hardcoded — `backup create` kafe melaporkan **0 record** padahal tabelnya
> berisi 9; gagal senyap yang tampak berhasil). Changelog `2026-09-24-005`.
> Sebelumnya: **kafe 10.28 SELESAI**: dilaporkan kolom relasi
> menampilkan **UUID** dan angka **tanpa pemisah ribuan** di `stock-levels` —
> kini Cabang `Kafe Senayan`, Bahan `Beras Putih`, Saldo `20.000`. Akar #1: tiga
> jalur render, hanya dua membaca alias relasi (`derive.ts` → dot-path,
> `DetailPage` → resolver sendiri), jadi tabel **tertulis** ber-`field: branch_id`
> mencetak kunci padahal namanya ada di baris yang sama; `ListingRenderer` bahkan
> membaca `row["branch.name"]` (kunci bersarang) sehingga dot-path pun
> `undefined`. Akar #2: `renderCellValue` tak punya cabang angka sama sekali,
> jadi `decimal` jatuh ke `String(value)` — `formatter.number()` ada tapi tak
> terjangkau kolom tabel. Kini satu resolver bersama dipakai **kedua** renderer,
> `format: number` masuk kosakata sel, dan **skala field menang** atas
> `settings.decimal_scale`. Sisa dari sesi itu: **10.29 ⏸️** (sortir kolom relasi
> mengurutkan UUID — `?sort=branch.name` → **422**). Plan
> `docs_internal/plan/relation-display-and-number-format.md`, changelog
> `2026-09-24-004`; spec `frontend/06` §3.1.2 ditulis. Sebelumnya: **kafe 10.26
> SELESAI**: `TableColumn.align`/
> `width` dijanjikan spec §3 + ditulis manifest kafe, tetapi **0 renderer**
> membacanya (`grep "col.align\|col.width"` → 0 hasil) — `align: right` pada
> kolom numerik hilang diam-diam. Kini helper bersama `src/lib/tableColumn.ts`
> dipakai kedua renderer; `align` kena `<th>` **dan** `<td>` (`<td>` sibling
> `<th>`), `width` kena `<th>`, dan `align` jadi **enum tertutup** di skema.
> Terukur: `stock-levels` tiga kolom numerik `thAlign=right` **dan**
> `tdAlign=right`. Sisa baru **10.27 ⏸️** (`TableColumn.link` — atribut terakhir
> yang tidak dikonsumsi; semantik `:param` belum ditetapkan, sudah ditandai
> **Open** di spec §3). Plan
> `docs_internal/plan/table-column-align-width.md`, changelog `2026-09-24-003`;
> spec `frontend/06` §3.1.1 ditulis. Sebelumnya: **kafe 10.25 SELESAI**: kolom
> `money` di Table
> turunan menampilkan JSON (`{"amount":"50000","currency":"IDR"}`) di
> `cash-movements` — akarnya `engine/derive.ts` `tableFormat()` yang tidak
> memetakan `money → currency`, padahal `cellHintsForField()` sudah. Heuristik
> `decimal + rules[min] → currency` dibuang sekaligus (menebak mata uang dari
> batas bawah; dilarang `05-field-types.md` §2). Terukur sesudah: Jumlah →
> `Rp50.000`. Plan
> `docs_internal/plan/money-table-format-derivation.md`, changelog
> `2026-09-24-002`. Sebelumnya: **kafe 10.19 + 10.23 + 10.24 SELESAI**: bundle
> kini mengirim **`authorized_actions`** — himpunan aksi yang boleh dilakukan
> pemanggil ini, di-resolve server dengan checker yang sama yang memutuskan entity
> ikut bundle; route turunan & tombol berhenti menebak dari `lifecycle`. Kosakata
> aksi diselaraskan (`view→find`, `edit→update`) sehingga `manajer` tak lagi
> kehilangan tombol Edit. Plan
> `docs_internal/plan/bundle-authorized-actions.md`, changelog `2026-09-24-001`;
> spec `frontend/04` §2 diperbarui. Temuan **struktural** yang masih terbuka:
> kontrol auth hidup **hanya** di chrome (`AuthArea` di header ketiga shell,
> `return null` untuk `auth: none`, `auth_action` tanpa `logout`, `useAutoLogout`
> mati di permukaan publik, `private`+`no-nav`+`auth: none` **lolos validasi**) —
> "hak akses menu" jadi _mandatory-by-omission_; tiga pertanyaan keputusan di
> `docs_internal/plan/chrome-composition-spec.md`, rencana gerbang App di
> `docs_internal/plan/app-entry-gate.md`. **Diperbarui 2026-09-29** (plan
> `chrome-regions.md`, §14.c): kontrol auth kini punya **region** tempat hidup dan
> **selalu** dirender bila ada token — jadi "tidak ada jalan keluar" sudah
> tertutup; yang masih terbuka adalah invarian validator dan `logout` di
> `auth_action` (14.c.1/14.c.3). Kafe sisa: 10.11/10.12/10.13/10.15/
> 10.16/10.17/**10.18**/20/21/22 (semua ⏸️, bukan blocker).
> Sebelumnya 2026-09-23: grant publik jadi **floor** — pemanggil yang sudah login
> tidak lagi lebih buruk daripada tamu; plan
> `docs_internal/plan/public-grant-signed-in-floor.md`, changelog `2026-09-23-003`.
> Kafe 10.14 selesai: aset
> seed diunggah lewat storage service via marker `$asset` — `make seed-kafe-assets`
> dihapus, reconcile menggantikan skip-murni, gambar es-jeruk diperbaiki + roti
> bakar/kopi susu ditambahkan; changelog `2026-09-23-002`).
> **Status**: ✅ Fase 0 complete · ✅ Fase 1 (1.1–1.5) · ✅ Fase 2.1 · ✅ Fase 2.2 · ✅ Fase 2.6 (2.6.1–2.6.3, 2.6.5–2.6.6) · ✅ Fase 2.7 (idempotency prepare flow) · ✅ Fase 2.8 (spec.expose) · ✅ Fase 2.9 (2.9.1–2.9.3: ctx.\* primitives + dev auto-provision) · ✅ Fase 5 (5.1–5.4) · ✅ Spec hot-reload · ✅ Fase 11 (review schema↔docs) · ✅ Audit spec↔schema + tambah TODO item · ✅ `formspec validate` (3.1.1, engine+schema) · ✅ Rename forma→formspec (docs_internal/plan/rename-formspec.md) · 🚧 Fase 12 Domain Infrastruktur (docs/architecture/09-domain-map.md) · ✅ Schema registry online (docs_internal/plan/schema-registry-online.md) · ✅ CLI repl/seed/diff (3.4.1, 3.6.2, 3.6.3) · ✅ **Fase 4 (4.1–4.10) complete** (incl. 4.3.1–4.3.5 entity extension, 4.8.3 restore remap) · ✅ Landing page (5.1.3 + 5.13.5, docs_internal/plan/landing-page.md) · ✅ App renderer archetypes (5.1.1–5.1.3: sidebar-nav/topnav/no-nav + access + persist_backend, docs_internal/plan/landing-page.md) · ✅ **Fase 6.1 (6.1.1–6.1.3: login + token, entity-backed auth, external/ merge, generate-auth)** (docs_internal/plan/auth-login-token.md) · ✅ **6.3.1 + 6.3.2 + 5.12.5 (role + role-assignment Entity, materialisasi grant page → permission)** (docs_internal/changelog/2026-08-20-001) · ✅ **6.2.3 (wire permission check semua handler, surface-aware 404)** (docs_internal/changelog/2026-08-20-002) · ✅ **Fase 6 COMPLETE (6.1–6.9, dogfooding auth module)** (docs_internal/plan/fase6-dogfooding-auth-module.md, changelog 2026-08-20-003 s/d 2026-08-21-014) · 📐 **Widget strategy** (docs_internal/plan/widget-strategy.md — sync 5.10, tambah 5.2.7/5.10a, cross-link 7.17.1) · ✅ **Role grants app-scope sync** (docs_internal/plan/role-grants-app-scope.md, changelog 2026-08-26-003) · ✅ **Fase 5 COMPLETE (5.1–5.16)** (docs_internal/plan/fase5-completion.md, changelog 2026-08-24-027 s/d -032; audit sinkronisasi todo 2026-08-27) · ✅ **Fase 7 hampir lengkap** (7.1–7.14, 7.16, 7.17.1–7.17.2 — changelog 2026-08-25-001 s/d 2026-08-26-001; sisa: 7.9.1–7.9.5, 7.15.1, 7.17.3, 7.18, 7.19) · ✅ **Fase 2 COMPLETE (2.9.4 ctx.db module-scoped)** (changelog 2026-08-27-002) · ✅ **3.1.1a honesty scan Starlark** (changelog 2026-08-27-003) · 🚧 **Fase 8 sebagian** (8.1.1–8.1.5, 8.2.1–8.2.6 — docs_internal/plan/fase8-production-serve.md, changelog 2026-08-27-004; sisa: 8.1.6, 8.2.7, 8.3 ⏸️) · ✅ **Fase 10.1 `formspec mcp-serve`** (local MCP tool server — docs_internal/plan/fase10-local-mcp.md, changelog 2026-08-27-005) · ✅ **Fase 10.2 `formspec consult` client (Go)** — docs_internal/plan/fase10-consult-client.md, changelog 2026-08-27-006 (deviasi TS→Go dicatat di docs/ai/01+05; 10.2.7 kompresi riwayat deferred) · ✅ **Fase 10.3/10.4/10.6/10.7 consult completion** — changelog 2026-08-27-007 (10.3.3 & 10.5 deferred; 10.2.7 deferred) · ✅ **Fase 13.1 vendoring** (module install/list/uninstall + verify + boot enforcement — docs_internal/plan/fase13-vendoring.md, changelog 2026-08-28-001; 13.2/13.3 menyusul) · ✅ **Fase 13.2 overrides** (shadow copy adopt/diff/list + whitelist + drift detection — changelog 2026-08-28-002; 13.3 registry menyusul) · ✅ **Fase 13.3 registry loop** (verticals/registry spec + formspec sign/publish + install --from dengan signature verification — changelog 2026-08-28-003; 13.3.3/13.3.5 deferred) · ✅ **Theme switcher + theme_ref binding registry portal** (docs_internal/plan/registry-theme-switcher.md, changelog 2026-08-31-005) · ✅ **Named workspaces (2.11)** (docs_internal/plan/named-workspaces.md, changelog 2026-09-07-004) · ✅ **AppSpec.Workspaces[] allowlist (2.12)** (changelog 2026-09-07-005) · ✅ **Fase 17 — auth form autofill (Chromium password-form guidance)** (docs_internal/plan/auth-form-autofill-chromium.md, changelog 2026-09-18-009)

> `⬜` not started · `✅` complete · `⏸️` deferred

**Scope**: `formspec dev` + `formspec serve --mode=production` single-server.  
**Deferred**: Control Plane (`formspec-ctl`), K8s Operator, Marketplace — untuk cloud phase berikutnya.  
**Sumber**: `docs/spec/backend/` (01–06), `docs/spec/frontend/` (01–08), `docs/spec/platform/` (01–10), `docs/cli-tools/` (01–05), `docs/renderers/jsonb-persist/` (01–04), `docs/renderers/shadcn-shell/` (01–04), `docs/ai/` (01–06).  
**Catatan status sumber**: seluruh spec masih **Draft** (jsonb-persist masih **Outline**; `platform/08` §3–§6 "target desain") — mismatch todo↔docs bisa berarti docs-nya yang perlu diperbaiki. Audit penuh todo vs docs terakhir: 2026-07-19.

**Catatan 2026-07-31**: `platform/08-project-layout.md` §1–§2 ditulis ulang agar match
layout contoh `examples/Clinic-UI-Showcase/spec/` (entity-centric + `spec/` container +
`formspec-app.yaml` sebagai config dev) — lihat `docs_internal/changelog/2026-07-31-002-update-project-layout-sesuai-clinic-ui-showcase.md`. §3–§6 tetap target desain.

**Catatan 2026-07-31**: `rtk` (CLI proxy token LLM) di-bake ke
`.devcontainer/Dockerfile` (binary pinned v0.44.1 + `rtk init -g`) karena
`~/.local/bin`/`~/.claude` tidak di-persist volume — lihat
`docs_internal/changelog/2026-07-31-003-bake-rtk-ke-devcontainer-dockerfile.md`.

**Catatan 2026-09-13**: ✅ **`rtk` dan `codebase-memory-mcp` di-uninstall** —
blok install di `.devcontainer/Dockerfile` + rantai `postCreateCommand` di
`.devcontainer/devcontainer.json` dihapus, seluruh binary/config/hook/skill/agent
keduanya dibersihkan dari host home (via bind-mount `/workspaces`) dan container
home, dan registrasi MCP di `~/.claude.json` + `~/.config/opencode/opencode.jsonc`
dicabut (catatan 2026-07-31 di atas tidak berlaku lagi) — lihat
`docs_internal/plan/uninstall-rtk-codebase-memory.md` dan
`docs_internal/changelog/2026-09-13-001-uninstall-rtk-codebase-memory.md`.

**Catatan 2026-09-13**: ✅ **App shape consultation + ekstraksi template `init`** —
skill App-authoring (`formspec-kinds` + `formspec-app-workflow`) kini
mengonsultasikan bentuk App (`access` private/public × `app_renderer`
sidebar-nav/topnav/no-nav; satu atau dua App); referensi kind di skill +
AGENTS.md scaffold diarahkan ke `https://docs.formspec.dev/kind/`; dan seluruh
hardcode `formspec init` dipindah ke `cmd/formspec/template_init/`
(embed `//go:embed all:template_init`). Lihat
`docs_internal/plan/app-shape-and-init-templates.md` dan
`docs_internal/changelog/2026-09-13-002-app-shape-and-init-templates.md`.

**Catatan 2026-09-20**: 🟡 **Fase 9 kafe — verifikasi end-to-end (9.1–9.4)**
(`examples/kafe/gaps_found/TODO.md` Fase 9). **9.1** `validate --schema ../../schemas`
→ **0 problem** (69 manifest, cocok baseline) · **9.2** `go test ./...` → **39
paket ok**, `make lint` **0 issues** · **9.3** `vitest` → **288 lulus**, `tsc`
bersih. **9.4 walkthrough** (dev server `:8099`, DB dev sudah diperbaiki 3.9):
rantai kasir terbukti lewat API — payment tunai 201 dengan `change`
`{85000 IDR}` (money arithmetic), lalu `PATCH` order
`awaiting_payment → paid → in_kitchen → ready → served → completed` semuanya
**200**, dan antrean KDS (`status[in]=paid,…`) konsisten. Dua koreksi metode yang
dicatat: transisi **tanpa `impl`** diterapkan lewat `PATCH …/{id}` `{"status":…}`
(`PUT` → 405; route `/{id}/{action}` hanya untuk action ber-`impl`), dan enum
`payment.status` = `pending|settled|failed|refunded`.
**Skenario 7 (stok/HPP) ✅ API — lewat satu bug nyata (kafe TODO 3.12, selesai):**
dua `stock-movement` in (1000@50 lalu 500@80) membalas 201 tapi proyeksi
`stock-level` **0 baris**. Log sementara satu baris di `RunAfterPhase` menunjukkan
hook **dipilih dan dijalankan** (`hooks=1 selected=1`) tetapi script gagal:
`float got money, want number or string` — `float(resource.field.unit_cost)` pada
nilai **money** (S7/1.3: operand tidak sah = error, bukan 0), dan kegagalan
after-hook **senyap bagi pemanggil** (tidak membatalkan respons; hanya tercatat di
log engine). Perbaikan: helper `money_amount()`/`money_like()` di
`stock_level_apply.star` (baca lewat `amount()`, tulis bentuk kanonik). Bukti:
`stock-level` → **1 baris** `quantity_on_hand 1500`, `moving_avg_cost {"amount":"60","currency":"IDR"}`
(= (1000×50+500×80)÷1500); validate 0 problem; `go test ./...` hijau; lint 0.
Changelog `2026-09-20-014`. Sisa kecil: script belum mengisi `stock_value`,
`last_movement_at`, `is_below_min`.
**Skenario 8 (jurnal GL) terhalang 6.3** — `/_ui/entity/gl/journal-entry` → 404
karena module `gl` tidak di-mount di App kafe dan cross-app grant memang
deferred. **Skenario 5 (shift & kas) ✅ API** — employee 201 → shift buka 201 →
kas masuk/keluar 201 → tutup shift 200 dengan **`difference = {-12500 IDR}`**
(computed). **⚠️ Temuan baru (kafe TODO 3.11 = master todo 15.7):** aturan bisnis
#10 (satu shift terbuka per cabang+kasir) **tidak ditegakkan** pada DB lama —
`_cashier_id` adalah kolom biasa hasil ALTER yang tidak pernah terisi (NULL),
sehingga partial unique index tidak menendang; shift terbuka kedua diterima 201.
**Sisa 9.4:** skenario 6 (void/approval), 7 (stok/HPP), serta bagian UI/browser
dari 1, 9. **Sisa Fase 9:** 9.4 (lanjutan), 9.5 (hapus marker `# GAP-nn`),
9.6 (update ledger/README/todo).

**Catatan 2026-09-20**: ✅ **3.9 kafe — rekonsiliasi index yang definisinya
berubah** (`examples/kafe/gaps_found/TODO.md` 3.9, temuan E2E 3.8; changelog
`2026-09-20-013`). Empat sub-masalah ditutup di
`renderers/jsonb-persist/{migrate,migrate_plan,diff}.go`: (i) diff index
membandingkan **bentuk** (`indexShape`: unique + kolom + predikat parsial)
bukan nama; (ii) `driftedIndexes` juga jalan di jalur **"checksum sama"**;
(iii) `planEntityChange` kini memeriksa drift **storage** (DDL sebelumnya hanya
dibangun dari `DiffShapes(snapshot, manifest)` — snapshot merekam niat, bukan
isi DB); (iv) `derivedColumnFields` menandai **scope field natural key**,
sehingga kolom `_branch_id` dibuat juga di jalur alter (sebelumnya
`CREATE UNIQUE INDEX … (_branch_id, …)` gagal `no such column`).
**Jebakan DX yang menyesatkan diagnosis:** `formspec migrate` default DSN-nya
`.formspec/data.db`, sedangkan `formspec dev` memakai `dsn:` dari
`formspec-app.yaml` (`.formspec/kafe.db`) — pemeriksaan awal menunjuk DB yang
salah. **Bukti:** tiga test pengunci · plan pada DB dev memuat `index_changed` +
`field_projection_changed` · `Applied 24 migration(s)` · **E2E**: order cabang B
→ `ORD-2026-00002`, cabang A → `ORD-2026-00004` (201; sebelumnya cabang B 500
`UNIQUE constraint failed`).
**Gap baru 3.10 (belum dikerjakan):** pada DB lama `migrate apply` menolak
seluruh run dengan 18 `[lossy] field_removed is_active` — `is_active` bukan field
manifest (engine menambahkannya dari `soft_deactivate`), jadi perubahannya tidak
bisa dideklarasikan; recovery sementara = buang snapshot (bootstrap).

**Catatan 2026-09-20**: ✅ **3.8 kafe — konteks sesi (principal, role, cabang)**
(`examples/kafe/gaps_found/TODO.md` 3.8; plan `docs_internal/plan/session-context-role-branch.md`).
Sesi kini selalu spesifik: satu baris `(role, dimension, value)` di
`formspec.core.user.assignments` (dimension = **nama field** yang dibandingkan
`row_scope.field`). **Login:** 0 assignment → perilaku lama (union role, tanpa
boundary); 1 → otomatis; >1 → **409 `CONTEXT_REQUIRED` + choices**; id dicabut →
409 juga (fail closed). **Token:** klaim `role` (tunggal) + `attrs{dimension: value}`;
validator mengabaikan `roles` bila `role` ada — daftar basi tidak bisa melebarkan
sesi. **Permission** memakai role terpilih **saja**; **refresh** memvalidasi ulang
assignment+role (`contextStillValid`) dan gagal = minta pilih ulang;
**`POST /_ui/auth/switch`** menerbitkan pair baru sambil me-revoke sesi lama.
Entity `session` menyimpan role + dimensi + nilai; `SetAssignments` untuk admin.
**Bukti E2E** (user `kasir2` = `sales@cabangA` + `admin@cabangB`): login tanpa
pilihan → 409 + choices; `sales@<A>` → `role=sales`, `roles=["sales"]`,
`attrs={branch_id: A}`, list order `total: 2` **hanya cabang A**; `admin@<B>` →
`total: 0`. Test `internal/auth/context_test.go` (11 kasus; akun tanpa assignment
tetap union = aditif) · `go test ./...` hijau · `make lint` **0 issues** · kafe
`validate` 0 problem. Changelog `2026-09-20-012`.
**Sisa:** tahap 4 plan (layar pemilih + pengalih + `localStorage`) dan adopsi kafe
(seed `employee.assignments` → `user.assignments`).
**Temuan E2E (gap baru 3.9):** membuat order di cabang kedua pada DB lama gagal
`UNIQUE constraint failed: … tenant_id, _number` — index di DB itu masih bentuk
pra-3.6 `(tenant_id, _number)`; schema sync tidak merekonsiliasi index yang
**definisinya berubah**. Dicatat sebagai item 3.9.

**Catatan 2026-09-20**: ✅ **2.6 kafe — QR di dokumen cetak + money terformat**
(`examples/kafe/gaps_found/TODO.md` 2.6; plan `docs_internal/plan/kafe-sisa-gap.md`).
`PrintBodyItem` mendapat body item **`qrcode`** (`payload` ber-token
`{dotted.path}`, `label`, `absolute`, `size_mm`); `absolute: true` menambahkan
**origin dokumen** (browser: origin halaman; server: scheme+host request) —
origin adalah pengetahuan deployment, bukan data entity. **Tiga pipeline
merender:** `html` (widget `QrCode`), `pdf` (PNG `skip2/go-qrcode` ditanam),
`thermal` (QR native ESC/POS `GS ( k`). Aturan jujur: token yang tak
ter-resolve ⇒ **elemen dihilangkan** (order kasir tanpa `guest_token` tercetak
tanpa QR; jalur QR memakai `interpolateStrict` karena interpolasi biasa
mengubahnya jadi `/status/` yang tampak valid), `payload` kosong literal ditolak
schema (`minLength: 1`). Sisa **#1** untuk print ikut tertutup:
`spec.FormatMoneyDisplay` + `printContext` menampilkan `Rp62.500`, bukan
`map[amount:25000 currency:IDR]`. **Bug generator schema ditemukan & ditutup:**
`Print.schema.json` meng-emit `$ref #/$defs/PrintQrcode` tanpa def-nya →
**semua manifest Print gagal compile**; guard `TestGeneratedKindSchemas_HaveNoDanglingRefs`
buta terhadapnya (hanya menelusuri schema kind, bukan isi `$defs`) dan kini
menelusuri seluruh `$defs` — **dibuktikan gagal lebih dulu** sebelum diperbaiki.
**Adopsi kafe:** QR di kedua struk (`receipt-thermal`, `receipt-digital`) →
`/kafe/status/{guest_token}` (halaman publik yang hidup), GAP-03 ditutup.
**Bukti E2E:** struk thermal 200/octet-stream 503 byte dengan `ESC @` + `GS ( k`

- payload absolut + money `Rp62.500`… tanpa `map[amount`; order tanpa token →
  tanpa QR; `receipt-digital` → 501 eksplisit (html dirender klien). Test:
  `go test ./...` hijau · `vitest` **288** · `tsc` bersih · `make lint` 0 issues ·
  kafe `validate --schema ../../schemas` **0 problem**. Changelog `2026-09-20-011`.
  **Sisa:** kartu meja QR = item **2.15 ⏸️** (butuh halaman masuk token→sesi,
  bukan jalur cetak). **Sisa Fase 2: nol** (2.1–2.14 ✅, 2.15 deferred).

**Catatan 2026-09-20**: ✅ **Fase 8 kafe (8.2, 8.4, 8.6) — dokumen, DX & artefak**
(`examples/kafe/gaps_found/TODO.md` Fase 8; plan `docs_internal/plan/kafe-sisa-gap.md`).
**8.4** (`internal/devserver/devserver.go`) — pesan error port kini menuntun: port
sibuk menyebut pemilik + PID dan dua cara lanjut (`kill <pid>` atau
`--addr :<port+1>`); error "owner tak teridentifikasi" juga menawarkan `--addr`.
Test baru `internal/devserver/devserver_test.go` (`TestEnsurePort_*`). Changelog
`2026-09-20-008`. **8.2** — drift dokumen `#19`/`#20` ditutup:
`docs/renderers/shadcn-shell/03-kind-renderers.md` (masih bertanggal 2026-07-16)
& `docs/renderers/realtime.md` diselaraskan dengan kode (registry dihapus,
`Form.render` dihormati, Dashboard/Widget & Report nyata, katalog 24+4 widget,
`ConfirmDialog`; realtime kini 7 kind). Klaim `spec.version` **gugur**
(`EntitySpec.Version`/`ModuleSpec.Version` memang ada & wajib; `formspec-app.yaml`
mang config CLI) — drift nyatanya `apiVersion`: `formspec/v1` di 3 dokumen +
`const spec.APIVersion` = `v1alpha1` (keduanya **ditolak** `schemaregistry.ParseVersion`)
→ kini `formspec.dev/v1` + guard `TestAPIVersionIsStable`. Changelog `2026-09-20-009`.
**8.6** — `make generate-schema` (162 shared types) + `make generate-kind-docs`
(33 kind docs) dijalankan; `make generate` **masih stub** (no-op, dicatat apa
adanya). Regenerasi **konvergen** (`md5sum -c` OK pada run kedua); churn tepat
perubahan yang belum di-generate (S10 `$defs`, `Timeline.realtime`,
`Form.autocomplete`). Cacat generator ikut ditutup: deskripsi terpotong karena
generator memakai baris pertama komentar Go — audit seluruh schema → 0 temuan.
Changelog `2026-09-20-010`. **Verifikasi batch:** `go build ./...` exit 0 ·
`go test ./...` hijau · `make build` hijau · kafe `validate --schema ../../schemas`
**0 problem** (69 manifest). **Sisa Fase 8: nol** (8.1–8.8 ✅).

**Catatan 2026-09-20**: ✅ **Fase 7 kafe (7.1–7.4) — struk & laporan**
(`examples/kafe/gaps_found/TODO.md` Fase 7). **7.1** — `internal/api/print.go`
kini bercabang pada `output.format`: `thermal` → `renderPrintThermal` (ESC/POS),
`pdf` → PDF, `html` → klien, lainnya → 501 (sebelumnya SELALU PDF, jadi
`format: thermal` lolos validasi tapi menghasilkan PDF). **7.2** — `ReportColumn`
mendapat `widget` + `aggregate`/`format` jadi himpunan tertutup
(`ReportAggregate`/`ReportFormat`), dokumentasi berdampingan di
`06-page-kinds.md`. **7.3** — lookup widget module-qualified (`byQualified` di
`stores/meta.ts`). **7.4** — `TimelineSpec.realtime` + `useRealtime` di
`TimelineRenderer`. Bukti: `TestRenderPrintThermal`,
`TestReportColumn_ClosedSets`, `meta.test.ts`; kafe `validate` 0 problem;
`go test ./...` hijau; `vitest` 276; `make lint` 0 issues. Changelog:
`2026-09-20-006`, `2026-09-20-007`.

**Catatan 2026-09-20**: ✅ **Fase 6 kafe (6.1–6.5) — akuntansi & integrasi
lintas-app** (`examples/kafe/gaps_found/TODO.md` Fase 6). **6.1** —
`TransitionDecl.Emit` + `ValidateTransitionEmits` + `ResolveTransitionEmission`
(dipancarkan di `HandleUpdate`): keterkaitan transisi↔event kini eksplisit dan
terverifikasi, bukan disimpulkan dari penamaan. **6.2** — `IntegratorCall.Map` +
`applyCallMap` (interpolasi template `{dotted.path}`, nilai satu-token
mempertahankan tipe): pemetaan payload dinyatakan di manifest, bukan di script
modul target. **6.5** — keputusan tertulis: kafe **tetap model sendiri** untuk
purchase (sudah lengkap: `supplier`/`purchase-order`/`receive-goods`); vertical
reusable adalah keputusan produk yang lebih besar. **6.3** (cross-app grant +
SyncAgent) & **6.4** (kepemilikan `publishes`) **DEFERRED** dengan alasan: 6.3
butuh Control Plane (cloud phase), 6.4 butuh keputusan desain pemilik proyek.
Bukti: `TestResolveTransitionEmission`, `TestValidateEntitySpec_TransitionEmits`,
`TestApplyCallMap`; kafe `validate` 0 problem; `go test ./...` hijau; `make lint`
0 issues. Changelog: `2026-09-20-005`.

**Catatan 2026-09-20**: ✅ **Fase 5 kafe (5.1–5.5) — kas, shift, void &
approval** (`examples/kafe/gaps_found/TODO.md` Fase 5). Lima item: **5.1**
(partial unique shift) & **5.2** (void multi-state-asal) sudah tertutup oleh 1.6
& 1.7 — diverifikasi ulang dengan test yang bisa gagal. **5.3** — `WorkflowStep`
mendapat `title`/`description`/`display_fields` (S15), dan validator menolak
`display_fields` yang menunjuk field yang tidak ada; kafe `order-void-approval`
mengadopsinya. **5.4** — `FormRenderDecl` schema kini `oneOf: [string, object]`,
menutup divergensi loader↔schema (`render: drawer` ditolak schema padahal loader
menerimanya). Kelas yang sama untuk `GuardDecl` (`guard:` inline, bentuk kanonis
Core Extended §14) ditutup **2026-10-06** — changelog `2026-10-06-004`. **5.5** — aturan simetri cancel 7.7.2 didokumentasikan di
`02-core-extended.md` §5 (mengapa + bagaimana + contoh pasangan Integrator).
Bukti: `TestValidateWorkflows_DisplayFieldsMustExist`,
`TestFormRenderDecl_AcceptsShorthandAndObject`,
`TestMigrationRunner_UniqueIndexRejectsDuplicates`, `TestRegistry_ForTransitionByName`;
kafe `validate` 0 problem; `go test ./...` hijau. Changelog: `2026-09-20-002`.
**Sisa 5.3:** wiring runtime ApprovalInbox (engine mengisi item dari step) —
renderer sudah menampilkan `item.title` bila ada.
✅ **DITUTUP 2026-10-04:** sumbernya bukan entity, jadi diselesaikan lewat
endpoint `/_ui/workflow/approvals` (5.13.6 → `[x]`, changelog `2026-10-04-004`).
Yang masih terbuka hanya `realtime` (**5.13.7 ⏸️**).

**Catatan 2026-09-20**: ✅ **4.1 / #13 — jalur tulis summary untuk script
pemelihara (`resource.upsert`, Opsi A)** (`examples/kafe/gaps_found/TODO.md`
4.1). Saat mulai dikerjakan ditemukan celah arsitektur: entity `summary` tidak
punya jalur tulis yang didukung (`EntityStore` menolak `CharSummary`;
`maintained_by` script tidak bisa menulis proyeksinya; engine rebuild hanya
merencanakan). **Keputusan pemilik proyek: Opsi A.** Yang dikerjakan:
`EntityStore.UpsertProjection` (upsert by match, hanya summary, atomik, tidak
diekspos ke API) + primitif Starlark `resource.upsert(entity, match, data)` +
aturan pemanggil (hanya script `maintained_by` entity itu). Adopsi kafe:
`stock_level_apply.star` ditulis ulang tanpa SQL, dipanggil lewat hook
`after create` pada `stock-movement`. Dua bug ikut ketemu & diperbaiki:
`UpsertProjection` menulis kolom `is_active` yang tidak ada; `FindByFields`
mengembalikan `ErrNotFound` alih-alih `nil` saat tidak ada baris. Bukti:
`TestUpsertProjection_*`, `TestFindByFields_NoMatchReturnsNil`,
`TestResourceAPI_Upsert_*`, `TestSummaryUpsert_MaintainerWritesProjection`
(5+3=8), `TestSummaryUpsert_NonMaintainerRefused`; kafe `validate` 0 problem;
`go test ./...` hijau. Plan: `docs_internal/plan/summary-maintainer-write-path.md`
· changelog: `2026-09-20-001`.

**Catatan 2026-09-18**: ✅ **Fase 0 kafe selesai — re-verifikasi penuh ledger
(#1–#53, S1–S16) + klasifikasi ulang** (`examples/kafe/gaps_found/TODO.md` 0.1,
0.2, 0.5). Hasil: **24 CLOSED · 3 PARTIAL · 22 OPEN · 1 RETIRED** untuk gap, dan
**10 CLOSED · 2 PARTIAL · 4 OPEN** untuk S-item. Re-verifikasi menemukan **nol
koreksi status** (semua ✅ memang tertutup, semua 🔴 memang terbuka) — berbeda
dari Fase 0 pertama yang membatalkan #7/#24/separuh #2/separuh #18. Klasifikasi
final: SPEC 5 · ENGINE 12 · DOC 6 · PERILAKU 0 · PARTIAL 5. Bukti:
`examples/kafe/gaps_found/14-temuan-fase-0.md` §7–§8 · changelog
`2026-09-18-003`.

**Catatan 2026-09-18**: ✅ **Fase 4 kafe (4.2–4.8) — stok, HPP & pembelian**
(`examples/kafe/gaps_found/TODO.md` Fase 4; plan
`docs_internal/plan/fase-4-kafe-stok-hpp.md`). Tujuh item selesai:

- **4.2** — entity `summary` **menolak** `hooks:`/`conditions:` (validator),
  karena penulisan summary tidak lewat action pipeline → manifest yang terlihat
  terlindungi padahal tidak. Changelog `2026-09-18-004`.
- **4.7** — `HookDecl.Uses` + registrasi + honesty scan: akses script hook kini
  terlihat di consent footprint; kafe mendeklarasikan `uses: {primitives: [db]}`
  pada 3 hook guard. Changelog `2026-09-18-005`.
- **4.5** — `ctx.db()` di dalam transaksi aksi tidak lagi deadlock di SQLite
  (`DBQuerier.Query` memakai `TxReadDB`). Test diuji bisa gagal. Changelog
  `2026-09-18-006`.
- **4.3** — `resource.find(entity, {field: value})`: API find-by-field, dua guard
  keunikan kafe ditulis ulang **tanpa SQL**. Changelog `2026-09-18-007`.
- **4.4** — keunikan atomik: jawaban kanonik = `indexes:` (database), bukan
  `ctx.lock`; dibuktikan constraint menolak duplikat (termasuk parsial).
  Changelog `2026-09-18-008`.
- **4.6** — `unit.factors` + `ctx.unit.convert()`: konversi satuan (gram↔kg)
  dihitung engine. Changelog `2026-09-18-009`.
- **4.8** — verifikasi agregasi `money` pada bentuk laporan stok kafe.
  Changelog `2026-09-18-010`.
  **4.1 (valuasi inventory) BLOCKED** — butuh keputusan desain: entity `summary`
  tidak punya jalur tulis yang didukung (`EntityStore` menolak `CharSummary`;
  `maintained_by` script tidak bisa menulis proyeksinya; engine rebuild hanya
  merencanakan). Lihat TODO 4.1.

**Catatan 2026-09-16**: ✅ **3.7 / #11+#12 — relasi: guard tidak lagi lolos senyap**
(`examples/kafe/gaps_found/TODO.md` 3.7). Akarnya satu: relasi yang tidak bisa
diresolusi diperlakukan sebagai "tidak ada yang perlu diperiksa". (a) **Runtime:**
`ValidateRelationTargets` dulu `continue` saat target tidak ditemukan **atau**
tabelnya tidak ada — sehingga referensi menggantung **diterima** dan nama tabel
yang salah membuat guard-nya dilewati; kini ketiganya dibedakan dan jadi error
bernama (target tak teresolusi / baris tidak ada / tabel tak terbaca), sementara
relasi opsional yang tidak diisi tetap lolos. (b) **Statik:** gerbang
cross-manifest `validateRelations` menolak `relation.resource` yang tidak
menunjuk entity terdaftar (termasuk di dalam `child`) dan relasi yang melintasi
`persist.category` — yang di runtime hanya memblokir sambil menulis log. Bukti: 4
kasus validator + 3 kasus runtime; `go test ./...` hijau; kafe `validate` 0
problem (seluruh relasi lintas module kafe memang resolve dengan nama). Plan:
`docs_internal/plan/relation-guard.md` · changelog: `2026-09-16-011`.

**Catatan 2026-09-16**: ✅ **3.6 / #9 — natural key ber-scope: scope menembus
counter, keunikan, dan DDL** (`examples/kafe/gaps_found/TODO.md` 3.6). Item ini
melebar dari perkiraan: saat diuji, cabang kedua **gagal 500** `UNIQUE constraint
failed`. Nilai scope harus menembus **tiga** lapis, bukan satu — (a) counter
(`ctx.next_key` mengirim scope kosong di jalur script; kini menerima
`scope=<nilai>` dan **menolak** mencetak nomor tanpa scope saat rule-nya
ber-`scope_field`), (b) **keunikan** (index uniknya `(tenant_id, _number)` tanpa
cabang → deret B2 yang mulai dari 1 menabrak B1; kini `(tenant_id, _branch_id,
_number)`), dan (c) **DDL** (scope field belum punya kolom turunan → index gagal
dibuat; generator kini membuatnya). Bukti runtime: empat create anonim (B1, B2,
B1, B2) → `ORD-2026-00001` di **kedua** cabang lalu `00002` di keduanya, semua
`201`; sebelumnya B2 → `500`. Unit: 3 test counter (+ fixture baru) + 2 test
Starlark. Plan: `docs_internal/plan/natural-key-scope.md` · changelog:
`2026-09-16-010`.

**Catatan 2026-09-16**: 📐 **3.8 — konteks sesi `(principal, role, cabang)`**
(desain disetujui pemilik proyek; belum diimplementasikan). Sesi selalu spesifik:
siapa, sebagai **role apa**, di **cabang mana** — bukan dua daftar terpisah,
melainkan satu daftar **pasangan (role, cabang)** (`sales@A`, `admin@B`). Login
memilih satu (otomatis bila hanya satu); OAuth memakai pilihan **terakhir** yang
disimpan **per-device di klien**; pengguna bisa pindah kapan saja lewat endpoint
switch. **Permission = grant role yang dipilih**, bukan union — itulah yang
membuat boundary-nya spesifik dan audit bisa menjawab "sebagai role apa, di cabang
mana". Ini **menggantikan** rencana lama untuk sisa 3.5 (daftar nilai + `op: in`):
pilihan konteks lebih baik karena boundary-nya tunggal, dan ia menutup multi-cabang
**dan** multi-role sekaligus. Efek samping: nilai cabang dibawa sesi, sehingga
`row_scope: from session` tidak lagi lewat resolusi `assignments` di jalur kritis
tiap request (jadi fallback saja). Rencana: `docs_internal/plan/session-context-role-branch.md`
(model, alur login/OAuth/switch, berkas yang disentuh, 5 tahapan, bukti yang
diminta) · item ledger: `examples/kafe/gaps_found/TODO.md` **3.8**.

**Catatan 2026-09-16**: ✅ **3.5 / #8+S5 — `row_scope` dinyalakan: isolasi cabang
ditegakkan engine** (`examples/kafe/gaps_found/TODO.md` 3.5). Konstruknya sudah
lengkap sejak 1.1 (`row_scope`) dan 1.8 (`assignments` + sumber nilai) plus
keputusan `read_all`; yang kurang hanya spec kafe belum memasangnya. Kini
`row_scope: from session` dipasang di **10 entity** yang dibaca hanya lewat
permukaan terautentikasi (`order`, `payment`, `shift`, `cash-movement`,
`stock-level`, `stock-movement`, `purchase-order`, `stock-opname`, `waste-entry`,
`menu-cost`). **Aturan baru yang wajib ada:** `row_scope` menyeluruh mem-403
anonim (tak punya atribut sesi, dan itu memang fail closed), jadi `applyRowScope`
melewati permintaan anonim yang datang lewat grant publik **ber-scope** — di situ
`applyPublicScope` yang membatasi. Celah terakhir ikut ditutup: grant publik
`menu-item-price` kini ber-scope `branch_id from route` (sebelumnya daftar harga
anonim mengembalikan **seluruh cabang** tanpa parameter). Bukti runtime (token dev
asli): kasir B1 → hanya `B1-1`; B1 + `?branch_id[eq]=B2` → **tetap `B1-1`**; kasir
B2 → hanya `B2-1`; pemilik (`.list`+`.read_all`, tanpa atribut) → **kedua cabang**;
tanpa atribut → **403**; anonim harga tanpa param → **403**, dengan → `200`.
Nuansa tercatat: `read_all` **tanpa** `.list` → **404** (gerbang permission, bukan
scope). Test: 3 test koherensi spec. Plan:
`docs_internal/plan/row-scope-enforced.md` · changelog: `2026-09-16-009`.

**Catatan 2026-09-16**: ✅ \*\*3.4 / #35+#36 — `kind: Migration`: `ddl_by` per-driver

- `dml` yang dinyatakan** (`examples/kafe/gaps_found/TODO.md` 3.4). Dua celah
  yang penutupnya sengaja dipisah: (#35) ekspresi JSONB berbeda antar driver —
  `json_extract(data,'$.x')` vs `data->>'x'` — sehingga satu string `ddl` benar di
  dev dan **salah di produksi** dengan kegagalan yang baru muncul saat deploy; kini
  `ddl_by` memuat varian per driver (dialek himpunan tertutup), menulis keduanya
  ditolak, dan driver tanpa varian melewati migration itu **dengan peringatan**.
  (#36) `CREATE UNIQUE INDEX` gagal bila duplikat sudah ada — dan duplikat itu
  muncul justru karena constraint-nya belum ada — sementara perbaikannya butuh DML
  yang ditolak; kini `dml` boleh dinyatakan dengan **`reason` wajib**, hanya DML,
  dijalankan **sebelum\*\* DDL, dan diumumkan saat apply/di `plan`. Bukti:
  `TestApplyCustomMigrations_DataRepairRunsBeforeDDL` (constraint gagal tanpa
  perbaikan → berhasil setelahnya, 3→2 baris, duplikat berikutnya ditolak),
  `TestLoadCustomMigrations_PicksDialect`, `TestValidateMigrationSpec` (6 bentuk
  ditolak), `go test ./...` hijau, kafe `validate` 0 problem. Normatif:
  `01-core-basic.md` §4.1. Plan: `docs_internal/plan/migration-dialect-and-dml.md` ·
  changelog: `2026-09-16-008`.

**Catatan 2026-09-16**: ✅ **3.2 / #23 — kolom turunan `money` numerik**
(`examples/kafe/gaps_found/TODO.md` 3.2, mulai Fase 3). Akarnya bukan sekadar
tipe kolom: kolom turunan menyimpan **objek** `{amount, currency}` sebagai teks
JSON, sehingga `"9000"` dianggap lebih besar dari `"10000"` dan index di atasnya
hanya mempercepat jawaban yang salah. `generateGeneratedColumn` kini menerima
tipe field; untuk `money` ekspresinya membaca `.amount` dan tipenya
`numeric(20,8)`; `fieldTypeToSQL` mendapat `case money`; jalur ALTER di
`migrate.go` ikut konsisten; dan `columnRefExpr` mendahulukan kolom turunan
sekarang kolomnya numerik (kalau tidak, index tidak akan terpakai). Bukti: DDL
nyata `_price numeric(20,8) GENERATED ALWAYS AS (CAST(json_extract(data,
'$.price.amount') AS REAL)) STORED` + index-nya, plus 2 test perilaku (sort naik
9000 sebelum 10000; `>= 9500` → hanya 10000). Adopsi kafe menghapus workaround:
`menu-item-price.price` + `order.total_amount` kini `index: true`, kolom Total di
table POS `sortable`. Normatif `05-field-types.md` §2.2. Plan:
`docs_internal/plan/money-derived-column-numeric.md` · changelog:
`2026-09-16-007`.

**Catatan 2026-09-16**: ✅ **2.8 / #48 — workspace aktif: dipakai kalau tunggal,
diperingatkan kalau ambigu** (`examples/kafe/gaps_found/TODO.md` 2.8).
`kind: Workspace` adalah _seed declaration_: ia mendaftarkan slug, bukan
memilihnya — workspace aktif datang dari `--workspace-id`. Salah membacanya tidak
memunculkan error apa pun: mendeklarasikan `kafe` lalu menjalankan dev tanpa flag
membuat semua tulisan tersimpan `tenant_id: "default"` sementara `GET /kafe/...`
menjawab `200` dengan nol baris. Tiga keadaan kini dibedakan: **tepat satu**
dideklarasikan → dipakai + diumumkan; **lebih dari satu** → tetap `default` plus
peringatan berisi daftar (memilih diam-diam mengejutkan); **flag yang tidak
dideklarasikan** → diperingatkan. Pembeda "diberikan pengguna" vs "nilai default"
ditambahkan sebagai `DevConfig.WorkspaceIDExplicit`. Bukti: dev pada spec kafe
mencetak `workspace: kafe (the only one declared…)`, 4 kasus
`TestResolveActiveWorkspace`, `go test ./...` hijau, kafe `validate` 0 problem.
Dokumen `platform/02-workspace-app-module.md` §1 menyatakan eksplisit
"mendaftarkan, bukan memilih". Plan:
`docs_internal/plan/active-workspace-resolution.md` · changelog: `2026-09-16-006`.
**Dengan ini Fase 2 tuntas.**

**Catatan 2026-09-16**: ✅ **2.7 / #47 — kontrak REST `/_ui/` terdokumentasi**
(`examples/kafe/gaps_found/TODO.md` 2.7). Surface `/_ui/` adalah kontrak publik
(SPA bawaan memakainya) tetapi bentuk body/envelope/endpoint-nya hanya ditemukan
dengan gagal berkali-kali. Dua bagian: halaman
`docs/runtimes/06-ui-rest-contract.md` (path singular, body flat — envelope
`{"data": …}` ditolak 400, tiga envelope respons, query list, catatan bahwa
parameter `row_scope` bukan filter, ringkasan surface publik), dan
**`formspec describe entity <name>` mencetak kontrak HTTP-nya dari generator yang
sama dengan server**. Yang kedua menuntut refactor kecil:
`GenerateUIRoutes`/`GenerateUICustomActionRoutes` dipecah jadi per-entity
(`UIRoutesForEntity`, `UICustomActionRoutesForEntity`) sehingga CLI bisa
memakainya tanpa database dan kedua jalur tidak bisa berbeda. Kontrak itu penuh
pengecualian yang mudah salah ditulis tangan — aksi `disabled` tanpa route,
lifecycle-free tanpa `submit`/`cancel`/`amend`, `summary` hanya `list`+`find`,
dan transisi state machine tanpa `impl` **tidak** punya endpoint (lewat `update`).
Bukti: `describe entity order` mencetak tepat 4 route (sesuai spec kafe yang
mematikan `delete`+`submit`), 2 test baru, `go test ./...` hijau, kafe `validate`
0 problem. Plan: `docs_internal/plan/ui-rest-contract.md` · changelog:
`2026-09-16-005`.

**Catatan 2026-09-16**: ✅ Keputusan untuk **3.5** diambil — `{module}.{plural}.read_all`.
Pertanyaan "apakah pemilik/super-admin tanpa baris `employee` boleh melihat semua
cabang" dijawab dengan **permission eksplisit**, bukan bypass `*` implisit dan
bukan nilai wildcard di atribut. `row_scope` tetap fail closed untuk semua orang
lain; pemegang `read_all` dilewati dari scope entity itu (atribut yang tak bisa
diresolusi bukan lagi error, karena bagi mereka itu keadaan normal), permission-nya
**didaftarkan** sehingga bisa diberikan dan terlihat di audit, `*` juga memenuhi
(identitas dev tetap hidup), dan cakupannya per entity. Penghambat 3.5 hilang:
spec kafe bisa memasang `row_scope` dan cukup memberi role pemilik `read_all` per
entity. Bukti: `TestApplyRowScope_ReadAllPermissionExempts` +
`TestReadAllPermission`, `go test ./...` hijau, kafe `validate` 0 problem. Plan:
`docs_internal/plan/read-all-scope-exemption.md` · changelog: `2026-09-16-004`.

**Catatan 2026-09-16**: ✅ **2.14 / #1 — widget `moneyinput` + `timeinput`**
(`examples/kafe/gaps_found/TODO.md` 2.14, separuh renderer dari gap #1). 1.4
menutup akarnya (kosakata tertutup) tetapi widget-nya belum ada, jadi `money` dan
`time` tetap jatuh ke input teks polos. Nama kanonik mengikuti keluarga yang ada
(`fileinput`/`datetimeinput`/`decimalinput`): **`moneyinput`**, **`timeinput`**.
Nilai money disimpan sebagai **teks** selama mengetik (uang eksak, tidak pernah
float) dan dikirim sebagai bentuk kanonik `{amount, currency}`; tampilan ikut
`settings.currency`/`locale`, override per field (`currency`, `decimal_places`)
dihormati, `inputMode: decimal` untuk numpad. `timeinput` memakai kontrol
`type=time` asli dan menyimpan `HH:MM:SS`. **Detail yang membuatnya sampai ke
kasir:** manifest form tidak menulis `widget:`, dan `FormFieldWidget` memakai
`field.widget ?? entityField.type` — jadi `money` akan tetap jadi input teks;
fallback itu kini lewat `implicitWidgetForType` (`money → moneyinput`,
`time → timeinput`), sengaja sempit. Marker `GAP-01` di seluruh spec kafe ditutup
(5 entitas + 4 form + 1 wizard), termasuk dibersihkan dari komentar gabungan
`GAP-01/GAP-02`. Bukti: `vitest` 265 (dari 258), `tsc` bersih, `go test ./...`
hijau, kafe `validate` 0 problem, FormWidget 24 nilai. Plan:
`docs_internal/plan/money-time-input-widgets.md` · changelog: `2026-09-16-003`.

**Catatan 2026-09-16**: 🟡 **2.6 / #3+S4 — widget `qrcode`** (sebagian;
`examples/kafe/gaps_found/TODO.md` 2.6). Keputusan pemilik proyek: jalur termurah
— widget read-only + dependency klien (`qrcode.react`, MIT, SVG supaya tajam saat
dicetak), bukan field type baru dan bukan Service engine. `qrcode` masuk **dua**
kosakata tertutup (S10) dengan nama sama: `FormWidget` (form — menggantikan
input, sebab nilainya ADALAH payload) dan `TableCellWidget` (sel tabel/listing);
ditambah komponen `QrCode.tsx`, cabang di `FormFieldWidget` **dan**
`renderCellValue`, barrel, dan enum schema ter-regenerasi. Bukti: test kosakata
tertutup + paritas `catalog.test.tsx`, `go test ./...` hijau, `vitest` 258,
`tsc` bersih. **Sengaja TIDAK ditandai selesai:** accept-nya menuntut "dirender &
**dicetak**", dan jalur cetak belum ada (`Print` memakai `resolveCellValue()`
yang menjadikan nilai teks — bagian 7.1), sementara adopsi kafe terhalang hal
konkret: QR yang bisa dipindai butuh URL absolut, dan `dining-table` hanya
menyimpan kode meja. Changelog: `2026-09-16-002`.

**Catatan 2026-09-16**: ✅ \*\*2.5 / #4+#4b — gambar tampil di cell/listing/detail

- kanonik `allowed_types`** (`examples/kafe/gaps_found/TODO.md` 2.5). Akarnya
  adalah #4b: bentuk `allowed_types` yang **didokumentasikan** (ekstensi tanpa
  titik, `[jpg]`) adalah satu-satunya yang **tidak cocok dengan apa pun\*_ —
  matcher klien maupun server hanya mengenal `.jpg`, MIME, dan `image/_` — jadi
spec yang benar menghasilkan "File type not allowed". Kini kanoniknya ekstensi
tanpa titik, keempat ejaan diperlakukan sama oleh matcher yang sepasang
(`internal/api/file.go`+`src/lib/media.ts`), dan validator menolak entri di
luar keempat bentuk itu. Lalu #4: `renderCellValue`punya cabang`widget: image`(nilai file = object key → route unduh sebagai`src`), `image`masuk kosakata tertutup`TableCellWidget`(S10) dengan paritas schema↔katalog↔
renderer, turunan otomatis untuk field file bergambar, Table/Listing meneruskan
URL lewat`CellRenderOpts.imageUrl`, DetailPage merender `<img>`, dan satu
helper `fileDownloadUrl`menggantikan URL yang disusun sendiri oleh`PickerPanel`
/`FileInput`. Bukti: `pkg/spec`5 test,`TestAllowedFileType`8 case, klien`media.test.ts`8 case,`vitest`258, kafe`validate`0 problem. Sisa:
verifikasi runtime di browser, Print (butuh URL absolut), dan verifikasi`transform`thumbnail. Plan:`docs_internal/plan/media-image-cells.md`·
changelog:`2026-09-16-001`.

**Catatan 2026-09-16**: ✅ **7.17.8 — object storage pindah ke Garage (driver
default), MinIO tetap didukung.** MinIO digantikan Garage sebagai object store
bawaan dev container (`dxflrs/garage:v2.4.1` + `--single-node --default-bucket`,
S3 di `:3900`, `garage.toml` baru, kredensial `GARAGE_DEFAULT_*` di `.env`,
port forward 19000/19003), dan driver `garage` menjadi default
(`spec.DefaultStorageDriver`). Karena Garage/MinIO/S3 berbicara API yang sama,
client S3 diekstrak ke `datastore/s3store` (beserta `Stat`/`Delete`/`Link`/
`ChunkUploader`) dan `datastore/garage` + `datastore/minio` menjadi wrapper
tipis yang hanya mem-pin default endpoint/port/bucket/region — perbedaan
driver jadi satu field `spec.driver`. `go.mod` tetap memakai `minio-go` (SDK
client S3, bukan server MinIO). Bukti: `go build ./...` bersih,
`TestDatastoreRegistry_ObjectStorageDrivers` (3 subtest) + suite
`./resource/` hijau. Plan:`docs_internal/plan/garage-object-storage-driver.md`·
changelog:`2026-09-16-013`.

**Catatan 2026-09-15**: ✅ **2.2 / #45 — `public_entities[].scope`: scoping baris
per-permukaan** (`examples/kafe/gaps_found/TODO.md` 2.2). Allowlist per-entity
(1.2) membatasi entity/aksi, bukan baris — sehingga `list` pada `order` tidak
bisa diberikan sama sekali dan pelanggan tak punya jalan membaca pesanannya
sendiri. Grant kini bisa mendeklarasikan `scope` (hanya `from: route`, tidak bisa
digabung `find`) yang nilainya dibaca **server** dari parameter request; parameter
yang tak ada menolak permintaan, dan nilai menimpa filter klien. **Dua bug ikut
tertutup** karena jalur ini menyentuhnya: (a) **grant publik adalah bypass
permission** — route publik menyetel permission `"public"`, sehingga pemanggil
yang login melewati permission entity di route bersama `/_ui/entity` (memberi
anonim `list` pada `order` akan mencabut gerbang POS/KDS); kini grant hanya
mengizinkan anonim, pemanggil terautentikasi tetap wajib ber-permission dan tidak
terkena scope anonim. (b) parameter scope grant diparse sebagai filter field
(422), sama seperti perbaikan 1.1 untuk `row_scope`. Adopsi kafe: `order.guest_token`
(dari sesi meja) + grant `create,list` di `kafe-qr` + halaman status pelanggan;
komentar GAP-06 di halaman itu dihapus. Bukti runtime: tanpa token → 403; TOKENA →
hanya TOKENA; TOKENB → hanya TOKENB; TOKENA + `guest_token[eq]=TOKENB` → tetap
hanya TOKENA; token salah → 0 baris. Plan:
`docs_internal/plan/public-grant-row-scope.md` · changelog: `2026-09-15-011`.

**Catatan 2026-09-18**: 🔧 **2.2 regresi — validasi `public_entities[].scope` hilang, dipulihkan.** Commit `ceaaf2a` ("many updates") menghapus blok validasinya di hunk `ValidateAppSpec` (`@@ -1204,28 +1102,6 @@`), sementara test-nya tetap ada — `go test ./pkg/spec/` **merah di HEAD** untuk tiga kasus. Yang **selamat** dan sudah diverifikasi: penegakan runtime (`applyPublicScope` di `internal/api/handler.go`, `PublicScope` di `descriptor.go`, wiring `router.go`). Jadi yang hilang hanya gerbang `formspec validate`, dengan konsekuensi persis pola berulang di ledger: manifest bisa menyatakan `scope` yang tidak masuk akal → lolos validate → gagal (atau terlihat aman) saat runtime, bukan saat spec ditulis.
**Bukti.** `TestValidateAppSpec_PublicEntities_Scope` hijau · `go test ./...` seluruhnya hijau · `make lint` **0 issues** (juga dengan `--max-same-issues 0 --max-issues-per-linter 0`) · **uji negatif pada spec kafe nyata** (salinan di `/tmp`, tiga varian, ketiganya ditolak `formspec validate`): `from: session` → _"must use `from: route`"_; `scope` tanpa `field` → _"scope[0]: field is required"_; `find` + `scope` → _"cannot grant `find` together with `scope`"_; spec asli tetap **0 problem**. Changelog: `2026-09-18-001`.

**Catatan 2026-09-15**: ✅ **1.8 / S5+S11+S12+S14 — konstruk pelengkap**
(`examples/kafe/gaps_found/TODO.md` 1.8) dan **1.9 — D1–D7 normatif** (item 1.9).
Tiga konstruk menggantikan satu nama di S5: `scope: {dimension, field, required}`
(entity dipartisi — fakta data, **tidak** memfilter), **`row_scope`** (nama baru
untuk filter yang ditegakkan; dulu bernama `scope`, dan rename-nya murah karena
tak ada spec yang memakainya — satu nama tidak bisa dua bentuk, preseden 1.7),
dan `assignments: [{dimension, field, principal_field}]` (**dari mana** nilai
dimensi seorang principal berasal — bagian S5 yang sebelumnya tidak ada).
Nilai `from: session` kini diselesaikan dari `assignments` bila token tidak
membawanya (memo 30 detik). **Gerbang validator jadi inti kejujurannya:**
`formspec validate` menolak `row_scope` `from: session` tanpa `attr` yang
atributnya tak punya sumber — bentuk yang kalau dibiarkan akan **403 selamanya
dengan manifest terlihat benar** (kelas #52/#53/GAP-33). S11: tipe field
`percent` (numerik = decimal, beda interpretasi/rendering). S12:
`unit: {base, convertible}` divalidasi terhadap `enum_values`; konversi = 4.6.
S14: `maintained_by` (script wajib ada **dan** bisa dikompilasi) + `invariants`
(wajib ditopang unique index — jadi database yang menegakkan, bukan disiplin
script). D1–D7 ditulis normatif di `01-core-basic.md` §1.2/§1.7/§7/§8.6/§11.
Adopsi kafe: 15 entity `scope`, `employee.assignments`, 3 field `percent`,
satuan gram/kg, 3 summary ber-`invariants`. Plan:
`docs_internal/plan/s5-s11-s12-s14-scope-percent-unit-summary.md` · changelog:
`2026-09-15-010`. **Adopsi `row_scope` kafe ditunda ke 3.5** (menyalakannya
sekarang mem-403 seluruh list kasir + identitas dev tanpa baris `employee`).

**Catatan 2026-09-15**: ✅ **1.7 / S9 — workflow merujuk nama transisi**
(`examples/kafe/gaps_found/TODO.md` 1.7, menutup **#38**).
`WorkflowTransitionRef.Name` + dua bentuk pemicu yang saling eksklusif; index
`byName` di `internal/workflow` dengan `ForTransition(entity, transition, from,
to)`; kedua call site API meneruskan `actionName`. **Lubang senyapnya ditutup
validator**, bukan cuma dilaporkan: Layer 1.5 `validateWorkflows`
(`cmd/formspec/validate_workflow.go`) menolak pasangan `from`/`to` yang hanya
mencakup sebagian transisi multi-asal — bentuk yang sebelumnya lolos hijau sambil
tidak menegakkan apa pun — dan menolak nama transisi yang tidak ada (dengan daftar
`via` tersedia). Adopsi kafe: `order-void-approval` → `name: void-order`.
Bukti: 4 state asal lolos interception (unit), 8 sub-test validator, `validate`
kafe 0 problem. Plan: `docs_internal/plan/s9-workflow-transition-ref.md` ·
changelog: `2026-09-15-009`.

**Catatan 2026-09-15**: ✅ **1.6 / S8 — index parsial jadi konstruk bahasa**
(`examples/kafe/gaps_found/TODO.md` 1.6, menutup akar **#22** sisa).
`IndexDecl.Where` + grammar tertutup (`pkg/spec/indexwhere.go`) → nama field
divalidasi saat validate dan diterjemahkan ke kolom turunan saat render; tiga
aturan keunikan kafe kini dinyatakan di manifest dan **ketiga `kind: Migration`
DDL mentahnya dihapus** (GAP-35 ikut tertutup untuk kasus ini). **Dua bug nyata
ikut ketemu:** jalur alter tidak pernah membuat index (hanya kolom) sehingga
`indexes:` yang ditambahkan setelah tabel ada tidak pernah terpasang; dan field
yang hanya disebut di predikat tidak mendapat kolom turunan → `no such column`.
Bukti: `migrate plan` memuat index komposit + `WHERE _status = 'open'`; constraint
terbukti menolak shift `open` kedua tapi membolehkan yang `closed`. Plan:
`docs_internal/plan/s8-partial-index.md` · changelog: `2026-09-15-008`.

**Catatan 2026-09-15**: ✅ **3.6.4 — `formspec summary rebuild`** (`02-core-extended.md`
§6). Verb baru + rencana rebuild (`internal/summary/`) + replay lewat jalur
delivery yang sama dengan live (`internal/subscription/replay.go`, consumer group
per run → cursor worker live tidak tersentuh). **Bug nyata ikut tertutup:**
channel `reliable_event` ternyata no-op di runtime (padahal `ValidateEventDurability`
mewajibkannya) — kini lewat outbox, sehingga proyeksi yang digerakkan event
durabel benar-benar terisi. Adopsi §6 di tiga proyeksi `kafe`; ketiganya
dilaporkan `orphaned` karena digerakkan `kind: Integrator` (tidak lewat stream)
— gap itu kini terlihat sebagai **3.6.6**. Guard baru menolak `schemas/` yang
stale terhadap `pkg/spec` (kelas kegagalan yang membuat validasi schema gagal
untuk _semua_ Entity). Kontrak + semantik rebuild: `docs/spec/backend/02-core-extended.md`
§6 · changelog: `2026-09-15-007`.

**Catatan 2026-09-15**: ✅ **S1 digeneralisasi — `picker` pada child field**
(`examples/kafe/gaps_found/TODO.md` 1.5, gap **#5**). Blok Page `order_builder`
(versi pertama, changelog `-003`) **dihapus**: terlalu sempit dan menduplikasi
jalur tulis Form. Konstruk umumnya sekarang `child.picker` — pilih baris dari
entity sumber + qty + snapshot — sehingga berlaku di **Form mana pun** dan
submit tetap milik Form (rules, permission, idempotency, lifecycle, event).
Ditambah tiga primitif umum: `FormField.default_from` (seed dari render
context), `widget: hidden`, dan `FormRender.picker_panel: inline|aside`.
Adopsi: kafe (order QR + purchase-order + stock-opname), inventory
(stock-movement), gl (journal-entry) — lima picker, tiga Form baru. E2E: pesanan
QR terkirim lewat Form pipe dengan `line_total`/`subtotal` terhitung server.
Plan: `docs_internal/plan/child-field-picker.md` · changelog: `2026-09-15-004`.

**Catatan 2026-09-15**: ✅ **S1 — blok Page `order_builder`**
(`examples/kafe/gaps_found/TODO.md` 1.5, menutup gap **#5** untuk sisi
pelanggan). Blok Page transaksional pertama: `catalog` + `lines` + `checkout`
(`pkg/spec/order_builder.go`, `PageBlock.OrderBuilder`), renderer
`kinds/page/blocks/OrderBuilderBlock.tsx` dengan logika murni di
`lib/orderBuilder.ts` (25 test). Mendukung join harga dari entity terpisah
(`price_entity` — kafe menyimpan harga per cabang) dan interpolasi `defaults`
(`{context}`, `{now}`, `{today}`). Blok tidak menghitung total pesanan — itu
kontrak Entity (`computed`). Adopsi kafe: halaman QR
`cafe-order/pages/menu-catalog.yaml` + `line_total`/`subtotal` kini `computed`.
Bukti runtime: anonim baca katalog+harga → keranjang → POST → order dengan
`line_total`/`subtotal` terhitung server. **Tiga bug "diam-diam salah" ikut
diperbaiki**: filter boolean `?flag=true` selalu 0 baris; baris tanpa
`created_by`/`updated_by` membuat list 500; gerbang permission `source: entity`
makai nama singular sehingga context entity tidak pernah resolve (+ permukaan
publik kini melewati pra-cek). Plan: `docs_internal/plan/order-builder-block.md`
· changelog: `2026-09-15-003`.

**Catatan 2026-09-15**: ✅ **S10 — kosakata `widget` jadi himpunan tertutup**
(`examples/kafe/gaps_found/TODO.md` 1.4, menutup akar gap **#1**). `widget:`
berhenti jadi string bebas: `pkg/spec/widget.go` mendefinisikan `FormWidget`
(20 nama) dan `TableCellWidget` (`badge`, `boolean`) sebagai tipe bernama +
blok `const`, yang oleh `internal/genjsonschema` otomatis jadi enum di JSON
Schema → `formspec validate` menolak salah ketik (`relaion-picker`) dan editor
dapat autocomplete. Dua himpunan terpisah per permukaan, sebab widget form pada
kolom tabel diabaikan renderer sel. Di runtime, `widget:` eksplisit di luar
katalog merender error yang terlihat, bukan `TextInput` senyap. Paritas
schema ↔ katalog ↔ implementasi dijaga `src/widgets/catalog.test.tsx` (14 test,
diuji bisa gagal). Dokumen `07-component-kinds.md` §1 dikoreksi — lima nama di
dokumen (`textinput`, `numberinput`, `dateinput`, `toggle`, `json-editor`)
ternyata tidak pernah ada di renderer. Separuh lain gap #1 (widget `MoneyInput`/
`TimeInput`) kini item **2.14**. Plan: `docs_internal/plan/widget-vocabulary-enum.md`
· changelog: `2026-09-15-002`.

**Catatan 2026-09-15**: ✅ **S7 — semantik aritmetika & agregasi `money`**
(`examples/kafe/gaps_found/TODO.md` 1.3, menutup gap **#28**). Bentuk kanonik:
uang beroperasi langsung (`money - money`, `number * money`, `sum([money…])`
→ money) — spec yang sudah ada tidak perlu diubah, engine yang menyusul.
Operand tidak sah (objek non-money, list, teks, angka mentah) **error**, bukan
`0`; agregasi `money` menjumlahkan `.amount`-nya dan `formspec check` menolak
agregat non-numerik secara statis. Diimplementasikan serentak di empat jalur:
server `computed` (`internal/starlark/money.go` — tipe Starlark `moneyValue`
presisi `math/big.Rat`), klien FormSpecExpr (`eval.ts`), agregasi persist
(`columnRefExpr` sub-path `.amount` + `requireNumericAggregateField`), dan
agregasi klien bersama (`src/lib/aggregate.ts`, dipakai Report/Widget/chart).
Bukti runtime: `payment.change` `{25000 IDR}`, `shift.difference` `{-12500 IDR}`.
Plan: `docs_internal/plan/money-arithmetic-semantics.md` · changelog:
`2026-09-15-001`.

**Catatan 2026-09-13**: ✅ **`formspec upgrade` (self-update binary)** — verb
baru (Fase 3.9) yang mengganti binary dari GitHub Releases tanpa install ulang:
resolve `releases/latest`/`--version`, verify `SHA256SUMS.txt`, smoke test, swap
atomik di `os.Executable()` (Windows rename-ke-`.old`); flag `--check`,
`--dry-run`, `--force`, `--yes`. Helper download/verify/extract diekstrak ke
`cmd/formspec/release.go` (dipakai bersama `spa install`), semver comparator
in-repo `cmd/formspec/semver.go` (tanpa dependency baru). E2E terverifikasi
(upgrade v0.0.6→v0.0.7, idempotent, rollback). Lihat
`docs_internal/plan/formspec-upgrade-command.md` dan
`docs_internal/changelog/2026-09-13-004-formspec-upgrade-command.md`.

**Catatan 2026-08-11**: Jalur **agent-assisted app development tanpa MCP** selesai —
lihat `docs_internal/plan/agent-assisted-app-development.md`, guide
`docs/guides/agent-assisted-app-development.md`, dan contoh `examples/cafe/`.
`formspec-app-workflow` skill kini punya Phase Detection + No-MCP Tool Map;
`formspec init` menulis copilot-instructions yang mereferensikan workflow 4 fase +
`formspec validate` sebagai gate. Fase 10 (`formspec consult`/MCP) tetap di-defer;
konten skill dibuat MCP-agnostic agar reuse saat Fase 10 landing.

**Catatan 2026-08-17**: 9 test gagal `examples/Clinic-UI-Showcase` (sebelumnya
"pre-existing") **diperbaiki** — ternyata 4 bug nyata yang saling menutupi:
(1) hook script tidak resolve karena `HandleCreate`/`HandleUpdate` tidak mengisi
`SpecDir`; (2) `resource.save()`/PATCH menulis balik alias relasi ter-enrich
(`patient`) → `stripEnrichedRelations` di `Update`/`Insert`; (3) guard
`!empty(items)` invalid Starlark → `not empty(items)`; (4) visit lifecycle-active
tanpa route submit → `submit: disabled` (lifecycle-free). Plus test time-dependent
(hardcoded date) → `recentDate()`. `go test ./...` kini **571 pass, 0 fail**.
Lihat `docs_internal/changelog/2026-08-17-003-fix-clinic-e2e-failures.md`.

**Catatan 2026-08-20**: **Fase 13 Module Registry & Vendoring** ditambahkan
(planned, belum dikerjakan) — ekosistem module registry npm-like:
`formspec module install/publish/list/uninstall`, `formspec override adopt/diff`,
`vendors/` + `overrides/` + `formspec.lock`, aktivasi berbasis marker, dan
registry server sebagai **FormSpec app (dogfooding)** untuk
`registry.formspec.dev`. Model: read-only vendoring + shadow copy (sesuai
`docs/spec/platform/08-project-layout.md` §6). Dependensi: Fase 6 (auth —
6.2 permission model + 6.4 API keys) untuk bagian auth-dependent; Fase 8
(production serve) untuk deploy nyata. Lihat section **Fase 13** di bawah.

**Catatan 2026-08-20**: **Fase 6 dikerjakan sebagai dogfooding** — auth dibangun
ulang sebagai **1 modul FormSpec** (`internal/auth/module/`, bundled + embed,
namespace `formspec.core`) yang bisa di-merge ke project lain via `external/`
atau `spec/modules/`. `formspec.core` dipindah dari registrasi programatik Go ke
YAML manifests; middleware tetap Go. Plan: `docs_internal/plan/fase6-dogfooding-auth-module.md`.
Demo merge: `verticals/reference-app` + `examples/Clinic-UI-Showcase`.

**Catatan 2026-08-24**: **Global Settings Config** selesai — namespace
`settings.*` (spec §10 "jangan pernah menebak") diimplementasikan sebagai
kontrak berlaku: `Settings`/`CurrencySettings` di `pkg/spec`, di-resolve dari
`kind: Config` manifest, dikirim via `/meta/ui` bundle, dan dipakai util
format terpusat `lib/format.ts` di frontend (money/date/number/relative).
Semua hard-code format per komponen (`en-US`/`USD` vs `id-ID`/`IDR`) di-refactor
ke formatter. Contoh: `examples/cafe/spec/modules/formspec.core/config.yaml`.
Plan: `docs_internal/plan/global-settings-config.md` · changelog: `2026-08-24-008`.
Follow-up: menu sidebar kategori **"Global"** (Akses User dan Peran +
Pengaturan) + halaman settings (`examples/cafe/spec/modules/formspec.core/
pages/settings.yaml`) — changelog `2026-08-24-009`.

**Catatan 2026-08-24**: **Date input global format + runtime settings** selesai —
`DateInput` di-rewrite (overlay native picker) agar tampil sesuai global
`date_format`; 3 input tanggal mentah (Kanban/Table filter, Wizard) diganti
`DateInput`. Global settings kini **runtime-editable**: Entity `app-setting`
(characteristic: reference, natural key "global") menyimpan running value di
DB; backend merge ke `bundle.settings`; halaman Pengaturan = Configuration
Page (Form edit); auto-apply via refresh meta setelah save. Fix bug widget
resolution integer/decimal di FormRenderer. Plan:
`docs_internal/plan/date-input-global-format-runtime-settings.md` · changelog:
`2026-08-24-010`.

**Catatan 2026-08-24**: **Fix: seed `app-setting` default dari manifest** —
halaman Pengaturan menampilkan form kosong pada akses pertama karena
find-or-create reference entity hanya meng-seed natural key, tidak menyalin
nilai default dari manifest `settings:`. Kini `HandleFind` men-seed record
`formspec.core/app-setting` dengan resolved settings (`seedSettingsData`) saat
find-or-create; `HandlerFactory.SetSettings` di-wire dari
`RouterBuilder.SetSettings`. Changelog: `2026-08-24-012`.

**Catatan 2026-08-24**: **Rounding: enum dropdown + diterapkan di formatting** —
(1) field `rounding` di halaman Pengaturan kini dropdown (Select) via
`enum_values` di entity + `widget: select` di form (tetap `type: string` untuk
hindari migrasi enum yang rapuh). (2) `rounding` yang tadinya "declared but
unused" kini benar-benar dipakai: `lib/format.ts` ekspor `RoundingMode` +
`roundTo(value, places, mode)` (semantik BigDecimal, snap presisi tinggi untuk
atasi drift biner), dan `createFormatter` menerapkannya di `money`/`number`.
Changelog: `2026-08-24-013`.

**Catatan 2026-08-24**: **Dokumentasi `formspec.core` sebagai special module** —
`docs/spec/platform/02-workspace-app-module.md` §9 diperkaya: intro menegaskan
`formspec.core` adalah special/reserved module (selalu ada, tidak perlu
`depends_on`, tidak boleh dideklarasikan user); subsection baru §9.1
"Karakteristik khusus" (reserved namespace, bundled module dogfooding,
special-casing framework untuk global settings `app-setting` + auth core,
selalu tersedia); tabel resource ditambah `app-setting`. Ditambah §9.2
"Route & Page yang disediakan" (page eksplisit + derived CRUD route entity
ui-exposed), §9.3 "Akses dari script" (`resource.fetch`, `ctx.config().get`,
`ctx.db`), §9.4 "Override default value" (runtime settings, `external/`,
`overrides/`, `auth_config_ref`). Changelog: `2026-08-24-014`.

**Catatan 2026-09-06**: **Custom screens spec-driven** selesai — tiga perluasan
closed-set agar custom screen sederhana (login, landing) bisa pure-YAML tanpa
`asset` JS: (1) standard slot `route` diperluas (`route.params/query/path`),
(2) auth action contract `FormSpec.auth_action` + `AuthFormRenderer`
(Phase C plan auth-screens-spec-driven ✅), (3) source `config` di
`spec.context` (opt-in per key, `GET /{ws}/_ui/config/{name}`). Plan:
`docs_internal/plan/custom-screens-spec-driven.md` · changelog:
`2026-09-06-002`.

---

## Fase 0: Documentation & Repo Foundation ✅ COMPLETE

| Item                                                                                                                | Status        |
| ------------------------------------------------------------------------------------------------------------------- | ------------- |
| 0.1 Fix CLI doc numbering (01-dev, 02-cli, 03-generate, 04-ctl)                                                     | ✅            |
| 0.2 Fix Document → Entity in docs/spec/                                                                             | ✅            |
| 0.3 Repo restructure: `web/` → `renderers/web/`                                                                     | ✅            |
| 0.4 Repo restructure: `internal/db/`+`datastore/` → `renderers/jsonbpersist/`                                       | ✅            |
| 0.5 AI instructions + 3 skills (backend, frontend, cli)                                                             | ✅            |
| 0.6 Verify no `docs_old/` refs in `docs/`                                                                           | ✅            |
| 0.7 Rename `renderers/web/` → `renderers/react-shadcn/` (+ cleanup ref `web/` stale di docs aktif)                  | ✅ 2026-08-14 |
| 0.8 Rename `renderers/jsonbpersist/` → `renderers/jsonb-persist/` (selaras nama docs; paket `db`/`datastore` tetap) | ✅ 2026-08-14 |

---

## Fase 1: `pkg/spec/` — Complete All Go Contract Types

**Goal**: Every kind, field type, and validation rule from `docs/spec/` has a Go struct.

### 1.1 Missing backend kind structs ✅

- [x] 1.1.1 `MigrationSpec` — DDL-only migration (`kind` + `spec.ddl` + `spec.module`), reject DML
- [x] 1.1.2 `ApprovalSpec` — approval gate (`steps[]`, `on_reject`) di `state_machine.transitions[].approval`; `WorkflowSpec` dihapus 2026-10-05 (changelog `2026-10-05-001`)
- [x] 1.1.3 `ApiSpec` — external surface override (`rest.base_path`, `rest.version`, `rest.disable`, `grpc.*`)
- [x] 1.1.4 `WebhookSpec` — verified inbound endpoint (`for`, `method`, `path`, `auth.strategy`, `auth.signature`, `idempotent`)
- [x] 1.1.5 `IntegratorSpec` — cross-module bridge (`listen.resource`+`event`, `call.resource`+`action`, `compensate`)
- [x] 1.1.6 `MockupSpec` — simulated connector (`for`, `config_ref`)
- [x] 1.1.7 `KindDefinitionSpec` — CRD-like kind extension (`group`, `version`, `schema`, `handler`, `scope`)
- [x] 1.1.8 Update `SubscriptionSpec` — add Tier 2 fields (`store`, `retention`, `position`, `max_retry`, `dead_letter`, `filter`, `transform`, `delivery` channel)
- [x] 1.1.9 Update `ConfigSpec` — replace `map[string]any` with structured `ConfigKey` type (`type`, `default`, `secret` — per `01-core-basic.md` §10; `required` tidak ada di spec)
- [x] 1.1.10 `MenuItem` — kontrak menu App/Module (`platform/02-workspace-app-module.md` §4: `[]MenuItem` langsung tanpa wrapper, array-index order, nesting max 3 level, node adopt/group/leaf, `when` FormSpecExpr) + validasi apply §6 (`module` di menu anggota `App.spec.modules`, `root_url` unik prefix `/app/`, `Form`/`Table` bukan target `view`)

### 1.2 Missing meta-kind structs ✅

- [x] 1.2.1 `VisualSpecKindSpec` — declare new view type (`tier`, `schema`, `renderer_contract`, `accepts_slots`/`implements_slot`)
- [x] 1.2.2 `RendererSpec` — concrete VisualSpecKind implementation (`implements`, `stack_family`, `trust_tier`)
- [x] 1.2.3 `PersistBackendSpec` — storage seam declaration (`implements`, `trust_tier`)

### 1.3 Missing frontend kind structs ✅

- [x] 1.3.1 `CalendarSpec` — calendar view (`entity`, `date_field`, `end_field`, `title_field`, `resource_field`, `color_field`, `views[]`, recurrence via RRULE)
- [x] 1.3.2 `ApprovalInboxSpec` — pending approvals (`realtime`, `filters`, `search`)
- [x] 1.3.3 `NotificationCenterSpec` — in-app notifications (`realtime`)
- [x] 1.3.4 `ListingSpec` — public catalog (`entity`, `columns`, `filters`, `search`)

### 1.4 Extended field type structs ✅

- [x] 1.4.1 `RateLimitSpec` — per-resource rate limit (`max`, `per`, `scope`, `strategy`: sliding_window/token_bucket)
- [x] 1.4.2 `SecretRef` — `ctx.secrets` access declaration (`uses.secrets: [key, ...]`) via `UsesDecl.Secrets`
- [x] 1.4.3 `FieldClassification` — governance label (`pii`|`financial`|`internal`)
- [x] 1.4.4 `FieldPermission` — field-level `required_permission`
- [x] 1.4.5 `FieldExclude` — per-surface field exclusion (`public_api`|`audit_log`|`webhook`|`ui` — per `05-field-types.md` §5.3)
- [x] 1.4.6 `EncryptedField` — at-rest encryption marker (`encrypted: true`)
- [x] 1.4.7 `MaskedField` — auto-mask in response/log (`masked: true`)
- [x] 1.4.8 `BackdatePolicy` / `ForwardDatePolicy` — max days, override_permission (already existed in entity.go)
- [x] 1.4.9 `TreeDecl` — self-referential hierarchy marker (`tree: true` on relation)
- [x] 1.4.10 `SoftDeactivateDecl` — `is_active` + `deactivate`/`reactivate` action pattern
- [x] 1.4.11 `StorageSpec` (file field) — `allowed_types`, `max_size_mb`, `max_count`, `visibility`, `signed_url_ttl`, `cdn`, `transform`
- [ ] 1.4.12 `MoneyType` FX & multi-currency — konversi antar mata uang (rate table, tanggal efektif, spread) untuk field `money`; belum dispesifikasikan di `05-field-types.md` § "Open — FX & multi-currency"

### 1.5 Error glossary Go types ✅

- [x] 1.5.1 Go const/type mapping dari `error-glossary.yaml` (22 error codes → `FORMSPEC.DOC.*`, `FORMSPEC.TXN.*`, `FORMSPEC.PERIOD.*`, `FORMSPEC.EVENT.*`, `FORMSPEC.SAGA.*`, `FORMSPEC.REF.*`, `FORMSPEC.PERSIST.*`, `FORMSPEC.ARCHIVE.*`, `FORMSPEC.VALIDATE.*`)
- [x] 1.5.2 Observability error codes — `OBSERVABILITY_METRICS_DISABLED`, `OBSERVABILITY_DEBUG_FORBIDDEN`, `LOGS_FILTER_INVALID` (`09-observability.md` §8)

---

## Fase 2: Engine Core — `formspec dev` Reliability

**Goal**: Atomic operations, correct PK, complete filters, lifecycle enforcement — agar `formspec dev` bisa diandalkan untuk testing.

**Progress**: 2.1 ✅ · 2.2 ✅ · 2.3 ✅ · 2.4 ✅ · 2.5 ✅ · 2.6 (2.6.1–2.6.3, 2.6.5–2.6.6 ✅; 2.6.4 ⬜ sebagian — cross-module resource access enforced, ctx.\*/secrets masih blocked on 2.9.1) · 2.7 ✅ · 2.8 ✅ · **2.9 ✅ COMPLETE (2.9.1–2.9.4)** · 2.10 ✅

### 2.1 Database integrity ✅

- [x] 2.1.1 Atomic mutation + outbox — wrap Entity INSERT/UPDATE/DELETE + outbox write dalam `BeginTx`/`Commit` (rollback on error). Terpenuhi untuk create/update HTTP (`InTx`) **dan** custom action (`HandleCustomAction` + `TxScope`, `renderers/jsonbpersist/txscope.go`) — satu transaksi request-scoped mencakup semua panggilan `resource.save()`/`.create()` (Starlark/native/sidecar via `X-FormSpec-Scope-Id`) dalam satu eksekusi action, join berdasar identitas store (bukan Module — multi-Module dalam satu Datastore fisik yang sama tetap atomik; lintas-Datastore genuinely berbeda → `ErrCrossStoreTx`). **Gap tersisa**: `RunAfterPhase` masih fire-and-forget (tidak rollback); SDK sidecar (`sdk/php`/`sdk/python`/`sdk/typescript`) belum mengirim `X-FormSpec-Scope-Id` (`01-architecture.md` §3, `runtimes/04-formspec-sidecar.md` §4.3a).
- [⏸️] 2.1.5 **SDK sidecar belum mengirim `X-FormSpec-Scope-Id`** — disebut sebagai sisa di dalam teks 2.1.1 (`sdk/php`, `sdk/python`, `sdk/typescript`; `01-architecture.md` §3, `runtimes/04-formspec-sidecar.md` §4.3a) tetapi tidak ada item bernomor yang melacaknya. Akibatnya request lewat sidecar tidak membawa scope id, jadi jalur tenant/scope yang bergantung padanya tidak bisa dibedakan dari request tanpa scope. Effort: small (tiga SDK, satu header + test).
- [x] 2.1.2 Natural key counter in same transaction as Entity insert — UPSERT counter + INSERT dalam satu `Tx` (`generateNaturalKeys` menerima DB terikat-transaksi; `04-query-and-keys.md` §2)
- [x] 2.1.3 UUID v7 PK — replace SQLite `INTEGER PRIMARY KEY AUTOINCREMENT` with UUID v7 generated at app layer (`NewUUIDv7`, kedua driver; child table PK juga ikut)
- [x] 2.1.4 Idempotency retention configurable — `IdempotencyStore` dikonstruksi di `resource.App`; TTL diselesaikan `resolveIdempotencyTTL` dengan presedensi **`core.idempotency_retention` (manifest, sejak 2.1.6) → `Config.IdempotencyTTL` → `db.DefaultIdempotencyTTL` (24h)**; diekspos lewat `App.Idempotency()`. **Dikoreksi 2026-09-26:** kalimat lama "resolusi dari manifest … menunggu runtime Config-kind (Fase 7.2, belum ada)" sudah **basi** — runtime-nya landing 2026-08-25 dan pemetaannya landing bersama 2.1.6.
- [x] 2.1.6 ✅ **2026-09-26** **`core.idempotency_retention` (key Config) kini dipetakan ke `IdempotencyTTL`.** Key ini sudah normatif di `01-core-basic.md` §5 tapi dibaca **0 tempat** di Go (hanya komentar). Kini `resolveIdempotencyTTL` (`resource/formspec.go`) membacanya lewat `Registry.ResolveKeyAny` (baru — key framework hidup di namespace `formspec.core` tanpa nama Config stabil) dan di-wire di **dua** titik: boot `New()` **dan** `ReloadSpec()`, jadi perubahan berlaku tanpa restart. Presedensi `core.idempotency_retention` → `Config.IdempotencyTTL` → default 24h. Dua keputusan eksplisit: (a) nilai tak terbaca (`"banana"`) **dilaporkan** lalu default dipakai — bukan diam-diam mematikan retention; (b) `0` = tanpa kedaluwarsa (selaras `BackdatePolicy.MaxDaysBack = 0`) dan bare integer (`"7"`) **ditolak** karena pada parser retention itu _count_, bukan durasi. Konsumen nyata: kafe mendeklarasikan `keys.idempotency_retention: "24h"` di `examples/kafe/spec/config/app.yaml`. **Bukti:** `TestResolveIdempotencyTTL` (7 sub-test) **dibuktikan gagal** saat pemetaan dinetralkan (`7d: got 24h0m0s, want 168h`); `TestRegistry_ResolveKeyAny`; kafe `validate` 85 manifest 0 problem, `check` 0/0. Changelog `2026-09-26-003`. Effort selesai: small.
- [x] 2.1.5 `natural_key_rule` lengkap — `strategy: sequence|custom` (custom = framework tidak auto-generate, diisi hook/script/import), `format`, `prefix`, `reset: never|yearly|monthly|daily` (divalidasi di `ValidateDocumentSpec`), `scope_field` (`01-core-basic.md` §2); counter komposit `(tenant, resource, field, scope, period, seq)` sudah ada (`jsonb-persist/04` §2)

### 2.2 Query correctness ✅

- [x] 2.2.1 Filter operators 13/13 (`eq neq gt gte lt lte between in nin like ilike null notnull` — `01-core-basic.md` §6) — added `between`, `ilike`, `null`, `notnull`; handler parsing supports `between` as comma-separated pair
- [x] 2.2.2 JSONB path fallback for non-indexed fields — `data->>'field'` (PG) / `json_extract(data, '$.field')` (SQLite) via `EntityStore.columnRefExpr()`
- [x] 2.2.3 Generated column dialect-aware — `generateGeneratedColumn` now accepts `DriverType`; PG uses `data->>'field'`, SQLite uses `json_extract`
- [x] 2.2.4 `exists:<resource>` real lookup — already wired in `resource/formspec.go` via `SetEntityLookup`, queries entity registry
- [x] 2.2.5 Cross-module relation resolution — `ValidateRelationTargets` parses `{module}.{entity}` from `Relation.Resource`; registry injects `targetTableResolver` using spec's Plural (not naive `+s`)

### 2.3 Lifecycle engine ✅

- [x] 2.3.1 8 reserved actions with guard enforcement — `LifecycleGuard` function for all 8; wired into `Update()`, `SoftDelete()`, `Submit()`, `Cancel()`; REST routes added for submit/cancel/amend
- [x] 2.3.2 Transitive gating — `TransitiveDisabled()` wired into route generation (`generator.go`)
- [x] 2.3.3 `update` after `submit` always rejected — `LifecycleGuard("update")` checked in `Update()`
- [x] 2.3.4 Referenceability — already implemented via `ValidateRelationTargets()` (unchanged)
- [x] 2.3.5 `delete` guard absolut — `LifecycleGuard("delete")` checked in `SoftDelete()`
- [x] 2.3.6 `create-submit`/`amend-submit` auto-derived — `DeriveReservedActions()` exists (route-level skip for now)
- [x] 2.3.7 Error codes lengkap — `FORMSPEC.DOC.ALREADY_SUBMITTED`, `ALREADY_CANCELLED`, `SUBMIT_NOT_DRAFT`, `CANCEL_NOT_SUBMITTED`, `UPDATE_NOT_DRAFT`, `DELETE_NOT_DRAFT`, `AMEND_NOT_SUBMITTED_OR_CANCELLED`, `FORMSPEC.REF.DELETE_BLOCKED`, `FORMSPEC.REF.CANCEL_BLOCKED`
- [x] 2.3.8 `child.sequence_field` enforcement — validate monotonically ordered line numbers on insert/reorder; auto-assign when client omits, validate when provided; reject duplicates/non-monotonic → `VALIDATION_ERROR` (422)
- [x] 2.3.9 `child` lifecycle — child follows parent submit/cancel via `SubmitChildren()`/`CancelChildren()`; `doc_status` column added to child table DDL
- [x] 2.3.10 `relation.on_delete` framework — `reference.go` with `CheckReferencingDocuments` + `EnforceReferenceGuard`; stub implementation (full on_delete 3 modes membutuhkan reference tracking system)
- [x] 2.3.11 `characteristic` enforcement at apply — validated in `ValidateDocumentSpec()`
- [x] 2.3.12 `characteristic: summary` — `create`/`update`/`delete` blocked di store level (`Insert`, `Update`, `SoftDelete`)

### 2.4 Event system core ✅

- [x] 2.4.1 Event naming convention enforcement — `ValidateEventNaming()` existed; verified called from `ValidateDocumentSpec`; `ValidateActionEmits` also exists
- [x] 2.4.2 Event priority ordering — hooks already support `Priority` field (0→default 10); `SelectHooks` sorts by priority; kelipatan 10 convention documented
- [x] 2.4.3 Durability contract validation — new `ValidateEventDurability()` checks publisher non-durable + subscriber durable → error at apply
- [x] 2.4.4 Outbox worker — `MarkFailed` enhanced with `backoff` strategy (exponential|linear|fixed) + `initial_delay_ms` support; outbox table DDL extended with columns
- [x] 2.4.5 `FORMSPEC.EVENT.TYPE_MISMATCH` + `FORMSPEC.EVENT.TYPE_MISSING` — wired into `ValidateEventNaming()` error messages

### 2.5 API infrastructure ✅

- [x] 2.5.1 Two API surfaces — `/_ui/entity/` (all entities, session auth) + `/api/v1/` (exposed-only, API key); both share same internal logic
- [x] 2.5.2 Radix-tree router — chi router is radix-tree based ✅
- [x] 2.5.3 Single internal logic path — handlers → store methods; same-process dispatch bypasses network ✅
- [x] 2.5.4 ListResponse `links` field — `buildListLinks()` with first/last/next/prev ✅
- [x] 2.5.5 ErrorResponse `details` array — `ErrorDetailItem` + `writeErrorWithDetails()` ✅
- [x] 2.5.6 `per_page` clamping — max 100 clamp in `EntityStore.List()` ✅
- [x] 2.5.7 Response envelope contract — `{data, meta}` / `{error: {code, message, details}, meta}` ✅
- [x] 2.5.8 Meta API backend-agnostic — `BuildEntitySchema` uses spec types, no SQL-specific leaks ✅
- [x] 2.5.9 Workspace slug prefix — `WorkspaceMiddleware` extracts slug from URL; fallback to "default" ✅ (diperluas 2.11 — registry + validasi 404)

### 2.6 Security basics

- [x] 2.6.1 Cross-tenant isolation — already in place: `AuthMiddleware` (`internal/api/middleware.go`) returns 404 (not 403) on identity-vs-URL workspace mismatch; every `EntityStore` query in `renderers/jsonbpersist/crud.go` scopes on `tenant_id`. Covered by existing `internal/api/api_test.go`/`renderers/jsonbpersist/crud_test.go`.
- [x] 2.6.2 Tenant ID auto-injection — already in place: `GenerateEntityDDL` (`renderers/jsonbpersist/ddl.go`) always emits `tenant_id` + tenant-scoped unique indexes.
- [x] 2.6.3 Permission auto-registration — `internal/entity/registry.go`'s `registerStandardPermissions()` (shared by `LoadEntities`/`RegisterArtifactManifest`) now also registers `submit`/`cancel`/`amend`, gated identically to route generation (`db.TransitiveDisabled` + `characteristic: summary`) so registered permissions never drift from actual routes. Format stays `{module}.{plural}.{action}`, matching `internal/api/generator.go`.
- [x] 2.6.4 UsesEnforcement wiring (cross-module resource access + ctx.\* primitives) — **complete**: blocker (a) resolved (cross-module `resource.call()`/`fetch()`/`create()` diblokir `USES_VIOLATION` bila target tak dideklarasikan di `uses.resources`; matcher `{module}.{entity}`, `{module}/{entity}`, `{module}.*`, `*`). **Blocker (b) resolved** (2026-08-17): `ctx.*` primitive enforcement kini di-thread — `internal/action/script.go` meneruskan `action.Uses` penuh → `internal/starlark.ScriptExecutor.Execute(uses)` → `CtxAPI.SetUses` + `SetStrictPrimitives`; di ProdMode/StrictMode, akses `ctx.db/cache/lock/queue/pubsub/storage/kvstore` yang tidak dideklarasikan di `uses.primitives` → `USES_VIOLATION` (dev mode relaxed). Test: `resource/uses_enforcement_test.go` + `uses_enforcement_e2e_test.go` + `ctx_uses_enforcement_test.go`. Module auto-suspend + incident audit tetap subsistem baru yang belum ada. Stub middleware `UsesEnforcement` di `internal/api/middleware.go` tetap dead code — enforcement nyata hidup di `resource/formspec.go` + `internal/starlark/context.go`. ✅ 2026-08-17
- [⏸️] 2.6.5 **Module auto-suspend + incident audit pada `USES_VIOLATION` belum ada** — enforcement-nya sudah hidup, tetapi konsekuensi `platform/05-plane-protocol.md` §4.4 (suspend module otomatis + insiden audit) belum. Stub middleware `UsesEnforcement` di `internal/api/middleware.go` tetap dead code. Dulu hanya tersirat di teks 2.6.4. Effort: medium (butuh status suspend per module + penulisan insiden).
- [x] 2.6.5 Optimistic concurrency — storage layer was already correct (`crud.go`'s `Update()` does `WHERE version = ?`; conflicts already mapped to 409), but `HandleUpdate` (`internal/api/handler.go`) silently ignored the client and always used the just-fetched version — meaning the `If-Match: version=N` header renderers/web's `apiPatch` (`renderers/web/src/lib/api/client.ts`) already sends on every Form autosave/Kanban drag-update was a no-op. Fixed: `HandleUpdate` now parses `If-Match` and uses the client's version for the CAS check when present; missing `If-Match` falls back to today's behavior in relaxed/dev mode but is `409 CONFLICT` when `SetStrictMode(true)` (production).
- [x] 2.6.6 WebSocket per-message permission filter — `wsConn` (`internal/api/wshub.go`) now carries the connection's `*auth.Identity` (captured in `HandleWS`); `Broadcast` resolves `EventMessage.Resource` to `{module}.{plural}.view` via the entity registry and skips connections lacking that permission. Fails open (delivers unfiltered) when identity is nil or the resource/registry can't be resolved, so it only engages once real auth is wired up — see the "identity/registry" branch in `internal/api/wshub_test.go`/`wshub_permission_test.go`.

### 2.7 Idempotency ✅

- [x] 2.7.1 Two-step prepare flow — `POST /{resource}/{action}/prepare` → receive key → retry action with key. Endpoint `HandlePrepare` (server-sourced idempotent actions only; header/param-sourced 404), route `POST /api/v1/{module}/{plural}/create/prepare` + `/{action}/prepare` di kedua surface (external + `/_ui/entity/`). Lihat `docs_internal/plan/idempotency-prepare-flow.md`.
- [x] 2.7.2 Idempotency store — `(tenant, action, key) → pending|completed + response` (`01-core-basic.md` §5 — tanpa state `failed`); enforcement di `HandleCreate` + `HandleCustomAction`: duplicate after completed → replay response asli (status + body); duplicate saat pending (in-flight) → 409; failed → retry diizinkan. `IdempotencyStore.Lookup` membedakan pending vs failed (TryClaim menggabungkan keduanya). Store di-wire ke router di `New()` + `ReloadSpec()`.

### 2.8 `spec.expose` enforcement ✅

- [x] 2.8.1 `spec.expose: []` → external API returns 404 for all endpoints; UI surface unaffected — `GenerateRoutes` skip entity tanpa expose; `GenerateUIRoutes` selalu include semua entity (`internal/api/generator.go`; test `TestGenerateRoutes_NoExpose`/`TestHTTPRouter_404OnUnexposed`)
- [x] 2.8.2 `spec.expose: [{type: rest, actions: [list, find]}]` → only those actions on `/api/v1/` — `generateRESTRoutes` filter `allowed` dari `exp.Actions` (test `TestGenerateRoutes_WithExpose`)

### 2.9 `ctx.*` infrastructure primitives

- [x] 2.9.1 Wire `CtxAPI.SetDatastoreResolver` + implementasi `datastore.Open()` nyata — `ctx.db().query()` kini jalan terhadap database utama app (SQLite dev / Postgres prod) via `datastore.DBQuerier`; resolver di-wire dari `newDispatcher` (`resource/formspec.go`) → `action.ScriptExecutor.SetDatastoreResolver` → `starlark.ScriptExecutor` → `CtxAPI`; Go context di-thread lewat `starlark.Thread.SetLocal`; `primitiveRunner` operasi (`query/get/set/delete/acquire/release`) memakai capability interfaces (`Querier`/`KVGetter`/`KVSetter`/`KVDeleter`/`Locker`). Primitif lain + named datastore saat itu masih error jelas; **2.9.2 + 2.9.4 kini sudah landing** (2026-08-27), jadi bagian ini tidak lagi berlaku — dikoreksi 2026-09-22. Lihat `docs_internal/plan/ctx-datastore-resolver.md`. (`runtimes/02-formspec-resource.md` §7, `runtimes/04-formspec-sidecar.md` §8)
- [x] 2.9.2 Closed set 9 primitive — `db`, `cache`, `lock`, `queue`, `pubsub`, `storage`, `config`, `kvstore`, `log` (`platform/06-datastore.md` §2), termasuk binding `.named()`. Primitif yang di-routing lewat resolver (`db`/`cache`/`lock`/`queue`/`pubsub`/`storage`/`kvstore`) kini resolve ke backend nyata; `config`/`log` adalah builtin terpisah (`ctx.config`/`ctx.log`). Operasi baru di `primitiveRunner`: `enqueue`/`dequeue`, `publish`/`subscribe`, `upload`/`download`. Lihat `docs_internal/plan/ctx-primitives-closed-set.md`.
- [x] 2.9.3 Dev auto-provision `'default'` per primitive — db→SQLite (database utama app), cache/lock/queue/pubsub/kvstore→in-memory, storage→filesystem (`platform/06-datastore.md` §5); named datastore kini menyelesaikan lewat `ResolveNamed` (`resource/datastoreregistry.go:761`, test `ctx_db_module_scoped_e2e_test.go`); kalimat "menunggu 2.9.4" sudah tidak berlaku — dikoreksi 2026-09-22. Resolver dibangun `ctxPrimitiveResolver` di `resource/ctxresolver.go`, dipakai `newDispatcher` (dan `formspec.New` → dev.go).
- [x] 2.9.4 `ctx.db()` module-scoped (normatif) — resolve ke Datastore milik Module; interaksi lintas-Module-lintas-Datastore WAJIB async, tanpa escape hatch `ctx.db` sekalipun dengan `uses` (`01-core-basic.md` §3/§5) — `resource/datastoreregistry.go`: `DatastoreRegistry` load `kind: Datastore` manifests + binding `ModuleSpec.Datastore`; resolver 3-arg `(primitiveType, name, module)` di-thread dari `ScriptExecutor.Execute` → `CtxAPI.SetModule` → handle closure; plain call → datastore milik module (fallback 'default' bila tak serve primitive); `.named(x)` hanya sah untuk binding module sendiri (error §1.1 selain itu — termasuk `.named("default")` dari module terikat); driver single-server: sqlite/postgres/memory/fs, cloud driver error jelas; validasi boot + `formspec check` (`checkDatastores`: ref binding + driver×serves §2). Test: unit `datastoreregistry_test.go` (6) + e2e `ctx_db_module_scoped_e2e_test.go` (isolasi 2 module × 2 datastore + blokir escape hatch). Catatan API: rantai yang benar adalah `ctx.db.named("x").query(...)` — `.named()` resolve langsung ke runner. ✅ 2026-08-27 (changelog 002)

#### 2.9.5 Infra Registry 3-level (docs_internal/plan/infra-registry-3-level.md)

> Model: 9 logical primitive (`db`, `cache`, `lock`, `queue`, `pubsub`,
> `storage`, `kvstore`, `config`, `log`) dimapping eksplisit lewat 3 level:
> **Infra Registry** (cloud control — service fisik, multi-service per
> primitive, default overridable) → **App Registry** (per `kind: App` —
> default + named logical primitive) → **Workspace Binding** (pemetaan
> logical→fisik via `access.filter`). Chain resolusi: action `uses` →
> module → app → workspace → infra service.

- [x] A.1 InfraRegistry per-primitive — `resource/datastoreregistry.go` restrukturisasi: unit registrasi = service (`serviceEntry`), `services` + `defaults map[PrimitiveType]string`; tiap primitive bisa punya >1 service (mis. 2 db); default = pointer ke service teregistrasi (bukan backend implisit), overridable via `SetDefault`/`Default`/`Services`; plain call resolve ke per-primitive default (bukan hardcoded `'default'`). Public API dipertahankan. ✅ 2026-08-30 (changelog 2026-08-30-003)
- [x] A.2 Backend per-primitive — `rediskv.Lock`/`Queue`/`PubSub` (Redis/Valkey: SET NX PX + token release, LPUSH/RPOP FIFO, pub/sub JSON) untuk primitive `lock`/`queue`/`pubsub`; driver `minio`/`s3` via named `kind: Datastore` (kredensial `spec.connection.extra`, fallback env); helper `dialRedis` bersama; `spec.AllPrimitiveTypes()`. ✅ 2026-08-30 (changelog 2026-08-30-003)
- [x] B App Registry — deklarasi `datastores` map (key `default` + named alias) di `kind: App`; `ModuleSpec` override per module; enforce `UsesDecl.Datastores` (`pkg/spec/entity.go` — key `primitive` atau `primitive/alias`) — ✅ 2026-08-30 (changelog 2026-08-30-004): `AppSpec.Datastores` + `ModuleSpec.Datastores` (key `primitive` atau `primitive/alias`); chain resolusi module binding → module `datastores` → App selection → registry default (`chainTarget`); gate `checkDatastoreAccess` di `CtxAPI` → `DATASTORE_ACCESS_DENIED`; validasi fail-loud selection; named key di module level ditolak sampai fase C. Test +4.
- [x] B.2 Workspace Binding — snapshot per-workspace (`internal/control/snapshot.go`) evaluasi `access.filter` → binding logical→fisik; `WorkspaceSpec.Datastores` (operator) diberi peran ini — ✅ 2026-08-31 (changelog 2026-08-31-001): `artifact.DatastoreBinding` + `DatastoreRegistration` + store methods (`UpsertDatastore`/`ListDatastores`); `buildSnapshot` evaluasi `access.filter` per workspace (service tak cocok tidak muncul di snapshot; permission ceiling ikut binding); `DatastoreRegistry.LoadSnapshotDatastores` (populate registry dari snapshot, built-in 'default' tak pernah diganti) + `Permission(service)`; test `TestBuildSnapshot_DatastoreFilter` + `TestDatastoreRegistry_SnapshotBinding`.
- [x] C Buka `.named()` resmi — resolve via App Registry named map, gate `uses.datastores`; error codes `DATASTORE_NOT_FOUND`/`DATASTORE_ACCESS_DENIED`/`DATASTORE_PERMISSION_DENIED` (spec §6) — ✅ 2026-08-30 (changelog 2026-08-30-005): `DatastoreRegistry.ResolveNamed` (alias app-scoped via `appNamed`, module-level named key merged ke App pemiliknya); `.named(alias)` kirim prefix `named:` → `CtxAPI` route ke `ResolveNamed` + gate `checkDatastoreAlias` (key `db/analytics`); `checkDatastoreAccess` kini mengizinkan base primitive bila ada named key-nya; `SetDatastoreResolverNamed` di `ScriptExecutor`/`action`; e2e `.named("default")` kini `DATASTORE_NOT_FOUND`. Test +3.
- [x] D `config`/`log` routable — builtin `ctx.config`/`ctx.log` jadi fallback, bisa diarahkan ke service `serves: [config]`/`[log]` (9 primitive lengkap) — ✅ 2026-08-30 (changelog 2026-08-30-006): capability `Logger` + runner `.info/.warn/.error`/`.get`; backend `KVConfig`/`KVLog` (memory/redis), `MemoryLog`, `FileLog` (fs), `DBConfigLog` (sqlite/postgres); driver×serves diperluas (config: memory/valkey/redis/sqlite/postgres; log: memory/fs/valkey/redis/sqlite/postgres); `ctx.config`/`ctx.log` probe resolver dulu (runner dengan conn non-nil), fallback builtin bila tidak ada service; test `TestDatastoreRegistry_ConfigLogRoutable`.
- [x] E Distribusi Control Plane — snapshot membawa infra registry + binding; hapus env-var implicit (`FORMSPEC_STORAGE`/`FORMSPEC_MINIO_*`/`FORMSPEC_STREAM`); auto-provision hanya dev mode — ✅ 2026-08-31 (changelog 2026-08-31-001): `buildStreamBackend(dsReg)` resolve via registry (service Redis/Valkey serves queue/pubsub → Redis stream; else memory); storage resolver resolve via registry (service minio/s3 serves storage → object store; else filesystem); env-var `FORMSPEC_STORAGE`/`FORMSPEC_MINIO_*`/`FORMSPEC_STREAM`/`FORMSPEC_REDIS_ADDR` dihapus dari boot path.
- [x] F Update spec normatif — `platform/06-datastore.md` §1.1 (`.named()` resmi), §4 (workspace binding), §5 (default per-app); `platform/05-plane-protocol.md` §4.1 (snapshot membawa datastore) — ✅ 2026-08-30/31: `06-datastore.md` v0.2.0 (kontrak 3-level lengkap — bagian dari konsolidasi docs, changelog 2026-08-30-007); `05-plane-protocol.md` §4.1 + field `datastores` (Workspace Binding) di snapshot.

### 2.10 Spec hot-reload ✅

- [x] 2.10.1 `App.ReloadSpec()` — rebuild semua registri dari spec directory, atomic swap
- [x] 2.10.2 `watchSpecForChanges()` — fsnotify watcher di `formspec dev`, debounce 300ms
- [x] 2.10.3 Native Go handlers preserved across reload via `nativeHandlers` map
- [x] 2.10.4 WebSocket connections preserved via WSHub transfer
- [x] 2.10.5 Auto-watch subdirektori baru
- [x] 2.10.6 ETag-aware Meta API — reload otomatis mengubah ETag, frontend fetch bundle baru
- [x] 2.10.7 **`ReloadSpec` melepas object store & link store — DIPERBAIKI 2026-09-22** (changelog `2026-09-22-014`). `ReloadSpec` membangun `api.NewRouterBuilder` BARU, dan resolver object store / link store hidup di App (bukan diturunkan dari spec, jadi reload tidak punya apa pun untuk di-resolve ulang) — sementara builder baru mulai kosong. Akibatnya **setiap operasi file** (upload maupun download, di entity mana pun) membalas `STORAGE_UNAVAILABLE: storage not configured` **setelah hot-reload pertama**, sampai proses di-restart. **Terukur di dev server kafe:** foto `200` → sentuh file spec (memicu watcher) → foto **`500`**. Ini kelas bug yang mahal karena gejala dan penyebabnya tidak berhubungan — yang diedit adalah spec, yang rusak adalah storage — dan hanya muncul pada alur yang justru jadi alasan dev server ada (edit file, lanjut kerja). Fix: `App` menyimpan `storageFn` + `linkStore`, dan reload me-wire ulang keduanya. **Test pengunci:** `TestReloadSpecKeepsFileStorage` (`resource/storage_reload_e2e_test.go`) — gagal (500) saat re-wire dinonaktifkan, hijau sesudahnya. **Sisa:** jalur reload lain (registri datastore/`buildDatastoreRegistry`) belum diaudit untuk pola "state milik App yang tidak ikut dibangun ulang" yang sama.

### 2.11 Named Workspaces ✅ (2026-09-07 — docs_internal/plan/named-workspaces.md, changelog 2026-09-07-004)

- [x] 2.11.1 Kind `Workspace` — `pkg/spec/workspace.go` (`WorkspaceSpec`, `ValidateWorkspaceSpec`: slug kebab-case + reserved segments `_ui/api/_admin/assets/health/login/register/_ws/print`); `KnownKinds` + branch validasi loader; JSON Schema (`make generate-schema` → `schemas/kinds/Workspace.schema.json`)
- [x] 2.11.2 Workspace registry — `internal/auth/workspace.go` (`WorkspaceRegistry`: Ensure/List/Delete/Registered) di atas entity bawaan `formspec.core/workspace` (slug unique+indexed); scope storage `formspec:workspaces`
- [x] 2.11.3 Middleware enforcement — `api.SetWorkspaceResolver` + `WorkspaceMiddleware`: slug tak terdaftar → 404 `WORKSPACE_NOT_FOUND`; lookup gagal → 500; resolver nil → passthrough (embedded/test); resolver di-wire `App.New` + `ReloadSpec` (`syncWorkspaceRegistry`)
- [x] 2.11.4 Seed `default` — workspace `default` selalu di-upsert saat boot (fallback URL tanpa slug tetap routable)
- [x] 2.11.5 CLI `formspec workspace create|list|delete` — `cmd/formspec/workspace.go` (flag-first parsing; `--dsn` wajib; `default` tak bisa dihapus)
- [x] 2.11.6 Unifikasi default workspace — `"demo"` → `"default"` (`workspaceFromContext`, fallback middleware, `Config.WorkspaceID`, `formspec logs`); test resource + e2e clinic dimigrasi ke `"default"`
- [x] 2.11.7 Dev-ui proxy multi-workspace — `viteSPAProxy` meneruskan `/{ws}/_ui|api/` untuk semua workspace (sebelumnya hanya workspace terkonfigurasi). **Koreksi 2026-09-26:** klaim ini hanya benar untuk sisi **Go**; `server.proxy` Vite (`renderers/react-shadcn/vite.config.ts`) masih literal `/default/...`, jadi `--dev-ui`/`npm run dev` (Vite menyajikan SPA langsung, bukan lewat `:8080`) gagal untuk workspace lain — `localhost:5174/kafe` → `Unexpected token '<', "<!doctype "... is not valid JSON` (terukur: `GET /kafe/_ui/_meta/apps` lewat Vite = `200 text/html`). Diperbaiki ke key RegExp `^/[a-z0-9-]+/_ui` + `api/v1` + guard `devProxyConfig.test.ts` (7/10 test gagal saat regresi disuntikkan; 529 vitest lulus). Changelog `2026-09-26-017`. **Sisa → 2.11.7a ⏸️.**
- [⏸️] 2.11.7a `/health` tidak di-proxy Vite di mode `--dev-ui` — Vite menjawab SPA, sedangkan `viteSPAProxy` Go meneruskannya ke backend. **Sengaja dibiarkan**: SPA tidak pernah memanggil `/health`, dan menambah key proxy untuknya tidak bisa tumpang-tindih dengan slug (`health` reserved). **Teramati:** `curl :5173/health` → `text/html`. Effort: small (tambah key `"/health"` bila ada flow CLI yang membutuhkannya).
- [x] 2.11.8 Contoh + docs — `examples/cafe/spec/workspaces/cafe-workspaces.yaml` (seed `cafe`); `docs/spec/platform/02-workspace-app-module.md` §1.1; `docs/cli-tools/02-formspec-cli.md` §12–§13; `docs/runtimes/05-engine-api-layer.md`
- [⏸️] 2.11.9 Slug→UUID resolution — deferred: slug = workspace ID untuk saat ini (data existing keyed by slug); mapping ke internal UUID menunggu kebutuhan nyata (rename workspace / merge tenant)
- [x] 2.11.10 Publish `Workspace.schema.json` ke registry online — **tertutup 2026-10-06:** live `https://schemas.formspec.dev/v1/kinds/Workspace.schema.json` → **200** dan file-nya tracked di `schemas/dist/v1/kinds/`; klaim lama "masih 404" sudah tidak berlaku. (Kind yang benar-benar tertinggal ternyata `Seed` — bukan `Workspace` — dan itu ditutup di **3.6.7**.) ✅ 2026-10-06

### 2.12 `AppSpec.Workspaces[]` — Allowlist Mount per App ✅ (2026-09-07 — changelog 2026-09-07-005)

- [x] 2.12.1 Tiga state — `Workspaces *[]string`: absen = semua workspace (backward compat); `[]` eksplisit = staged (mounted nowhere, `formspec check` warning); `[slug,...]` = allowlist. `AppSpec.MountsWithin(ws)`
- [x] 2.12.2 Validasi format-only di `ValidateAppSpec` (kebab-case + bukan reserved; keberadaan registry TIDAK divalidasi — workspace bisa dibuat belakangan via CLI)
- [x] 2.12.3 Enforcement request-time — `/_meta/apps` terfilter per workspace; app-scoped `/_meta/ui` menolak (pesan "unknown app", anti-enumeration); SPA mount root_url 404 di luar allowlist
- [x] 2.12.4 `version`/`vendor` diekspos di `/_meta/apps` + validasi semver opsional (keputusan AppVersion: artifact + binding control plane, BUKAN in-process versioning)
- [x] 2.12.5 Middleware fix — fallback "no validator" tidak lagi memaksa workspace `"demo"` (workspace URL dipertahankan); genjsonschema dukung `*[]T`
- [x] 2.12.6 `formspec check` warning App staged
- [⏸️] 2.12.7 `publicEntities` per-workspace — anonymous surface App public masih global (limitasi didokumentasikan §3.1)
- [⏸️] 2.12.8 Layer 2 — binding runtime workspace↔App di registry (CLI/admin "install App ke workspace") + upgrade per-workspace = control plane/cloud phase

---

## Fase 3: CLI — `formspec` Command Completion

**Goal**: `validate` → `check` → `new` → `dev` → `generate` → `diff/get/describe/delete` → `migrate/repl/seed` → `backup/restore/logs`.  
**Priority**: per `docs/cli-tools/02-formspec-cli.md` §13.1.

### 3.1 High-priority (no backend dependency)

- [x] 3.1.1 `formspec validate --spec <path>` — dry-run validation dua lapis: engine loader (`internal/manifest`; parse + Entity deep-validation) + JSON Schema per kind (`schemas/kinds/*` via `santhosh-tekuri/jsonschema`, lihat `cmd/formspec/validate.go`). Exit 1 bila ada gagal. 2026-07-31. Catatan: lapis schema lebih ketat dari engine untuk shorthand `guard`/`render` (Go `UnmarshalYAML` scalar+map tak bisa diekspresikan generator schema) — gunakan bentuk objek. **DITUTUP 2026-10-06:** kedua shorthand kini `oneOf: [string, object]` di schema hasil generate — `render` lewat 5.4, `guard` lewat special-case `GuardDecl` (changelog `2026-10-06-004`); bentuk kanonis diterima editor **dan** loader. Sisa: honesty scan Starlark → 3.1.1a.
- [x] 3.1.1a `formspec validate` honesty scan Starlark (undeclared usage → error, declared-but-unused → warning, `ctx.environment` branching → warning) + flag `--fix` — `cmd/formspec/honesty.go`: parser AST `go.starlark.net/syntax` (walker manual — versi starlark-go ini tidak punya visitor generik; `DotExpr.Name` = `*Ident`, `AssignStmt` mencakup augmented, `syntax.Parse(path, src, 0)`); ekstraksi `ctx.<primitive>` (closed set resolver-routed), `resource.call/fetch/create` first-arg literal, `ctx.secrets.get("key")`, `ctx.environment`; pembandingan vs `uses` per action/hook (Entity + Service). Semantik: hanya target cross-module (ber-dot/slash) yang wajib deklarasi — bare name = same-module implicit (match runtime enforcement); wildcard `{m}.*`/`*` dihormati; `*` unused hanya bila tidak ada usage sama sekali. `--fix` HANYA menghapus declared-but-unused (prune list kosong + blok `uses` kosong) — penambahan deklarasi = perluasan consent tetap manual (preseden 3.1.2). Validasi nyata: Clinic → 0 false positive, 4 warning genuine (deklarasi mati `medicine.find`/`medicine.update` di pharmacy). Test: `honesty_test.go` (6). ✅ 2026-08-27 (changelog 003)
- [x] 3.1.2 `formspec check [--fix] -f <path>` — cross-file analysis: Form field ref ke field tak ada (error), FormSpecExpr ref ke field tak ada (error), `uses.resources` ref ke `{module}.{entity}` tak ada (error). `--fix`: hapus deklarasi `uses.resources` yang broken (target tak ada — aman, tidak mengubah footprint consent; penambahan deklarasi = perluasan consent → interaktif, di-defer). Lihat `cmd/formspec/check.go` + `docs_internal/plan/formspec-check.md`.
- [x] 3.1.3 `formspec new <kind>` — scaffold: `new app <name>`, `new entity <name>`, `new module <name>`. Generate boilerplate YAML + directory. `new module` → `spec/modules/{module}/module.yaml`; `new entity` → `spec/modules/{module}/{characteristic}/{entity}/entity.yaml` (fields dasar code/name/description + expose default; characteristic divalidasi closed set; module di-detect dari CWD atau `--module`). Lihat `cmd/formspec/new.go`.
- [x] 3.1.4 `formspec init` tulis `.vscode/settings.json` (`yaml.schemas` → `https://schemas.formspec.dev/v1/formspec.schema.json` untuk `spec/**/*.yaml|yml`), agar YAML editor punya autocomplete/validasi langsung setelah scaffold — tanpa download `schemas/` (init jadi offline-friendly). `copySchemas` pindah ke `cmd/formspec/schema.go` (dipakai `formspec schema fetch --out`). ✅ awal 2026-07; revisi URL langsung 2026-09-11 (changelog 2026-09-11-004)

### 3.2 `formspec dev` — verify against spec

- [x] 3.2.1 Verify 12 flags work: `--spec`, `--dsn`, `--addr`, `--listen` (none/local_http/unix_socket), `--app-endpoint` (none/local_http/unix_socket), `--runtime` (auto-detect + explicit override), `--dev`, `--dev-ui` (implies `--dev`+`--force`), `--force`, `--web-dir`, `--state-dir`, `--workspace-id`
- [x] 3.2.2 Runtime auto-detect — `go.mod` → go (local), `package.json` → node, `composer.json` → php, `requirements.txt`/`pyproject.toml` → python, `*.csproj` → dotnet (SDK belum tersedia) — per `01-formspec-dev.md` §4; ruby/java TIDAK termasuk auto-detect `formspec dev` (hanya konteks sidecar `spec.runtime`, lihat 7.15.1)
- [⏸️] 3.2.6 **SDK `dotnet` belum tersedia** — `*.csproj` sudah dikenali auto-detect `formspec dev` (`01-formspec-dev.md` §4) tetapi SDK untuk menjalankan handler-nya belum ada. Dulu hanya frasa di teks 3.2.2. Effort: large (SDK baru, satu bahasa penuh).
- [x] 3.2.3 SPA serving priority — explicit `--web-dir` > embedded `//go:embed` FS > auto-detect `renderers/web/dist/` (urutan per `01-formspec-dev.md` §6; path auto-detect di docs masih `web/dist/` — stale pasca-restructure 0.3, perbaiki docs)
- [x] 3.2.4 Config file `formspec-app.yaml` support
- [x] 3.2.5 Two personas: Persona A (embedded SPA, 80%) + Persona B (`--dev-ui` Vite HMR, 20%)
- [x] 3.2.6 Add `check`, `promote`, `logs` to CLI dispatcher switch (currently fall to `usage()`)

### 3.3 `formspec generate`

- [x] 3.3.1 `formspec generate --lang typescript --spec <path> --out <dir>` — generate typed TS client from manifests (sudah ada di `cmd/formspec/generate.go`; deny-by-default: entity tanpa `expose` → 0 kode; error bila tak ada entity exposed)
- [x] 3.3.2 Generate: typed interfaces, create/update input types, custom action params, `createApi()` function (semua di `writeEntityTypes`/`writeEntityApi`; key field literal, tidak di-camelCase — match wire JSON)
- [x] 3.3.3 Field type mapping per `03-formspec-generate.md` §3: string/uuid/date/datetime→string, integer→number, decimal/number→**string** (presisi), boolean→boolean, enum→union, json→unknown, relation→string, child→array; **`money`→`{amount: string; currency: string}`** (amount wajib string) dan **`file`/`attachment`→`{key, filename, content_type, size, checksum}`** kini ditetapkan di spec + diimplementasikan di `tsFieldType` (sebelumnya jatuh ke `unknown`)

### 3.4 Read-only CLI ops

- [x] 3.4.1 `formspec diff -f <path>` — compare local vs deployed (dry-run) — dalam scope single-server, "deployed" = schema DB vs manifest lokal via `MigrationRunner.PlanMigrations`; exit 1 bila ada perbedaan (gate CI). Lihat `docs_internal/plan/formspec-repl-seed-diff.md`. ✅ 2026-08-17
- [x] 3.4.2 `formspec get <kind> <name>` — fetch resource, table/JSON output. Beroperasi terhadap manifest lokal (Control Plane di-defer): `get <kind> [name] [--output table|json]`; `document` = alias `entity`. Lihat `cmd/formspec/get.go` + `docs_internal/plan/formspec-get-describe.md`.
- [x] 3.4.3 `formspec describe <kind> <name>` — detailed view: field, action, state machine, permission (`02-formspec-cli.md` §2). Untuk Entity: fields, actions (+ permission + impl), state machine, expose; non-Entity: spec JSON. Lihat `cmd/formspec/get.go`.

### 3.5 Mutation CLI ops

- [x] 3.5.1 `formspec delete <kind> <name> --confirm` — remove resource. Beroperasi terhadap manifest lokal (Control Plane di-defer): file satu-dokumen → hapus file; file multi-dokumen → hapus dokumen yang cocok (yaml.v3 node), sisakan lainnya. `--confirm` wajib. Lihat `cmd/formspec/delete.go` + `docs_internal/plan/formspec-delete.md`.

### 3.6 Engine-dependent CLI ops

- [x] 3.6.1 `formspec migrate plan|apply` — structural diff from Entity changes, applied via migration runner. `plan` → `PlanMigrations` + cetak DDL (tanpa eksekusi); `apply` → `ApplyMigrations` (idempotent). Daftar entity dibangun dari manifest lokal. Lihat `cmd/formspec/migrate.go` + `docs_internal/plan/formspec-migrate.md`.
- [x] 3.6.2 `formspec repl [--environment]` — interactive Starlark console, full `ctx.*` (via `NewCtxPrimitiveResolver` + `App.Database()`); mode one-shot `-e <expr>`; `--environment` diterima (policy Control Plane di-defer). Lihat `docs_internal/plan/formspec-repl-seed-diff.md`. ✅ 2026-08-17
- [x] 3.6.3 `formspec seed [--module]` — run seeders from YAML seed files (`kind: Seed`, format baru karena `formspec/seed` official module belum ada); idempotent via natural key. Lihat `docs_internal/plan/formspec-repl-seed-diff.md`. ✅ 2026-08-17
      → **Diperluas 2026-09-23** (changelog `2026-09-23-002`, plan
      `docs_internal/plan/seed-assets-and-reconcile.md`): marker `$asset`
      mengunggah aset seed **lewat storage service** dan menulis key kanonik
      (karena `cp` Makefile tidak punya padanan di prod); record yang sudah ada
      kini **di-reconcile** (field berbeda di-update, dilaporkan) alih-alih
      di-skip; field `masked` (write-only) & record non-`draft` dikecualikan;
      `resolveDSN` dipakai sehingga DSN relatif di-anchor ke project root.
- [x] 3.6.3a **`docs/kind/` belum punya halaman `Seed`** — ✅ **2026-09-26:
      TIDAK BERLAKU LAGI (stale).** Ketiga klaim diverifikasi ulang dan semuanya
      sudah berubah: (a) `docs/kind/data/Seed.md` **ada** (dibuat commit `dd3adc6`,
      "feat: kafe landed-cost/purchase events, seed kind…"); (b) `kindGroups`
      (`internal/genkinddocs/markdown.go:47`) **memuat** `"Seed": {Group: "data",
Plane: "resource"}` beserta komentar alasannya; (c) `docs/kind/README.md`
      menulis **34 kind** yang kini **cocok** dengan porosnya — grup `data/`
      berisi 11 file (Entity, Service, Config, Subscription, Workflow, Api, Webhook,
      Mockup, Integrator, KindDefinition, Seed) sesuai tabel README. `make
generate-kind-docs` karena itu menyentuh `Seed.md` (generated block
      `generated:meta` memuat `SeedSpec`). Tidak ada aksi lanjutan.
      **Bukti:** `ls docs/kind/data/Seed.md` → ada; `grep -n '"Seed"' internal/genkinddocs/markdown.go`
      → 1 hit; `head -8 docs/kind/data/Seed.md` menampilkan `Grup | data` +
      `Spec struct | SeedSpec`.
- [x] 3.6.5 DSN relatif di-anchor ke lokasi spec — path SQLite relative pada `--dsn` (dev/migrate/backup/restore/repl/archive) di-anchor ke project root yang di-derive dari `--spec` (bukan CWD), sehingga file db statis di mana pun perintah dijalankan; absolute & postgres tidak diubah. Lihat `docs_internal/plan/dsn-spec-anchored.md`. ✅ 2026-09-06 — `seed` menyusul 2026-09-23 (jalur ini sebelumnya terlewat: `make seed-kafe` meng-`cd` ke `examples/kafe` untuk menutupinya, yang berarti seed dari IDE/`make -C` menulis database lain).
- [x] 3.6.4 `formspec summary rebuild <entity>` — rebuild summary Entity dari replay event durable (`02-core-extended.md` §6). ✅ 2026-09-15 (`docs_internal/changelog/2026-09-15-007-summary-rebuild-command.md`). Kontrak `sources`/`join_key`/`rebuild` ada di canonical `pkg/spec.EntitySpec` + divalidasi (`ValidateEntitySpec`); rencana rebuild di `internal/summary/` (resolve entity → sources → stream durabel, orphaned dilaporkan); replay di `internal/subscription/replay.go` (`ReplayingSummaryProjection`, consumer group per run sehingga cursor worker live tidak tersentuh, filter/transform sama dengan delivery live); CLI `formspec summary list|rebuild` (`cmd/formspec/summary.go`) dengan `--dry-run`/`--reset`/`--subscriber`/`--workspace`/`--json`. **Bug nyata ikut tertutup:** channel `reliable_event` ternyata no-op di runtime (jatuh ke `default:` + warning) padahal `ValidateEventDurability` mewajibkannya — sekarang lewat outbox (`internal/action/deliver.go`); tanpa ini proyeksi yang digerakkan event durabel (jantung kafe) tidak pernah terisi. Adopsi `examples/kafe`: `cafe-stock/stock-level`, `cafe-stock/menu-cost`, `cafe-loyalty/member-point` kini mendeklarasikan §6 (`strategy: full` — biaya rata-rata bergerak & saldo poin bergantung seluruh riwayat). Bukti: `summary list` menampilkan 3 proyeksi + orphaned; `validate --schema schemas` → 72 manifest, 0 problem; `check` → 0 error/0 warning; test `internal/summary` (8), `internal/subscription/replay_test.go` (7), `internal/action/reliable_event_test.go` (2) hijau. **Sisa:** (a) 3.6.6, (b) 3.6.7, (c) 3.6.8.
- [⏸️] 3.6.6 Replay untuk proyeksi yang digerakkan `kind: Integrator` — integrator di-dispatch langsung saat delivery (`internal/integrator/dispatch.go`) dan **tidak** melewati stream durabel, jadi ia tidak punya riwayat untuk di-replay. Pilihan: (a) beri integrator jalur durabel (append ke stream + worker), atau (b) migrasikan proyeksi ke `kind: Subscription` `durability: durable`. Perlu keputusan desain dulu — jangan pilih salah satu tanpa itu.
  ⚠️ **Koreksi 2026-09-22 — contoh kafe di item ini sudah tidak berlaku.** Teks lama menyatakan "hari ini **semua** proyeksi `kafe` digerakkan integrator (`cafe-gl-integrator`), sehingga `summary rebuild` melaporkan sumbernya `orphaned`". Diverifikasi ulang: `grep "^kind: Integrator" examples/kafe/spec` → **0 hasil** (`cafe-gl-integrator` tidak ada lagi), dan ketiga proyeksi kafe kini `kind: Subscription`: `cafe-order` meng-`emit:` event `on_paid` pada transisi, lalu `gl/subscriptions/sales-to-journal.yaml` mendengarkannya dan `journalize.star` mem-posting jurnal — jalur yang **sudah** durabel (outbox, `payload.fields: [id]`, idempoten per `source_id`; E2E `ORD-2026-00021` → `JRN-2026-000055`, changelog `2026-09-21-003`). Jadi argumen item ini tetap berdiri **untuk integrator secara umum**, tetapi **bukan** sebagai deskripsi kafe: kafe sudah menjadi contoh pilihan (b).
- [⏸️] 3.6.7 Refresh schema registry dengan `sources`/`join_key`/`rebuild` — `formspec validate` tanpa `--schema` dulu melaporkan problem pada spec kafe karena schema App/Entity di `schemas.formspec.dev` belum memuat field kontrak §6. **Koreksi 2026-10-06: sisi kontrak sudah tertutup** — `make publish-schemas` dijalankan dan `dist` ter-commit (`df660c6`, `063e813`), jadi registry **bukan lagi** ketinggalan field §6. Yang tersisa hanyalah **publish yang belum sampai ke live**: file `Seed` belum ter-push (lihat catatan 2026-10-06 di bawah). Tidak bisa ditutup dari workspace — butuh `git push`. Tiket yang sama dengan `scope`/`public_entities` (`docs_internal/plan/schema-registry-sync.md`). Sementara itu: `--schema schemas` → 0 problem.
      **Ditambah 2026-09-25** (changelog `2026-09-25-008`): root online juga belum memuat kind `Seed` — `formspec validate` (mode registry) gagal `404 .../kinds/Seed.schema.json` dengan exit 2 pada SETIAP proyek yang memakai `kind: Seed`; kafe punya 4 manifest seperti itu. Root lokal sudah benar (`Seed` + `$defs.SeedEntity`), jadi ini murni publish tertinggal — tiket yang sama. Bukti: online kinds=35 tanpa `Seed` & tanpa `defs.SeedEntity`; `--schema schemas` → 85 manifest, 0 problem.
      **Ditambah 2026-10-06** (changelog `2026-10-06-005`): **akar masalah `Seed` 404 ketemu — lapis git, bukan generator.** `.gitignore` baris 45 memuat pola telanjang `dist/` (ditambahkan 2026-09-10 untuk artefak build Go) yang **juga mencocoki `schemas/dist/`**: `git check-ignore -v schemas/dist/v1/kinds/Seed.schema.json` → `.gitignore:45:dist/`. Karena file `dist/` yang lama sudah **tracked**, ignore tidak berlaku bagi mereka — sehingga `git add schemas/dist` (persis instruksi di `schemas/README.md`) tampak berhasil sementara **setiap file kind BARU dilewati senyap**. Terukur: `schemas/dist/v1/kinds/` 34 file di disk vs **33 tracked**; `git ls-tree -r HEAD \| grep -i seed.schema.json` → hanya `schemas/kinds/…`; commit terakhir yang menyentuh `dist` membawa root schema + `index.json` **tanpa** `kinds/`. Efeknya `index.json` (tracked) menyebut `Seed` yang file skemanya tak ada → 404 untuk setiap proyek ber-`kind: Seed`. **Ditutup di repo:** `!schemas/dist/` + dua `Seed.schema.json` di-stage + **guard `scripts/publish-schemas.sh`** (tiap file `dist/` dicek `git check-ignore --no-index`; ada yang ter-ignore → `exit 1`) + catatan di `schemas/README.md`. **Bukti:** delapan `dist/` lain tetap IGNORED · registry-mirror lokal tanpa `--schema` → service-demo **13/0**, kafe **88/0** (sebelumnya 404, exit 2) · guard merah saat negasi dihapus sementara (74 file). **Sisa yang perlu push:** `curl -sI .../v1/kinds/Seed.schema.json` → 200 dan `formspec validate --spec examples/kafe/spec` tanpa `--schema` → 0 problem; sesudah itu item ini bisa ditutup.
      ⚠️ **Koreksi 2026-09-22 — angka & daftar manifest di teks lama sudah drift.** Teks lama: "3 problem untuk ketiga summary kafe". Diukur ulang (`./bin/formspec validate --spec examples/kafe/spec`, v0.0.9): **78 manifest, 7 problem**, dan **tidak satu pun** pada ketiga summary. Yang gagal: `unit` (`ingredient`, `recipe.lines[].unit`), `hooks/0` (`stock-movement`), `emit` pada 4 transisi (`order`), `steps[0]` (`order-void-approval`), `body[5]`/`body[7]` (`receipt-digital`/`receipt-thermal` — item `qrcode` di `Print`). Dengan `--schema schemas` → **0 problem**, jadi diagnosisnya (registry ketinggalan kontrak) tetap benar; yang perlu diperbarui hanya angkanya. Sekaligus menutup klaim 2026-09-20 bahwa registry sudah selaras.
- [ ] 3.6.8 `rebuild.strategy: partial` belum benar-benar parsial — `RebuildSpec.Window`/`Since` **dideklarasikan dan dicetak di rencana, tetapi tidak dikonsumsi**: `subscription.ReplayOptions` tidak punya field window/since, dan replay selalu membaca stream dari `earliest`. Jadi `partial` hari ini hanya berarti "menolak `--reset`" (`cmd/formspec/summary.go`), bukan "hanya bangun ulang jendela itu". Dua hal yang harus diputuskan lebih dulu: (a) **format** `window` — docs §6 memakai `"7d"` sementara fixture memakai `"month"`, dan `ValidateEntitySpec` hanya memvalidasi `strategy` sehingga `window: "banana"` pun lolos; (b) **semantik `since`** (batas absolut vs relatif ke kalender bisnis, §9.4). Setelah itu: teruskan ke `ReplayOptions` + filter entri stream (pakai kolom waktu entri, `stream.Entry.Timestamp`/`occurred_at`) + `formspec check` menolak `partial` tanpa `window`/`since`.

### 3.7 Data lifecycle CLI ops

- [x] **3.10 ✅ 2026-09-29 — Command CLI memakai default project yang sama dengan `formspec dev` (config file).** Sebelumnya setiap command membawa literal sendiri (`specPath := "spec"`, `dsn := "sqlite:.formspec/data.db"`, workspace `"demo"`) — nilai yang `dev` pakai justru saat `formspec-app.yaml` **tidak ada**. Di setiap project yang punya config file, command dari direktori project menyasar database/tenant berbeda dari yang disajikan server. **Terukur di `examples/kafe`** (config `spec: spec`, `dsn: sqlite:.formspec/kafe.db`): binary lama → `repl` membuka `.formspec/data.db` dengan workspace `demo`; `dev` menyajikan `.formspec/kafe.db` sebagai `kafe`. **Akibat nyata:** alur repair 4.2.7 (`repl --no-sync -f` → `migrate apply`) mengerjakan DSN default masing-masing sehingga repair tidak pernah menyentuh DB yang dibaca `dev`; `formspec seed` tanpa flag membuat tenant **ketiga** (`74 inserted`, cabang → `[default:1, demo:2, kafe:2]`) yang tak pernah dibaca App. Ditutup dengan `loadProjectDefaults()`+`finishProjectDefaults()` (`cmd/formspec/project_defaults.go`) menerapkan urutan `dev` (flag → config → fallback → anchor DSN → aturan #48) di 12 command; `repl` mencetak `spec=… dsn=… workspace=…`. **Guard kelasnya** `TestNoCommandHardcodesProjectDefaults` — langsung menemukan 3 tersangka tambahan (`get`/`describe`/`delete`) yang juga diperbaiki. Plan `docs_internal/plan/cli-command-config-parity.md`, changelog `2026-09-29-001`. Effort selesai: medium.
- [⏸️] **3.10.1 Config file hanya dicari di CWD.** Sama seperti `dev` (parity disengaja), tetapi berarti menjalankan command dari luar direktori project (`cd /tmp && formspec migrate --spec …/examples/kafe/spec`) jatuh ke fallback (`spec`, `data.db`, workspace `default`) — dan tetap terlihat sukses. Menutupnya menuntut keputusan: mencari `formspec-app.yaml` di sebelah spec membuat CLI **menyimpang** dari `dev`, sedangkan menaikkan keduanya sekaligus mengubah perilaku `dev` (yang punya `chdirIfPositionalArg` untuk kasus ini). Effort: small–medium setelah keputusannya.

- [x] 3.7.1 `formspec backup create [--full|--incremental|--filter]` — backup DB + artifacts, open format — `--full` implemented (tar: manifest.json + `<module>_<entity>.jsonl`). **Dikoreksi 2026-09-26:** kalimat lama "`--incremental`/`--filter` belum (gap)" **setengah salah** — `--filter` **sudah jalan** (lihat 3.7.6, ditutup), sedangkan `--incremental` **ditolak secara eksplisit** dengan pesan jujur (`backup.go:230`: "`--full` is required (incremental not yet implemented)"), bukan diperlakukan sebagai `--full`. Sisa `--incremental` → 4.8.6 ⏸️ (3.7.5 ditutup 2026-10-02 sebagai duplikat). Lihat `docs_internal/plan/formspec-repl-seed-diff.md`. ✅ 2026-08-17
- [x] 3.7.2 `formspec backup inspect <file>` — inspect backup contents — baca manifest.json (created_at, driver, tables + counts). ✅ 2026-08-17
- [x] 3.7.3 `formspec restore --from <file> [--map-resource] [--conflict skip|overwrite|remap] [--dry-run]` — restore with conflict resolution — `--conflict skip|overwrite` + `--dry-run` implemented; `--map-resource` (memetakan `module/entity` sumber → target, **berbeda** dari `--conflict remap` yang mengganti natural key) belum → 3.7.7 ⏸️. ✅ 2026-08-17 (**Diperjelas 2026-09-26:** kalimat lama "`--map-resource`/`remap` belum" mencampur dua hal; `remap` **sudah** jalan sebagai nilai `--conflict`.)
- [x] 3.7.4 `formspec logs [--workspace] [--module] [--entity] [--action] [--level] [--since] [--until] [--request-id] [--output pretty|json] [--follow]` — tail structured logs (`09-observability.md` §7) — baca event log (`formspec_event_log`, channel audit_log) dengan filter workspace/module/entity + output pretty|json; `--action/--level/--since/--until/--request-id/--follow` belum (full 12-field request logging = Fase 8.2). ✅ 2026-08-17

#### 3.7b Sisa flag CLI data-lifecycle — dicatat 2026-09-22

Ditemukan oleh audit `docs_internal/plan/audit-open-items-prosa.md`: kelima gap di
bawah dulu hanya hidup sebagai frasa "belum (gap)" di dalam item `[x]`
3.7.1/3.7.3/3.7.4/4.8.1/4.9.5 — tidak ada item bernomor mana pun yang
melacaknya, dan grep nama flag-nya menghasilkan 0. Kelimanya **diterima parser**
tapi belum melakukan apa pun: CLI tampak mendukung, perilakunya tidak.

- [x] 3.7.5 `backup create --incremental` — **dikonsolidasikan 2026-10-02 ke 4.8.6 ⏸️** (dua item melacak satu masalah; 4.8.6 yang berlaku). Ditutup tanpa implementasi — tidak ada kode yang berubah. Riwayat: Saat ini `--incremental` **ditolak** dengan pesan eksplisit (`backup.go:230`: "`--full` is required (incremental not yet implemented)") — bukan diam-diam melakukan `--full`; redaksi lama "diperlakukan sama dengan `--full`" sudah dikoreksi 2026-09-26. Effort: medium (butuh penanda posisi/watermark per `<module>_<entity>` + uji round-trip full→incremental→restore).
- [x] 3.7.6 `backup create --filter` — batasi backup ke subset module/entity. ✅ **2026-09-26: SUDAH JALAN (stale).** `matchesFilter` (`cmd/formspec/backup.go:392`, menerima `module` atau `module/entity`) dipanggil di **dua** jalur: enumerasi tabel (`backup.go:92`) dan koleksi object storage (`backup.go:261` — sehingga `storage/` di tar pun ikut terfilter, bukan hanya tabelnya), dengan test `backup_test.go:206`. **Bukti:** `grep -n matchesFilter cmd/formspec/backup.go` → definisi + 2 pemakaian + test; pesan usage `backup.go:218` mencantumkan `[--filter <module|module/entity>]`.
- [x] 3.7.7 ✅ **2026-09-26** **`restore --map-resource` / `remap` — `--map-resource` kini ADA; `remap` ternyata sudah jalan.** `formspec restore --map-resource <src>=<dst>` (boleh diulang) mengarahkan record dari resource di arsip ke resource **lain** di spec tujuan (mis. sample produksi → entity dev). Tiga keputusan perilaku: (a) **target wajib ada di spec** — perintah berhenti `exit 2` **sebelum** menyentuh database, karena tanpa itu target yang tidak resolve membuat record dilewati dan laporan berbunyi `0 failed` (terbaca sukses); (b) kedua sisi menerima `module/entity` **atau** `module_entity` (spelling `backup inspect`), karena memaksa satu spelling membuat operator menerjemahkan sendiri tanpa terlihat; (c) laporan dikunci ke **entri arsip** (`RestoreEntityReport.MappedTo`) sehingga "berkas mana yang menghasilkan angka ini" tetap bisa dijawab. Pemetaan berlaku sebelum store **dan** spec entity di-resolve, jadi validasi memakai aturan entitas **tujuan**. Sekalian dikoreksi: `docs/cli-tools/02-formspec-cli.md` menulis `--map-resource` dengan komentar "remap saat konflik ID" — **keliru**; kini ada tabel yang membedakan pemetaan resource dari `--conflict remap` (ganti natural key). **Bukti:** `TestRestoreMapResource` (2 entity, **dibuktikan gagal** saat pemetaan dinetralkan: `MappedTo = ""`, `lead has 0 records`, `customer has 2 records`) + `TestParseResourceMap` (2 spelling + 4 bentuk ditolak). Changelog `2026-09-26-006`. Effort selesai: medium.
- [⏸️] 3.7.8 `logs` filter lanjutan (`--action`, `--level`, `--since`, `--until`, `--request-id`, `--follow`) — belum ada; `logs` saat ini hanya `--workspace/--module/--entity/--output`. Effort: medium (predicate per kolom event log + mode `--follow` streaming).
- [⏸️] 3.7.9 `archive restore-batch` — memulihkan satu batch archive. `archive run` + `view --batch-id` sudah jalan (JSONL open format, batch subdir); timpalannya belum. Effort: medium (baca batch dir + replay lewat jalur restore yang ada, dengan `--conflict`).

### 3.8 Deferred CLI ops

- [⏸️] `promote`, `archive`, `saga`, `module`, `sign`, `script`, `freeze`, `rollback`, `lock`, `workspace create`, `suspend scripts` — depend on Control Plane or backend maturity

### 3.9 Distribution — self-update

- [x] 3.9.1 `formspec upgrade` — self-update binary dari GitHub Releases tanpa install ulang: resolve versi (`releases/latest` / `--version`), download artifact `formspec-<os>-<arch>.{tar.gz,zip}` + verify `SHA256SUMS.txt`, smoke test, lalu swap atomik di `os.Executable()` (Windows: rename-ke-`.old`). Flag `--check`, `--dry-run`, `--force`, `--yes`. Prinsip eksplisit (bukan auto-update), tanpa `sudo`, tanpa dependency baru (semver comparator in-repo). File: `cmd/formspec/upgrade.go`, `cmd/formspec/release.go` (helper bersama, `spa.go` di-refactor memakainya), `cmd/formspec/semver.go`. Docs: `02-formspec-cli.md`, `install.md`, `releasing.md`, CLI skill, landing page. E2E terverifikasi (upgrade/rollback/idempotent). Lihat `docs_internal/plan/formspec-upgrade-command.md`, changelog `2026-09-13-004`. ✅ 2026-09-13
- [⏸️] 3.9.2 `formspec upgrade --spa` — auto-install cache SPA versi baru setelah upgrade (saat ini hanya hint `formspec spa install`). Di-defer: butuh `upgrade` stabil dulu; lihat plan §Out of Scope.

---

## Fase 4: JSONB Persist — Clean Renderer

**Goal**: Clean PersistBackend interface, extension, categories, migration engine, backup/restore, archiving, audit trail, query builder.

### 4.1 Clean PersistBackend interface

- [x] 4.1.1 Define `PersistBackend` Go interface — technology-agnostic (no SQL types: `*sql.DB`, `ExecContext`, `QueryContext`, `Driver()`) — `renderers/jsonb-persist/persist_backend.go` (SyncSchema/PlanSchema/NextKey/UninstallExtension/EntityStore/DriverName). ✅ 2026-08-17
- [x] 4.1.2 Required capabilities: structural diff apply, query resolution (identical results across backends), `ctx.next_key` (gap-free, atomic), index generation, clean extension uninstall — semua sudah ada di jsonb-persist (migrate diff, List/Aggregate/Window, natural-key counter, persist.indexes, UninstallExtension). ✅ 2026-08-17 (verifikasi)
- [x] 4.1.3 Refactor `renderers/jsonbpersist/` to implement `PersistBackend` interface — `MigrationRunner` kini memenuhi `PersistBackend` (SyncSchema/PlanSchema/NextKey/UninstallExtension/EntityStore/DriverName via `SetRegistry`). ✅ 2026-08-17

### 4.2 Migration engine

- [x] 4.2.1 Structural diff from Entity spec changes — field add/remove/type-change → storage-agnostic diff (not SQL text) — `PlanMigrations` kini diff tabel existing: field indexed/unique/natural-key baru → `ALTER TABLE ADD COLUMN` (SQLite plain column karena modernc tak bisa ADD generated column; PG generated). Field removal/type-change tetap dua-fase (4.2.2). ✅ 2026-08-17
- [x] 4.2.2 `renamed_from` field — two-phase removal (deprecate then drop) — `Field.RenamedFrom` ditambahkan + validasi (tidak boleh reserved/collide). Diff field-add tidak menandai kolom lama sebagai removal (rename ≠ drop+add). Drop dua-fase penuh tetap enhancement. ✅ 2026-08-17
- [x] 4.2.3 Per-Entity migration in one transaction — fail = full rollback; data in `data` JSONB never rewritten by structural migration — `ApplyMigrations` kini wrap DDL + record per entity dalam satu `BeginTx`/`Commit` (rollback on error). ✅ 2026-08-17
- [x] 4.2.4 `kind: Migration` — custom DDL (index, function, trigger, extension, materialized view); DML rejected at runtime — **DICABUT 2026-09-16** (changelog `2026-09-16-012`). Penggantinya `Entity.spec.persist.raw_ddl` (`pkg/spec/entity.go:2219`, `ValidateRawDDL`): DDL-only, `reason` wajib, `ddl` **atau** `ddl_by` per-dialek, forward-only, dan ikut jalur sync normal (`renderers/jsonb-persist/alter.go` langkah 6). **Dikoreksi 2026-09-26:** teks lama item ini masih menyatakan verb `formspec migrate plan|apply` "load `kind: Migration` manifests" — tidak lagi benar. ✅ 2026-08-17 → digantikan.
- [x] 4.2.5 Data migration ber-versi — script backfill dengan run/rollback manual — **DICABUT 2026-09-16** bersama `kind: DataMigration` (changelog `2026-09-16-012`). Keputusan penggantinya **eksplisit: perbaikan data TIDAK punya permukaan spec.** `formspec migrate` menolak perubahan yang butuh perbaikan data **dengan hitungan** (`RefuseUndeclared`, `renderers/jsonb-persist/diff.go:563` — menyebut jumlah baris/grup duplikat + `Remedy`), operator merapikannya sekali lewat `formspec repl -f <script>` (verb nyata, `cmd/formspec/repl.go:56`), lalu apply diulang. Alasan pencabutan: script backfill ber-versi di dalam spec berarti _framework menjalankan SQL/DML yang ditulis tangan pada data produksi_, yang justru ingin dihindari. **Dikoreksi 2026-09-28:** `formspec repl -f` **tidak cukup** — console juga menyelaraskan schema, sehingga gagal dengan penolakan yang sama; perintah yang benar `formspec repl --no-sync -f <script>` (4.2.7). **Dikoreksi 2026-09-26:** teks lama item ini masih menyatakan `kind: DataMigration` + `formspec migrate data <name> run|rollback` ada. ✅ 2026-08-17 → digantikan.
- [x] 4.2.6 ✅ **2026-09-26: TIDAK BERLAKU LAGI (moot) — subjeknya sudah dicabut.** Item ini meminta `dml`+`ddl` dalam satu manifest `kind: Migration` dibungkus satu transaksi. Tetapi **`kind: Migration` (beserta `DataMigrationSpec`, `MigrationSpec`, `ValidateMigrationSpec`, `MigrationDialects`, `dml`, `ddl_by`, schema, kind doc, dan verb `formspec migrate data`) DICABUT SELURUHNYA** pada changelog `2026-09-16-012` — **satu hari setelah** 4.2.6 difile (`2026-09-16-008`). Terverifikasi di kode hari ini: `grep -rn '"dml"\|yaml:"dml\|DDLByDialect' --include='*.go'` → **0 hasil**; `kind: Migration` **bukan** anggota `KnownKinds` (`internal/manifest/loader.go:309`); `formspec migrate` hanya punya verb `plan|apply` (`cmd/formspec/migrate.go:51`); `docs/kind/` tidak punya halaman `Migration`; dan **terukur**: manifest `kind: Migration` pada spec uji ditolak `formspec validate` dengan `unknown kind "Migration" for spec version v1` + `read Migration.schema.json: no such file`. **Penggantinya tidak punya `dml` sama sekali:** DDL di luar bahasa spec hidup di `Entity.spec.persist.raw_ddl` (DDL-only, `reason` wajib, **forward-only**) dan ikut jalur sync normal — jadi ia dijalankan di dalam tx per-entity yang sama dengan DDL struktural (`alter.go` langkah 1–6 dibangun jadi satu string lalu dieksekusi dalam tx `applyPlans`), sehingga masalah "dua pernyataan tidak atomik" yang item ini khawatirkan tidak punya bentuk lagi. Perbaikan data sengaja **tidak** punya permukaan spec: `formspec migrate` menolak dengan hitungan, operator merapikan lewat `formspec repl -f <script>` (verb yang memang ada), lalu apply diulang. Tidak ada aksi lanjutan. **Dikoreksi 2026-09-28:** perintahnya `formspec repl --no-sync -f <script>` — tanpa `--no-sync` console gagal dengan penolakan yang sama (4.2.7).

- [x] **4.2.7 ✅ 2026-09-28 — Permukaan repair bisa dibuka saat migrasi ditolak (deadlock gerbang).** Penolakan migrasi berlaku juga pada boot console: `formspec repl` → `formspec.New` → `SyncSchema` → penolakan yang sama, sehingga `Remedy` menunjuk perintah yang **tidak bisa start**. Terukur di kafe: `formspec dev` gagal `1 destructive change(s) refused` (partial unique index `table-session`, 19 sesi `open` pada satu meja, data menumpuk sebelum 10.34c), dan `formspec repl ... -e 'print("alive")'` gagal dengan pesan yang identik. Ditutup dengan `Config.SkipSchemaSync` (`resource/formspec.go`) + flag `formspec repl --no-sync` (bukan melewati gerbang: `dev`/`serve` tetap menolak) + `Remedy` menyebut perintah yang bisa dijalankan + hint saat penolakan. **Dua bug permukaan ditemukan saat memakainya dan ikut ditutup:** (a) `-f` melewati `syntax.ParseCompoundStmt` (parser modal REPL) sehingga mengeksekusi paling banyak statement pertama — file yang diawali komentar mengeksekusi **nol** sementara exit 0 dan mencetak `Ran <file>.`; kini `starlark.ExecFileOptions` + test yang dibuktikan gagal saat di-inject; (b) `formspec help` exit 1 karena berbagi jalur "unknown command". **Bukti E2E:** DB pra-repair (`/tmp/refuse/kafe.db`) → tanpa `--no-sync` menolak + hint; dengan `--no-sync` `alive`; repair 18 sesi → `open dupes: []`; `migrate apply` → `Applied 1 migration(s)`; index `idx_cafe_order_table_sessions_dining_table_id` ada & menegakkan; `formspec dev --dev-ui` → `engine loaded: 178 routes`. Plan `docs_internal/plan/migrasi-ditolak-permukaan-repair.md`, changelog `2026-09-28-008`. Effort selesai: small.
- [⏸️] **4.2.8 Script `.star` tidak diperiksa statis.** `formspec validate` dan `formspec check` tidak mem-parse file script, jadi script yang **rusak sintaksis** lolos keduanya dan baru gagal saat action-nya dipanggil. Terukur 2026-09-28: `def broken_repair(` disisipkan ke `examples/kafe/.../close_session_on_clear.star` → `89 manifest(s) validated, 0 problem(s) found` dan `0 error(s), 0 warning(s)`. Repair manual ikut terdampak (skrip repair sesi ini gagal karena implicit string concatenation, yang kini syntax error). Effort: small–medium (parse tiap `script`/`script_ref` dengan `starlark.SourceProgramOptions` + laporkan sebagai problem `validate`; batasi pada file yang memang direferensikan manifest).

### 4.3 Entity extension

- [x] 4.3.1 Extension read — `entity.ext("namespace").field` via JSONB column access — `EntityStore.mergeExtensions` membaca kolom `ext_{namespace}` dan menggabungkannya ke `Data` di bawah key namespace saat `hydrateAndCompute`; registry me-wire `SetExtensions` dari semua entity `ExtendStorage` yang menarget entity ini. ✅ 2026-08-17
- [x] 4.3.2 Extension write — populate `ext_{namespace}` column — `EntityStore.splitExtensions` memisahkan data namespace dari base data; Insert & Update menulis payload ke kolom `ext_{namespace}` (terisolasi dari JSONB base); `validateKnownFields` menerima key namespace. ✅ 2026-08-17
- [x] 4.3.3 Extension uninstall — `DROP COLUMN ext_{namespace}` + remove registry entry + namespace lock (never reused) — `MigrationRunner.UninstallExtension` (drop column + set status='locked' dalam satu tx). ✅ 2026-08-17
- [x] 4.3.4 Extension namespace collision prevention — `formspec apply` rejects duplicate namespace for same target — `PlanMigrations` cek `formspec_extensions` (namespace reservation) dan tolak bila sudah dipakai. ✅ 2026-08-17 (verifikasi — sudah terimplementasi)
- [x] 4.3.5 Extension `validate:` (additive business rule) — runs after base Entity L1–L6 validation, never overrides it; read-only access to base fields, may only require its own namespaced fields (`docs/spec/backend/03-entity-extension.md` §5) — `ExtendStorage.Validate` (script ref) ditambahkan; eksekusi runtime script validate = enhancement. ✅ 2026-08-17

### 4.4 Category schemas

- [x] 4.4.1 6 category schemas: operational, financial, compliance, analytics, master, archive — `CategorySchema` map (ddl.go). ✅ 2026-08-17 (verifikasi — sudah terimplementasi)
- [x] 4.4.2 Cross-category JOIN block — `FORMSPEC.PERSIST.CROSS_CATEGORY` error — `resolveRelations` memblokir resolusi relasi lintas kategori (via `SetTargetCategoryResolver` di registry). ✅ 2026-08-17
- [x] 4.4.3 `spec.persist.category` enforcement at query time — `qualifiedTable()` memakai schema kategori (PG). ✅ 2026-08-17 (verifikasi — sudah terimplementasi)
- [x] 4.4.4 **Blokir cross-category di jalur baca: tetap log + skip — keputusan eksplisit, bukan sisa.** ✅ **Diputuskan 2026-09-26.** Item ini menawarkan tiga perilaku (error / warning terhitung / tetap skip) dan mencatat satu alasan menahan: "list yang sudah berjalan di produksi akan putus". Setelah diperiksa, **tidak ada yang perlu diubah**, karena pertahanan berlapis sudah ada lebih dulu: `ValidateRelations` **menolak** relasi lintas kategori saat spec dimuat (`formspec validate` + boot), sehingga `resolveRelations` hanya bisa mencapai cabang ini bila spec ditulis/di-bypass di luar jalur normal. Jadi mengubahnya menjadi hard error menukar **nol** cakupan tambahan dengan **risiko outage pada list yang sudah jalan** — persis trade-off yang tidak diambil item ini. Yang **kurang** hanyalah kejelasan bahwa ini keputusan; sekarang tercatat di sini. **Teramati:** `renderers/jsonb-persist/crud.go:2429-2437` (`log.Printf("[WARN] resolve relation %s: cross-category JOIN blocked …")` + `continue`); validator di `ValidateRelations`. Tidak ada aksi lanjutan.

### 4.5 Query Builder

- [x] 4.5.1 Aggregate functions — `sum`, `count`, `avg`, `min`, `max` — `EntityStore.Aggregate()` (renderers/jsonb-persist/crud.go), pre-aggregation filters sama dengan List. ✅ 2026-08-17
- [x] 4.5.2 `group_by` — single + multi-field grouping — `AggregateParams.GroupBy []string`. ✅ 2026-08-17
- [x] 4.5.3 `having` — post-aggregation filter — `AggregateParams.Having []FilterOp` diterapkan ke ekspresi agregat (mis. `HAVING SUM(amount) > 500`). ✅ 2026-08-17
- [x] 4.5.4 `date_trunc` — time bucketing (day/week/month/quarter/year) — `AggregateParams.DateTrunc` (PG `date_trunc`, SQLite `strftime`). ✅ 2026-08-17
- [x] 4.5.5 Window functions — running total, ranking — `EntityStore.Window()` (`running_total`/`rank`/`row_number`, `PartitionBy`/`OrderBy`). ✅ 2026-08-17
- [x] 4.5.6 `include()` batched — eager-load relations in one query per level (N+1 prevention) — `resolveRelations` (crud.go) batch-fetch per relation field (`WHERE id IN (...)`), bukan per record. ✅ 2026-08-17 (verifikasi — sudah terimplementasi)

### 4.6 Tree/hierarchy

- [x] 4.6.1 Materialized path column — `_tpath_{field_name}` for `tree: true` self-referential relations; path format: `""` (root) or `parent.child.grandchild` — DDL + `setTreePaths` (compute on insert). ✅ 2026-08-17
- [x] 4.6.2 Tree operators — `descendant_of` → `LIKE 'prefix.%'`, `ancestor_of` → PK lookup, `child_of` → FK query, `root` → `parent_id IS NULL` — filter ops di List (`descendant_of`/`child_of`/`root`; `ancestor_of` = PK lookup via `eq`). ✅ 2026-08-17
- [x] 4.6.3 Cycle detection — server-side on create/update/move/reparent → `VALIDATION_ERROR` (422) — `setTreePaths` menolak bila path parent mengandung id record (cycle). ✅ 2026-08-17

### 4.7 Business audit trail

- [x] 4.7.1 `audit: true` on action → append-only audit entries — `writeAuditLog` dipanggil dari Insert/Update/SoftDelete (crud.go); `AuditAction` create/update/delete/action. ✅ 2026-08-17 (verifikasi — sudah terimplementasi; audit ditulis untuk semua mutasi CRUD, bukan hanya action ber-`audited`)
- [x] 4.7.2 Per-entry: actor, action name (not "document updated"), timestamp (`created_at`), before/after diff, request_id — `AuditRecord` kini punya `request_id` (kolom + write + scan); `InsertParams`/`UpdateParams.RequestID` di-thread dari handler. Actor/action/timestamp/before-after diff sudah ada. ✅ 2026-08-17
- [x] 4.7.3 Immutability — no API update/delete; framework writes only — audit log append-only, tidak ada route update/delete. ✅ 2026-08-17 (verifikasi — sudah terimplementasi)
- [x] 4.7.4 Queryable per record — source for Timeline kind; filterable with standard query operators — `AuditStore.ListByEntity`/`ListByWorkspace`. ✅ 2026-08-17 (verifikasi — sudah terimplementasi)

### 4.8 Backup & restore

- [x] 4.8.1 Full + incremental backup — DB dump + file storage (ctx.storage), open format — `--full` + file storage lewat **storage service** (`ResolveStorage`, bukan path `{state}/storage` hardcoded — gap kafe **10.16**), `storage/<object key>` di tar. `--incremental` **belum** → 4.8.6 ⏸️. ✅ 2026-09-24
- [x] 4.8.2 Filterable backup — by workspace, module, entity — `formspec backup create --filter <module|module/entity>`. **Catatan:** `--filter` menyaring module/entity; **workspace masih hardcoded `"demo"`** → 4.8.7 ⏸️. ✅ 2026-08-17 · **Diperjelas 2026-09-26:** `--filter` **memang sudah berfungsi** (`matchesFilter` `cmd/formspec/backup.go:392` dipakai di dua tempat: enumerasi tabel `:92` dan koleksi object storage `:261`, dengan test `backup_test.go:206`). Yang belum hanyalah poros **workspace**, dan itu item 4.8.7 — bukan `--filter`-nya.
- [x] 4.8.3 Restore with conflict resolution — `skip|overwrite|remap`, `--dry-run` compatibility report — `formspec restore --conflict skip|overwrite|remap`; `remap` menetapkan natural key baru (`-r1`, `-r2`, …) dan insert sebagai record baru; `--dry-run` mencetak compatibility report per-entity (restore/skip/remap/fail). ✅ 2026-08-17
- [x] 4.8.4 Credible exit — read/export operations never license-gated — backup/restore tidak license-gated (prinsip desain, terpenuhi). ✅ 2026-08-17
- [x] 4.8.5 Outbox reconciliation pass WAJIB setelah restore — entri outbox pending di-replay/diverifikasi terhadap state hasil restore sebelum workspace kembali melayani (`platform/04-control-plane.md` §6.1, MUST — berlaku juga single-server) — `formspec restore` kini menjalankan `reconcileOutbox` (hitung pending + lapor; replay penuh = tugas outbox worker). ✅ 2026-08-17
- [x] 3.7.6 (baris pengalih) — **ditutup 2026-10-02.** Nomor 3.7.6 sudah dipakai item `--filter` yang selesai (✅ di atas); sisa yang dulu dialihkan ke sini kini dilacak di tempatnya masing-masing: `--incremental` → **4.8.6 ⏸️**, poros workspace `backup`/`restore` → **4.8.7 ✅**. Tidak ada pekerjaan yang hilang.
- [⏸️] **4.8.6 `backup create --incremental` belum ada.** `--full` wajib; `--incremental` ditolak dengan pesan eksplisit (bukan diam-diam melakukan full). Belum ada definisi "sejak backup terakhir" (watermark per tabel? `updated_at`? manifest terakhir?), jadi butuh keputusan bentuk dulu. Effort: large.
- [x] **4.8.7 ✅ 2026-09-26 — `backup`/`restore` membaca workspace AKTIF, bukan `"demo"` hardcoded.** Tujuh literal `"demo"` di `cmd/formspec/backup.go` diganti aturan **#48 yang sama dengan `formspec dev`** (aturan diekstrak jadi `activeWorkspaceFor(specPath, current, explicit)` di `workspace_active.go`, dipakai bersama — bukan disalin). Kedua perintah menerima `--workspace <slug>`; `manifest.json` mencatat `workspace`; `backup inspect` menampilkannya (atau menyatakan arsip lama tidak mencatatnya); `restore --dry-run` memakai tenant yang sama. Cabang tambahan untuk jalur CLI: bila pemanggil **tidak tahu** tenant-nya dan spec mendeklarasikan **tepat satu**, slug itu diadopsi; bila **beberapa**, ia **tidak menebak** — default dipakai + peringatan. **Terukur pada DB kafe nyata** (salinan `examples/kafe/.formspec/kafe.db`): `27 table(s), 0 record(s)` → **`160 record(s)`**, `"workspace": "kafe"` di manifest, `restore --dry-run` mengenali `kafe`. **Bukti:** `TestBackupReadsNamedWorkspace` (seed ke `staging`, memverifikasi `staging=2` **dan** `demo=0` lebih dulu, lalu mem-pin dua cabang aturan); `go test ./cmd/formspec/ ./resource/` hijau. Changelog `2026-09-26-007`. Effort selesai: medium.

### 4.9 Data archiving

- [x] 4.9.1 Archive transactions (`characteristic: transaction`) to Parquet when age ≥ `retention.archive_after` — `formspec archive run --max-age <dur> [--dry-run]` mengarsip transaksi tua ke format JSONL open (Parquet = enhancement), hapus baris transaksi. ✅ 2026-08-17
- [x] 4.9.2 Master snapshot "as-of" — referenced masters snapshotted alongside archived transactions — `snapshotMasters` (archive run) snapshot master yang direferensikan belongs_to + set `locked_for_deletion`. ✅ 2026-08-17
- [x] 4.9.3 `locked_for_deletion` flag — master referenced by archived transaction cannot be deleted — `SoftDelete` memblokir bila `data.locked_for_deletion == true` (4.9.4). ✅ 2026-08-17
- [x] 4.9.4 `FORMSPEC.ARCHIVE.LOCKED_FOR_DELETION` error code — `spec.ErrorArchiveLockedForDeletion` + enforcement di `SoftDelete`. ✅ 2026-08-17
- [x] 4.9.5 `formspec archive run [--dry-run]` / `view --batch-id` / `restore-batch` — `run` + `view` implemented (JSONL open format, batch subdir); `restore-batch` belum. ✅ 2026-08-17

### 4.10 Soft-delete & soft-deactivation

- [x] 4.10.1 `persist.soft_delete: true` → `deleted_at` column + query auto-filters — sudah ada: `deleted_at` column di DDL (default true, bisa di-disable), semua query auto-filter `deleted_at IS NULL`, `SoftDelete()` method. ✅ 2026-08-17 (verifikasi — sudah terimplementasi sebelumnya)
- [x] 4.10.2 `is_active` + `deactivate`/`reactivate` pattern — dropdown filters `is_active: true` for new transactions; list shows all — `soft_deactivate: {enabled: true}` kini inject `is_active` field (default true) + `deactivate`/`reactivate` actions (store methods, handlers, routes, permissions). Dropdown filter `is_active: true` untuk transaksi baru = concern frontend (Fase 5). Lihat `docs_internal/plan/soft-deactivate.md`. ✅ 2026-08-17

---

## Fase 5: Frontend — shadcn-shell Completeness

**Progress (2026-08-27 audit)**: COMPLETE — semua item 5.1–5.16 ✅ kecuali yang ditandai ⏸️ (5.6.7 RRULE exception, Report export async job). Rincian implementasi: `docs_internal/plan/fase5-completion.md` + changelog 2026-08-24-027 s/d -032.

**Goal**: Semua UI kind, widget, contract, dan FormSpecExpr sesuai spec. Bisa dites end-to-end.

### 5.1 App Shell

- [x] 5.1.1 `sidebar-nav` — full chrome, side navigation, breadcrumb (verified, working)
- [x] 5.1.2 `topnav` — full chrome, top navigation — `TopNavShell` (nav atas + dropdown group + breadcrumb + mobile drawer), menu di-resolve via `useResolvedMenu` (sama dgn Sidebar). Contoh `examples/arisan/`. ✅ 2026-08-19
- [x] 5.1.3 `no-nav` — chrome minimal tanpa nav standar — App renderer archetype (bukan "landing"/marketing): chrome & auth dipisah (`app_renderer` = chrome; `access: public|private` = auth). `NoNavShell` chrome-only + blok `section:` declarative (hero/feature_grid/card/carousel/cta) + anonim create (list/find/create publik di module App `access: public`) + login `returnTo`. Contoh `examples/storefront/`. Lihat `docs_internal/plan/landing-page.md` + changelog 2026-08-19-001/002. ✅ 2026-08-19
- [x] 5.1.4 Entity detail URL dan breadcrumb memakai `natural_key` bila tersedia, fallback ke UUID; `unique` biasa tidak dipilih implisit; URL UUID lama tetap valid. ✅ 2026-09-25 — `docs_internal/plan/entity-natural-key-routing.md`
- [x] 5.1.5 `natural_key` mengimplikasikan `unique` (bukan `required` — presence keputusan author); `unique: false` eksplisit ditolak lewat presence flag `Field.uniqueSet`. `natural_key_entry` (`auto_generated` | `user_entry` | `auto_generated_if_empty`) menyatakan siapa pemasok nilai, diresolusi konvensi sehingga deklarasi lama tetap sah; `auto_generated` mengabaikan nilai caller dan tidak muncul sebagai input di form; key opsional melewatkan nilai kosong pada index unik. ✅ 2026-09-26 — changelog `2026-09-26-001`

### 5.1a App-level fields (chrome/auth/shell/persist)

- [x] 5.1a.1 `App.spec.access` — `private` (default) | `public` — sumbu auth terpisah dari `app_renderer`; pemicu bundle anonim + data seam publik + boleh root `/`. ✅ 2026-08-19
- [x] 5.1a.2 `App.spec.stack_family` — shell implementasi (default `react-shadcn`); ekspos di bundle; validasi renderer penuh = 5.16. ✅ 2026-08-19
- [x] 5.1a.3 `App.spec.persist_backend` — backend persist entity (default `jsonb-persist`); nama tak ter-install / tak implement kontrak `formspec/storage.entity-persist` → ERROR di apply/check. ✅ 2026-08-19
- [x] 5.1a.4 `App.spec.public_entities` **dihapus** — grant anonim App publik diturunkan dari permukaan view (menu ∪ `registered_views`) × `public` per-view, lewat `(*ui.Registry).DerivePublicGrants` (`internal/ui/surface.go`). Aturannya graf **fetch klien** (Table→`list`, Form mode→`create`/`update`+`find`/`find`, `context.entity`→`find`, `picker.entity`/`price_entity`→`list`, `relation` ter-render→`list`+`find`; `delete` tak pernah implisit), scope baris dari `param` route, dan `find` dibuang saat grant ber-scope. `BuildBundle` + `publicGrants()` memakai helper reachability yang sama sehingga bundle dan endpoint tak bisa berbeda. ✅ 2026-10-04 — plan `implicit-public-grants.md`, changelog `2026-10-04-002`. **Bukti:** `TestDerivePublicGrants_KafeQR` · `TestKafeQR_BundleAndEndpointAgree` (**5 = 5**; insiden lama **13 vs 4**) · lint 0 · validate 89/0. **Menutup** kafe 10.13. **Membuka** kafe 10.76 ⏸️ (`menu-item-price` kehilangan scope `branch_id` → anonim membaca harga semua cabang).

### 5.2 `kind: Page`

- [x] 5.2.1 Blocks composition — form, table, component blocks (himpunan tertutup `06-page-kinds.md` §1; `widget` milik Dashboard §7, `html` block tidak ada di spec); permission-gated per block
- [x] 5.2.2 Tabs variant — mutually exclusive with blocks; permission-checked per tab
- [x] 5.2.3 Master-detail split — `layout.mode: split`, `binds: {source, param}`; detail refetch on selection change — `PageSplit` di `PageRenderer.tsx` (master Table block + detail block via `binds`, refetch on selection, empty-state tanpa seleksi). ✅ 2026-08-24
- [x] 5.2.4 Full-custom — single `component:` block — full-bleed render tanpa grid wrapper (blocks.length===1 && blocks[0].component). ✅ 2026-08-24
- [x] 5.2.5 Custom Page (`mode: custom`) — full-code page with `binds` footprint (entities, actions, subscribe); top rung of frontend control — `CustomPage` di `PageRenderer.tsx` + `bindsToNeeds` → `AssetNeeds`. ✅ 2026-08-24
- [x] 5.2.6 Configuration Page pattern — `characteristic: reference` entities → no New/Delete buttons, only Update surfaced
- [x] 5.2.7 Declarative banner/alert/notice block — `AlertBlock` di `SectionBlocks.tsx` (variant info/success/warning/destructive); perluasan `SectionBlock` closed set. ✅ 2026-08-24
- [x] 5.2.8 Section `align` + card stretch & icon — `SectionBlock.align: left|center|right` (default left, konsisten `TableColumn.align`); hapus `mx-auto` pada `<section>` card/feature_grid/carousel (grid item + auto margin → shrink-to-fit → ilusi center/left bergantung panjang konten); `CardBlock` kini merender `item.icon`. Plan `docs_internal/plan/section-block-align.md`, changelog `2026-09-01-003`. ✅ 2026-09-01
- [x] 5.2.9 Devserver DX parity untuk native binary — package `internal/devserver` (PID file, auto-kill, EnsurePort, WatchSpec, ServeAppUntilSignal) diekstrak dari `cmd/formspec/dev.go`; `formspec-registry` kini auto-kill instance sebelumnya, resolve port, graceful shutdown, dan spec hot-reload saat `--spec` eksplisit. Changelog `2026-09-01-004`. ✅ 2026-09-01
- [x] 5.2.10 Fix hot-reload native binary — `App.ListenAndServe()` memakai `a.handler` langsung → `http.Server` menyajikan handler lama setelah `ReloadSpec()` (reload "complete" tapi konten stale). Kini pakai `a.Handler()` (wrapper dinamis, sama seperti `formspec dev`). Changelog `2026-09-01-005`. ✅ 2026-09-01
- [x] 5.2.11 Auth redesign Fase 1 — Unified User Model: hapus avatar workspace kedua di shell (satu identitas user); workspace jadi label di dropdown UserMenu; tambah field user `email` + `status` (active|pending|disabled); login blokir user pending. Changelog `2026-09-02-001`. ✅ 2026-09-02
- [x] 5.2.12 Auth redesign Fase 2 — First-Run Setup Wizard: deteksi `setup_required` (workspace tanpa user); endpoint publik `GET/POST /{ws}/_ui/setup` (SetupFirstAdmin + seed owner roles); flag di meta bundle; `SetupScreen.tsx` + redirect. Changelog `2026-09-02-001`. ✅ 2026-09-02
- [x] 5.2.13 Auth redesign Fase 4 — Registration Policies: `Settings.Registration` (open|approval|closed + default_role); `Register` per policy (open→active+role, approval→pending, closed→403); `ApproveUser` + endpoint admin `POST /_ui/auth/approve`; wire policy saat boot + hot-reload. Changelog `2026-09-02-002`. ✅ 2026-09-02
- [x] 5.2.14 Auth redesign Fase 3 — Page-Level Access Gating: `App.spec.access` jadi default; page override per-page (`public: true` anonim di App private, `public: false` session di App public); guard frontend surface-aware (`router.tsx` + `SurfaceShell`); test page public di App private ship ke anonim. Changelog `2026-09-02-003`. ✅ 2026-09-02
- [x] 5.2.15 Auth redesign Fase 6 — Vendor Upgrade Flow: vendor entity + field `status` (pending|active|rejected) + action `approve` (native, `registry.vendors.update`); handler `registry.vendor.approve` aktifkan vendor + grant role `vendor` + perms `registry.vendor.*`/`registry.module.*` ke owner; `GrantRoles` di auth service; UI vendor-signup 4 langkah. Changelog `2026-09-02-004`. ✅ 2026-09-02
- [x] 5.2.16 Auth redesign Fase 5 — OAuth Multi-Provider: package `internal/auth/oauth` (Provider interface, OIDC discovery, OAuth2 generic, preset google/microsoft/github); `Settings.Auth.Providers` deklaratif via Config; `OAuthLogin` find-or-create by email; endpoint authorize + callback (CSRF state, token di URL fragment); tombol provider di LoginScreen + `OAuthCallback`; wire boot + hot-reload. Changelog `2026-09-03-001`. ✅ 2026-09-03
- [x] 5.2.17 Dokumentasi auth — `docs/guides/authentication.md`: model user, setup wizard, registration policy, page gating, cara mengaktifkan OAuth multi-provider + memilih provider (preset vs custom), panduan sandbox testing (Keycloak/GitHub/Google/Microsoft), keterbatasan relative redirect_uri. ✅ 2026-09-03
- [x] 5.2.18 Password management — guard password kosong di service layer (`Login` tolak `""`); `ChangePassword` self-service (endpoint `POST /{ws}/_ui/auth/change-password` + dialog di user menu, verifikasi current password hanya jika user punya hash); reset password via email (`internal/mail` SMTP mailer, default Mailpit; `POST /{ws}/_ui/auth/forgot-password` selalu 200 tanpa bocorkan email; `POST /{ws}/_ui/auth/reset-password` single-use token TTL 15 menit; link `?reset_token=` karena middleware auth membaca `?token=` sebagai JWT). Changelog `2026-09-03-002`, plan `docs_internal/plan/password-management-plan.md`. ✅ 2026-09-03
- [x] 5.2.19 Fix Account Pre-Hijacking (OAuth auto-link by unverified email) — `oauth.UserInfo.EmailVerified` (parse klaim `email_verified`); field user `email_verified`/`oauth_provider`/`oauth_sub`; `Register` terima email → akun unverified + email verifikasi (token single-use TTL 24 jam, endpoint `POST /_ui/auth/verify-email` + `resend-verification`); `OAuthLogin` rewrite: identity-first `(provider,sub)` → email match gated verified (takeover akun unverified via provider-verified, blokir `ErrEmailUnverified`, `ErrAccountLinkRequired` untuk akun password) → create baru; explicit linking `POST /_ui/auth/oauth/{provider}/link`; notifikasi email saat akun di-link; `/meta/me` expose `email_verified`; frontend register + email field + OAuth callback handle error fragment. Changelog `2026-09-03-004`, plan `docs_internal/plan/account-pre-hijacking-fix.md`. ✅ 2026-09-03
- [⏸️] 5.2.20 Multi-identity per user — saat ini satu `oauth_provider`/`oauth_sub` per user (identity lookup best-effort, email sebagai fallback). Deferred: array `oauth_identities` + query by element. ⏸️ Deferred — butuh perubahan store/query JSONB; endpoint explicit link sudah ada.
- [x] 5.2.21 Auth seragam dev/prod — hapus bypass `DevValidator` + auto-seed `admin/admin`; flag `--dev-auth` (dan key config `dev-auth:`) dihapus; first-run via setup wizard di semua mode; secret JWT dev auto-generate + persist ke `.formspec/dev-jwt-secret` (sesi bertahan antar restart, prod tetap wajib secret eksplisit); `SetupScreen` redirect ke login bila setup tidak diperlukan / 409 `SETUP_COMPLETE`. Changelog `2026-09-07-001`. ✅ 2026-09-07
- [x] 5.2.22 Guard first-run register — `Register` menolak saat workspace belum punya user (403 `SETUP_REQUIRED`); tanpa ini register membuat user non-admin pertama → setup terkunci 409 → `_admin` 403 selamanya (loop error). Frontend: register 403 → redirect setup; route `/register` saat `setup_required` → setup; route login/register saat authenticated → bounce surface root. Changelog `2026-09-07-002`. ✅ 2026-09-07
- [x] 5.2.23 Fix loop setup↔login lintas surface — meta store simpan satu bundle global tanpa catatan surface: bundle app (`setup_required=true`) dipakai guard admin surface setelah setup sukses → redirect balik ke setup ∞ (forward tumbuh rekursif). Fix: `loadedSurface` di meta store, bundle lintas surface dianggap null di SurfaceShell, bundle dikosongkan saat load gagal (403/401/error), SetupScreen reset store sebelum navigate. Changelog `2026-09-07-003`. ✅ 2026-09-07
- [x] 5.2.21 Tombol "Link {provider}" di UI profile — `oauthState.Mode` ("" = login, "link"); `HandleOAuthAuthorize` baca `?mode=link`; `HandleOAuthCallback` mode=link redirect ke `/{ws}/_admin/oauth/link-callback#code=...&provider=...` (tanpa `OAuthLogin`); `OAuthLinkCallback.tsx` POST code ke `POST /_ui/auth/oauth/{provider}/link`; `LinkedAccountsDialog.tsx` (daftar provider + status Linked/Link) di user menu; `/meta/me` expose `oauth_provider`; route link-callback. + **Unlink** `POST /_ui/auth/oauth/{provider}/unlink` (`EntityUserStore.UnlinkOAuthIdentity`, `ErrNotLinked`, `ErrUnlinkRequiresPassword` cegah lockout akun pure-OAuth, notifikasi email; tombol Unlink two-step confirm di dialog + refresh identity). Changelog `2026-09-04-001` + `2026-09-04-002`, plan `docs_internal/plan/oauth-link-ui-plan.md`. ✅ 2026-09-04
- [x] 5.2.22 Auth screens spec-driven — `App.spec.auth` (login_page/setup_page/change_password_page/reset_password_page/oauth_callback_page + chrome_auth) mereferensikan `kind: Page`; default = spec default embedded di module `formspec.core` (`internal/auth/module/pages/*.yaml`, `mode: custom` + `asset: formspec-core/auth/<slot>`); resolve di meta bundle (`bundle.app.auth`, `resolveAuth`); frontend `AuthPage` render default (built-in component) atau override (PageRenderer); `ChangePasswordDialog` → page `ChangePasswordPage` (dialog tetap ada); `AuthArea` overridable via `chrome_auth`; `formspec.auth` namespace (login/register/logout/changePassword/resetPassword) untuk custom auth pages; validasi ref `module/name` di `ValidateAppSpec`; schema `AppAuth` di-generate. Changelog `2026-09-06-001`, plan `docs_internal/plan/auth-screens-spec-driven.md`. ✅ 2026-09-06
- [x] 5.2.23 Fix blank login screen (regresi 5.2.22) — `AuthPage` lookup builtin dengan ref format salah (`formspec.core/login` vs key map `formspec-core/auth/login`) → return `null` untuk semua default auth screen; diperbaiki dengan map jembatan `SLOT_BUILTIN_REFS`. Plus: pindahkan fallback `?? []` ke luar selector zustand di 4 komponen (`LoginScreen`, `DashboardRenderer`, `FormRenderer`, `DetailPage`) — array baru tiap `getSnapshot` memicu React error #185 (max update depth) saat `bundle` masih `null`. Changelog `2026-09-06-005`. ✅ 2026-09-06
- [x] 5.2.24 Fix 5 pre-existing type-error `tsc -b` — `useRenderContext` (fetchPublicConfig butuh getter `() => KyInstance`, bukan instance), `TableRenderer`/`OverlayHost`/`GrantsEditor` (field optional `spec.entity`/`module` → `?? ""` sebelum dipass ke param `string`). `tsc -b` kini exit 0; `npm run build` + 166 test vitest green. Changelog `2026-09-06-006`. ✅ 2026-09-06

### 5.3 `kind: Form`

- [x] 5.3.1 `render` mode enforcement — `modal` (dialog overlay), `drawer` (slide-in panel), `separate_page` (own route); design-time, no runtime switch
- [x] 5.3.2 Wire `OverlayHost` — connected to Form.render modal/drawer
- [x] 5.3.3 409 conflict handling — CAS version mismatch → "Data telah diubah oleh pengguna lain", offer reload + re-apply changes — `FormRenderer` catches `FormaApiError` with `status === 409` from both auto-save and manual submit, stashes the pending edits, and shows `ConfirmDialog` ("Reload & Reapply"); confirming re-fetches the record (fresh `recordVersion`) then layers the stashed edits back on top via `reset()`. ✅ 2026-08-22
- [x] 5.3.4 Lifecycle UI patterns — plain_crud (no submit), 2-step+auto-save (default), 2-step manual (Save Draft + Submit buttons), 1-step create-submit (single button, no draft)
- [x] 5.3.5 FormSpecExpr — `visible_when`, `readonly_when`, `required_when`, `compute` per field

### 5.4 `kind: Table`

- [x] 5.4.1 Fix hardcoded `/_admin` prefix — surface-aware navigation (`/app` vs `/_admin`)
- [x] 5.4.2 Inline editing — `inline_edit: true`, cell editable for non-readonly/computed/immutable fields; CAS per baris; submitted rows reject inline-edit — `TableRenderer` (editingCell + commitInlineEdit via `apiPatch` + CAS version; 409 → stale badge). ✅ 2026-08-24
- [x] 5.4.3 Batch editing — `batch_edit: [field, ...]`, update per baris, partial failure reported (not all-or-nothing) — `TableRenderer` (batchDraft + applyBatchEdit loop PATCH per row + per-row report). ✅ 2026-08-24
- [x] 5.4.4 Column derivation fix — N priority columns (natural key → label_field → status → transaction_date → rest), overflow accessible via row expand/detail; NEVER silently dropped — `derive.ts` `DERIVED_TABLE_VISIBLE_COLUMNS=8` + priority sort; `TableRenderer` row-expand toggle. ✅ 2026-08-24
- [x] 5.4.5 `realtime: true` — auto-subscribe + patch rows in-place (depends on 5.8) — `useRealtime` di `TableRenderer` → silent refetch saat event entity cocok. ✅ 2026-08-24
- [x] 5.4.6 Fix table auto-refresh setelah overlay (modal/drawer) close — `TableRenderer` mendeteksi transisi URL `action` ada → hilang (overlay `OverlayHost` ditutup setelah save/cancel) lalu silent refetch; mencakup create & edit, dan berlaku untuk table derived maupun table block/tab di Page. Changelog `2026-08-24-011`. ✅ 2026-08-24

### 5.5 `kind: Kanban`

- [x] 5.5.1 Drag-and-drop — wire `@dnd-kit/core`; drag card antar kolom → PATCH `status_field`
- [x] 5.5.2 Optimistic update with server-enforced rollback (409 → snapshot restore)
- [x] 5.5.3 `drag_guard` FormSpecExpr — pre-check UX, prevent drop that server will reject — `KanbanRenderer.tsx` eval `drag_guard` sebelum drop (server tetap final). ✅ 2026-08-24 (WS-C, changelog 027)
- [x] 5.5.4 WIP limits — `max_cards_per_column`, soft UX enforcement (visual + toast)
- [x] 5.5.5 Zero-config — derive columns from state machine or `group_by` enum — ✅ 2026-08-24 (WS-C, changelog 027)
- [x] 5.5.6 Click card → detail page navigation
- [x] 5.5.7 Row actions (view/edit/delete/custom) with confirm + permission check
- [x] 5.5.8 Filter columns from `filters` manifest — Select dropdown per filter field
- [x] 5.5.9 Filter generik server-side — `filters` objek (`default` seed, type `select`/`date`/`text`, `today()`) + `fixed_filters` immutable; `transaction_date[eq]=` untuk scope tanggal board (lihat `docs_internal/plan/kanban-filter-tanggal-filter-generik.md`)

### 5.6 `kind: Calendar`

- [x] 5.6.1 Month/week/day/resource views — `views: [month, week, day, resource]` — ✅ 2026-08-24 (WS-D, changelog 028)
- [x] 5.6.2 Event rendering — from `date_field` + optional `end_field`; title from `label_field` or `title_field` — ✅ 2026-08-24 (changelog 028)
- [x] 5.6.3 Click event → detail Page/Form; click empty slot → Form create with date pre-filled — ✅ 2026-08-24 (`prefill.{date_field}`, changelog 028)
- [x] 5.6.4 Drag reschedule — call `update` action on date_field (server-enforced); submitted immutable rows disable drag — ✅ 2026-08-24 (changelog 028)
- [x] 5.6.5 RRULE recurrence — parse RFC 5545, expand to instances for visible date range (render-time, not materialized) — library `rrule`; ✅ 2026-08-24 (changelog 028)
- [x] 5.6.6 Resource view — one lane per `resource_field` value; color by `color_field` — ✅ 2026-08-24 (changelog 028)
- [⏸️] 5.6.7 RRULE exception per-instance — ubah/batalkan satu occurrence tanpa ubah pattern; butuh model data exception tersendiri (row terpisah + override tanggal asli); ditunda ke iterasi berikutnya (`06-page-kinds.md` §5 "Di luar cakupan v1")

### 5.7 `kind: Dashboard` + `kind: Widget`

- [x] 5.7.1 Widget `stat` — fetch from summary entity, display number with label — `MetricWidget` di `DashboardRenderer.tsx` (query FormSpecExpr subset → server list filters). ✅ 2026-08-24
- [x] 5.7.2 Widget `chart` — bar/line/pie from summary entity; add chart library dependency (katalog widget bawaan spec HANYA `stat` + `chart` — `07-component-kinds.md` §2; ListWidget/SummaryWidget tidak ada di spec, usulkan ke spec dulu bila dibutuhkan) — `ChartWidget` + `LineChart` SVG (tanpa dependency chart library; satu series per `group_by`). ✅ 2026-08-24
- [x] 5.7.3 Dashboard customizable — `customizable: true`, user add/remove/reorder widgets from catalog; preference stored as runtime preference (not YAML) — `DashboardRenderer.tsx` + `stores/prefs.ts`. ✅ 2026-08-24 (WS-E, changelog 027)
- [x] 5.7.4 Widget catalog visibility — derived from user's `list`/`view` permission on underlying entity (not manual flag) — widget terpasang di-filter permission di `DashboardRenderer`; katalog add/remove ikut filter. ✅ 2026-08-24 (WS-E, changelog 027)

### 5.8 Realtime WebSocket

- [x] 5.8.1 `useRealtime(entityRef)` hook — subscribe to `entity:{module}.{name}` channels — `hooks/useRealtime.ts` (singleton WS, subscribe/unsubscribe frames, union subscriber). ✅ 2026-08-24
- [x] 5.8.2 Optimistic update — patch rendered data in-place on event — konsumen (TableRenderer) silent refetch saat `tick` berubah (non-durable, no replay). ✅ 2026-08-24
- [x] 5.8.3 Reconnect → refetch via `/_meta/ui`, no replay — `tick` naik saat reconnect → konsumen re-run load; re-register subscription penuh. ✅ 2026-08-24
- [x] 5.8.4 WS handshake auth via single-use ticket (`?ticket=`) — `POST /_ui/_ws/ticket` (Bearer) issue opaque ticket TTL 30s single-use bound ke identity+workspace; `HandleWS` konsumsi sebelum upgrade; `?token=` tetap fallback (deprekasi bertahap); client `useRealtime` fetch ticket per koneksi + reconnect. ✅ 2026-09-22 — plan `docs_internal/plan/ws-ticket-auth-plan.md`; `internal/api/wsticket.go` (store + `HandleWSTicket` + rate limit), `HandleWS` konsumsi `?ticket=` **sebelum** upgrade (ticket lintas-workspace → 401), `useRealtime.ts` minta ticket per koneksi/reconnect + fallback `?token=`. 7 test baru (`wsticket_test.go`): issue+connect, unauthenticated→401, single-use (replay→401), expired, cross-workspace→401, fallback `?token=` tetap hijau, rate limit. Doc: `docs/renderers/realtime.md` §2.1 + `docs/spec/frontend/04-spec-resolution-api.md` §5.
      ⏸️ **Sisa**: pencabutan jalur `?token=` untuk `_ws` setelah semua konsumen migrasi (dan rekomendasi strip query-string path `_ws` di access log proxy) — sengaja ditahan agar client lama tidak putus; lihat plan §6.

### 5.9 Asset Component Contract

> **Track C widget strategy** (docs_internal/plan/widget-strategy.md): 5.9.2 `formspec.components`, 5.9.3
> `formspec.ui`, 5.9.4 `formspec.files` = jalur #2 "UI rich" — expose chrome struktural shadcn ke
> component `asset`, bukan dijadikan field widget.

- [x] 5.9.1 Dynamic ES module loader — `shell/AssetRenderer.tsx` (dynamic `import()` + `mount`/`unmount`) + backend `GET /_ui/assets/{module}/{path*}` (`internal/api/asset.go`, serve `{root}/modules/{module}/assets/{path}`). ✅ 2026-08-24
- [x] 5.9.2 `formspec` client injection — `lib/formspec-client.ts` (`api`, `subscribe`, `navigate`, `theme`, `ui`, `components`); di-inject ke asset via `AssetRenderer`. ✅ 2026-08-24
- [x] 5.9.3 `formspec.ui` centralized service — `lib/ui.ts` (`toast` re-export + `confirm`/`dialog`/`drawer` promise-based) + `shell/UiHost.tsx` (ConfirmDialog + Sheet); 9 renderer migrasi import `sonner` → `@/lib/ui`. ✅ 2026-08-24
- [x] 5.9.4 `formspec.files` — `lib/files.ts` (download tray store + `files` API) + `shell/DownloadTray.tsx`; di-inject ke asset. ✅ 2026-08-24
- [x] 5.9.5 `formspec.form(entity, {mode, id?})` — `lib/headless-form.ts` (`createHeadlessForm`): field state, dirty tracking, validasi client dari field rules (zod via `lib/zod-schema.ts`), FormSpecExpr eval, `submit()` dengan CAS version. ✅ 2026-08-24
- [x] 5.9.6 `needs:` declaration — `BlockRef.needs` (`AssetNeeds`); `formspec.api` di-wrap `withNeeds` — panggilan di luar `needs` gagal client-side. ✅ 2026-08-24
- [x] 5.9.7 CSP sandbox — asset endpoint set `Content-Security-Policy` (`connect-src 'self'`). ✅ 2026-08-24
- [x] 5.9.8 CSS scoped — `AssetRenderer` mount ke Shadow DOM host (CSS component tidak bocor). ✅ 2026-08-24

### 5.10 Missing input widgets

> **Keputusan strategi widget** (docs_internal/plan/widget-strategy.md): registry widget dasar = **closed set**
> yang dikurasi (`07-component-kinds.md` §1) — **TIDAK** semua komponen shadcn di-mapping ke widget.
> Tiga jalur "UI rich": (1) field widget — set tertutup dikurasi (bagian ini), (2) chrome struktural via
> `formspec.ui`/`formspec.components`/`formspec.files` untuk component `asset` (5.9), (3) block presentasi
> deklaratif di Page (5.2.7 + section blocks). Komponen shadcn struktural (alert, alertDialog,
> dropdown-menu, popover, dll) dipakai internal kinds / di-expose via `formspec.*` — bukan dijadikan widget.

- [x] 5.10.1 DatePicker — `DateInput` SUDAH meng-cover `date`/`datetime` via native `showPicker()` (keputusan desain, bukan `react-day-picker`) + input ketik terformat. ✅
- [x] 5.10.2 JsonEditor — `JsonInput` SUDAH ada (`widgets/JsonInput.tsx`): textarea + pretty-print + parse validasi live. ✅
- [x] 5.10.3 ChildGrid — `ChildTable` SUDAH ada (`widgets/ChildTable.tsx`): inline table utk `child` entities `storage: table`, sorting, computed, readonly_when, auto-fill. ✅
- [x] 5.10.4 RichText — `RichText` widget (`widgets/RichText.tsx`): toolbar bold/italic/list/link/heading via contentEditable + `document.execCommand`; client sanitizer `lib/sanitize.ts` (mirror server `sanitizeHTML`); render sanitized di DetailPage. ✅ 2026-08-24
- [x] 5.10.5 FileInput — `FileInput` widget (`widgets/FileInput.tsx`): upload via `POST /{module}/{entity}/{id}/{field}`, preview image/PDF, size/type enforcement dari `StorageSpec`; object key disimpan di field. ✅ 2026-08-24
- [x] 5.10.6 DecimalInput — nama manifest distinct `decimalinput` terdaftar di router (`FormFieldWidget`) + `derive.formWidget()` (decimal → `decimalinput`); `NumberInput` handle scale/rounding. ✅ 2026-08-24
- [x] 5.10.7 DateTimeInput — nama manifest distinct `datetimeinput` terdaftar di router + `derive.formWidget()` (datetime → `datetimeinput`); `DateInput` handle `withTime`. ✅ 2026-08-24
- [x] 5.10.8 Base UI components — breadcrumb/skeleton/badge/card/pagination SUDAH ada + `EmptyState` (`components/ui/empty-state.tsx`). ✅ 2026-08-24
- [x] 5.10.9 Textarea — `TextareaInput` widget (`widgets/TextareaInput.tsx`, wrap `components/ui/textarea.tsx`); router case `textarea` + field type `text`; render pre-wrap di DetailPage. ✅ 2026-08-24

#### 5.10a Field widget kurasi (Track B, docs_internal/plan/widget-strategy.md)

- [x] 5.10.10 RadioGroup — `RadioGroup` widget (`widgets/RadioGroup.tsx`, button-based, no dep); single-choice enum alternatif `select`. ✅ 2026-08-24
- [x] 5.10.11 Combobox — `Combobox` widget (`widgets/Combobox.tsx`, custom dropdown + search, no dep); searchable select utk enum besar. ✅ 2026-08-24
- [x] 5.10.12 Password — `PasswordInput` widget (`widgets/PasswordInput.tsx`); masking + reveal toggle. ✅ 2026-08-24
- [x] 5.10.13 Slider — `SliderInput` widget (`widgets/SliderInput.tsx`, native range); number field utk range (min/max dari rules, step dari scale). ✅ 2026-08-24
- [x] 5.10.14 Tags — `TagsInput` widget (`widgets/TagsInput.tsx`); multi-select disimpan sebagai **comma-separated string** (frontend-only, tanpa backend change). Opsi array (backend) ditunda. ✅ 2026-08-24
- [⏸️] 5.10.16 **Opsi array (backend) untuk field comma-separated belum ada** — 5.10.14 memakai comma-separated string frontend-only; bentuk array-nya ditunda tanpa item pelacak. Effort: medium (kontrak field array + migrasi nilai lama ke array).
- [x] 5.10.15 SelectMultiTag — widget `select-multi-tag` (`widgets/SelectMultiTag.tsx`) + atribut `Field.options` (`pkg/spec/entity.go`): tag yang sumbernya **deklarasi**, bukan ketikan. Opsi terpilih tidak ditawarkan lagi; chip urut deklarasi (tampilan saja — array tersimpan mempertahankan urutan isian); nilai di luar deklarasi tetap tampil & tidak dibuang saat save; nilai non-daftar → error terlihat; bentuk nilai dipertahankan (`json` array / `string` comma-separated, tipe skalar ikut deklarasi). Satu resolver bersama (`lib/field-options.ts`) dipakai Form, DetailPage, dan sel tabel/listing. Katalog form 24 → 25. ✅ 2026-09-24
      **Terukur** (browser `:8099`): pilih Senin → daftar tinggal `[Selasa…Minggu]`; chip `Senin, Selasa, Jumat` (urutan deklarasi) walau urutan klik berbeda; detail page `preCount: 0` (chip berlabel, bukan `[1,2]` mentah); simpan UI → API membaca `days_of_week: [1, 5, 2]` dengan `types: [int,int,int]` (angka, bukan `"1"`). Duplikat ditolak engine: `options[1] duplicates the value "1" from options[0]`. `go test ./...` 39 paket hijau · vitest **403 lulus** (+20) · `tsc -b` bersih · kafe `validate` 85 manifest 0 problem. Plan `docs_internal/plan/select-multi-tag-widget.md`, changelog `2026-09-24-010`. **Sisa → 5.10.17 ⏸️ + 5.10.18 ⏸️.**
- [x] 5.10.17 ✅ **2026-09-26** **Wizard step kini memakai kosakata widget yang sama dengan Form.** `WizardFormStep` merender setiap input dengan tangan dan tidak pernah memanggil `FormFieldWidget`, jadi **seluruh katalog diabaikan di dalam wizard**. Bukti pada manifest nyata: `cafe-order/shift.counted_cash` (`type: money`) tampil sebagai `<Input type="number">` polos — currency tidak terlihat; `close-shift-wizard.yaml` juga menulis `widget: relation-picker`. Perbaikannya **bukan menyalin switch** (itu dua implementasi katalog yang harus disinkronkan — kelas 5.11.7/5.14.6), melainkan **mendelegasikan cabang generik+date+numeric ke `FormFieldWidget`** dengan satu set `WIZARD_NATIVE_WIDGETS`. **Tiga cabang sengaja tetap ditulis tangan** karena berperilaku khusus Form tidak punya: relation (opsi di-fetch + filter `depends_on`), enum (`<select>` polos), boolean (checkbox ber-label terikat `id`). Hasilnya widget baru di katalog **otomatis bekerja di wizard**. Sekalian: help/`label` kini lewat helper `wrap(...)` (satu situs), menutup pola yang sama dengan 5.23.3. **Bukti:** `npx vitest run` **503 lulus** / 36 file, `tsc -b` bersih; test baru (4) **dibuktikan gagal** saat kedua guard dinetralkan (`if (true)`), dan ikut mem-pin yang **tidak** boleh hilang (`depends_on`, `relation?.resource`, `enum_values`). Changelog `2026-09-26-011`. Effort selesai: medium.
- [x] 5.10.18 ✅ 2026-09-25 **Filter `select` pada field non-enum/non-relasi kini memakai `Field.options`.** `useSelectFilterOptions` memakai `fieldOptions()`, jadi field `json`/`string` ber-`options` menghasilkan filter ber-caption; `ListingRenderer`/`TableRenderer` memakai resolver yang sama (nilai terkirim tetap nilai deklarasi, bukan caption). Menutup sisa 5.10.15. Sebelumnya: `useSelectFilterOptions` hanya menurunkan opsi dari `enum_values` dan relasi, sehingga field ber-`options` menghasilkan filter kosong. ✅ 2026-09-25
- [x] 5.10.19 Cardinality `options` di Entity — `Field.multiple` (pointer bool) + matriks tipe × cardinality: `json`/`string` **wajib** `multiple` bila `options` dinyatakan; skalar non-enum boleh `options` (single-select ber-caption); `multiple: true` pada skalar dan `enum`+`options` ditolak. `options` kini sah di field skalar, jadi celah "known, separately tracked gap" di godoc `pkg/spec/entity.go` akhirnya punya kontrak (caption `enum` tetap terbuka → 5.10.23). **Form mengikuti Entity** di dua lapis: `deriveFormWidget` (`engine/derive.ts`) menjadi satu sumber untuk Form turunan **dan** Form yang ditulis (sebelumnya router authored hanya mengenal `money`/`time` → field `json` ber-`options` jatuh ke editor JSON mentah), dan `formspec check` menolak widget yang bertentangan **dua arah**. Konsumen ikut: sel tabel + halaman detail (nilai tunggal ber-caption = badge), derivasi kolom Table/Listing/Kanban, tiga picker single (`select`/`radio-group`/`combobox`) membaca `options` + mengirim nilai skalar. **Terukur** (browser `:8099`, App `kafe-pos`, form promo **tanpa** `widget:` di YAML): chip hari urut deklarasi ("Senin, Jumat" walau klik Jumat dulu) · DB `days_of_week=[5,1]` **int** (urutan isian, tampilan tidak menulis ulang data) · `channel='qris'` skalar · detail page menampilkan chip + caption `QRIS`. Uji negatif: 3 bentuk ditolak `validate` (hapus `multiple`, `multiple: true` pada `integer`, `options` pada `enum`), 2 kontradiksi ditolak `check`. `go test ./...` 39 paket · vitest **454** · `tsc -b` bersih · kafe `validate` 85 manifest 0 problem · `check` 0/0. Plan `docs_internal/plan/options-cardinality-entity.md`, changelog `2026-09-25-007`. **Sisa → 5.10.20 ⏸️ + 5.10.21 ⏸️ + 5.10.22 ⏸️ + 5.10.23 ⏸️.** ✅ 2026-09-25
- [⏸️] 5.10.20 **Penegakan server cardinality/keanggotaan `options` belum ada.** Cardinality hanya menentukan **bentuk** nilai di klien; server tidak menolak array untuk field `multiple: false`, dan nilai di luar deklarasi tetap tersimpan (keputusan sadar, lihat 10.33 — data lama/spec menyusut tidak boleh hilang senyap). Yang belum diputuskan: apakah `multiple: false` harus menolak array, dan apakah nilai di luar set harus ditolak atau sekadar ditandai. **Teramati:** `grep -n "Multiple" renderers/jsonb-persist/*.go` → 0 hasil (tidak ada pembacaan `Field.Multiple` di jalur tulis). Effort: medium (keputusan perilaku + validasi di `validateFieldRules` + 422 dengan pesan yang bisa ditindak).
- [⏸️] 5.10.21 **Paritas lintas-shell untuk aturan "Form mengikuti Entity".** Aturan cardinality (derivasi widget + gerbang `formspec check`) ditegakkan di renderer `react-shadcn` dan validator engine; shell lain yang mengimplementasikan kontrak renderer tidak mendapat warisan ini. Sama kelasnya dengan 5.14.6 (presedensi caption). **Teramati:** `grep -rn "deriveFormWidget" renderers/` → hanya `react-shadcn`. Effort: medium (angkut aturan ke kontrak renderer + fixture bersama).
- [x] 5.10.22 ✅ **2026-09-26** **`formspec generate --lang typescript` kini mengemit union dari `options`.** `tsFieldType` (`cmd/formspec/generate.go`) sebelumnya hanya membaca `EnumValues`, jadi field skalar ber-`options` (`type: integer` + `options 1=Senin`) digenerate sebagai `number` dan field `json` ber-`options` sebagai `unknown` — pemanggil boleh menulis nilai yang tidak dideklarasikan server, padahal `options` ada untuk mencegahnya. Kini `tsOptionType`/`tsOptionUnion` mengemit literal union, dan **cardinality menentukan bentuk** (properti data, bukan form): nilai tunggal → `1 | 2 | 7` / `"qris" | "cash"`; `type: json` + `multiple: true` → `Array<1 | 2 | 7>`; `type: string` + `multiple: true` → tetap `string` karena wire form-nya comma-separated list dan tidak ada tipe string TS yang bisa mempersempitnya (**penolakan yang disengaja** — union di sana akan menjadi janji palsu). Label tidak ikut (tipe bukan tempat caption). **Terukur pada spec sungguhan:** `"days_of_week"?: Array<1 | 2 | 7> | null;` dan `"channel"?: "qris" | "cash" | null;`. **Bukti:** 4 test baru, 2 di antaranya **dibuktikan gagal** saat cabang union dikembalikan ke tipe terbuka; `go test ./cmd/formspec/` hijau. Changelog `2026-09-26-004`. Effort selesai: small.
- [⏸️] 5.10.23 **Caption untuk `enum` belum ada (hanya sebagian tertutup).** 5.10.19 membuat `options` sah di field skalar **non-enum**, jadi single-select ber-caption kini bisa dinyatakan — tetapi `enum` sengaja tetap hanya `enum_values` (punya CHECK constraint di DB), sehingga `enum` masih menampilkan nilai mentah tanpa caption. Godoc `pkg/spec/entity.go` (`validateFieldOptionsShape`) menyebutnya "tracked separately" — item ini rujukannya. Effort: medium (bentuk deklarasi caption untuk enum + validasi 1:1 terhadap `enum_values` + pembacaan di tiga picker).
- [x] 5.10.24 ✅ **2026-09-26** **`TestKafe_OnPaidCreatesBalancedJournal`/`TestKafe_PurchaseReceivedCreatesJournal` flaky — DIPERBAIKI, dan akarnya DUA, bukan satu.** (a) **Balapan di helper test:** `waitForJournal` (`resource/o2c_e2e_test.go:304`) hanya menunggu `countJournalEntries() > 0` — barisnya **ada** — lalu test meng-assert `status == "posted"`; baris lahir saat `journalize.star` meng-insert, sedangkan `posted` di-set handler setelahnya. Kini kedua helper (`waitForJournal` + `waitForJournalSource`) mem-poll **status** dengan deadline dan mencetak status terakhir saat timeout. (b) **Ditemukan saat mengerjakannya — dan ini yang membuat keenam test gagal PERMANEN hari ini: tanggal hardcoded `2026-09-22` di test melewati `BackdatePolicy` default (`max_days_back: 3`)** → `FORMSPEC.TXN.BACKDATE_EXCEEDED … got 4 days`. Jadi keempat berkas test memakai helper `recentDate()` yang sudah ada di paket ini. **Terukur sebelum fix:** `go test ./resource/ -run TestKafe` → **6 FAIL** (`OnPaidCreatesBalancedJournal`, `OnPaidIsIdempotent`, `JournalizeRejectsOrderWithNoAmount`, `PurchaseReceivedCreatesJournal`, `ReceiveGoodsMaintainsStockLevel`, `LandedCostAllocationStrategy`, `WeightStrategyRefusesUnknownWeight`). **Sesudah fix:** `-run TestKafe` → `ok`, dan `go test ./resource/ -count=1` **5/5 run hijau** (sebelumnya HEAD bersih `dd3adc6` gagal **2/8**). Atribusi lama ke item kafe 10.7 (`gl/config/gl.yaml`) tetap **salah**: `git diff HEAD -- examples/kafe/spec/modules/gl/` kosong. Changelog `2026-09-26-002`. Effort selesai: small.

### 5.11 FormSpecExpr

- [x] 5.11.1 Audit grammar vs spec — verify lexer→parser→evaluator supports all operators from `08-formspec-expr.md` §2 — ✅ 2026-08-24 (WS-F, changelog 027)
- [x] 5.11.2 Deploy-time static validation — `formspec apply`/`formspec check` rejects unresolvable field references + invalid grammar (ERROR, not warning) — `cmd/formspec/check.go` `checkExpr` + `validateExprGrammar`. ✅ 2026-08-24 (WS-F)
- [x] 5.11.3 Runtime error state — nonexistent field reference → visible error state (never silent fail-safe/evaluate to `false`) — `FormRenderer.tsx` expression error banner. ✅ 2026-08-24 (WS-F)
- [x] 5.11.4 `title` interpolation — `"Order {order.number}"` pattern in Page/Wizard/Print titles — `PageRenderer` fetch record utk token title + `interpolate()`; Print/Wizard sudah pakai pola sama. ✅ 2026-08-24
- [x] 5.11.5 Cross-shell conformance test suite — identical interpretation across shells — ✅ 2026-08-24 (WS-F, changelog 027). **Koreksi jujur 2026-09-24:** yang ada adalah `lib/formspec-expr/formaexpr.test.ts` (94 test saat itu, kini 109) yang mem-pin **satu** shell (`react-shadcn`); tidak ada fixture lintas-shell (`find . -iname "*conformance*"` → 0 hasil), dan `react-shadcn` memang satu-satunya shell (`ls renderers/` → `jsonb-persist`, `react-shadcn`). Klaim "identical interpretation across shells" belum bisa diverifikasi sampai shell kedua ada → lihat 5.11.7.
- [x] 5.11.6 Paritas literal string + gate token `formspec check` — kutip tunggal kini diterima klien (`lexer.ts` `readString(quote)`, `'x'` ≡ `"x"` seperti Starlark) dan gate memindai token (string opaque, karakter/operator asing & string tak tertutup ditolak), sehingga ekspresi yang lolos apply benar-benar bisa dievaluasi. Akar bug: 6 field `promo-form` memakai `fields.type == 'percentage'` → 4 banner "unexpected token: '", padahal `formspec check` melaporkan 0 error. ✅ 2026-09-24. Changelog `2026-09-24-007`. **Sisa → 5.11.7 ⏸️ + 5.11.8 ⏸️.**
- [⏸️] 5.11.7 **Paritas grammar dijaga manual — aturan lexer diduplikasi, tanpa fixture bersama.** Gate `validateExprGrammar` (`cmd/formspec/check.go`) memindai token dengan aturan yang disalin dari `lib/formspec-expr/lexer.ts`; tidak ada sumber tunggal maupun fixture yang dieksekusi **kedua** sisi. Jadi kelas bug yang sama (klien menolak yang server/gate terima, atau sebaliknya) masih bisa terulang untuk konstruk baru, dan test saat ini hanya mem-pin ekspresi yang sudah diketahui — bukan paritas itu sendiri. **Teramati**: versi pertama gate 5.11.6 menolak `*` (`sum([i.quantity * i.unit_price for i in fields.items])` di `examples/cafe`) — hierarki operator Go vs TS tidak identik; ketahuan hanya karena kebetulan ada contoh yang memakainya. Effort: medium (fixture JSON dibagi: input ekspresi + valid/tidak, dikonsumsi test Go **dan** vitest).
- [⏸️] 5.11.8 **Identifier tak dikenal dalam perbandingan lolos sebagai `null` tanpa warning.** `fields.status == open` (nilai tanpa kutip — salah tulis yang wajar, karena YAML di sekitarnya juga tanpa kutip) dievaluasi `false` **diam-diam**: `evalFormSpecExpr` → `{value:false, valid:true, warnings:[]}` (teramati lewat probe vitest 2026-09-24), sehingga `strictEvalFormSpecExpr` tidak melaporkan error, banner §4 tidak muncul, dan gate pun meloloskannya (`validateExprGrammar("fields.status == open")` → `""`). Akibatnya field diam-diam tersembunyi/terkunci dan penulis tidak tahu ekspresinya salah — kelas "fail-safe" yang dilarang §4, berbeda dari 5.11.6 (yang bisa diparse, hanya bermakna lain). Effort: medium (butuh keputusan kontrak: daftar identifier yang sah selain `fields.*` — `None`/`null`, `true`/`false`, `i`/variabel comprehension — agar perbaikan tidak menolak komprehensi).

### 5.12 Spec Resolution API

- [x] 5.12.1 ETag caching — conditional GET with 304 for `/_meta/ui` bundle — `internal/api/meta.go` (ETag over data portion + `If-None-Match` → 304). ✅ 2026-08-24
- [x] 5.12.2 `label_field` fallback — `natural key` → `name` → `title` → `number` → `id` (`04-spec-resolution-api.md` §2) — `internal/ui/meta.go` `labelField()` + `TestLabelFieldFallbacks`. ✅ 2026-08-24
- [x] 5.12.3 Entity schema shape — `label_field`, `lifecycle`, `actions` with embedded `permission` — `EntitySchema`/`ActionSummary` di `internal/ui/meta.go`. ✅ 2026-08-24
- [x] 5.12.4 Permission filtering — entity (404 if no list/view), page (hidden if missing permission), action (permission string sent, not filtered) — ✅ 2026-09-22 (**ketiga klaim diverifikasi terhadap kode + test, bukan hanya dianggap selesai**). **entity:** `BuildBundle` (`internal/ui/meta.go:398`) menyaring entitas tanpa `{module}.{plural}.list`/`.view` dari bundle, dan `internal/api/handler.go` mengembalikan **404** di surface UI tanpa list/view (`TestEnforcement_UISurface_EntityList_NoPerm_404`) sementara surface eksternal **403** (`TestEnforcement_External_EntityList_NoPerm_403`) — surface-aware, sesuai Core §15.2. **page:** `allowedPage` (`meta.go:634`) menyembunyikan Page yang `permissions`-nya tidak dimiliki caller; diverifikasi `TestBuildBundlePermissionFiltering` (sub-test "no permissions sees nothing entity-backed" → hanya `settings` yang lolos). **action:** `buildEntitySchema` (`meta.go:684`) **selalu** menyertakan `ActionSummary{Permission: …}` tanpa memanggil `can()` — memang aditif, karena GrantsEditor harus bisa menawarkan action yang belum dipegang caller. Dulu tidak ada yang meng-assert bagian ini; kini dipin `TestBuildBundle_ActionPermissionsAreAdditiveNotFiltered` (action list + permission string tetap utuh untuk caller yang hanya punya `list`, dan invariant terhadap permission caller). **Bukti**: `go test ./internal/ui/ ./internal/api/` hijau. **Catatan (bukan celah baru, tapi sengaja dicatat):** konsekuensi aditif-nya — caller yang boleh `view` sebuah entity menerima **semua** action (create/update/delete/dll.) di bundle selama entity-nya kelihatan; objek `permission` per-action belum dibaca renderer (`grep '\.permission'` di `renderers/react-shadcn/src` → 0 hit), jadi tombol action hari ini tidak di-gate klien. Enforcement tetap di server; yang belum ada hanya penyembunyian tombol di UI.
      **Sisa ditutup 2026-09-22 (B)**: klien kini menyembunyikan tombol yang tidak boleh dipakai. Helper bersama `entityActionPermission`/`canDoEntityAction` (`engine/permissions.ts`) dipakai di 5 situs render (Table row action + batch edit + inline edit, Kanban menu, DetailPage transisi, Form submit) — sebelumnya tiap situs menulis sendiri `${module}.${plural}.${action}`, yang **mengabaikan `required_permission` eksplisit** (justru kasus yang `ActionSummary.permission` ada untuk menyampaikannya). **Dua bug nyata** ketemu saat pemasangan: (1) `can()` TS mengklaim parity dengan Go tetapi **tidak punya cabang wildcard tingkat-module** — `billing.*` gagal mencocokkan `billing.orders.delete`, jadi pemegang `RoleModuleOwner` (yang memang memakai `billing.*`, lihat `internal/auth/owner_test.go`) akan **kehilangan semua tombol action** padahal server mengizinkan; kini diparitas-kan + dipin 7 test. (2) Untuk entity yang di-allow lewat `view`, tabel menyembunyikan tombol sementara DetailPage/Kanban/Form tidak — kini keduanya setuju. **Bukti**: `vitest` **295 lulus** (20 file, +7 baru), `tsc` bersih, `go test ./...` hijau. Changelog `2026-09-22-007`.
      **Catatan jujur (tanpa klaim lebih)**: `row_actions` turunan engine (view/edit/delete) kini ikut tersaring bila caller tidak punya permission-nya — perubahan perilaku yang **disengaja** tetapi belum diverifikasi di browser nyata.
      **Sisa baru → item 5.12.8 ⏸️** (bulk action ber-tombol tanpa handler sehingga tidak dieksekusi).
- [x] 5.12.8 ✅ **2026-09-26** **Bulk action kini benar-benar dieksekusi.** `BulkActionsBar` merender satu `<Button>` per `tableSpec.bulk_actions` **tanpa `onClick`** dan `TableRenderer` tidak pernah mengoper handler kolektif — bar muncul, tampak bisa diklik, tidak melakukan apa pun, sementara **Batch edit (5.4.3) di bar yang sama berfungsi nyata**. Kontrak yang dipakai adalah kontrak batch edit itu sendiri: per baris, **partial failure dilaporkan per baris** (`BulkResultReport`), baris 409 ditandai stale. Keputusan yang membuatnya aman: `view`/`edit` **ditolak** (navigasi satu-baris; "edit 12 baris" tak bermakna), tombol yang tidak bisa dijalankan **di-disable** (bukan disembunyikan — aksi yang dideklarasikan tapi tidak berlaku harus terlihat), permission diperiksa **sebelum baris pertama disentuh** dan **sebelum** prompt konfirmasi, dan aksi destruktif meminta konfirmasi dengan tombol yang **menyebut jumlah baris**. **Bukti:** `npx vitest run` **511 lulus** / 37 file, `tsc -b` bersih; test baru (8) **dibuktikan gagal** saat `onClick`+`disabled` dihapus (persis bug lamanya). Changelog `2026-09-26-012`. Effort selesai: medium.
- [x] 5.12.9 ✅ **2026-09-28** **Aksi massal kini bisa mengumpulkan parameter.**
      Ditutup oleh kontrak input (`docs_internal/plan/action-input-contract.md`,
      changelog `2026-09-28-004`) — jadi bukan perbaikan terpisah: yang hilang
      dulu memang **bentuk deklarasi parameter**, dan bentuk itu sekarang milik
      transisi/action (`params.inputs`), dibaca satu resolver bersama.
      **Terukur:** `requestBulkAction` membuka **satu** dialog untuk seluruh
      seleksi dan mengirim payload yang sama per baris
      (`{ json: inputs ?? {} }`), jadi aksi ber-parameter (mis. `void-order`
      yang butuh `void_reason`) tidak lagi gagal validasi per baris. Alasan
      aslinya dicatat di bawah untuk jejak.
      <details><summary>alasan awal</summary>`requestBulkAction` mengirim POST tanpa body, jadi hanya aksi **tanpa parameter** yang berguna lewat bar massal — aksi yang butuh input (mis. `void-order` butuh `void_reason`, lihat `conditions` di `cafe-order/order`) akan gagal validasi per baris dan dilaporkan sebagai kegagalan biasa, bukan sebagai "aksi ini tidak bisa dijalankan massal". **Teramati:** tidak ada permukaan deklaratif untuk parameter kolektif (`TableAction` hanya punya `action`/`label`/`icon`/`confirm_msg`), dan tidak ada item pelacak sebelum ini — dicatat saat menutup 5.12.8. Effort: medium (bentuk deklarasi parameter + dialog input + satu payload untuk seluruh seleksi).</details>

- [x] 5.12.5 Task-based admin granting → materialized permission strings — `Materializer` (`internal/auth/materialize.go`) menurunkan footprint page (blocks/tabs → entity-action) + derived entity page (`{entity}-page`) + navigation kind (`{kind}:{name}`) dan meng-expand grant role → permission strings; di-wire ke auth service (`permissionsForUser` saat login). Admin UI granting: `GrantsEditor` menampilkan semua page app (authored + derived entity + navigation kinds) dengan label action + permission string inline + search + preview permission termaterialisasi. ✅ 2026-08-20 (materializer) · ✅ 2026-08-22 (GrantsEditor semua page, changelog 004)

### 5.13 Other UI kinds

- [x] 5.13.1 `kind: Report` — totals row + grouping/subtotal per group (`computeTotals` shared); ⏸️ export sebagai async job → download tray belum (butuh backend job infra; saat ini masih CSV Blob client-side) (`06-page-kinds.md` §8) — ✅ 2026-08-24 (WS-G, changelog 030)
- [x] 5.13.1a Report `source.filter` — filter parameterized deklaratif (`source: { entity, filter }` dengan `":param"` placeholder) di-resolve dari `parameters[]`; literal pass-through. ✅ 2026-08-24 (WS-G, changelog 030)
- [x] 5.13.2 `kind: Print` — PDF server-side generation via `go-pdf/fpdf` (`GET /_ui/print/{module}/{name}/{id}`); `format: html` via `window.print()` (existing). ✅ 2026-08-24 (WS-H, changelog 031)
- [x] 5.13.3 `kind: ApprovalInbox` — pending approvals list, `approve`/`reject` inline actions, badge count, `realtime: true`. ✅ 2026-08-24 (WS-I, changelog 029)
- [x] 5.13.4 `kind: NotificationCenter` — notification list, badge unread, `mark-read` action, `realtime: true`, deep-link on click. ✅ 2026-08-24 (WS-I, changelog 029)
- [x] 5.13.5 `kind: Listing` — public catalog, no auth wrap, no row/bulk actions — `ListingRenderer` read-only (search + filter, tanpa create/row/bulk; klik baris → detail) + kind `Listing` end-to-end (spec, registry, bundle, route). Contoh `examples/storefront/`. Lihat `docs_internal/plan/landing-page.md` + changelog 2026-08-19-001. ✅ 2026-08-19
- [x] 5.13.6 ✅ **2026-10-04** **Wiring runtime `ApprovalInbox` zero-config — SELESAI lewat opsi (b), endpoint khusus.** Renderer merender **"No approval source configured"** secara permanen karena ia mencari entity konvensional (`formspec.core.approval` / `approval-task` / `workflow-task`) yang tidak ada di repo ini, sementara barisnya hidup di tabel framework `formspec_workflow_approval` — bukan Entity, jadi **tidak ada route yang mengeksposnya**.

  **Keputusan:** dari tiga jalur yang dicatat item ini, (b) adalah satu-satunya yang bisa diimplementasikan — (a) entity bawaan mustahil karena `WorkflowApprovalRow` bukan baris entity (tanpa `title`/`display_fields`), dan (c) bertentangan dengan kontrak zero-config (`06-page-kinds.md` §11).

  **Kontraknya:**

  ```
  GET  /{ws}/_ui/workflow/approvals?app={app}
  POST /{ws}/_ui/workflow/approvals/{id}   {"decision":"approve"|"reject"}
  ```

  Empat aturan: (1) `ListPendingForTenant` — `ListPending` yang dipakai worker eskalasi sengaja tenant-blind, jadi melayani request darinya akan menyerahkan approval satu workspace ke workspace lain (`tenantID == ""` → kosong, fail closed); (2) eligibility memakai `workflow.Engine.CanApprove`, predikat **yang sama** dengan jalur approve nyata (termasuk 7.4.5); (3) `can_decide` memisahkan **terdaftar** (role step) dari **boleh dijalankan** (permission route transisi, diambil dari `RouteDescriptor` yang sama dengan registrasi route; fallback `{module}.{plural}.update` untuk transisi tanpa `impl`) — tugas yang tak bisa dieksekusi tetap terdaftar, tombolnya non-aktif; (4) `display_fields` dipagari `{module}.{plural}.view`, karena pembacaan store di handler melewati pemeriksaan permission HTTP. `POST` **mendelegasikan ke `handleWorkflowApproval`**, bukan implementasi kedua.

  **Bug yang ditemukan jalan ini:** `handleWorkflowApproval` menaikkan `ActiveStep` melewati step terakhir tetapi tidak pernah mengubah `Status`, sehingga baris yang **sudah selesai** tetap `status='pending'` — request void kedua untuk record yang sama membalas **403 "workflow step out of range"**, bukan approval baru. Kini `Status = approved` diset saat `AllStepsApproved`.

  **Bukti:** `go test ./...` hijau · `make lint` **0 issues** · `npx vitest run` **629 lulus**/51 file · `tsc --noEmit` bersih. Test baru `workflow_inbox_test.go` (8; termasuk route lewat `BuildHTTP` → **401 bukan 404**, id lintas-workspace → **404**, keputusan kedua → **404**) + `approvalInbox.test.ts` (3; path dibuktikan ke server HTTP nyata dengan assertion **negatif** untuk dua salah tulis yang gagal senyap). Dua test **dibuktikan gagal** saat baris `Status = approved` dihapus. Changelog `2026-10-04-004`. Effort selesai: medium.
  **Sisa → 5.13.7 ⏸️** (realtime) dan 5.25.10 ⏸️ (grant per-inbox, sudah ada).
  <details><summary>teks awal, dipertahankan sebagai jejak</summary>Teks aslinya menyebut "engine belum mengisi `title`/`display_fields` ke item inbox dari step". Pemeriksaan menunjukkan **tidak ada entity yang bisa dibaca sama sekali**: `APPROVAL_ENTITY_REFS` mencari entity yang 0 hasil di seluruh repo; `WorkflowApprovalRow` (`jsonb-persist/workflow_approval.go:21`) punya `Entity`/`RecordID`/`FromState`/`ToState`/`WorkflowName`/`RequesterID` — **tidak ada** `title`/`subject`/`display_fields`; dan tabel itu bukan Entity, jadi tidak ada route HTTP ke `formspec_workflow_approval`. Itulah kenapa pesan di layar bersifat permanen. Effort: medium (keputusan + implementasi salah satu dari tiga + test).</details>

- [x] 5.13.8 ✅ **2026-10-04** **Role step workflow divalidasi + kafe memakai nama yang benar.** Pelanggaran yang ditutup: `steps[].roles` adalah **lookup nama** yang tidak pernah diperiksa. Kafe menulis `roles: [cafe-order.supervisor]` sedangkan role yang di-seed bernama `supervisor` → validate **89/0** sementara `CanApprove` (bandingkan literal) menolak selamanya, inbox kosong bagi satu-satunya approver, dan seandainya nama qualified itu dipegang user, `RoleStore.GetByName` gagal → resolver melewatinya **tanpa suara** → nol permission. **Fase 0:** `workflowRoleError` + `buildRoleNameIndex` (`cmd/formspec/validate_workflow.go`) memeriksa `roles`, `escalation.reassign_roles`, dan `escalation.notify_roles`; role dibaca dari `kind: Seed` (pola `validate_seed_grants.go`) plus 4 role owner yang di-seed auth service. **Fase 1:** kafe → `[supervisor]` / `[manajer]`, dan komentar "memetakan ke employee berposisi supervisor/manajer" **dihapus** — mekanisme itu tidak ada (`position` hanya enum; tidak ada hook yang menyentuh `roles`). **Keputusan yang sempat salah lalu diperbaiki:** versi pertama melewati pemeriksaan bila tree tak punya role sama sekali; preseden `validateScopeSources` (`canSupply` menyalakan error) menunjukkan itu keliru dan guard itu dihapus. **Bukti:** kafe **MERAH** setelah Fase 0 dan **89/0** setelah Fase 1 (properti pembuktian yang direncanakan); 5 test baru, dua di antaranya **dibuktikan gagal** saat pemanggilan `workflowRoleError` dihapus; `cmd/formspec` hijau · `go test ./...` hijau · `make lint` 0 issues. Plan `docs_internal/plan/approval-duty-permission.md`, changelog `2026-10-04-005`. **Sisa → 5.13.9 ⏸️ + Fase 2–4 plan.**
- [x] 5.13.10 ✅ **2026-10-04 (bagian a)** **Grant duty approval hidup — `navigationFootprint case "workflow"`.** Grant `{ page: "workflow:order-void-approval", actions: [{ name: supervisor-check }] }` **mematerialisasi ke nol** karena `navigationFootprint` hanya mengenal enam kind (`dashboard/report/wizard/kanban/timeline/print`); terukur: `/_ui/_meta/me` supervisor = **44** permission, tidak satu pun `workflow.*`. Kini: `case "workflow"` menurunkan satu `FootprintAction` per step ber-duty (di-key nama step, permission **diturunkan** lewat `spec.StepPermission`), `Materializer.SetWorkflowDuties` (lookup di-wire, pola `grantScopeLookup`; `nil` → dilaporkan sebagai problem, **bukan dibuang**), `workflow.Registry.GetByName` (grant menulis nama tanpa module), dan wiring di `resource/formspec.go`. **Kafe kemudian dibuat duty-only:** `roles:` dihapus dari step `order-void-approval` — siapa boleh menandatangani ditentukan grant, bukan nama role di manifest workflow. **Terbukti live:** grant dipasang → 45 permission termasuk duty-nya · alur penuh hijau (202 → inbox `can_decide: true` → approve 200 → record `cancelled` + `void_reason`) · **grant dicabut** (DB saja, manifest workflow **tidak disentuh**) → permission turun ke 44, inbox **0 task**, approve **403**, record tetap `paid`. **Gate yang sudah ada menangkapnya:** `TestKafeSeed_GrantsAllResolve` MERAH saat grant dipasang (materializer test belum punya lookup) dan diperbaiki dengan **mewire seperti produksi**, bukan melonggarkan asersi. Test baru: `workflow_duty_grant_test.go` (6, termasuk regresi "tanpa registry dilaporkan bukan dibuang") + `pkg/spec/workflow_step_test.go` (4, termasuk aturan **step tanpa roles & permission ditolak** — `hasAnyRole` atas daftar kosong false untuk semua orang, jadi step itu mustahil mencapai kuorum dan hanya terlihat "menunggu"). `go test ./...` hijau · `make lint` 0 issues · kafe 89/0 · `vitest` 636/636. Changelog `2026-10-04-007`. **Sisa → 5.13.12**, ✅ ditutup 2026-10-06 sebagai **peringatan advisory** (`roles`-saja), `sequential` dikecualikan — changelog `2026-10-06-001`.
- [x] 5.13.11 ✅ **2026-10-04** **Kuorum datang dari manifest; `mode: all`/`sequential` berhenti berbohong.** Tiga cacat sekelas dari satu baris (`eligibleCount := len(step.Roles)`): (1) kuorum menghitung **nama role**, bukan orang — `roles: [a, b]` yang dipegang satu orang menuntut dua tanda tangan yang mustahil ia berikan (approval ganda satu step ditolak) → step yang tidak akan pernah bisa disetujui; (2) `mode: sequential` **tidak pernah mengurutkan apa pun** (`sequential` hanya muncul di `Quorum()` dan mengembalikan 1, jadi identik dengan `any` sementara spec menjanjikan rantai); (3) jalur approve memanggil `CanApprove(wf, …, ActiveStep, …)` yang meng-index `wf.Steps` (**authored**) padahal langkahnya berasal dari `ApplicableSteps` (**terfilter**) — kemunculan ketiga bug daftar yang sama, dan yang paling panas. **Perbaikan:** `Quorum(step)` selalu dari manifest (`approvers`, default 1; `sequential` = `len(roles)`, satu tanda tangan per mata rantai), **`mode: all` ditolak** `formspec validate` dengan alasan + jalan keluar (angkanya tidak bisa diturunkan: `roles` bukan daftar orang, pemegang duty tidak bisa dienumerasi), rantai sequential **ditegakkan** (giliran = `len(approvals[step])`, penandatangan harus memegang role giliran itu, atau role eskalasi), kombinasi tak terdefinisi ditolak (`sequential`+`permission`, `sequential`+`approvers`, `sequential` tanpa `roles`), dan `CanApprove(a, steps, approver)` menjadikan daftar step **parameter** sehingga pemanggil harus menyatakan daftar mana yang dimaksud. **Jebakan yang ikut tertutup:** `NewApproval` mengisi `WorkflowName` dengan **alamat pointer** (`%p`) — selamat selama nama hanya label, tetapi duty **diturunkan** dari nama itu, jadi nama placeholder = semua duty tak bisa dipegang dengan gejala **403 senyap**; kini nama wajib sebagai parameter. **Bukti:** live → 202 · approve 200 · record `cancelled` + `void_reason` · 5 test baru di `internal/workflow/quorum_test.go` (termasuk "memasukkan daftar authored justru gagal") · 5 sub-test validator `pkg/spec` · `go test ./...` hijau · `make lint` 0 issues · kafe 89/0 · skema di-regenerate. Changelog `2026-10-04-008`. **Sisa → 5.13.13 ⏸️.**
- [x] 5.13.13 ✅ **2026-10-04** **Riwayat approval di-key NAMA step, dan jalur approve berhenti membaca posisi.** Dua cacat dari akar yang sama — sebuah step dirujuk lewat **posisinya**, bukan identitasnya. (1) `handleWorkflowApproval` mengambil langkah dari `ApplicableSteps` (**list berlaku**) tetapi `CanApprove(wf, …, ActiveStep, …)` meng-index `wf.Steps` (**list authored**): begitu satu step di-skip `when`, kedua list berbeda di **setiap** index, sehingga step yang benar-benar menunggu **tidak pernah diperiksa** — kemunculan **keempat** dari bug yang sama di satu alur (escalation, inbox, approve), dan tiga perbaikan sebelumnya hanya menambal satu pembaca. (2) `Approvals`/`EscalatedSteps` disimpan sebagai `{"0": [...]}` — index step — sehingga menyisipkan step di depan membuat `approvals[0]` kemarin menunjuk step **berbeda** hari ini (penandatangan tercatat diam-diam jadi "siapa pun di index 0 sekarang"), dan `when` melakukan hal yang sama **tanpa ada yang mengedit apa pun**. **Perbaikan:** satu aturan `Approval.ActiveIndex(steps)` (nama dulu, index sebagai fallback baris lama) dipakai oleh `CanApprove`, `StepApproved`, `Approve`, `Reject`, `Advance`, dan audit — sehingga kedua list tidak bisa lagi berbeda pendapat; key riwayat jadi `StepKey` = nama step, dengan `#{index}` untuk step tanpa nama (`#` tidak bisa muncul di nama, jadi kedua ruang tidak bertabrakan). **Baris lama tetap terbaca tanpa migrasi data** (key objek JSON memang string) plus `mergeLegacyBucket` saat menulis — tanpa itu, tanda tangan yang sudah tercatat akan tertinggal di key numerik dan hilang dari hitungan kuorum. **Terbukti live:** baris pending gaya lama (`active_step_name` kosong, `{"0":[…manajer]}`) di-approve → **200** dan tersimpan `{"supervisor-check":[…manajer,…supervisor]}` — tanda tangan lama terbaca **dan** dimigrasikan; baris baru tersimpan ber-key nama. **Bukti:** 5 test baru `step_key_test.go` (termasuk "migrasi tidak menghilangkan tanda tangan" dan "riwayat selamat dari penyisipan step") · 1 test API lewat store+endpoint nyata · satu asersi test lama **ditulis ulang** karena perilaku yang benar berubah (kedua list kini resolve ke step yang sama) · `go test ./...` hijau · `make lint` 0 issues · kafe 89/0. Changelog `2026-10-04-009`.
- [x] 5.13.9 ✅ **2026-10-04** **Empat workflow yang tidak bisa disetujui kini punya role — dan gate-nya berlaku untuk SEMUA example.** `crc-management` (3 workflow) dan `service-demo` (1) menamai role yang **tidak pernah dideklarasikan di tree-nya sendiri** (`crc.cap-approver`, `crc.foreman`, `crc.customer`, `demo.manager`); `hasAnyRole` membandingkan nama literal, jadi approval-nya **403 selamanya** dengan manifest yang terlihat benar. **Perbaikan:** dua seed role baru (`crc` 5 role, `service-demo` 2 role) **dan keempat workflow dipindah ke DUTY** (`permission:` + grant `workflow:{name}`) — bentuk yang dituju plan, sama seperti kafe. Hasilnya: crc **5 → 2 problem**, service-demo **2 → 1** (sisanya yang sudah ada sebelumnya: integrator tanpa cancel handler, dua schema). **Seed dijalankan sungguhan:** `formspec seed` crc → 5 inserted, service-demo → 2 inserted. **Gate baru** (`internal/auth/examples_seed_grants_test.go`): gate yang tadinya hanya menjaga kafe kini menjaga **setiap** example yang punya role seed, dan menemukan tree-nya sendiri via glob, jadi example baru tertutup otomatis. Diperiksa: (1) setiap page/action seed resolve + role tidak materialisasi ke nol; (2) setiap step workflow bisa dipenuhi — `roles`-nya dideklarasikan ATAU ada role yang **di-grant** duty-nya (deklarasi tanpa grant = separuh kontrak). **Gate dibuktikan nyata:** mencabut satu grant duty → merah dengan nama permission persisnya. `go test ./...` hijau. Changelog `2026-10-04-010`.
- [x] 5.13.14 ✅ **2026-10-04** **`escalation` level workflow ditolak; eskalasi crc jadi nyata.** `WorkflowEscalation` hanya punya `after` + `notify_roles` — **tanpa `reassign_roles`** — sementara satu-satunya efek eskalasi yang diimplementasikan adalah **reassignment** (`EscalationWorker` membaca `step.Escalation.ReassignRoles`) dan pengiriman notifikasi belum ada. Jadi bentuk itu **tidak bisa menghasilkan apa pun**: "dieskalasi setelah 48h" dan tidak pernah terjadi apa-apa. **Terukur:** ketiga approval `crc-management` mendeklarasikannya di level workflow — eskalasi 48h/72h mereka tidak pernah berjalan. **Keputusan: tolak, bukan implementasikan.** Mengimplementasikannya tetap tidak menghasilkan apa-apa (field-nya tidak punya reassign, dan notifikasi belum ada), jadi menolaknya adalah ujung yang jujur: `formspec validate` menolak `spec.escalation` dengan menyebut `reassign_roles` dan `steps[].escalation` sebagai jalan keluar. Keempat contoh di dokumentasi (spec §2, kind ref, skill) memakai bentuk mati itu dan kini diperbaiki. **Ketiga workflow crc dimigrasikan** ke eskalasi step-level dengan reassign nyata (`crc.foreman-supervisor`, `crc.cap-approver-supervisor`, `crc.foreman`) — `notify_roles` sengaja **tidak** ditulis karena tidak punya akibat. **Bukti:** test baru (bentuk level workflow ditolak, timeout di step diterima) · nama role `escalation` divalidasi — dibuktikan dengan mengganti `reassign_roles` ke role tak ada → merah · seed crc 5 role tersimpan · `go test ./...` hijau · `make lint` 0 issues · kafe 89/0 · crc 2 & service-demo 1 (pra-ada). Changelog `2026-10-04-011`.
- [x] 5.13.15 ✅ **2026-10-05** **`notify_roles` dihapus; `reassign` jadi duty.** Field `notify_roles` **tidak punya pembaca** (tidak ada kanal notifikasi), jadi mengganti kosa katanya jadi permission tak akan mengubah apa pun — ia **dibuang**, dan kembali sebagai `notify` hanya saat kanalnya ada. Yang diperbaiki adalah `reassign_roles` → `reassign` (duty, bukan nama role; aturan 6). Tiga bentuk ditolak validator: `after` tanpa `reassign`, `reassign` tanpa `after`, dan `reassign` = duty step sendiri (no-op). Changelog `2026-10-05-002`.
- [x] 5.13.12 ✅ **2026-10-06** **`roles`-saja: di-deprecate sebagai PERINGATAN (advisory), bukan error atau penghapusan field.** Step roles-only **bekerja** (`CanApprove` menerima pemegang role), jadi memfailkan run akan mematahkan manifest yang baik-baik saja — yang layak disampaikan adalah **apa yang dirugikan**: nama role ditulis langsung di manifest, sehingga mengganti nama role diam-diam mematikan approval itu (persis yang AGENTS.md aturan 6 minta dihindari; duty `resource + action` yang di-grant dari seed role tidak begitu). **`mode: sequential` DIKECUALIKAN, dan itu load-bearing:** rantai diurutkan oleh `roles`, sedangkan duty adalah permission datar tanpa posisi di urutan itu — `validateApprovalStepMode` sendiri menolak kombinasi `sequential` + `permission`, jadi pada rantai `roles` adalah **satu-satunya** bentuk yang bisa menyatakan urutan. Ini sekaligus menegaskan koreksi 2026-10-04 pada item ini (`roles` tidak boleh dihapus). **Yang ditambahkan:** `scanApprovalRoleOnlySteps` (`cmd/formspec/validate_approval.go`) — peringatan per step non-rantai, **beserta jalan keluarnya** (duty yang perlu ditambahkan + grant persisnya); docs `02-core-extended.md` §2.1 + godoc `ApprovalStep.Roles` di `pkg/spec`. **Bukti:** 4 test (peringatan menyebut step/role/`permission:`/grant + alasan; duty senyap termasuk bila disertai roles; `sequential` exempt; step tanpa keduanya tidak dilaporkan dua kali) · **bukti live**: step menjadi roles-only → `[WARN]` dengan grant konkret, ringkasan **1 warning / 0 problem**; rantai sequential → **0** peringatan · **blast radius nol di repo** (tidak ada step roles-only, tidak ada rantai) · `go test ./...` hijau · kafe 88/0 · crc 33/0 · service-demo 13/0. Changelog `2026-10-06-001`.

- [⏸️] 5.13.7 **`realtime: true` pada `ApprovalInbox` belum dihormati — view-nya statis sampai di-refresh.** Permukaan approval kini punya data (5.13.6 ditutup), tetapi hub WS mendorong per `{module}/{entity}` dan approval **bukan entity**, jadi tidak ada topik yang bisa disubscribe klien. Supervisor yang meninggalkan layar ini tidak melihat tugas void baru; ia harus reload — persis kekhawatiran yang sudah ditulis di `GAP-17` pada `supervisor-inbox.yaml`. **Yang belum diputuskan:** apakah server mem-broadcast pada topik workflow (mis. `workflow/approvals`) atau renderer mem-poll. `filters` juga masih tanpa arti yang jelas (`FilterSpec.field` merujuk apa kalau sumbernya bukan entity) — tidak ditebak. Effort: medium (keputusan bentuk topik + broadcast + test).

### 5.14 Derivation engine

- [x] 5.14.1 Derivation fix — Table: N priority columns, overflow accessible via expand (never silently dropped) — sama dgn 5.4.4 (`derive.ts` priority sort + `TableRenderer` row expand). ✅ 2026-08-24
- [x] 5.14.2 Wire `deriveMenuItems()` — currently dead code; `_admin` menu built inline in Sidebar — `useResolvedMenu()` (`hooks/useResolvedMenu.ts`) now calls `deriveMenuItems(bundle.entities)` for the `_admin` branch instead of duplicating the grouping logic inline; "Access Management" shortcut still prepended. ✅ 2026-08-22
- [x] 5.14.3 Derivation: Form mode heuristic — >12 fields OR has child with `storage: table` → `separate_page`; >5 fields → `drawer`; else → `modal` — `deriveFormRenderMode` di `engine/derive.ts`. ✅ 2026-08-24
- [x] 5.14.4 Pola UI lifecycle tambahan — `two_step_manual` dan `one_step_create_submit` via hint `ui:` (`06-page-kinds.md` §2.1); catatan: enum `lifecycle` di EntitySchema tetap 2 nilai (`plain_crud|two_step_autosave`, `04-spec-resolution-api.md` §2) — ini pola UI, bukan nilai enum baru — `engine/lifecycle.ts` (4 pola) + `FormRenderer` (Save Draft/Submit/Create-Submit). ✅ 2026-08-24
- [x] 5.14.5 Caption field: fallback ke `title` entity — presedensi `label` manifest → `title` field Entity → nama di-humanise, diseragamkan di **fungsi resolusi** (`entityFieldLabel`/`withEntityFieldLabels`/`withEntityColumnLabels` di `engine/derive.ts`, dipakai `resolveForm`/`resolveTable`) lalu dikonsumsi Form/Table/Wizard/SearchSelect/Listing/Report/DetailPage. Sebelumnya lima ejaan berbeda hidup berdampingan dan hanya jalur **derivasi** yang membaca `title` — jadi mendeklarasikan `kind: Form`/`Table` menurunkan seluruh caption ke nama mentah (`min_purchase`, bukan `Minimum Belanja`); DetailPage bahkan mengabaikan `title` sepenuhnya. Skala 111/161 field form authored tanpa `label:`. ✅ 2026-09-24. Changelog `2026-09-24-008`. **Sisa → 5.14.6 ⏸️.**
- [⏸️] 5.14.6 **Presedensi caption hanya dipaksakan di renderer `react-shadcn`, tidak di kontrak lintas-shell.** Aturan sudah ditulis normatif di `06-page-kinds.md` §2 ("renderer **wajib** menerapkan presedensi ini"), tetapi tidak ada test konformansi bersama — shell kedua (vue/flutter) harus mengimplementasikan ulang `entityFieldLabel`, dan tidak ada yang akan gagal bila lupa. Sama bentuknya dengan 5.11.7 (aturan lexer FormSpecExpr diduplikasi): kontrak yang hanya ditegakkan satu implementasi. **Teramati**: `find . -iname "*conformance*"` → 0 hasil; `ls renderers/` → `jsonb-persist`, `react-shadcn` (satu shell). Effort: medium (fixture bersama: entity + manifest authored → caption yang diharapkan, dikonsumsi tiap shell).

### 5.15 Dead code cleanup

- [x] 5.15.1 Remove `engine/registry.tsx` — replaced by hardcoded `lazy()` map in router — deleted (zero remaining references; `shell/router.tsx`'s hardcoded `lazy()` map is the only path used). ✅ 2026-08-22
- [x] 5.15.2 Wire `OverlayHost` — connect to Form.render modal/drawer and other overlay needs — `shell/OverlayHost.tsx` (URL `?action=&form=&mode=` → Dialog/Sheet; derived-form fallback via `entity=`). ✅ 2026-08-24
- [x] 5.15.3 **SPA tidak bisa di-build — DIPERBAIKI 2026-09-22** (changelog `2026-09-22-013`). Tiga cacat di working tree: (1) dua `import` tergabung dalam satu baris (`} from "@/engine/permissions"import {`) di `DetailPage.tsx` + `TableRenderer.tsx` → `TS1005`; (2) **komentar menelan baris kode** di `DetailPage.tsx` — `// Find transition to check for confirm message    const transition = transitions.find(...)` menempelkan deklarasi ke komentar, sehingga `transition` tidak pernah ada dan tiga pemakaian di bawahnya `TS2552`; (3) impor `can as checkPermission` yang tidak dipakai lagi (digantikan `canDoEntityAction`) di kedua berkas → `TS6133`. **Verifikasi:** `tsc -p tsconfig.app.json --noEmit` bersih · `vitest` **295 lulus** · `make web-build` sukses · `make build` penuh sukses (4 binary). **Catatan:** cacat (2) adalah contoh kenapa kompilasi harus dijalankan sebelum menyimpulkan "refactor selesai" — komentar yang menelan kode terlihat wajar saat dibaca sepintas.
      _(catatan awal, dipertahankan sebagai jejak)_ Ditemukan 2026-09-22 saat `make seed-kafe` masih bergantung pada `build-spa`. **Teramati (sebelum saya memperbaiki tipe sintaksnya):** `DetailPage.tsx` dan `TableRenderer.tsx` punya dua `import` yang tergabung dalam satu baris (`} from "@/engine/permissions"import {`) — `TS1005 ';' expected`. Setelah baris itu dipisah, sisa errornya **nyata dan belum selesai**: `DetailPage.tsx` memakai `transition` yang tidak terdeklarasi (3×, `TS2552` — penggantinya `transitions`), dan `checkPermission` diimpor tetapi tidak dipakai di dua berkas (`TS6133`). Jadi ini refactor yang setengah jalan di working tree, bukan sekadar sintaks. **Dampak:** `make build`/`web-build` gagal untuk siapa pun yang mengambil working tree ini; `make seed-kafe` sudah dilepas dari ketergantungan itu supaya seeding tidak ikut terblokir. Effort: small–medium (selesaikan pemakaian `transitions` + hapus impor mati).

### 5.16 VisualSpecKind/Renderer registry & resolution

- [x] 5.16.1 Renderer resolution engine — pilih Renderer via `(implements, stack_family)`; hanya `official` auto-select; tanpa official → error + sarankan kandidat `verified`/`community` (tidak pernah silent fallback); override via map `renderers:` di App manifest + field `renderer:` per-instance — `internal/manifest/renderer.go` `ResolveRenderer`. ✅ 2026-08-24 (WS-J, changelog 032)
- [x] 5.16.2 Slot-tier validation at apply — `accepts_slots` hanya sah dari `tier: page|app`, `implements_slot` hanya dari `tier: component`; kombinasi lain ditolak (`02-visual-spec-kind.md` §4–§5) — `ValidateSlotTiers`. ✅ 2026-08-24 (changelog 032)
- [x] 5.16.3 `stack_family` compatibility check — App shell + Page shell-integrated + Component wajib satu family; mismatch = compile-time error; Page independen tidak dicek (`01-visual-hierarchy.md` §3) — `ValidateStackFamily`; wired ke `formspec check` (`checkRenderers`). ✅ 2026-08-24 (changelog 032)

- [x] 5.17 Page transition via App spec — `App.spec.page_transition` (`none | fade | slide`, default `fade`) → View Transitions API; trigger manual `startViewTransition` + `flushSync` (react-router declarative membuang opsi `viewTransition`); `useAppNavigate()` + `AppLink`/`AppNavLink` di `lib/navigation.tsx`; mode di-mirror ke `<html data-page-transition>` + CSS `::view-transition-*` (fallback `prefers-reduced-motion`); `replace`/delta navigation tidak dianimasikan — `docs_internal/plan/page-transition-app-spec.md`. ✅ 2026-09-08 (changelog 2026-09-08-002)

### 5.18 Table column presentation — `money`, `align`, `width`

- [x] 5.18.1 **Kolom `money` di Table turunan tampil JSON** — kafe **10.25**.
      `tableFormat()` hanya memetakan `datetime`/`date`/`percent`, jadi field
      `money` tanpa `format` jatuh ke `JSON.stringify` (`{"amount":"50000",
"currency":"IDR"}`); `cellHintsForField()` sudah memetakan
      `money → currency` — satu kosakata, dua tabel, satu bolong (child-grid benar,
      kolom tabel salah). Kini `money → currency`; heuristik
      `decimal + rules[min] → currency` dibuang (menebak mata uang dari batas bawah
      yang juga dipakai field non-uang — `gl-balance.opening_balance`,
      `setting.tax_percent`, `visit.total` — dilarang `05-field-types.md` §2).
      `MoneyInput` juga memakai mata uang **nilai** sebagai fallback pratinjau.
      Terukur: `cash-movements` Jumlah → `Rp50.000` (tanpa `{"amount"` di DOM).
      Plan `docs_internal/plan/money-table-format-derivation.md`, changelog
      `2026-09-24-002`; 4 test baru dibuktikan gagal sebelum patch. ✅ 2026-09-24
- [x] 5.18.2 **`TableColumn.align`/`width` diabaikan renderer** — kafe **10.26**.
      Dijanjikan `docs/spec/frontend/06-page-kinds.md` §3 + ditulis
      `order-table-pos.yaml`/`stock-level-table.yaml`/`visit/tables/list.yaml`,
      tetapi **0 renderer** membacanya. Kini helper bersama
      `src/lib/tableColumn.ts` dipakai **kedua** renderer; `align` kena `<th>`
      **dan** `<td>` (`<td>` sibling `<th>` — `text-align` tidak diwarisi) plus
      `justify-*` untuk header sortable (flex row); `width` kena `<th>` +
      `minWidth`. `align` jadi **enum tertutup** di skema. Terukur (browser):
      `stock-levels` tiga kolom numerik `thAlign=right` **dan** `tdAlign=right`;
      `orders` `Total` `text-right` + `justify-end`. Kontrak normatif di spec
      §3.1.1 + gotcha `docs/kind/ui/Table.md`. Plan
      `docs_internal/plan/table-column-align-width.md`, changelog
      `2026-09-24-003`; 10 test baru, 4 gagal sebelum patch. ✅ 2026-09-24
- [⏸️] 5.18.3 **`TableColumn.link` diterima skema tetapi tidak berefek** — kafe
  **10.27 ⏸️**. Satu-satunya atribut `TableColumn` yang tersisa tidak
  dikonsumsi (`grep "col\.link" src` → 0 hasil; 0 manifest memakainya).
  Semantik pengisian `:param` route Page tujuan dari record belum ditetapkan —
  konvensi yang sama juga belum ada untuk `Calendar`/`Kanban`, jadi
  mengimplementasikan sekarang = menebak. Sudah ditandai **Open** di spec §3.
  Effort: medium (butuh keputusan kontrak dulu).
- [x] 5.18.4 **Kolom relasi tampil UUID; angka tanpa pemisah ribuan** — kafe
      **10.28**. Tiga jalur render, hanya dua yang membaca alias relasi
      (`derive.ts` menulis ulang jadi dot-path; `DetailPage` punya resolver
      sendiri), jadi tabel **tertulis** ber-`field: branch_id` mencetak UUID
      padahal `branch.name` ada di baris yang sama — dan `ListingRenderer` membaca
      `row["branch.name"]` (kunci bersarang) sehingga dot-path pun `undefined` di
      Listing. Angka: `renderCellValue` tidak punya cabang angka sama sekali
      (`currency`/`date`/`relative`/`percent`), jadi `decimal` jatuh ke
      `String(value)` — `formatter.number()` ada tapi tak terjangkau kolom tabel.
      Kini satu resolver bersama `src/lib/relation.ts` + `resolveColumnCell()`
      dipakai **kedua** renderer, `format: number` masuk kosakata sel, dan
      **skala field menang** atas `settings.decimal_scale`. Terukur: Cabang `Kafe
Senayan`, Bahan `Beras Putih`, Saldo `20.000`, tanpa UUID. Plan
      `docs_internal/plan/relation-display-and-number-format.md`, changelog
      `2026-09-24-004`; 12 test baru (1 DOM dibuktikan gagal sebelum patch).
      ✅ 2026-09-24
- [x] 5.18.5 ✅ **2026-09-26 — opsi (b) diambil: tombol sortir kolom relasi dimatikan.** Item ini memberi dua pilihan; yang dipilih adalah yang **kecil dan jujur** — berhenti menawarkan urutan yang ditolak server. Dua tempat: `engine/derive.ts` `isSortable` (sudah mengecualikan `relation`, kini dengan komentar **mengapa**) dan **`kinds/table/TableRenderer.tsx` — ini yang sebenarnya menutup bug**, karena jalur derivasi sudah benar sejak awal; yang salah adalah tabel **authored** yang menulis `sortable: true` sendiri (`cafe-stock/stock-level-table.yaml` pada `branch_id`/`ingredient_id`, tabel kunjungan klinik pada `patient.name`). Helper baru `isRelationColumn(entity, col.field)` me-resolve **akar** nama kolom (`branch.name` → `branch`/`branch_id`) terhadap entity, jadi alias dan dot-path sama-sama tertangkap. **Bukti:** `npx vitest run` **519 lulus** / 38 file, `tsc -b` bersih; test **dibuktikan gagal** saat guard dikembalikan ke `col.sortable ?? true`. Changelog `2026-09-26-015`. Catatan kalibrasi: percobaan pertama saya menambahkan guard di `isSortable` dan test-nya **lulus tanpa patch** (daftar tipe di sana memang sudah tidak memuat `relation`) — guard itu no-op dan diganti. **Opsi (a) tetap terbuka → 5.18.7 ⏸️.**
- [⏸️] **5.18.7 Sortir relasi yang benar-benar bekerja (opsi (a) dari 5.18.5).** `?sort=branch.name` masih `422 unknown field`: `checkField` (`internal/api/handler.go:543`) hanya menerima field entity + kolom normatif, dan `sortParam` diteruskan apa adanya ke `EntityStore.List` yang tidak punya jalur JOIN. Jadi mengurutkan kolom relasi menurut **nama target** belum mungkin. **Teramati:** `?sort=branch_id` → 200 tetapi mengurutkan UUID; `?sort=branch.name` → 422. Effort: medium–large (JOIN atau tabel lookup di PersistBackend, plus jeda antara `checkField` yang menerima dot-path dan store yang bisa menghormatinya).
- [x] 5.18.6 **`TableColumn.format` kini himpunan tertutup** — kafe **10.30**.
      `TableCellFormat` + `ValidateTableCellFormat` (`pkg/spec/widget.go`),
      dipanggil `ValidateTableColumns` yang sudah ada. Himpunannya **bukan**
      salinan `ReportFormat` (`relative`/`number` hanya di sel, `datetime` hanya di
      laporan) — `format: datetime` pada kolom tabel ditolak **dengan petunjuk**
      agar penulis tahu ia format laporan. **Terukur:** `format: currncy` →
      `unknown cell format "currncy" (allowed: currency, number, date, relative,
percent)`. Drift di spec §3.1.2 (mencantumkan `datetime` padahal
      `renderCellValue` tidak mengimplementasikannya) ikut ditutup. Changelog
      `2026-09-24-005`. ✅ 2026-09-24

### 5.19 Temuan lint yang sudah ada sebelumnya (bukan efek 5.18)

- [x] 5.19.1 **`useSelectFilterOptions` dipanggil kondisional** —
      `TableRenderer.tsx` `FilterControl` memanggil hook itu di dalam
      `case "select":` sebuah `switch (filter.type)`, sehingga jumlah hook berubah
      bila tipe filter sebuah baris berubah antar-render. `oxlint` menandainya
      **error** `react-hooks(rules-of-hooks)`.
      **Selesai.** Hook diangkat ke atas `switch`, dan syarat "hanya untuk select"
      dipindah **ke dalam hook** (`isSelect`) — tanpa itu, pemanggilan yang kini
      selalu jalan akan memicu fetch relasi untuk filter `date`/`text`. Dua
      `exhaustive-deps` di file yang sama ikut diperbaiki
      (`tableSpec.fixed_filters` — manifest scope immutable yang berubah **harus**
      memicu refetch; `entity.module`).
      **Terukur:** `oxlint src/kinds/table/TableRenderer.tsx` → **0 temuan**
      (sebelumnya 1 error + 1 warning); filter `Cabang` (`type: select` pada field
      relasi) di `stock-levels` tetap memuat opsinya (**Kafe Dago, Kafe Senayan**),
      jadi perilakunya tidak berubah. Changelog `2026-09-24-005`. ✅ 2026-09-24

### 5.20 Shell — sidebar nav harus bisa di-scroll

- [x] 5.20.1 **Sidebar nav tidak bisa di-scroll** — kafe **10.31**.
      `ScrollArea` sudah dipasang di `Sidebar.tsx`, tetapi `min-height: auto`
      membuat Root tumbuh mengikuti isi sehingga `flex-1` tidak meng-clamp:
      menu 1383px di viewport 560px berakhir tanpa scroll bar dan item terbawah
      tak terjangkau. Diperbaiki di primitif
      (`src/components/ui/scroll-area.tsx` → `min-h-0`), karena memperbaiki dua
      call-site saja akan mengulang bentuk 10.25/10.26 (satu kosakata, dua
      tempat, satu bolong).
      **Terukur:** root 1354 → 504, viewport 488/1338 scrollable, scrollbar
      10×504, item terbawah bottom 1398 → 548, wheel → `scrollTop` 850.
      Test `scroll-area.test.tsx` mengunci kedua varian sidebar. Changelog
      `2026-09-24-006`. ✅ 2026-09-24

### 5.21 Pratinjau gambar dibuka di dialog, bukan tab

- [x] 5.21.1 **Klik gambar membuka tab peramban baru, bukan dialog** — kafe
      (dilaporkan pengguna di halaman detail menu-item). `<img>` sudah benar,
      tetapi pembungkusnya anchor `target="_blank"`: klik membuka JPEG
      telanjang di tab baru sehingga App/sidebar/record hilang. Tiga situs
      bercacat sama (`DetailPage` field file, `FileInput` pratinjau readonly,
      `FileInput` thumbnail mode edit) → komponen bersama
      `components/ui/image-lightbox.tsx` (thumbnail ber-`aria-label` + Dialog +
      judul nama file). Tautan unduh berkas non-gambar tetap `target="_blank"`.
      **Terukur** (browser, viewport 923×560): `openTabs` 2 → **1**, URL record
      tidak berubah, sesudah Close dialog 0, foto 655×439 di panel 831×489
      tercentang, rasio asli dipertahankan. Ukuran panel: default dialog
      (`sm:max-w-sm`, 384px) hanya memberi **423px** (`<` foto 960px) dan
      `w-auto` **lebih buruk** — elemen `fixed` pada `left: 50%` shrink-to-fit
      ke `viewport − 50%` → **462px** di viewport 923px. `w-full
sm:max-w-[92vw]` yang dipakai. Test `image-lightbox.test.tsx` (6) mengunci DOM
      **dan** ketiga situs render, dibuktikan gagal (1 failed / 5 passed) saat
      `DetailPage` dikembalikan ke anchor `target="_blank"`. vitest **383
      lulus** (+6), `tsc -b` bersih. Plan
      `docs_internal/plan/image-preview-dialog.md`, changelog
      `2026-09-24-009`. ✅ 2026-09-24. **Sisa → 5.21.2 ⏸️.**
- [x] 5.21.2 ✅ **2026-09-26** **Sel tabel + kartu katalog kini memakai dialog.** Dua situs tersisa (sel `widget: image` di `lib/renderCell.tsx` — dipakai Table/Listing/Report/ChildTable sekaligus — dan kartu katalog `PickerPanel`) kini memakai `ImageLightbox` yang sama dengan dua situs lain. **Konflik yang item ini sebut belum diputuskan, diputuskan:** pada kedua situs thumbnail adalah bagian dari **hit target induk** (`<tr onClick>` membuka record; kartu katalog **adalah** `<button>` yang menambah ke keranjang — terukur klik foto Kopi Tubruk → Rp18.000), jadi pola situs lain (satu `<button>` membungkus thumbnail) akan **menelan** aksi induk. Keputusannya: **kontrol terpisah di sudut** — `ImageLightbox` mendapat prop `renderTrigger(open)` (pemanggil menyediakan pemicunya sendiri, jadi tidak ada `<button>` bersarang yang tidak sah) dan komponen baru `ImageLightboxTrigger` merender thumbnail + tombol `⤢` kecil yang `stopPropagation()`+`preventDefault()` sebelum membuka dialog. **Bukti:** `npx vitest run` **515 lulus** / 37 file, `tsc -b` bersih; test **dibuktikan gagal** saat `stopPropagation` dihapus (induk menerima klik — tepat kegagalan yang dikhawatirkan item). Changelog `2026-09-26-013`. **Sisa → 5.21.3 ⏸️.**
- [⏸️] **5.21.3 Penempatan tombol `⤢` belum diukur di browser.** Letak/ukuran kontrol pembesaran (sudut kanan-bawah thumbnail sel 40×40px dan kartu katalog) dipilih agar tidak menutupi bagian penting foto dan cukup besar disentuh di POS — tetapi diverifikasi hanya lewat `fireEvent.click` di jsdom, yang tidak punya layout engine. **Teramati:** tidak ada pengukuran `getBoundingClientRect` maupun klik nyata; kelas yang sama dengan 17.7. Effort: small (walkthrough browser + ukur, atau naikkan ukuran hit target bila ternyata sempit).

---

### 5.22 Routing — dokumen otoritatif & visibilitas menu

Plan: `docs_internal/plan/routing-docs-and-menu-visibility.md`

Empat jenis route (A authored Page · B derived Form/Table Page · C derived entity
CRUD · D overlay query-driven) tersebar di empat berkas tanpa dokumen perangkum,
dan penelusuran menemukan tiga cacat nyata: `ResolveViewRoute` kehilangan
`Listing` (validate hijau → App gagal resolve), `MenuItem.When` tidak dievaluasi
sekaligus tidak divalidasi, dan `MenuItem.Permissions` diperiksa klien padahal
field-nya tidak ada. Semua ditutup di fase ini, dengan dokumen sebagai
deliverable utama.

- [x] 5.22.1 `ResolveViewRoute` — cabang `Listings` + doc comment "empat salinan
      konvensi", supaya `view: <listing>` yang lolos `formspec validate` tidak
      lagi membuat `app.Resolve` gagal. ✅ 2026-09-25. Changelog `2026-09-25-001`.
- [x] 5.22.2 `MenuItem.Permissions` (RBAC) + penyaringan **server** di
      `filterMenu(can)`; cek mati di klien dihapus. `when:` tetap kondisi bisnis
      di klien. ✅ 2026-09-25. Changelog `2026-09-25-002`.
- [x] 5.22.3 `MenuItem.When` dievaluasi klien (`filterMenuItem`, fail-open +
      error dilaporkan) + gate deploy-time: himpunan tertutup callable
      `{len, sum, amount, currency, today}` dan `checkMenuExpr` untuk menu. ✅
      2026-09-25. Changelog `2026-09-25-003`.
- [x] 5.22.4 Migrasi contoh `clinic` (`when: user.has(…)` →
      `permissions:`) + perbaikan spec: `02-workspace-app-module.md` §4 hapus
      "Form/Table bukan target `view`", tambah `Listing`/`permissions`/`when`;
      `08-formspec-expr.md` §2 `today()` + §3 cakupan larangan identitas. ✅
      2026-09-25. Changelog `2026-09-25-004`.
- [x] 5.22.5 `docs/renderers/shadcn-shell/05-routing.md` — dokumen otoritatif
      (komposisi URL, 4 jenis route, cara membedakan, cara resolve, lapisan menu,
      family kind navigasi, resep diagnosis, divergensi). ✅ 2026-09-25. Changelog
      `2026-09-25-005`.
- [x] 5.22.6 ✅ **2026-09-26** **`routeExists` kini membaca `bundle.pages`, bukan registri Form/Table.** Dua sumber yang tidak sepakat: `BuildBundle` **tidak** menurunkan `<name>-page` untuk Form/Table yang sudah direferensikan blok Page lain (`covered[...]`) atau yang `public: false` → SPA tidak mendaftarkan route-nya, tetapi registri masih memuat Form/Table-nya, sehingga `routeExists` menjawab "ada" dan item menu mengarah ke 404 yang terlihat hidup. Kini cabang 2 memeriksa `b.Pages` — **persis** apa yang di-iterasi `buildRoutes` (`shell/router.tsx`). Bonus tanpa kode tambahan: `b.Pages` sudah disaring per pemanggil (`allowedPage`), jadi item yang menunjuk page yang tidak boleh dibuka ikut turun dengan alasan yang sama. **Dua assertion di `menu_filter_test.go` ikut diperbarui** — keduanya mengharapkan `"Order table"` muncul padahal `order-table` direferensikan Page `order-list` di fixture, jadi keduanya mem-pin bug-nya. **Bukti:** `TestRouteExists_FormTableFollowBundlePages` (fixture sendiri: dua Table untuk satu entity, satu Page mereferensikan hanya salah satunya — asimetri yang membuat bug terlihat) **dibuktikan gagal** saat `routeExists` dikembalikan ke versi registri, dan kedua sub-test lama kembali memunculkan `"Order table"`; `go test ./...` 0 FAIL; kafe `validate` 85/0, `check` 0/0. Changelog `2026-09-26-009`. Effort selesai: small.
- [x] 5.22.7 ✅ **2026-10-02** **Surface `_admin` buta `permissions`/`when` —
      DITUTUP dengan dihapusnya surface-nya, bukan dengan menyaring menunya.**
      Keputusan pemilik proyek 2026-10-02: varian bundle unscoped `?admin=true` + gerbang biner `_admin.access` adalah **attack surface** (satu permission
      membuka seluruh module; role `app-owner` wildcard `"*"` otomatis
      memegangnya) — jadi panel entity `_admin` dipensiunkan, rute framework
      (`setup`, `oauth/*`, `change-password`) dipertahankan. Setiap bundle kini
      App-scoped dan permission-filtered, sehingga tidak mungkin lagi sidebar
      `_admin` menampilkan entity yang endpoint-nya akan 403. Kode: cabang
      `alwaysVisible` + `adminAccessPermission` dibuang (`internal/api/meta.go`,
      → 400 `ADMIN_BUNDLE_REMOVED`); cabang `isAdmin`/`deriveMenuItems` dibuang
      (`useResolvedMenu.ts`); route `:_admin/*` panel dibuang (`App.tsx`). Plan
      `docs_internal/plan/app-scoped-login.md` D4. ✅ 2026-10-02 · changelog
      `2026-10-02-006`
- [x] 5.22.8 ✅ **2026-09-26** **`docs/renderers/shadcn-shell/01-architecture.md` §5 memuat klaim usang — DIPERBAIKI, dan ternyata ENAM klaim, bukan tiga.** Item ini menyebut tiga (OverlayHost tidak terhubung · `deriveMenuItems()` kode mati · `TableRenderer` hardcode `/_admin`); verifikasi ke kode menemukan **tiga lagi**: `engine/registry.tsx` **sudah dihapus** (`ls` → tidak ada), realtime **sudah ada** (`hooks/useRealtime.ts` dipakai 7 renderer), dan component contract `asset` **sudah ada** (`shell/AssetRenderer.tsx` memanggil `mount`/`unmount` + `formspec` client). Yang membuat ini bukan sekadar catatan basi: §5 **bertentangan dengan `03-kind-renderers.md:60`** di repo yang sama, yang sudah lama menulis dengan benar ("navigasi memakai `useSurface().surfacePath`", "`Form.render` **dihormati**") — dua halaman memberi jawaban berlawanan. §5 ditulis ulang dengan bukti per baris + paragraf "cara memakai section ini" (hapus baris saat tertutup, jangan tumpuk narasi historis). Changelog `2026-09-26-005`. Effort selesai: small.

### 5.23 Field `help` — warisan `description` entity + situs bolong

- [x] 5.23.1 **`description` entity hilang begitu entity punya `kind: Form`; help
      hanya dirender di 3 situs (1 di antaranya 1 dari 6 cabang).** Diukur dari
      pertanyaan pengguna pada `promo-form`. Akar: `formField()` (jalur derivasi)
      membaca `field.description`, `resolveForm()` mengembalikan manifest
      authored apa adanya — jadi penulis menyalin manual (`promo-form.yaml` dan
      `promo/entity.yaml` memuat kalimat **identik**). `withEntityFieldDefaults()`
      kini mengisi `label` + `help` + `sections[0].description` di keempat jalur
      `resolveForm()`; `WizardFormStep` merender help di **keenam** cabang (dulu
      hanya `relation`) dan melewati `resolveForm` (dibaca dari bundle) sehingga
      memanggil resolver yang sama; `WizardRenderer` jalur `steps[].fields`
      inline (0 help) ikut; `OverlayHost` subtitle memakai resolver. `@schema`
      ditambahkan pada `FormField.Help`/`Field.Description` + `schemas/`/
      `docs/kind/` di-regenerate. **Terukur**: vitest **444 lulus** (+11 di
      antaranya; dibuktikan **3 failed** saat warisan dinetralkan), drawer promo
      help `branch_id`/`priority` tampil **tanpa** `help:` di YAML, subtitle
      drawer beralih dari `"Fill in the details for this promo."` →
      deskripsi entity, wizard close-shift step 2 menampilkan **dua** help
      (money + string, dua cabang berbeda), `go test ./...` hijau, `formspec
check` 0 error. Plan `docs_internal/plan/field-help-inheritance.md`,
      changelog `2026-09-25-006`. ✅ 2026-09-25. **Sisa → 5.23.2 ⏸️, 5.23.3 ⏸️.**
- [x] 5.23.2 ✅ **2026-09-26** **Deskripsi field kafe ditulis sebagai teks pengguna, bukan catatan desain.** `description` (Entity) diwarisi sebagai `help` (5.23.1), jadi ia dirender di bawah input — dan di kafe banyak yang berbunyi `"compute: kas awal + tunai masuk - kas keluar"`, `"Array angka 1=Senin..7=Minggu, mis. [1,2,3,4,5]"`, `"Posting — script membuat stock-movement (adjust) sebesar selisih"`. **18 deskripsi di 11 berkas** ditulis ulang dengan prinsip **memindahkan** detail, bukan menghapus: rumus & penanda tipe → komentar YAML (`#`) di atas field; rujukan ledger (`aturan bisnis #7`, `(S12)`, `(D5)`) → komentar; deskripsi menyebut **hasilnya** untuk pengguna, bukan nama script. Contoh: `expected_cash` kini `"Kas yang seharusnya ada di laci menurut sistem."`. Contoh nilai (`"Mis. KFE-JKT-01"`) **sengaja dibiarkan** — percobaan pertama saya menandainya sebagai pelanggaran dan itu **salah**; marker dipersempit ke `"mis. ["` (literal array = dokumentasi bentuk tipe). **Bukti:** guard baru `TestKafeFieldDescriptionsAreUserFacing` (12 marker, scan `examples/kafe/spec`) melaporkan **22 pelanggaran sebelum → 0 sesudah**, dan **dibuktikan gagal** saat satu deskripsi dikembalikan ke bentuk lama; kafe `validate` 85/0 · `check` 0/0. Changelog `2026-09-26-014`. Effort selesai: medium. **Sisa → 5.23.4 ⏸️.**
- [⏸️] **5.23.4 `verticals/`/`Clinic-UI-Showcase`/`cmd/formspec-registry` belum diaudit untuk kelas yang sama.** Item 5.23.2 hanya menyebut `examples/kafe`; changelog 5.23.1 mencatat deklarasi serupa di tiga tempat lain. Guard `TestKafeFieldDescriptionsAreUserFacing` **sengaja hanya menegakkan kafe** — memperluasnya ke seluruh repo sekarang akan gagal pada berkas yang belum ditinjau dan memaksa perbaikan yang belum diperiksa. **Teramati:** guard di-scan 1 direktori; pelanggaran di tiga tempat lain belum dihitung. Effort: medium (audit + perbaiki + perluas guard).
- [x] 5.23.3 ✅ **2026-09-26** **`SearchSelect` kini memakai presedensi `help` yang sama** — ia satu-satunya situs baca-Form yang melewatkannya (hanya `entityFieldLabel`), sehingga dialog quick-create di wizard adalah tempat **satu-satunya** di aplikasi di mana `description` sebuah Entity tidak sampai ke pengguna: label diwarisi, help tidak. Premis item ("putuskan apakah ia perlu help — ia layar pemilihan") menyempit begitu kodenya dibaca: `SearchSelect` memang layar pemilihan, **tetapi ia punya dialog quick-create berisi input sungguhan** (`renderField` → `handleCreate` → `apiPost`), dan bagian itulah yang diperbaiki. **Kelima cabang `renderField` dikonsolidasikan lewat satu helper `wrap(control)`** — sebelumnya tiap cabang menulis `<label>` sendiri, jadi menambah help berarti mengedit lima tempat dan yang terlewat tidak terlihat (itulah bagaimana kelimanya sama-sama tidak punya help). **Bukti:** `npx vitest run` **499 lulus** / 35 file (baseline 473/31), `tsc -b` bersih; test baru `search-select-help.test.tsx` (10) **dibuktikan gagal** saat satu cabang dikembalikan ke bentuk lama (`Expected 5, Received 4`). Changelog `2026-09-26-010`. Effort selesai: small.

---

### 5.24 Kontrak input transisi & action (`params.inputs`)

Plan: `docs_internal/plan/action-input-contract.md`. Changelog `2026-09-28-004`.

- [x] 5.24.1 ✅ **2026-09-28** **Transisi & action punya kontrak input deklaratif
      yang ditegakkan DAN bisa dirender.** Akar: `params` hanya berisi rule
      validasi (`{field, rules}`), jadi tidak ada yang bisa _membuat_ body —
      hanya menolaknya. Tiga cacat nyata: (1) jalur PATCH (satu-satunya jalur
      transisi tanpa `impl`) hanya mengevaluasi `guard`, **bukan** `conditions`
      transisi maupun `params.validate`-nya; (2) tidak ada permukaan yang bisa
      mengumpulkan input (tombol transisi POST tanpa body); (3) input pemohon
      **hilang** saat approval — 202 sebelum write, dan `handleWorkflowApproval`
      hanya membaca verb `decision`. Kini: `ParamInput` (merujuk field entity
      → tipe/enum diwarisi + nilai disimpan; ad-hoc wajib `type`),
      `inputs_from` (set bernama di entity), `render.mode`;
      `EffectiveParamValidation` dipakai di **3 jalur tulis**; kolom `params` di
      `formspec_workflow_approval` + `seedStoredApprovalParams`/
      `mergeApprovalParams`; `ActionSummary.params` + `EntitySchema.input_sets`
      di bundle; `formspec generate` bertipe dari `inputs`; satu
      `ActionInputDialog` generik dipakai DetailPage/Table (baris + massal)/
      Kanban. **Terukur:** PATCH `{"status":"voided"}` → **422** (sebelumnya
      lolos), PATCH dengan nilai → 200 + nilai tersimpan, approval → nilai
      pemohon bertahan. `go test ./...` hijau · vitest **580** hijau · kafe 89
      manifest 0 problem.
- [x] 5.24.2 ✅ **2026-09-28 — DITUTUP SEBAGAI TIDAK DIPERLUKAN** (bukan
      dikerjakan). **Kenapa ditutup, bukan dibiarkan ⏸️:** alasan deferralnya
      melingkar — "keputusan D7" adalah rekomendasi saya sendiri di percakapan
      yang sama, jadi ia tidak lolos aturan repo ini ("pekerjaan tertunda harus
      punya alasan yang bisa diperiksa"). Dua pengukuran menggantikannya:
      **(1) klaim teknisnya benar** — `FormRenderer.doSubmit` punya empat cabang
      (service `submit.call`, PATCH edit, `create-submit`, POST create) dan
      **tidak satu pun** mem-POST ke `{id}/{action}`, jadi `kind: Form` memang
      tidak bisa menyasar action entity; **(2) permintaannya nol** — jumlah input
      terdeklarasi terbesar di seluruh repo adalah **2**
      (`examples/service-demo/.../tax-calculator.yaml`); kafe `void-order` = 1.
      Satu-satunya justifikasi Tier 2 adalah ">12 input yang butuh
      sections/columns", dan tidak ada satu pun yang mendekati. Kind yang sudah
      ada juga bukan penggantinya: `Wizard.action` mem-POST path apa adanya
      (dirancang untuk "memfinalkan draft", halaman penuh, tidak sadar `{id}`).
      **Trigger untuk membuka kembali:** ada transisi/action nyata dengan input
      yang butuh sections/columns atau `default_from` dari render-context — saat
      itu bentuknya pun sebaiknya **memperluas `ParamInput`** (tambah
      `sections`), bukan menautkan `kind: Form` yang terikat satu entity + satu
      mode. Effort: medium (tetap medium bila terpicu).
      <details><summary>teks asli</summary>**Tier 2 — `params.form.ref` ke `kind: Form` bernama.** Keputusan D7 menunda bentuk ini: deklarasi inline + `inputs_from` sudah menutup seluruh kasus yang ada, sedangkan `params.form.ref` menuntut `FormRenderer` menerima target submit ketiga (kini hanya `submit.call` service dan `{id}/submit`), jadi bukan sekadar penambahan deklarasi. Perlu bila ada transisi dengan >12 input yang butuh sections/columns. Effort: medium.</details>
- [⏸️] 5.24.6 **Identitas approver tidak pernah masuk ke field record.** Approval
  mencatat SIAPA yang menyetujui di dua tempat — `formspec_workflow_approval.approvals`
  (stepIdx → userIDs) dan audit trail — tetapi tidak ada cara deklaratif
  memproyeksikannya ke field entity. **Teramati (bukti, bukan dugaan):**
  `void_approved_by` muncul **3 kali** di seluruh repo (`grep -rn` di luar
  `.git`/`node_modules`) — deklarasi field
  (`examples/kafe/.../cafe-order/transaction/order/entity.yaml:170`), satu
  entri `read_only: true` di `order-form-pos.yaml:60`, dan baris dokumentasi
  di `examples/kafe/docs/domain-model.md:390` yang menyebutnya "Supervisor
  penyetu" — dan **tidak ada satu pun penulis**: bukan script, bukan seed,
  bukan engine. Jadi field yang sengaja dideklarasikan `read_only` (author
  sudah tahu user tidak boleh mengisinya) tetap kosong selamanya, termasuk
  setelah approval supervisor berhasil dijalankan. Ini sisa nyata dari jalur
  approval, dan lebih layak dilacak daripada "Tier 2" yang tidak punya
  permintaan. Effort: small–medium (nama field bisa dideklarasikan pada
  `Workflow`/step, atau engine mengisi konvensi `{prefix}_approved_by`).
- [⏸️] 5.24.7 **`resource.*` di console belum ter-wire, sehingga repair data
  terpaksa SQL mentah.** `formspec repl` membuat `resource` dengan
  `fsstarlark.NewResourceAPI("", "", "", 0, map[string]any{})` dan **tidak**
  memanggil `SetFindFunc`/`SetLoadFunc`/`SetSaveFunc`/`SetCreateFunc`, jadi
  `resource.find`/`fetch`/`save` gagal — terbukti 2026-09-28 saat menulis repair
  kafe: satu-satunya jalur yang bekerja adalah `ctx.db().query(...)` dengan SQL
  mentah, yang justru **dilarang konvensi repo ini** untuk business logic. Dua
  konsekuensi nyata pada repair yang jadi contoh resmi: (a) menulis field
  `status` lewat SQL **tidak memicu transisi** (`emit`/subscription tidak jalan,
  `updated_by` tidak terisi) — bandingkan jalur `resource.save()`; (b) perbaikan
  duplikat jadi tanggung jawab operator sepenuhnya, tanpa validasi entity.
  Effort: medium (wire `resource` dari registry yang sama dengan
  `CtxPrimitiveResolver`; `SetFindFunc` sudah arah yang benar).
- [x] 5.24.3 ✅ **2026-09-28** **Transisi `via`+`impl` kini dapat route REST dan
      muncul di `formspec generate`.** `GenerateCustomActionRoutes` dipindah ke
      `ActionSources()` — menyamakannya dengan tiga situs lain yang sudah memakai
      union (`UICustomActionRoutesForEntity`, `generatePrepareRoutes`, cabang
      `custom` di router). **Terukur:** sebelum → `[]` (nol route) untuk transisi
      `via: post` ber-`impl`; sesudah → `POST /api/v1/cafe-order/orders/{id}/post`
      dengan permission `cafe-order.orders.post`; `formspec generate` menghasilkan
      `GlJournalEntryPostParams { "post_note": string }` + method `post(...)` yang
      sebelumnya tidak ada sama sekali. **Temuan tambahan di luar teks asli item
      ini:** satu aksi bisa dapat **dua** descriptor `(Method, Path)` — generator
      generik membaca `es.Actions` untuk memutuskan "punya `impl` sendiri?",
      sementara generator custom membaca union, sehingga transisi
      `via: submit|cancel|amend` ber-`impl` tak terlihat oleh yang pertama; karena
      `mergeRoutes` menyimpan descriptor PERTAMA, handler generik menang dan
      `impl` yang dideklarasikan **diam-diam tidak pernah berjalan** (laten —
      contoh yang ada mendeklarasikan `cancel` ber-`impl` lewat `actions:`, yang
      terbaca `es.Actions`). Ditutup dengan param `customHandled` +
      `TestGeneratedRoutes_HaveNoDuplicatePath` yang menguji invariant
      `(Method, Path)` unik di kedua surface. Changelog `2026-09-28-005`; klaim L3
      di `plan/via-sebagai-action-penuh.md` **dikoreksi** (sebelumnya menyebut
      `GenerateCustomActionRoutes` sudah membaca union — tidak).
      <details><summary>teks asli</summary>`GenerateCustomActionRoutes` (`internal/api/generator.go`) membangun dari `es.Actions` — **bukan** `ActionSources()` — berbeda dari `UICustomActionRoutesForEntity`, `generatePrepareRoutes`, dan `meta.go` yang semuanya sudah memakai union. **Teramati:** manifest dengan transisi `via`+`impl` (tanpa entri `actions:`) menghasilkan route `/_ui/entity/…` tetapi tidak ada `/api/v1/…`, jadi klien TypeScript tidak punya method maupun tipe params untuknya, padahal endpoint UI-nya ada. Ditemukan saat mengerjakan 5.24.1 (test codegen terpaksa memakai entri `actions:` agar deskriptornya muncul). Effort: small.</details>
- [x] 5.24.4 ✅ **2026-09-28** **Kafe jalur void sudah diuji end-to-end melalui
      approval — dan menemukan 2 bug, keduanya ditutup.** Sekarang tertutup oleh
      `resource/kafe_void_approval_e2e_test.go` pada spec kafe asli: void tanpa
      alasan → **422**, dengan alasan → **202** (tidak ada yang tertulis),
      pemohon menyetujui sendiri → **403**, supervisor menyetujui → **200**, lalu
      status `cancelled` **dan** `void_reason` tersimpan bersama. **Bug 1 ada di
      kode saya sendiri** (`2026-09-28-004`): `EffectiveActionSpec` membiarkan
      entri `actions:` menang, sehingga `params.inputs` yang dideklarasikan di
      TRANSISI terbuang ketika entri itu juga ada (kafe: entri menyumbang
      `description`+`audit`+`conditions`, transisi menyumbang inputs) —
      terukur `PATCH {status, void_reason}` → **422 "Alasan void wajib diisi"**
      padahal body memuat alasannya; artinya perbaikan 004 belum bekerja pada
      manifest kafe. Ditutup dengan overlay per-field (+`Conditions` digabung,
      bukan diganti). **Bug 2 → 5.24.5.** Changelog `2026-09-28-006`.
      <details><summary>teks asli</summary>`resource/kafe_table_lifecycle_e2e_test.go` sengaja menghindari void (`void-order` approval-gated, dan sebelum 5.24.1 alasannya tidak bisa dikumpulkan) — jadi rantai sesungguhnya (kasir void → 202 → supervisor approve → order `cancelled` **dengan** `void_reason`) belum pernah dijalankan pada app kafe. Semantiknya kini dikunci di level API dengan fixture berbentuk sama (`internal/api/approval_input_test.go`), bukan pada manifest kafe. Effort: small (tambahkan langkah ke harness `bootKafe`; manifestnya sudah mendeklarasikan `params.inputs` sejak 5.24.1).</details>
- [x] 5.24.5 ✅ **2026-09-28** **Transisi ber-approval tidak pernah memancarkan
      `emit`-nya.** Ditemukan oleh e2e 5.24.4, bukan diduga: `HandleUpdate`
      `return` **sebelum** blok resolusi emisi begitu approval diperlukan, dan
      `executeWorkflowTransition` tidak meresolusinya sendiri. **Terukur:** kafe
      `void-order` (`emit: on_cancel`) → order `cancelled`, meja tetap
      **`occupied`** selamanya (timeout pada `available`) — jembatan meja-lah
      yang mendengarkan `on_cancel`, dan indeks unik parsial menolak sesi OPEN
      kedua, jadi **tamu berikutnya tidak bisa check-in**. Asimetri ini tak
      terlihat karena transisi yang sama **tanpa** workflow memancarkan
      event-nya dengan benar. Ditutup di `executeWorkflowTransition`: `fromState`
      diteruskan (dari baris approval, jadi pasangan `(from,to)` tetap
      mengidentifikasi transisinya), `ResolveTransitionEmission` dijalankan, dan
      `PendingEvents` ikut ke `store.Update` yang sama → state + event
      **atomik**. Terkunci oleh `TestKafe_VoidOrder_EmitsOnCancel` (gagal dulu
      dengan timeout). Changelog `2026-09-28-006`; kontraknya kini ditulis di
      `01-core-basic.md` (S13) dan `02-core-extended.md` §2.

### 5.25 Permukaan App — `registered_views` (allowlist view per App)

Plan: `docs_internal/plan/registered-views.md` · Changelog `2026-10-02-001`

`App.spec.modules` memilih modul, tetapi bundle mengirim **semua** entity
modul yang lolos permission; klien mendaftarkan route CRUD turunan untuk setiap
entity — jadi entity yang tidak dipakai App tetap bisa dibuka lewat URL
langsung. `registered_views` menutupnya: **permukaan App = setiap target leaf
menu ∪ `registered_views`**; route di luar itu tidak didaftarkan (SPA 404).
Entity non-routable tetap dikirim (relasi/picker tetap resolve) dengan penanda
`routable: false`.

- [x] 5.25.1 ✅ **2026-10-02** **`App.spec.registered_views` + gating permukaan.**
      Tipe `RegisteredViewDecl{entity|view}`; `ValidateAppSpec` (tepat satu field,
      modul ter-mount, tolak duplikat); resolusi existence di `app.Resolve`
      (view/entity harus ada → tolak saat boot). Gating di `BuildBundle`:
      `reachableViewRoutes`/`reachableEntities` dari menu ∪ `registered_views`;
      entity → `EntitySchema.Routable`; Page authored (kecuali home `"/"` & auth
      screen), derived wrapper Form/Table, dan seluruh nav kind digerbang route.
      `routeExists` menghormati `Routable`. Surface `_admin`/editor grants
      (`?grants=true`) **tidak** digerbang. Klien: `buildRoutes` tidak
      mendaftarkan route entity `routable === false`; `canLandOnList` menolaknya.
      Warning `formspec check` bila App memount modul tanpa menu &
      `registered_views`. Contoh kafe dimigrasikan (`kafe-qr` 3 Page,
      `kafe-kds` board, `kafe-pos` view non-menu). Test: `registered_views_test.go`
      (spec/ui/app, termasuk `TestResolve_KafeSpec` nyata) + vitest. Effort
      selesai: medium.
- [⏸️] 5.25.2 Entity non-routable yang dinavigasi dari picker/detail → 404.
  Entity yang tidak didaftarkan tetap dikirim (`routable: false`) agar
  relasi/picker resolve, tetapi picker yang menautkan ke **halaman detail**
  entity itu akan mendarat di 404 (route-nya tidak ada). Trade-off sengaja;
  yang terbuka: memberi picker tahu `routable` sehingga tautan detailnya
  tidak dirender. Effort: small.
- [⏸️] 5.25.3 Lint **pre-existing** (bukan efek 5.25): `(*entityIndex).isModule`
  di `internal/ui/meta.go` dan `assetSource` di `cmd/formspec/seed_asset.go`
  tak terpakai → `make lint` gagal (2 issue `unused`). Keduanya sudah mati di
  `HEAD` (diverifikasi `git show HEAD:… | grep`), jadi bukan regresi sesi ini.
  Effort: small (hapus, atau pakai kembali bila memang disiapkan).
- [⏸️] 5.25.4 **`kind: Wizard` tidak punya peluncur di UI** — wizard hanya bisa
  dibuka lewat URL langsung. `close-shift-wizard` (kafe) mengikat dirinya ke UI
  lewat `entity: cafe-order.shift` + `action: close-shift` (transisi
  `via: close-shift` pada entity `shift`), **tetapi tidak ada konsumen** yang
  me-resolve edge itu: `DetailPage`/`engine/lifecycle.ts` merender transisi
  sebagai `PATCH`/`ActionInputDialog`, tanpa memeriksa apakah ada Wizard untuk
  action itu (diverifikasi — `getWizard` tidak dipanggil dari jalur transisi).
  Juga tidak ada Page "Shift Kasir": yang ada hanya derived entity page
  `/cafe-order/shifts` (Jalur C), yang tidak menautkan ke wizard. Akibatnya
  `registered_views` hanya membuat route-nya _ada_, bukan _terjangkau_.
  Dua pilihan: (a) sambungkan tombol transisi → wizard bila ada Wizard
  ber-`action` yang sama; (b) transisi/action menyatakan `wizard:` eksplisit.
  Efek samping ke desain gate: bila (a)/(b) landing, wizard bisa **inherit**
  reachability dari entity-nya (opsi A diskusi 2026-10-02) sehingga tidak perlu
  lagi masuk `registered_views`. Effort: medium.
- [⏸️] 5.25.5 **`close-shift-wizard` tidak bisa commit — `spec.action` menunjuk
  action yang tidak punya route.** Terverifikasi 2026-10-02:
  - `close-shift` hanya ada sebagai transisi `via: close-shift`
    (`shift/entity.yaml`), BUKAN action di `actions:` (isinya hanya
    `delete`/`submit`, keduanya `disabled: true`), dan tanpa `impl`.
  - `internal/api/generator.go`: route aksi hanya dipancarkan untuk action
    ber-`impl` (`GenerateUICustomActionRoutes` dari `ActionSources()`) + action
    standar; `via` tanpa `impl` = action sintetis **tanpa route**. Jadi
    `POST .../{id}/close-shift` **tidak ada** (jalur sahnya `PATCH status`).
  - `WizardRenderer.handleSubmit` juga mem-POST `entry.spec.action` mentah ke
    client ber-prefix `/{ws}/_ui/entity` → `POST /…/_ui/entity/close-shift`,
    padahal router mengharapkan `/{module}/{entity}/{id}/{action}` → bentuknya
    salah walau route-nya ada.
  - Wizard juga tidak pernah membawa **id record** (shift mana yang ditutup).
    **Konsekuensi terukur:** di halaman detail shift (`DetailPage`) tombol
    transisi MEMANG dirender (label dari `description`, tampil bila pemanggil
    memegang `cafe-order.shifts.close-shift`; `authorized_actions` memuat
    `close-shift` lewat `ActionSources()` — `internal/ui/meta.go:1586`), dan
    karena `HasRoute: false` (`meta.go:1462`, `impl == nil`) kliknya menjalankan
    **`PATCH status=closed` langsung** — **melompati wizard sepenuhnya**:
    `counted_cash`/`note`/`supervisor_id` tidak pernah dikumpulkan, sehingga
    `difference` (computed `counted_cash - expected_cash`) dihitung dari field
    kosong. Jadi bukan "tidak ada jalan menutup shift", melainkan "jalan yang ada
    melewati wizard-nya". Perbaikan: (a) `close-shift` jadi action ber-`impl`
    (pola `gl/journal_post.star`), wizard diberi id + path lengkap; atau (b)
    tambah mode commit `PATCH {state_field: to}` di WizardRenderer; atau (c)
    sambungkan tombol transisi ke wizard bila ada Wizard ber-`action` yang sama
    (5.25.4). Effort: medium.
- [x] 5.25.4 ✅ **2026-10-02** **Wizard kini punya peluncur: tombol transisi →
      wizard.** `DetailPage.handleTransition` memeriksa `findWizardForTransition`
      (wizard dengan `spec.entity` menunjuk entity ini **dan** `spec.action ===
via`) SEBELUM dialog input/konfirmasi, lalu menavigasi ke
      `surfacePath("wizard", name)?id=<record-id>` — wizard sendirilah yang
      mengumpulkan input, jadi membuka dialog generik dulu akan bertanya dua kali.
      Aturan matching ditaruh di `src/engine/wizardCommit.ts` (murni, teruji).
      **Terukur di browser (kafe-pos, kasir):** klik "Tutup shift: hitung fisik…"
      di `/cafe-order/shifts/<id>` → URL
      `/kafe/app/pos/wizard/close-shift-wizard?id=01a0bf96-…` (Step 1 of 3).
      Changelog `2026-10-02-002`. Effort selesai: small.
- [x] 5.25.5 ✅ **2026-10-02** **Wizard bisa commit transisi `via` (PATCH) dan
      path action diperbaiki.** `resolveWizardCommit` memilih call dengan aturan
      yang sama yang sudah dipakai `DetailPage` (kafe 10.48, `has_route` dari
      bundle): `has_route` → `POST /{module}/{entity}/{id}/{action}`; selain itu,
      bila `action` cocok transisi (`via`) → `PATCH /{module}/{entity}/{id}` dengan
      `{state_field: to, …field terkumpul}`. Id dibaca dari `?id=`. Sekaligus:
      (i) path POST tidak lagi mengirim `spec.action` mentah (dulu
      `…/_ui/entity/close-shift` — mustahil); (ii) payload hanya memuat field
      entity; (iii) `on_complete` default mendarat di **daftar entity wizard di
      surface saat ini** (sebelumnya `adminPath()` — salah untuk App surface;
      kasir kafe-pos tidak punya `_admin.access`). **Terukur di browser:**
      `PATCH /kafe/_ui/entity/cafe-order/shift/01a0bf96-… -> 200`, toast
      "Wizard completed successfully", dan record: `status=closed`,
      `counted_cash={amount:"150000",currency:"IDR"}` (sebelumnya selalu `None`),
      `note="kurang 10000"`, `version` 1→3. Unit: `src/engine/wizardCommit.test.ts`
      (11 test). Changelog `2026-10-02-002`. Effort selesai: small.
- [⏸️] 5.25.6 **Seed role usang menyembunyikan action (kafe).** Saat verifikasi
  5.25.4/5.25.5, `cafe-order.shifts.close-shift` **tidak ada** di
  `authorized_actions`/JWT kasir meski `examples/kafe/spec/modules/formspec.core/
seeds/roles.yaml` mendeklarasikannya di `{page: shift-page, actions: […,
{name: close-shift}]}`: DB seed lama menyimpan grant tanpa action itu. Otoritas
  adalah **DB grant**, bukan file seed; permission role dimaterialisasi saat
  boot, jadi urutannya `make seed-kafe` → **restart server** (sudah dicatat di
  memori repo). Yang belum tertutup: **tak ada sinyal** bahwa seed file dan DB
  menyimpang — gejalanya adalah action yang hilang diam-diam. Usulan: `formspec
check`/`validate` membandingkan grant seed vs DB (atau `seed --dry-run`
  melaporkan drift). Effort: medium.
- [x] 5.25.7 ✅ **2026-10-02** **Wizard yang mengikat transisi mewarisi
      reachability entity-nya.** Sebelumnya wizard hanya masuk bundle bila
      didaftarkan di `registered_views` (`viewRoutable`), sehingga melepas
      pendaftaran itu diam-diam menghidupkan lagi jalur bypass: peluncur
      (`DetailPage`) membaca `bundle.wizards`, tidak menemukan apa pun, lalu jatuh
      ke `PATCH status` mentah. Kini `BuildBundle` menambahkan aturan: wizard dengan
      `spec.entity` + `spec.action` yang cocok dengan transisi (`via`) dari entity
      yang **routable** ikut reachable. Pendaftaran eksplisit tetap berlaku
      (aditif), dan `_admin`/`?grants=true` tak terpengaruh. **Terukur:** daftar
      `registered_views` kafe-pos dihapus barisnya → `wizards:
['close-shift-wizard']` untuk kafe-pos (shift routable lewat menu), `wizards:
[]` untuk kafe-kds (shift tidak ada di permukaannya). Unit: 4 test di
      `internal/ui/wizard_reachability_test.go`. Changelog `2026-10-02-003`.
      Effort selesai: small.

- [x] 5.25.8 ✅ **2026-10-02** **Tutup shift bisa dari daftar (1 klik dari
      menu).** Dua perubahan: (1) `TableRenderer.handleRowAction` memakai
      `findWizardForTransition` seperti `DetailPage` (dulu jatuh ke
      `POST /{entity}/{id}/{action}` — route yang transisi `via` tidak punya, jadi
      404); (2) transisi `close-shift` diberi `ui: {button_label: "Tutup Shift",
icon: clock}` sehingga derived table menawarkannya sebagai row action
      (`engine/derive.ts` hanya menambahkan custom action ber-`ui`). Baris non-`open`
      tetap tidak menawarkannya (`isActionAllowedForRow` menyaring per `from`).
      **Terukur di browser (kasir):** daftar Shift Kasir → baris **Open**
      menampilkan tombol **"Tutup Shift"**, tiga baris **Closed** tidak; klik →
      `/kafe/app/pos/wizard/close-shift-wizard?id=01a0fc5b-…`; selesai →
      `counted_cash={amount:"260000"}` + `note="lebih 10000"` tersimpan,
      `status=closed`, dan tombolnya hilang dari baris itu. Guard test
      `src/kinds/table/wizard-row-action.test.ts` (3) — **dibuktikan gagal** saat
      cabang wizard disuntik-keluar. Changelog `2026-10-02-004`. Effort selesai: small.

- [⏸️] 5.25.9 **View yang routable tetapi tanpa pemicu di UI — SEBAGIAN ditutup
  2026-10-04.** `registered_views` membuat route-nya **ada** (SPA tidak 404),
  bukan **terjangkau** — pembedaan yang sama dengan yang sudah dicatat untuk
  wizard di 5.25.4.
  **Yang sudah landing:** `supervisor-inbox` kini punya entri menu authored di
  `kafe-pos` (grup "Persetujuan" → "Antrean Void"), jadi jalurnya nyata dan
  entri `registered_views`-nya dihapus. Menulis entri itu memaksa perbaikan
  `viewKinds` (`cmd/formspec/validate_dangling.go`) yang tertinggal empat kind —
  lihat changelog `2026-10-04-003`.
  **Yang belum:** `kind: Print` (`receipt-thermal`, `receipt-digital`) **tetap
  tanpa pemicu apa pun**. Route-nya ada (`buildRoutes` mendaftarkan
  `{basePath}/print/{name}` dan `.../print/{name}/:id`), `PrintRenderer` sudah
  menerima `useParams().id`, dan `roles.yaml` sudah memberi grant
  `print:receipt-thermal` — tetapi **tidak ada satu pun** `print/...` di
  `renderers/react-shadcn/src/kinds/**` maupun `src/engine/**`: tidak ada tombol
  "Cetak struk" di `DetailPage`/`TableRenderer`, tidak ada yang menautkan dari
  record pesanan. Jadi kasir masih mengetik
  `/kafe/app/pos/print/receipt-thermal/<order-id>` sendiri, dan tidak ada test
  yang gagal karenanya.
  **Teramati:** `grep -rn 'print/' renderers/react-shadcn/src/kinds renderers/react-shadcn/src/engine`
  → 0 hasil; `grep -rn 'print' src/shell/` → hanya `router.tsx` (registrasi route).
  Yang terbuka: keputusan bentuk pemicunya — (a) aksi baris/record yang
  menavigasi ke `surfacePath("print", name) + "?" + id`, atau (b) `ui.print:`
  deklaratif pada entity/action (pola yang sama dengan `ui.button_label` pada
  transisi, 5.25.8). Effort: medium (pemicu + test yang **dibuktikan gagal** bila
  cabangnya disuntik-keluar).
- [⏸️] 5.25.10 **Grant per-inbox belum bisa — `navigationFootprint` tidak
  mengenal `approval-inbox:` / `notification-center:`.** Akibatnya supervisor
  tidak bisa diberi hak atas inbox sebagai inbox; ia hanya boleh approve karena
  kebetulan memegang hak `cancel` pada `order` (`seeds/roles.yaml`).
  **Kenapa berbahaya:** `internal/auth/materialize.go:287` meneruskan nama tak
  dikenal ke `navigationFootprint`, yang `default:` membalas error, dan resolver
  **melewati seluruh role tanpa suara** (`continue // malformed grant — skip
role`) — persis insiden yang sudah tercatat di `seeds/roles.yaml:33-37`:
  `supervisor` DAN `manajer` kehilangan **seluruh** grant-nya (semua request 404)
  karena satu nama `approval-inbox:supervisor-inbox`. Jadi menambahkan
  footprint untuk kind ini bukan kemudahan, melainkan menutup jebakan
  fail-open-yang-jadi-fail-closed-total.
  **Catatan:** `print:` **sudah** dikenali (`navigationFootprint` case `print`),
  jadi hanya dua kind ini yang tertinggal.
  **Diperbarui 2026-10-04:** ketergantungan pada "keputusan 5.13.6" **sudah
  tidak ada** — sumbernya kini endpoint `/_ui/workflow/approvals`, bukan entity
  row, sehingga footprint per-inbox bisa dirancang tanpa menunggu apa pun.
  Effort: medium (footprint per-inbox + test penolakan nama
  tak dikenal).

## Fase 6: Auth & Authorization ✅ COMPLETE (inti) — sebagian item ⏸️ deferred (dogfooding — `docs_internal/plan/fase6-dogfooding-auth-module.md`)

**Goal**: Login, JWT, permission model, roles, API keys, sessions, field security. Prod requirement.
**Pendekatan**: `formspec.core` = bundled module YAML (`internal/auth/module/`, embed + loader),
mergeable ke project lain via `external/`/`spec/modules/`; middleware tetap Go.
**Progress**: ✅ Fase A–L selesai (2026-08-20/21). Changelog `2026-08-20-003` s/d `2026-08-21-014`.
**Deferred/partial** (lihat item ⏸️ di bawah): 6.2.5 consent-flow penuh, 6.2.6 ABAC enforcement,
6.3.3 delegation-chain enforcement, 6.3.5 job/audit-log/setting, 6.7.1 classification-tag,
6.7.4 encrypted at-rest, 6.8 store populasi (Config runtime 7.2).

### 6.1 Login & token

- [x] 6.1.1 Login endpoint — `POST /api/v1/auth/login`, credential verification (password hash), JWT issuance (access + refresh) — `POST /{ws}/api/v1/auth/login`; bcrypt verify; access + refresh JWT. Backed by `formspec.core.user`/`session` entities (internal, tanpa route). Dev seed `admin/admin`. Lihat `docs_internal/plan/auth-login-token.md`. ✅ 2026-08-19
- [x] 6.1.2 Token claims — `sub`, `workspace`, `roles`, `permissions`, `exp`, `iat` — access claims: `sub`, `ws`, `roles`, `perms`, `typ=access`, `iat`, `exp`, `iss`, `aud`; refresh claims: `sub`, `ws`, `typ=refresh`, `jti`, `iat`, `exp`. ✅ 2026-08-19
- [x] 6.1.3 Token refresh — rotate (invalidate old, issue new) — `POST /{ws}/api/v1/auth/refresh`; session (jti) di-rotate: hapus jti lama + issue baru; replay token lama → 401. ✅ 2026-08-19
- [x] 6.1.4 Auth per-App via `auth_config_ref` — App me-resolve strategy autentikasi dari yang terpasang (`basic-auth` minimum untuk single-server; `sso` OIDC/SAML, `social-sso`, `passwordless`, `passkey` = set terbuka) (`platform/02-workspace-app-module.md` §3) — `ResolveAppAuth` + `RoleResolver.SetOverride`. ✅ 2026-08-20 (Fase F, changelog 008)

### 6.2 Permission model

- [x] 6.2.1 Resource + action permission — format `{module}.{entity}.{action}` — `ValidatePermissionFormat`/`ParseResourceTarget`/`AutoPrefixPermission`. ✅ (sudah ada, diverifikasi Fase C)
- [x] 6.2.2 Wildcard support — `{module}.{entity}.*`, `*` (super-wildcard), `public` — `Identity.HasPermission` (+ module-level `{module}.*` Fase G). ✅ (sudah ada, diverifikasi Fase C/G)
- [x] 6.2.3 Wire permission check to every API handler — both surfaces — enforcement inti sudah ter-wire di semua route (`RequirePermission` + `RequiredPermission` per route, kedua surface); surface-aware — UI surface entity list/view tanpa izin → 404 (spec §4, tidak bocor keberadaan entity), selain itu → 403. Test `internal/api/permission_enforcement_test.go`. ✅ 2026-08-20 (surface-aware 404)
- [x] 6.2.4 Permission resolution — role → permissions list; cache per session — `PermissionResolver` + cache per-session. ✅ 2026-08-20 (Fase C, changelog 005)
- [⏸️] 6.2.5 Consent footprint — aggregate `required_permission` + `uses` presented to workspace owner at install; cross-module write = high-risk consent — **sebagian**: `formspec check --footprint` (Fase K, changelog 013); alur consent penuh saat install di-defer.
- [⏸️] 6.2.6 Attribute-based authorization — pemeriksaan atribut App/user/membership melengkapi RBAC; pola multi-cabang = `scope_field` pada natural key + atribut membership (mis. kode cabang) (`platform/02-workspace-app-module.md` §3) — **sebagian**: `EvaluateGrantConditions` evaluator (Fase K, changelog 013) **masih inert**; yang **sudah** mendarat di request (2026-10-03, changelog `2026-10-03-011`) adalah **`row_scope` pada grant** — batas baris per (role, action) yang ditegakkan di store untuk baca **dan** tulis, dengan `value` literal sebagai konstanta manifest. Ia menutup aturan bisnis #1 kafe ("hanya pesanan lunas yang masuk dapur" — dulu hanya kolom Kanban) dan GAP-08 (penyaring cabang KDS dulu `fixed_filters` klien), serta mengadopsi bentuk kanonik untuk kualifikasi permission di **satu** tempat (`spec.QualifyPermission`, dipakai store + HTTP + generator + check).
  **Sisa ⏸️:** (c) `ActionGrant.Conditions` tetap inert — pakai `RowScope` untuk baris, dan putuskan terpisah apakah `Conditions` dihidupkan sebagai kendala atribut **tulis** atau dihapus. **Dua sisa sebelumnya sudah ditutup 2026-10-04:** (a) **6.2.6a — bentuk `grants` kini punya gerbang statis.** Satu pembaca bentuk mentah (`auth.ValidateGrantListShape`) dipakai **dua** konsumen: gerbang deploy (`formspec check` **dan** `formspec validate`) dan loader role runtime. Wajib karena pembacaan bertipe bersifat **lossy** — `json.Unmarshal` membuang kunci yang tidak dikenal, jadi `row_scopes:` tetap memberikan permission-nya sambil **menghilangkan batasnya** (fail-open). Kunci yang salah ketik kini **ditolak** oleh resolver (403), bukan disajikan tanpa batas; cacat yang hanya menghilangkan grant tetap sekadar dilaporkan, karena ia sudah fail-closed. (b) **6.2.6b — `resource.find`/`resource.fetch` kini mengoper predikat baris pemanggil.** `scriptRowPredicates` meresolusi batas baris untuk satu baca script dari sumber yang sama dengan HTTP (`row_scope` entity AND batas grant, permission `view`), dan store mendapat `FindByFieldsScoped` sehingga baris di luar batas tidak match. Resolver kedua lapisan **satu** (`appGrantScope.fn = authSvc.GrantRowScope`). Fail closed pada batas yang tak terbaca, atribut sesi yang hilang, dan `from: route` dari script; `SystemCaller` tidak disaring karena itu diputuskan lapisan dispatch, bukan disimpulkan dari identitas yang absen. Bukti: `resource/script_row_scope_e2e_test.go`, `internal/auth/grantshape_test.go`, `internal/auth/grant_scope_failclosed_test.go`, `cmd/formspec/seed_grants_validation_test.go`, `resource/row_scope_grant_e2e_test.go` (2 test penolakan baru); kalibrasi keduanya menghasilkan ulang kebocorannya.

### 6.3 Roles & membership

- [x] 6.3.1 `role` Entity — collection of grants (page → tab → action + conditions) — `formspec.core.role` (internal), tipe `Grant`/`TabGrant`/`ActionGrant`/`ConditionGrant` di `internal/auth/grant.go`; `RoleStore` baca role. ✅ 2026-08-20
- [x] 6.3.2 `app-membership` Entity — populasi user per App + atribut membership (mis. kode cabang) — `formspec.core.app-membership` (user_id, app, attributes, active). ✅ 2026-08-20 (Fase B, changelog 004)
- [⏸️] 6.3.3 Admin delegation chain — workspace owner → app admin → module staff — **sebagian**: 4 owner roles di-seed + di-recognize (Fase G); enforcement rantai delegasi (siapa assign role apa) di-defer. **Catatan 2026-08-26**: permission resolution kini app-scoped (`PermissionResolver.Resolve(ctx, ws, app, user)` — role dengan `app` non-kosong hanya berkontribusi saat app cocok; login membawa `app`; changelog 003). Enforcement rantai delegasi (validasi grant dalam scope caller saat role create/update) masih di-defer.
- [x] 6.3.4 4 symmetric owner roles — Workspace Owner, App Owner, Module Owner, Cloud Owner — `SeedOwnerRoles` + `ownerRolePermission`. ✅ 2026-08-20 (Fase G, changelog 009)
- [⏸️] 6.3.5 `formspec.core` resource set lengkap — `workspace`, `user`, `app-membership`, `role`, `api-key`, `session`, `job`, `audit-log`, `setting` — **sebagian**: user/session/role/api-key/app-membership/workspace ada (bundled module); `job`/`audit-log`/`setting` milik sistem lain (7.13/4.7/7.2).

### 6.4 API keys

- [x] 6.4.1 `api_key` Entity — create (return key once), list (masked), revoke, expiry — `ApiKeyStore`. ✅ 2026-08-20 (Fase B, changelog 004)
- [x] 6.4.2 Scope — per workspace or per app — field `scope` (workspace|app). ✅ 2026-08-20 (Fase B)
- [x] 6.4.3 API key auth middleware — header `X-FormSpec-Key` (`01-core-basic.md` §8.2; surface external TIDAK menerima session cookie) — `AuthMiddleware` resolve key hanya di `/api/v1/`. ✅ 2026-08-20 (Fase B)

### 6.5 Session management

- [x] 6.5.1 `session` Entity — session_id, user, workspace, IP, user-agent, created/expires/last_active — `formspec.core.session`. ✅ (sudah ada)
- [x] 6.5.2 Refresh token rotation — invalidate old, issue new — `POST /auth/refresh`. ✅ (sudah ada)
- [x] 6.5.3 Concurrent session limit — configurable per user — `SetMaxSessionsPerUser` + evict oldest. ✅ 2026-08-20 (Fase D, changelog 006)
- [x] 6.5.4 Global revoke — logout all devices — `LogoutAll`. ✅ 2026-08-20 (Fase D)
- [x] 6.5.5 Session expiry + cleanup job — `PurgeExpired` + `StartSessionCleanup`. ✅ 2026-08-20 (Fase D)
- [x] 6.5.6 Frontend session expiry → login redirect + auto-logout timer — 401 dari API client (entity + meta) menandai session unauthenticated → auth guard redirect ke `{surfacePath}/login?returnTo=...` (bukan error); boot `fetchMe` membedakan 401 (→ login) dari network error; auto-logout idle timer configurable (`prefs.sessionTimeoutMinutes`, default 30, 0=never) di-set dari LoginScreen, hook `useAutoLogout` di `SurfaceShell`. Changelog `2026-08-22-001`. ✅ 2026-08-22
- [x] 6.5.7 Refresh token flow (frontend) + fix session workspace (backend) — frontend simpan `refreshToken`, auto-refresh access token saat 401 (`authHooks` shared: 401 → `ky.retry()` → `beforeRetry` refresh single-flight → retry); `refreshSession()` di session store; login meneruskan refresh token. Backend fix: `sessWorkspaceForJTI` hardcode "demo" → thread workspace melalui `SessionStore` (`Get`/`Delete`/`DeleteForUser`/`CountForUser`/`ListForUser`), `Refresh` pakai `claims.Workspace`. Changelog `2026-08-22-002`. ✅ 2026-08-22
- [x] 6.5.8 Session persistence ke sessionStorage — access + refresh token dipersist ke `sessionStorage` (key `formspec-session`) sehingga browser refresh (F5) me-restore session tanpa login ulang; `boot` restore saat workspace cocok, `setSession`/`refreshSession` tulis, `clearSession`/`expireSession` clear. Changelog `2026-08-22-003`. ✅ 2026-08-22
- [x] 6.5.9 **Layar pemilih konteks sesi — SEBAGIAN BESAR SUDAH LANDING tanpa item ini ditutup.** ✅ **Dikoreksi & ditutup 2026-09-26.** Teks item menyatakan tiga hal belum ada; **dua di antaranya sudah ada** sejak changelog `2026-09-25-010` dan tidak pernah tercatat di sini — kelas misinformasi yang sama dengan 4.2.4–4.2.6. Verifikasi per bagian:

  | Bagian                            | Status (terverifikasi)                                                                                                                                                                                                |
  | --------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
  | layar pemilih konteks             | **ADA** — `shell/ContextPicker.tsx` (radiogroup, label `role · value`), 11 test di `ContextPicker.test.tsx`                                                                                                           |
  | penanganan 409 `CONTEXT_REQUIRED` | **ADA** — `LoginScreen.tsx:330` merender picker dari `choices`; `AuthFormRenderer` ikut; `stores/session.ts` menyimpan `pendingContext` sehingga refresh 409 tidak meng-expire sesi                                   |
  | persistensi pilihan terakhir      | **ADA** — `lib/session-context.ts`, `localStorage formspec-context:<ws>:<app>`, prefill **tanpa** auto-submit (boundary tetap dinyatakan pemanggil, jawaban audit tetap sah)                                          |
  | **pengalih di header**            | **BELUM** — `grep` di ketiga shell (`SideNav/TopNav/NoNav`) tidak menemukan pengalih konteks; yang ada hanya `ThemeSwitcher`. `SwitchContextScreen` (`App.tsx:436`) hidup sebagai **layar**, bukan pengalih di header |

  Jadi yang benar-benar tersisa adalah **satu** bagian, bukan tiga. Tidak ada aksi pada tiga yang pertama. Sisa pengalih header → **6.5.10 ⏸️** di bawah.

- [⏸️] 6.5.10 **Pengalih konteks di header belum ada (sisa 6.5.9).** Konteks sesi saat ini hanya bisa dipilih saat **login** (`ContextPicker` di `LoginScreen`) atau lewat **layar** `SwitchContextScreen` (`App.tsx:436`) — tidak ada kontrol di chrome untuk berganti role/cabang **tanpa** meninggalkan halaman, yang merupakan inti janji "role × cabang" (kasir pindah cabang di tengah shift). **Teramati:** `grep -rn "ContextPicker\|switch" src/shell/{TopNavShell,SideNavShell,NoNavShell}.tsx` → hanya `ThemeSwitcher`; `SwitchContextScreen` dirender sebagai route, bukan di dalam shell. Effort: medium (kontrol di chrome + pemanggilan `POST /_ui/auth/switch` + refetch bundle; chrome composition sudah punya preseden `ThemeSwitcher` dan `AuthArea`).

### 6.6 Auth middleware pipeline

- [x] 6.6.1 Auth method detection — Bearer JWT vs `X-FormSpec-Key` API key vs session cookie (session cookie hanya surface `/_ui`) — `AuthMiddleware` (JWT + API key; cookie belum ada mekanisme). ✅ 2026-08-20 (Fase E, changelog 007)
- [⏸️] 6.6.5 **Auth via cookie untuk surface `/_ui` belum ada** — `AuthMiddleware` menerima JWT + API key saja; cookie (yang lebih tahan XSS untuk sesi browser) hanya disebut sebagai "belum ada mekanisme" di teks 6.6.1. Effort: medium (cookie issue/refresh + CSRF + aturan SameSite per surface).
- [x] 6.6.2 Token validation → identity extraction → permission loading → workspace context — pipeline di `AuthMiddleware`. ✅ 2026-08-20 (Fase E)
- [x] 6.6.3 Rate limiting per auth method — token bucket per IP (login/refresh). ✅ 2026-08-20 (Fase E)
- [x] 6.6.4 Audit log every auth attempt (success + failure) — `authAudit`. ✅ 2026-08-20 (Fase E)

### 6.7 Field-level security

- [⏸️] 6.7.1 `classification` label — tag field `pii|financial|internal`; log/export auto-tag — struct ada (1.4.3); tagging di log/export di-defer (butuh kebijakan log/export terpusat).
- [x] 6.7.2 `required_permission` (field-level) — user without permission → field excluded from response — `sanitizeData`. ✅ 2026-08-20 (Fase H, changelog 010)
- [x] 6.7.3 `exclude` — per-surface field exclusion (`public_api`, `audit_log`, `webhook`, `ui` — `05-field-types.md` §5.3) — `sanitizeData` (public_api vs ui). ✅ 2026-08-20 (Fase H)
- [⏸️] 6.7.4 `encrypted: true` — AES-256-GCM at-rest encryption for field — struct ada (1.4.6); enforcement di-defer (butuh master key/keystore).
- [x] 6.7.5 `masked: true` — auto-mask in JSON response and structured log (`***`) — `sanitizeData`/`maskValue`. ✅ 2026-08-20 (Fase H)
- [x] 6.7.6 `computed` — server-derived, never client-writable; recompute on every create/update — `evaluateComputed`. ✅ (sudah ada)

### 6.8 `ctx.secrets`

- [x] 6.8.1 `ctx.secrets.get("key")` — only path for `secret: true` Config keys — `secretsAPI`; store **sudah** dipopulasi dari Config (`SetSecretsStore(cfgReg.Secrets())`, `resource/formspec.go:1851`) sejak 7.2 landing — kalimat "menunggu Config runtime 7.2" dikoreksi 2026-09-22. ✅ 2026-08-21 (Fase I, changelog 011)
- [x] 6.8.2 `uses: { secrets: [key, ...] }` — must declare access; undeclared → blocked — `declaredUsesSecrets`. ✅ 2026-08-21 (Fase I)
- [x] 6.8.3 Secret never appears in logs at any level — `secretsAPI` tidak log nilai. ✅ 2026-08-21 (Fase I)
- [x] 6.8.4 Every secret read audited — who read what secret, when — `SecretsAudit` hook. ✅ 2026-08-21 (Fase I)

### 6.9 RichText sanitization

- [x] 6.9.1 Server-side HTML sanitize — strip script/markup before persist; client HTML never trusted raw — `sanitizeRichText`/`sanitizeHTML` di Insert/Update. ✅ 2026-08-21 (Fase J, changelog 012)

---

## Fase 7: Engine Extended — Missing Kind Runtimes

**Goal**: Service, Config, Subscription, Workflow, Webhook, Integrator, Hook engine, Validation L4–L6, State machine, Denormalisasi, Period closing, Rate limiter, Async job.  
**Progress (2026-08-27 audit)**: 7.1 ✅ · 7.2 ✅ · 7.3 ✅ · 7.4 ✅ · 7.5 ✅ · 7.6 ✅ · 7.7 ✅ · 7.8 ✅ · 7.9 (hanya 7.9.6; L4–L6 ⏸️ butuh keputusan desain kontrak deklarasi) · 7.10 ✅ · 7.11 ✅ · 7.12 ✅ · 7.13 ✅ · 7.14 ✅ · 7.15 sebagian (spawn+protocol ✅, `spec.runtime` ⬜) · 7.16 ✅ · 7.17 (7.17.1–7.17.2 ✅, transform ⬜) · 7.18 ⬜ · 7.19 ⬜

### 7.1 `kind: Service` runtime

- [x] 7.1.1 Service registry — register by `{module}.{name}` — ✅ 2026-08-25 (changelog 005)
- [x] 7.1.2 Resolve `impl.native` — scan `impl/**/*.go`, `ref: "{Type}.{Method}"`, must be unique in module — ✅ 2026-08-25 (changelog 005)
- [x] 7.1.3 Resolve `impl.script` / `impl.script_ref` / `impl.compiled` / `impl.sidecar` — permission enforcement seragam untuk KELIMA jenis impl (`01-core-basic.md` §5) — ✅ 2026-08-25 (changelog 005)
- [x] 7.1.4 `call: async` — fire-and-forget (no job_id, no progress, no result) — goroutine + 202 tanpa job_id, distinct dari tracked async (7.13). ✅ 2026-08-25 (changelog 021)

### 7.2 `kind: Config` runtime

- [x] 7.2.1 Config registry — load Config manifests, resolve per environment — ✅ 2026-08-25 (changelog 003)
- [x] 7.2.2 `ctx.config.get("key")` — Starlark access; non-secret keys only — ✅ 2026-08-25 (changelog 003)
- [x] 7.2.3 `settings.*` namespace — global settings: currency, locale, timezone, date_format, fiscal_year_start — runtime-editable via entity `app-setting`; merge ke `/meta/ui` bundle. ✅ 2026-08-24 (changelog 008/010) + 2026-08-25 (changelog 003)
- [x] 7.2.4 Global settings defaults — spec MUST provide acceptable defaults for every setting; components MUST NOT guess — seed default dari manifest saat find-or-create. ✅ 2026-08-24 (changelog 008/012)

### 7.3 `kind: Subscription` engine

- [x] 7.3.1 Tier 1 (outbox) — event → match Subscription → call handler; transactional — ✅ 2026-08-25 (changelog 011)
- [x] 7.3.2 Tier 2 (streaming) — Redis/Kafka; at-least-once, positioned replay, filter/transform Starlark — ✅ 2026-08-25 (changelog 019)
- [x] 7.3.3 `emits:` custom event emission — action declares `emits: <event-name>` → event emitted on action success — ✅ 2026-08-25 (changelog 011)
- [x] 7.3.4 Dynamic subscriptions — runtime-created subscriptions as data (not manifest); live in `formspec.core` — ✅ 2026-08-25 (changelog 020)
- [x] 7.3.5 Delivery channels — `pubsub` (non-durable, at-most-once) **✅ 2026-08-25 (changelog 002 + 018)**; `queue` **✅ 2026-10-05** (changelog `2026-10-05-006`); `webhook` (keluar) + `notification` **✅ 2026-10-06** (changelog `2026-10-06-003`) → **semua kanal yang dideklarasikan terkirim**, `unsupportedChannels` di `pkg/spec/delivery_channels.go` **kosong**. **Sisa yang tetap terbuka dipindah ke item bernomor:** signature `webhook` (butuh secret per-subscriber) → **7.7.7 ⏸️**; blok `delivery:` Tier-2 pada `kind: Subscription` yang masih inert → **7.7.8 ⏸️**. Sepanjang masa transisi itu keadaan tak-terkirim **tidak pernah senyap**: sejak **2026-10-05** validator memperingatkan dan runtime menggagalkannya (changelog `2026-10-05-005`).
- [x] 7.3.6 ✅ **2026-10-05** **Subscription: tiga vertical dimigrasikan ke bentuk sah + cek `events:` resolve.** `verticals/notifications`, `sales-gl-integrator`, `sales-inventory-integrator` memakai bentuk lama `on: {resource, event}` + `deliver: [{channel: queue, job: ...}]` — **bukan** field `SubscriptionSpec` (`events:` + `handler:`; schema `additionalProperties: false`), jadi ketiganya **gagal validasi sejak sebelum sesi ini** (terukur: 1 problem masing-masing; kini 0). Bentuk itu juga salah konsep: `deliver:` milik PUBLISHER, bukan consumer; handler Subscription sudah berjalan di worker outbox, jadi ia sendiri eksekusi latarnya. Sekalian: **cek baru** `validateSubscriptionEvents` (`cmd/formspec/validate_subscriptions.go`) — sebelumnya tidak ada yang memeriksa nama event di `events:`, jadi typo/rename menghasilkan subscription yang **tidak pernah menyala** dengan validate hijau (kelas yang sama dengan target `deliver`/`job:`). Dua penjaga false-positive: event reserved life-cycle (`before_*`/`on_*` untuk 8 reserved action) **implied** per Entity tanpa deklarasi, dan entity di luar tree **di-skip**. **Blast radius diukur: 0 false positive di 10 tree.** Changelog `2026-10-05-007`.

### 7.4 Approval engine (dulu `kind: Workflow`)

- [x] 7.4.1 Approval state machine — attach to Entity transition without modifying Entity — ✅ 2026-08-25 (changelog 012)
- [x] 7.4.2 Multi-approver modes — `all` (all eligible must approve), `any` (quorum from pool), `sequential` (chain order) — ✅ 2026-08-25 (changelog 012)
- [x] 7.4.3 `when` condition — FormSpecExpr on `resource`; step skipped if false — ✅ 2026-08-25 (changelog 012)
- [x] 7.4.4 `escalation` — timeout (`after`) + takeover. **Diperbarui 2026-10-05:** `reassign` adalah DUTY (permission, di-qualify `workflow.{module}.{entity}.{transition}.{reassign}`), `notify_roles`/`reassign_roles` dihapus — changelog `2026-10-05-002`. ✅ 2026-08-25 (changelog 015)
- [x] 7.4.5 Requester can never approve own request — ✅ 2026-08-25 (changelog 012)
- [x] 7.4.6 Approval = signed statement recorded in audit trail — ✅ 2026-08-25 (changelog 014)
- [x] 7.4.7 Test level-API untuk interception approval — ✅ 2026-09-22 (`internal/api/workflow_approval_api_test.go`). Harness baru menyambungkan dependensi yang sama seperti produksi (entity dengan state machine `posted→voided` via `void-order` + workflow registry satu step role `supervisor` + `WorkflowApprovalStore` + `specLookup`) lalu mendorong record lewat HTTP sungguhan (`HandleCustomAction`, route `/{id}/{action}`): **5 test** — (1) interrupt → **202** `approval_required` + record **tidak** pindah state; (2) role salah → **403**; (3) requester meng-approve request sendiri → **403**; (4) approver ber-role → **200** + record pindah; (5) reject → 200 `rejected` + state tidak berubah; plus kontrol tanpa workflow → **200** (dispatch langsung, bukan 202). **Harness ini langsung menemukan bug nyata:** `handleWorkflowApproval` membaca `resourceData["created_by"]`, padahal `created_by` adalah **kolom framework** (`EntityRecord.CreatedBy`) yang hanya diproyeksikan ke wire oleh `MarshalJSON` — tidak pernah ada di map `Data`. Akibatnya `RequesterID` **selalu kosong**, sehingga jaminan **7.4.5** ("requester can never approve their own request") **tidak pernah menendang di jalur nyata**: requester yang punya role bisa menyetujui requestnya sendiri (terbukti: 200 `transition_completed` sebelum perbaikan; 403 sesudah). Perbaikan: helper `HandlerFactory.requesterIDFor` — baca `Data` dulu (hormati entity yang benar-benar mendeklarasikan field `created_by`), fallback ke `store.GetByID().CreatedBy`. **Bukti**: `TestWorkflowApproval_RequesterCannotSelfApprove_Regression` gagal sebelum / hijau sesudah; `go test ./...` hijau; `go vet ./...` bersih.
- [x] 7.4.8 ✅ **2026-09-28** **Interception approval kini punya test lewat jalur
      `PATCH` (transisi tanpa `impl`), sekaligus penegakan kontrak inputnya.**
      Ditutup saat mengeksekusi `docs_internal/plan/action-input-contract.md`
      (changelog `2026-09-28-004`), karena jalur PATCH justru harus dibuka untuk
      kontrak input — dua hal yang tidak bisa dikerjakan terpisah. **Terukur
      (`internal/api/approval_input_test.go`, `transition_contract_test.go`):**
      PATCH transisi approval-gated → **202 tanpa write** (status tetap `posted`),
      approver `{"decision":"approve"}` → **200**, order `voided` **dan**
      `void_reason` tersimpan atomik; `decision` tidak ikut tersimpan sebagai
      field; baris `formspec_workflow_approval` memuat input pemohon. Alasan
      aslinya dicatat di bawah untuk jejak.
      <details><summary>alasan awal</summary>Kafe TODO 1.7 mencatatnya sebagai "**Sisa (bukan bagian 1.7)**": tidak ada test level-API karena harness auth+seed belum ada. 7.4.7 (2026-09-22) **menutup sebagian** — harness itu kini ada, tetapi ia menguji jalur `HandleCustomAction` (`/{id}/{action}`), sedangkan jalur yang dipakai **produksi untuk transisi tanpa `impl`** adalah `PATCH …/{id}` `{"status": …}` (kontrak 2.7; di kafe, `void-order` memang tanpa `impl`). **Teramati:** justru di jalur itulah bug bypass approval 2026-09-21 hidup (kafe TODO 9.4 skenario 6 — void langsung `cancelled` tanpa approval); fix-nya berjalan dan dikunci `TestFindTransitionByStates`, tetapi **tidak ada test HTTP** yang mengunci perilaku PATCH → `202 approval_required`, jadi regresi di jalur itu akan lolos tanpa ada yang gagal. Effort: small (dua case PATCH ditambahkan ke harness 7.4.7).</details>
- [x] 7.4.9 ✅ **2026-10-05** **Nama runtime menyusul kontrak: paket `internal/approval`, tipe `ApprovalStep`/`ApprovalReject`.** Approval sudah bukan kind sejak 2026-10-05-001, tetapi nama runtime masih "workflow" — terbaca seolah `kind: Workflow` masih ada. **Yang di-rename (Go-side):** paket `internal/workflow` → `internal/approval`; `spec.WorkflowStep` → `ApprovalStep` (dan `WorkflowStepMode`, `IsWorkflowStepMode`, `workflowStepModes`, `validateWorkflowStepMode`, `validWorkflowStepName`); `spec.WorkflowReject` → `ApprovalReject`; `db.WorkflowApprovalStore`/`Row` → `ApprovalRequestStore`/`Row` (fungsi `NewApprovalRequestStore`); `Registry.WorkflowsFor` → `ApprovalsFor`, `WorkflowInfo` → `ApprovalInfo`; `SetWorkflowRegistry`/`SetWorkflowApprovalStore`/`SetWorkflowDuties` → `SetApprovalRegistry`/`SetApprovalRequestStore`/`SetApprovalDuties`; `handleWorkflowApproval` → `handleApproval`, `executeWorkflowTransition` → `executeApprovalTransition`, `recordWorkflowAudit` → `recordApprovalAudit`; `WorkflowName`/`WorkflowModule` → `GateName`/`GateModule`; file `workflow_inbox.go`/`workflow_approval.go`/`validate_workflow.go` → `approval_*`; nama test `TestWorkflowApproval_*`/`TestValidateWorkflows_*`/`TestMaterialize_Workflow*` → `TestApproval_*`/`TestValidateApprovals_*`/`TestMaterialize_Approval*`. **Ikut ditemukan & dibuang:** entri kind `"Workflow"` yang masih tersisa di `internal/genkinddocs/markdown.go` (grup data) dan union `ResourceKind` di `renderers/react-shadcn/src/types/manifest.ts` — dua sisa dari penghapusan kind yang lolos karena keduanya bukan `KnownKinds`. **Sengaja TIDAK di-rename (kontrak persisten, dinyatakan di kode + `06-ui-rest-contract.md` §5):** tabel `formspec_workflow_approval` + kolom `workflow_module`/`workflow_name` (butuh migrasi DB), route `/_ui/workflow/approvals`, dan field wire `workflow`/`workflow_module` (kompatibilitas klien). Go field-nya sudah bernama `Gate`/`GateModule` dengan JSON tag lama. **Bukti:** `go build ./...` + `go vet ./...` bersih · `go test ./...` hijau · `npx tsc -b` bersih · vitest `approval-inbox` + `approvalInbox` **10/10** · `formspec validate` kafe **88/0**, crc **33/0**, service-demo **13/0** · `grep -rn "WorkflowStep\|WorkflowReject\|WorkflowApproval" --include='*.go'` → hanya komentar. Changelog `2026-10-05-003`.

### 7.5 State machine engine (basic)

- [x] 7.5.1 Transition validation — declared transitions only; undeclared → `STATE_TRANSITION_ERROR` — ✅ 2026-08-25 (changelog 004)
- [x] 7.5.2 Starlark inline guards — guard on transition — ✅ 2026-08-25 (changelog 004)
- [x] 7.5.3 Builtin aggregates — `sum_line(field)`, `len(resource.items)` for guard expressions — ✅ 2026-08-25 (changelog 004)
- [x] 7.5.4 Satukan dua implementasi state machine — guard terpadu di `internal/starlark/guard.go`, dipakai `StateMachineEngine` + `EntityStore.validateStateTransition`. ✅ 2026-08-25 (changelog 004)

### 7.6 `kind: Webhook` engine

- [x] 7.6.1 Inbound endpoint — route registration, method validation — ✅ 2026-08-25 (changelog 010)
- [x] 7.6.2 Signature verification — HMAC (strategy: `signature`, algorithm, header, payload) before handler — ✅ 2026-08-25 (changelog 010)
- [x] 7.6.3 Token auth — strategy: `token` — ✅ 2026-08-25 (changelog 010)
- [x] 7.6.4 Verification failure → rejected BEFORE handler runs — ✅ 2026-08-25 (changelog 010)

### 7.7 `kind: Integrator` engine

- [x] 7.7.1 Listen → call bridge — `listen.resource`+`event` triggers `call.resource`+`action` — ✅ 2026-08-25 (changelog 013)
- [x] 7.7.2 Mandatory symmetric cancel handler — every Integrator MUST provide cancel handler — validasi apply. ✅ 2026-08-25 (changelog 016)
- [x] 7.7.3 Target action must be `idempotent: true` for cross-boundary calls — ✅ 2026-08-25 (changelog 016)
- [x] 7.7.4 Saga compensate — cross-boundary call registers `compensate` to Saga log; `FORMSPEC.SAGA.*` errors — ✅ 2026-08-25 (changelog 017)
- [x] 7.7.5 ✅ **2026-10-05** **Idempotency consequence `deliver: target` akhirnya ditegakkan.** `deliver` yang menargetkan action (channel `reliable_event`) mendeklarasikan `idempotency_key: "balance.{id}"`, tetapi enforcement-nya hanya hidup di jalur HTTP — `resolveIdempotencyKey` membaca request, dan outbox worker tidak punya request; yang belum terpasang justru langkah **"cek idempotency"** yang sudah normatif di `01-core-basic.md` §7. **Terbukti bukan teoretis:** me-requeue satu event `journal-posted` yang sama (persis yang dilakukan retry) menjalankan `gl-balance.update` dua kali — `debit_movement` 143750 → **287500**. **Perbaikan:** (1) `db.ActionDispatch` kini menerima **entry utuh** (`spec.EventDeliveryDecl`, bukan `*spec.DeliveryTarget`) supaya `idempotency_key` sampai ke dispatcher — sebelumnya dibuang di `DeliveryEventHandler`; (2) `runTargetOnce` menjaga panggilan: key **completed → consequence DILEWATI** (replay bagi konsekuensi = tidak melakukan apa-apa), key **pending/failed → dijalankan** (at-least-once, gagal harus bisa di-retry), key dideklarasikan tanpa store → **gagal** (bukan jalan tanpa perlindungan), template tak bisa di-resolve → **gagal** (literal `balance.{id}` akan memberi semua event tanpa `id` kunci yang SAMA → delivery kedua dilewati sebagai "selesai", konsekuensi hilang); (3) scope `deliver:{resource}.{action}` di tabel `formspec_idempotency_keys` yang sama dengan kunci HTTP; (4) store dibuat **sebelum** delivery handler di `resource.New()` dan diteruskan di jalur reload (`a.idempotency`). **Batas dinyatakan:** kunci diklaim `pending` sebelum target dipanggil dan ditandai `completed` sesudahnya, jadi crash di antara keduanya masih bisa mengulang sekali — hanya idempotensi alami target yang menutupnya; ditulis di `01-core-basic.md` §7. **Bukti:** `TestRunTargetOnce_RetryAfterSuccessDoesNotReRun` **gagal sebelum / hijau sesudah** (dengan skip dihapus: consequence jalan **3×**; dengan skip: **1×**) · 6 test guard + 2 test plumbing (`resource/deliver_idempotency_test.go`, `renderers/jsonb-persist/event_handler_test.go`) · `go test ./...` hijau · kafe 88/0 · crc 33/0 · service-demo 13/0. Changelog `2026-10-05-004`.
- [x] 7.7.6 ✅ **2026-10-06** **`notification` + `webhook` (keluar) akhirnya terkirim — tiga increment tuntas.** ***Increment 3 (hari ini):** (a) **`notification`** — module resmi `internal/notify` (pola `//go:embed module` + `RegisterCoreEntities`, seperti `internal/auth`/`internal/period`) menyediakan entity `formspec.core.notification` dengan `row_scope {field: recipient_id, from: session, attr: user_id}`, jadi barisnya **hanya terbaca pemiliknya** dan pemanggil tanpa identitas **fail-closed**; `deliver: {channel: notification, notification: {recipient, title, body, level}}` menulis barisnya lewat `EntityStoreWriter` (`SystemCaller: true`), `recipient` **wajib dan harus resolve** — baris tanpa penerima adalah baris yang tidak bisa dibaca siapa pun, jadi ditolak, bukan ditulis tak terlihat; `handler:` opsional menamai Service action untuk kanal luar, kosa kata referensi **sama** dengan `job:`. **Menutup dua hal sekaligus:** `kind: NotificationCenter` selama ini mencari `formspec.core.notification` yang tidak ada — kelas kegagalan yang sama dengan `ApprovalInbox` sebelum 5.13.6. (b) **`webhook` (keluar)** — `internal/webhookout` + `WebhookDeliveryDecl{URL, URLFrom, Headers}`; tepat **satu** dari `url:` / `url_from: {config:}` (config di-resolve saat delivery), `headers:` mendukung template `{dotted.path}`; non-2xx = **error** (retry → dead-letter), bukan sukses. **UNSIGNED dan dinyatakan begitu** — lihat **7.7.7 ⏸️**. (c) `spec.EventChannel` bertambah `notification`+`webhook` (tujuh kanal); `renderers/jsonb-persist/event_handler.go` mendapat kedua cabang dan `default:` mengembalikan **error** yang menyebut nama kanalnya; `pkg/spec/delivery_channels.go` `unsupportedChannels` kini **kosong** (mekanismenya tetap ada untuk kanal berikutnya); `sharedTypes` di `internal/genjsonschema` bertambah `NotificationDecl`/`WebhookDeliveryDecl`/`WebhookURLRef`. **Bug yang ditemukan test suite:** `interpolate`/`interpolateHeaders` menulis ulang token yang tidak resolve dengan potongan **no-op** (`out[:end+1] + out[end+1:]`), sehingga `{` yang sama ditemukan selamanya — `go test ./internal/notify/` **menggantung >180 detik**; diganti akumulasi berbasis kursor + regresi `TestInterpolateHeaders_UnresolvedTokenTerminates`. **Bukti:** `go test ./...` **41 ok / 0 FAIL**, `go vet` + `gofmt` bersih, test baru 10 (`internal/notify`) + 8 (`internal/webhookout`); validate kafe 88/0 · crc 33/0 · service-demo 13/0 · arisan 17/0 · verticals/notifications 3/0 (billing 35/20 = **pre-existing**, nol baru). Changelog `2026-10-06-003`. Dokumen: `docs/spec/backend/02-core-extended.md` §3 (tabel status kanal kini **semua terkirim** + batas unsigned) · `docs/kind/ui/NotificationCenter.md`. **Sisa dipisah ke item bernomor:** signature `webhook` → **7.7.7 ⏸️**; blok `delivery:` Tier-2 yang masih inert → **7.7.8 ⏸️**.
- [⏸️] 7.7.7 **`webhook` keluar belum ditandatangani (UNSIGNED).** Yang belum: HMAC signature + **penyimpanan secret per-subscriber**, plus registry subscriber kalau endpoint memang harus per-langganan, bukan per-manifest. Sekarang endpoint hanya dari `url:`/`url_from: {config:}` dan penerima tidak punya cara memverifikasi asal. **Kenapa sengaja tidak dipaksakan:** mengumumkan signature yang runtime belum bisa hasilkan adalah promise yang `02-core-extended.md` §3 justru terus tolak — lebih baik batasnya dinyatakan apa adanya di docs daripada hijau di validator. **Teramati:** `internal/webhookout` tidak mengirim header signature apa pun; tidak ada field manifest untuk secret. **Selama ini:** webhook keluar hanya untuk penerima yang memang tidak menuntut verifikasi. Effort: medium.
- [⏸️] 7.7.8 **Blok `delivery:` Tier-2 pada `kind: Subscription` masih inert.** Field-nya (`channel`, `retry`, `dead_letter`, `store`, `retention`, `position`, `max_retry`, `filter`, `transform`) **tidak dibaca runtime sama sekali** — 0 pembaca `.Delivery`. Sejak `2026-10-05-005` validator **memperingatkan** untuk blok ini, dan sekarang satu-satunya baris ⏸️ di tabel status kanal. **Teramati:** deklarasi `subscriptions[].delivery.channel: webhook` tidak mengubah perilaku apa pun dibanding tanpa blok itu (event tetap tidak dikirim ke sana). **Yang dibutuhkan lebih dulu adalah keputusan, bukan kode:** apakah Tier-2 Subscription penuh akan diimplementasikan, atau bloknya **dipensiunkan dari skema** dan deklarasi kanal dipindahkan seluruhnya ke `events[].deliver[]` (yang sudah bekerja). Effort: medium (implementasi) / small (pensiun dari skema).


### 7.8 Hook engine

- [x] 7.8.1 5 hook points — `before`, `after`, `on_error`, `before_deliver`, `after_deliver` — ✅ 2026-08-25 (changelog 002)
- [x] 7.8.2 `before` — may modify action params or call `fail()` to abort — ✅ 2026-08-25 (changelog 002)
- [x] 7.8.3 `after` — post-action side effects — ✅ 2026-08-25 (changelog 002)
- [x] 7.8.4 `on_error` — compensation/cleanup — ✅ 2026-08-25 (changelog 002)
- [x] 7.8.5 `before_deliver` — may suppress delivery or enrich payload — `internal/action/deliver.go`. ✅ 2026-08-25 (changelog 002)
- [x] 7.8.6 Priority ordering — consistent with event priority (smaller first, kelipatan 10) — ✅ 2026-08-25 (changelog 002)
- [x] 7.8.7 Cross-module hooks — must declare `uses`; appear in consent footprint — ✅ 2026-08-25 (changelog 002)
- [x] 7.8.8 **`resource.create` dari script kini menjalankan hook `after create` entitas target** — ditemukan saat mengerjakan kafe 10.1/10.2 (changelog `2026-09-22-012`). Sebelumnya jalur itu berhenti di `EntityStore.Insert`: barisnya masuk, tetapi hook `after create` milik entitas TARGET tidak pernah dijalankan (jalur HTTP menjalankannya, jalur script tidak). Akibat konkretnya di kafe: `receive-goods` membuat `stock-movement` dari script, dan `stock-level` — proyeksi yang dipelihara hook `after create` pada `stock-movement` — **tidak pernah ter-update**. Baris pergerakan ada (`qty=10000 unit_cost=40`), saldonya tidak (`stock-level` absen), dan tidak ada error di mana pun. Fix: jalur `SetCreateHandler` (`resource/formspec.go`) memanggil `action.RunAfterPhase` setelah insert, dengan guard `len(Hooks) > 0` supaya entitas tanpa hook tidak berubah perilakunya. **Test pengunci:** `TestKafe_ReceiveGoodsMaintainsStockLevel` (`resource/script_hooks_e2e_test.go`) — dibuktikan **gagal** saat pemanggilan hook dinonaktifkan (`stock-level row was NOT created`), hijau setelahnya. **Sisa:** `resource.save`/`update` dari script belum diperiksa untuk kelas yang sama.
- [x] 7.8.10 **`emit:` pada transisi tanpa `emits:` pada action = event tidak pernah dikirim (SENYAP).** Ditemukan 2026-09-22 saat kafe 10.7 (changelog `2026-09-22-014`). State machine membaca benar (`emit: on_po_received` pada transisi `submitted → received`), `formspec validate` **hijau**, dan subscription menunggu event yang **tidak pernah datang** — penerimaan barang tidak menghasilkan akuntansi apa pun, tanpa satu pun error atau log. Deklarasi `emits:` pada action-lah yang meneruskan event ke outbox. **Terukur:** tanpa `emits:` → outbox tidak punya baris `on_po_received`; dengan → baris masuk dan jurnal lahir. **Test pengunci:** `TestKafe_PurchaseReceivedCreatesJournal` (timeout saat `emits:` dihapus). **Sisa:** `formspec validate` belum menolak pasangan ini — lihat 7.8.11 ⏸️.
- [⏸️] 7.8.11 **Validator belum memeriksa konsistensi `emit:` (transisi) ↔ `emits:` (action).** ⚠️ **Premis item ini DIKOREKSI 2026-09-26 oleh pembacaan kode: "`emit:` tanpa `emits:` = event tidak pernah dikirim" TIDAK selalu benar — ia bergantung pada jalur HTTP yang dipakai.** Ada **tiga** jalur, dan mereka tidak sepakat:

  | Jalur                         | Sumber event                                                                              | Perlu `emits:`?           |
  | ----------------------------- | ----------------------------------------------------------------------------------------- | ------------------------- |
  | `PATCH /{id}` `{"status": …}` | `ResolveTransitionEmission` (`handler.go:1068`, `:1111`)                                  | **tidak**                 |
  | `POST /{id}/{action}`         | `ResolveEmission(actionSpec.Emits)` (`handler.go:2181`) — transisi **tidak** dikonsultasi | **ya**                    |
  | script `resource.save`        | transisi bila state berubah, jika tidak `emits:` (if/else, `formspec.go:2283`)            | tidak, bila state berubah |

  **Konsekuensi konkret pada kafe (terukur dari manifest):** `order.confirm-payment` **tidak** punya `emits:`, sedangkan transisi `awaiting_payment → paid` punya `emit: on_paid`. Jadi perintah yang sama — "konfirmasi pembayaran" — **menerbitkan `on_paid` lewat PATCH** tetapi **tidak menerbitkan apa pun lewat `POST /{id}/confirm-payment`**, dan tidak ada yang memberitahukannya. Itu asimetri nyata, bukan "manifest kurang `emits:`": menambahkan `emits: on_paid` akan membuat jalur action bekerja, tetapi jalur PATCH tetap tidak membacanya, jadi keduanya masih tidak sepakat bila nilainya berbeda. `cafe-stock` tidak terkena karena `receive-goods`/`cancel-po` mendeklarasikan `emits:` **dan** transisinya `emit:` dengan nama sama (publish tunggal di ketiga jalur).

  **Yang perlu diputuskan (sekarang lebih sempit):** pilih **satu** model, lalu buat ketiga jalur menaatinya. (a) `emit:` adalah otoritas dan jalur action juga harus membacanya (lalu `emits:` menjadi opsional/redundan — perlu aturan bila keduanya berbeda); (b) `emits:` adalah otoritas dan transisi hanya menyatakan _kenapa_, dengan PATCH menyintesis action spec dari `via:`; atau (c) keduanya wajib identik — inilah yang paling dekat dengan item ini semula, tetapi **tidak bisa** menjadi error polos karena kafe sendiri melanggarnya di jalur PATCH yang sudah jalan. Jadi pemeriksaan statis yang benar bukan "`emit:` tanpa `emits:` → error" melainkan "`via:` yang transisinya ber-`emit:` dan action-nya ber-`emits:` **berbeda nama** → error", plus **perbandingan perilaku per-jalur** agar (a)/(b)/(c) tidak berbeda diam-diam. **Bukti:** `grep -n 'emits:' examples/kafe/spec/modules/cafe-order/transaction/order/entity.yaml` → **0 hit**, padahal tiga transisi mendeklarasikan `emit:` (`on_paid`, `on_cancel`×2). Effort: medium (satu keputusan kontrak + penyelarasan tiga jalur + test per-jalur; bukan "satu cabang di `ValidateTransitionEmits`").

- [x] 7.8.12 **Script `.star` adalah unit kompilasi sendiri — fungsi dari file lain gagal di RUNTIME, dan `validate` tidak bisa melihatnya.** Ditemukan 2026-09-22 saat kafe 10.7 (changelog `2026-09-22-014`). `journalize_purchase.star` memanggil `journalize_sale` yang didefinisikan di `journalize.star` → outbox worker mencatat `undefined: journalize_sale` dan meng-retry 5×, sementara `formspec validate` **hijau** (ia mengompilasi tiap file terpisah). **Diperbaiki** dengan membuat script itu mandiri. **Sisa:** tidak ada mekanisme berbagi fungsi lintas-script — lihat 7.8.13 ⏸️.
- [x] 7.8.14 **Menu App bocor: link ke route yang bundle-nya tidak melayani.**
      Ditemukan 2026-09-22 saat kafe 10.10 (changelog `2026-09-22-015`). Menu App
      ditulis SEKALI untuk semua role sementara entity/form/report/dashboard difilter
      per role, dan catch-all SPA mengalihkan **diam-diam** ke root surface — jadi
      link mati tampak hidup (klik "Pelanggan" merender daftar Order). **Diperbaiki:**
      menu difilter terhadap isi bundle (aturan "route ini ada?", bukan "pemanggil
      punya permission?"), catch-all menjawab 404, dashboard hanya dikirim bila ada
      widget yang hidup. **Test pengunci:** `TestBuildBundle_MenuDropsUnreachableItems`,
      `TestBuildBundle_MenuDropsRoutesTheBundleRemoved`,
      `TestBuildBundle_DashboardFollowsItsWidgets`. **Sisa:** validator belum
      memperingatkan entri menu tanpa grant — lihat kafe 10.11 ⏸️.

- [x] 7.8.15 **App `access: public` membocorkan permukaan admin framework dan
      mengabaikan `public_entities`.** Ditemukan 2026-09-22 saat kafe 10.10
      (changelog `2026-09-22-015`). `AppContext.allows` selalu meloloskan `core` dan
      `formspec.core`, sementara untuk App publik `internal/api/meta.go` mengganti
      permission checker dengan selalu-true (tidak ada sesi) — sehingga `allows` jadi
      satu-satunya gerbang dan meloloskan segalanya. Terukur pada `GET
/_meta/ui?app=kafe-qr` tanpa autentikasi: entity `formspec.core`
      (user/role/api-key/session) + halaman `/access-management` (kolom `Password
Hash`) ikut terkirim, DAN bundle memuat seluruh module yang di-mount (13 entity
      termasuk `members` dengan nomor HP pelanggan) meski `public_entities` sempit.
      Endpoint data tetap 401 → tidak ada baris bocor; yang bocor permukaan admin dan
      skema. **Diperbaiki:** hanya `/_auth/*` dari framework yang lolos untuk scope
      publik, dan `BuildBundle` menurunkan checker dari `public_entities` (13 → 4
      entity). **Test pengunci:** `TestBuildBundle_PublicAppHidesFrameworkAdmin`.
      **Sisa:** kesepakatan bundle vs penegakan permintaan belum diuji menyeluruh
      — lihat kafe 10.13 ⏸️.

- [x] 7.8.16 **`check` belum memeriksa NAMA lintas-file (`spec.entity`, ref view).**
      Ditutup 2026-09-23 (changelog `2026-09-23-004`, item kafe 10.12). Sebelumnya
      `formspec validate` **hijau** pada manifest yang menunjuk entity/view/widget
      yang tidak pernah dideklarasikan — pemeriksaan `validate` per-manifest, jadi
      nama yang tidak ada di mana pun tidak pernah dibandingkan dengan apa pun;
      kegagalannya muncul saat runtime sebagai 404/placeholder. Terukur pada kafe:
      `entity: ledger`, Table → `journal_entry` (manifest `journal-entry`),
      dashboard → widget `recent-journals`. **Diperbaiki:** `checkReferences`
      memeriksa `spec.entity` pada sepuluh kind + rujukan form/table/component/widget
      di Page (blocks & tabs) + rujukan widget di Dashboard (`widgets` dan `defaults`).
      Field entity TIDAK diperiksa (sudah ada pemeriksanya; jalur kolom ber-titik butuh
      penelusuran relasi). **False positive ditemukan & dihapus sebelum commit:** ref
      telanjang yang dideklarasikan modul LAIN (dashboard `clinic` → widget
      `pharmacy-queue-count` milik `pharmacy` di Clinic-UI-Showcase) kini diterima;
      ref yang menyebut modul tetap ketat. **Hasil pada contoh:** kafe/cafe/
      crc-management/service-demo/storefront 0, arisan 4→4, Clinic 4→4,
      **Midtrans 0→2 bug nyata**. **Test pengunci:**
      `TestCheckReferences_{DanglingNames,NoFalsePositives,CleanSpecIsSilent}`
      (`cmd/formspec/check_references_test.go`) — dibuktikan gagal saat lookup
      dinonaktifkan.

- [⏸️] 7.8.17 **Tidak ada kind UI untuk mengedit `kind: Config` atau
  menampilkan log (`kind: Webhook`).** Ditemukan oleh pemeriksa 7.8.16
  (changelog `2026-09-23-004`): contoh `Midtrans-Payment-Gateway` punya dua Page
  — `midtrans-config-page` (`block form: midtrans-config-form`) dan
  `midtrans-webhook-log` (`block table: midtrans-webhook-table`) — yang
  merujuk manifest Form/Table yang **tidak pernah ada**, dan contoh itu memang
  tidak punya satu pun `kind: Form`/`kind: Table`. Tambalannya bukan menambah
  manifest palsu (itu hanya menyembunyikan gap): `FormSpec` hanya bisa mengikat
  `entity` atau `auth_action` — **tidak ada** yang mengikat `Config` —
  dan `WebhookSpec` tidak punya hook query/list, sehingga log webhook tidak bisa
  di-render sebagai Table. Yang perlu diputuskan: (a) Form mengikat Config
  (`config_ref` + key-path), (b) Table/Listing bisa bersumber dari Config/log
  store (bukan hanya Entity), atau (c) bentuk lain (mis. Page `mode: settings`
  dengan blok khusus). Effort: medium. **Teramati:** `formspec check` pada contoh
  itu 0 → 2 error sesudah 7.8.16; `grep "kind: Form\|kind: Table"` di contoh
  → 0.

- [x] 7.8.18 ✅ **2026-09-26: DIGABUNG ke 5.10.24 (duplikat) dan sudah ditutup di sana.**
      Item ini melacak flake yang **sama** (`TestKafe_OnPaidCreatesBalancedJournal`) —
      ditulis 2026-09-23 sebelum akarnya ditemukan, dengan hipotesis "worker outbox
      poll 1s melewati jendela tunggu, naikkan batas tunggu". Hipotesis itu **tidak
      benar dan berbahaya**: menaikkan timeout tidak akan menyembuhkan tanggal
      hardcoded yang ditolak `BackdatePolicy` (temuan nyata), dan untuk balapan
      `draft` vs `posted` ia hanya memperkecil peluang, bukan menghapusnya. Fix
      sebenarnya ada di 5.10.24 (poll **status**). Lihat changelog `2026-09-26-002`.

- [⏸️] 7.8.13 **Tidak ada cara berbagi kode antar-script Starlark, dan tidak ada peringatan untuk rujukan lintas-file.** 7.8.12 menutup satu kasus dengan menulis ulang script, tetapi polanya akan terulang: setiap module dengan dua aksi serupa (jurnal penjualan vs pembelian, dua guard keunikan) menghadapi pilihan antara meng-copy fungsi atau memaksakan satu file besar. Perlu keputusan: (a) dukung `load()` dengan allowlist path relatif spec, (b) file "lib" yang di-include saat kompilasi, atau (c) dokumentasikan duplikasi sebagai aturan. Apa pun pilihannya, validator sebaiknya memperingatkan pemanggilan nama yang tidak terdefinisi di file itu (analisis statis sederhana: nama yang di-define vs yang dipanggil). Effort: medium. **Teramati:** `grep "load(" examples/kafe/spec/modules/*/scripts/*.star` → 0, dan kegagalan 7.8.12 muncul sebagai retry outbox, bukan error saat authoring.
- [x] 7.8.9 **`resource.save`/`update` dari script: gap-nya NYATA — dan lebih besar dari dugaan awalnya.** ✅ **2026-09-22** (changelog `2026-09-22-013`). Diperiksa dan diperbaiki:
      **(a) `after update`/`after create` hilang di `resource.save`** — sama seperti 7.8.8, kini dijalankan (jalur `id == ""` → create, non-empty → update).
      **(b) Ditemukan gap yang TIDAK diduga: hook `before` juga tidak berjalan di SEMUA jalur script** (`resource.save` maupun `resource.create`). Ini lebih serius dari (a), karena `before` adalah GUARD: entity yang mendeklarasikan `before create` untuk menolak nilai di luar rentang **ditegakkan di setiap create HTTP dan dilewati di setiap create script** — baris buruknya masuk, tanpa error, tanpa log, dan tidak ada bedanya di manifest. **Teramati:** spec uji dengan guard `quantity > 0`, ditulis lewat script dengan `quantity: -3` → **2 widget tersimpan padahal 1 ditolak** (sebelum fix); 1 sesudahnya.
      **(c) `resource.save` juga tidak memeriksa `uses.resources`** (cross-module consent) padahal `create`/`call`/`load`/`find` melakukannya — script bisa menulis entity module lain hanya dengan menyebut namanya, sehingga consent footprint menjadi deskripsi niat, bukan batas. Kini diperiksa.
      **Yang dikerjakan:** `SaveHandler` mendapat `fromModule` + `callerResources`; engine mendapat `ActionUses` (per-execution, seperti `MaintainerRef`) supaya hook mewarisi consent action-nya; helper bersama `runBeforeWriteHooks` (mengembalikan error → write dibatalkan) dan `runAfterWriteHooks` dipakai di ketiga titik tulis script.
      **Bukti:** `TestScriptWriteEnforcesBeforeHook` + `TestScriptWriteEnforcesBeforeHookOnSave` (`resource/script_before_hook_e2e_test.go`) — keduanya **gagal** saat pemanggilan fase dinonaktifkan (`guard bypassed on the script path: 2 widgets … want 1`), hijau sesudahnya · `go test ./...` hijau · `vitest` 295 lulus.
      **Sisa:** `before`/`after` masih best-effort untuk fase `after` (tidak rollback) — limitasi model fase yang sudah tercatat di 2.1.1, bukan dari perubahan ini. 7.8.8 menutup `resource.create` (hook `after create` kini dijalankan). Jalur `update`/`save` punya hook `after update`, dan kelas kegagalannya identik: baris ter-update, proyeksi yang bergantung padanya diam. Belum diukur apakah gap-nya ada — perlu diperiksa sebelum dianggap aman, karena pola "script menulis entity yang memelihara proyeksi lain" adalah cara normal satu aksi bisnis memberi umpan ke aksi lain. **Cara mengukur:** action script yang memanggil `resource.save` pada entity ber-hook `after update` → periksa apakah hook berjalan (bukan hanya apakah barisnya berubah). Effort: small (periksa + fix + test, mirroring 7.8.8).

### 7.9 Validation levels L4–L6

- [⏸️] 7.9.1 L4 `business_rules` — single-record business constraints via script — ⏸️ kontrak deklarasi L4–L6 belum dispesifikasikan di `pkg/spec` (changelog 2026-08-25-008); perlu keputusan desain dulu
- [⏸️] 7.9.2 L5 `cross_validate` — multi-field/child-record validation within same record — ⏸️ idem 7.9.1
- [⏸️] 7.9.3 L6 `consistency` — cross-entity consistency (e.g., aggregate balance vs GL); requires `uses: db` — ⏸️ idem 7.9.1
- [⏸️] 7.9.4 Sequential evaluation — L1–L3 → L4 → L5 → L6; stop at first failure — ⏸️ menunggu 7.9.1–7.9.3
- [x] 7.9.5 Error response with `details: [{level, field?, message}]` — ✅ 2026-09-22. **Sebelumnya**: envelope `ErrorDetailItem{Level, Field, Message}` ada (`internal/api/handler.go`) tapi `Level` hardcoded `"error"` dan `Field` tidak pernah diisi; jalur validasi record bahkan memakai `writeError` polos sehingga **tidak mengirim `details` sama sekali**. **Sekarang**: tipe `validation.ValidationError{Level, Field, Message, Cause}` (`internal/validation/error.go`) + kosakata level dari `02-core-extended.md` §14 (`field`, `cross_field`, `business_rules`, `cross_validate`, `consistency`) — L4–L6 didefinisikan agar envelope stabil saat 7.9.1–7.9.4 mendarat; `ValidateCrossField`, `applyInlineRule`, `checkExists`, dan rule `required` mengembalikan tipe itu (level + nama field); `writeValidationErrors` & `writeStoreValidationError` mengisi `details[]` dari tipe tersebut (error tak-terstruktur jatuh dengan hormat ke `level:"error"`). **Bukti**: `TestTypedErrors_CarryLevelAndField` (validation) + `TestWriteValidationErrors_TypedLevelAndField` / `TestStoreValidationDetail_Levels` / `TestWriteStoreValidationError_HasDetails` (api); `go test ./...` hijau. ⏸️ Sisa sengaja: `Field` pada `ErrValidationRequired`/`ErrImmutableFieldChanged` belum terisi (pesannya menyebut field tapi tidak membawanya secara struktural) — lihat catatan di `storeValidationDetail`.
- [x] 7.9.6 Katalog rule L1–L3 lengkap server-side — himpunan tertutup ~20 rule: `required, min_length, max_length, length, pattern, email, url, min, max, positive, precision, in, future, past, after:<field>, before:<field>, min_items, max_items, unique, exists, script` (`05-field-types.md` §3) — ✅ 2026-08-25 (changelog 008)

### 7.10 Denormalisasi finansial

- [x] 7.10.1 Master financial fields snapshot to transaction on `create`/`submit` — not live-join — `RelationDecl.Snapshot` + `applyFinancialSnapshot` di crud. ✅ 2026-08-26 (changelog 001)
- [x] 7.10.2 Old transactions unaffected by master value changes — ✅ 2026-08-26 (changelog 001)

### 7.11 Period closing

- [x] 7.11.1 `period-closing` as Entity — gets lifecycle, reference guard, audit trail for free — `internal/period/module/entities/period-closing.yaml`. ✅
- [x] 7.11.2 `submit` → finalize summary period; `cancel` (reopen) → unfinalize — ✅
- [x] 7.11.3 Reopen requires elevated permission + recorded reason → `FORMSPEC.PERIOD.REOPEN_DENIED` — reopen action `required_permission: accounting.reopen_period` + reason wajib. ✅
- [x] 7.11.4 Business calendar day resolution — `today`/`current` from EOD, not system clock — `internal/period/calendar.go` `BusinessToday`. ✅
- [x] 7.11.5 `FORMSPEC.PERIOD.CLOSED` enforcement for create/update/submit/amend in closed period — `renderers/jsonb-persist/transaction_date.go` + guard wiring di `crud.go`. ✅

### 7.12 Rate limiter

- [x] 7.12.1 Per-resource rate limit — `max`, `per`, `scope` (tenant|user|ip|global), `strategy` (sliding_window|token_bucket) — `internal/api/resource_ratelimit.go`. ✅ 2026-08-25 (changelog 001)
- [x] 7.12.2 Per-action override — overrides resource default — ✅ 2026-08-25 (changelog 001)
- [x] 7.12.3 `429` response before handler runs — ✅ 2026-08-25 (changelog 001)

### 7.13 Async job tracker

- [x] 7.13.1 `call: async` (tracked) → `202` with `job_id` — `internal/job/tracker.go` + `HandleTrackedAsyncAction`. ✅ 2026-08-25 (changelog 021)
- [x] 7.13.2 Progress via WebSocket `jobs` channel — `progress`/`completed`/`failed` events — ✅ 2026-08-25 (changelog 021)
- [x] 7.13.3 `ctx.job.progress(pct, message)` from handler — `internal/starlark/context.go` `jobAPI`. ✅ 2026-08-25 (changelog 021)
- [x] 7.13.4 Callback webhook delivery — HMAC-signed, durable retry — ✅ 2026-08-25 (changelog 021)

### 7.14 Starlark sandbox

- [x] 7.14.1 Hard limits enforcement — wall-clock 5000ms, memory 64MB, iterations 100K, max 50 DB queries, max 1000 records read — `internal/starlark/limits.go`: wall-clock/steps/query/read di-enforce; memory 64MB tidak langsung (step limit adalah bound praktis, didokumentasikan). ✅ 2026-08-25 (changelog 001)
- [x] 7.14.2 No network/filesystem/subprocess access — ✅ 2026-08-25 (changelog 001)
- [x] 7.14.3 Exceeding any limit → abort with error, no partial results — ✅ 2026-08-25 (changelog 001)
- [x] 7.14.4 Kontrak API script runtime — `resource.field`/`resource.set/save/new`, `<Entity>.query()`, `<resource>.load/call`, `ok()`/`fail()` (fail = rollback transaksi) (`06-script-runtime.md` §2/§4/§6) — `resource.new` ✅ 2026-08-25 (changelog 006)

### 7.15 Sidecar multi-runtime

- [⏸️] 7.15.1 **Read `spec.runtime` per Module manifest** — go, node, php, python, ruby, java, dotnet, rust. — **Status 2026-09-22**: `spec.runtime` **sudah ada** di `ModuleSpec` (`pkg/spec/resources.go:26`) dan divalidasi (`ValidateModuleSpec` menolak nilai di luar enum), tetapi **tidak pernah dibaca** saat spawn sidecar: `formspec dev` hanya auto-detect runtime dari file marker di root proyek (`cmd/formspec/dev_runtime.go`: `composer.json`→php, `package.json`→node, …) dan spawn **satu** proses app. Kontraknya sudah lengkap dan bahkan menandai ini sendiri sebagai gap: `docs/spec/platform/08-project-layout.md` §3 (runtime per Module) + §4 (satu proses sidecar **per runtime unik**, socket `{state-dir}/sidecar/{module}.sock`, resolusi `action → Module → runtime → socket`) vs **§5 "Status Implementasi Hari Ini (Gap)"** yang menyatakan §3–§4 masih target desain.
  **Ditunda sebagai keputusan** (2026-09-22): membuatnya benar berarti orkestrasi multi-proses di `cmd/formspec/dev.go` (spawn N proses, resolusi socket per Module, dispatch sidecar per Module) — medium/large, dan berisiko meregresi model satu-runtime yang kini dipakai contoh yang berjalan (`examples/Clinic-UI-Showcase`: satu `runtime: node`, satu `app/`). **Teramati**: `grep -rn 'ModuleSpec' cmd internal resource` → `Runtime` tidak pernah dikonsumsi di jalur spawn; `dev.go:126` hanya `detectRuntime(cfg.AppDir)`.
  Effort: medium (baca + validasi + dispatch per Module) hingga large (multi-proses penuh).
- [x] 7.15.2 Spawn one sidecar process per unique runtime — ✅ (`cmd/formspec/dev.go` child-process spawn)
- [x] 7.15.3 Sidecar protocol — entity CRUD via `POST /ctx/entity/{op}` (get, set, update, increment, decrement) — ✅ (`internal/sidecar/`)

### 7.16 Money type

- [x] 7.16.1 Money as first-class type — pair of exact amount (decimal) + currency code (ISO-4217) — `pkg/spec/money.go`. ✅ 2026-08-25 (changelog 001)
- [x] 7.16.2 Currency resolution order — explicit field → `settings.currency` → error (never guess) — `ResolveMoneyCurrency`. ✅ 2026-08-25 (changelog 001)
- [x] 7.16.3 Banker's rounding default — `RoundMoney` + `RoundingHalfEven`. ✅ 2026-08-25 (changelog 001)
- [x] 7.16.4 Non-default currency MUST declare `decimal_places` — `ValidateMoneyField`. ✅ 2026-08-25 (changelog 001)

### 7.17 File storage

- [x] 7.17.1 File upload route — `POST /:resource/:id/{field}` + `GET` download; storage resolver dua backend: `file` (default, `memory.Storage`) atau `minio` (`datastore/minio`, `FORMSPEC_STORAGE=minio`); permission update/view; StorageSpec enforcement. ✅ 2026-08-24
      → **Refactor 2026-09-23** (changelog `2026-09-23-002`): helper upload
      diekspor (`ObjectKey`, `SanitizeFilename`, `AllowedFileType`,
      `MinUploadLimitMB`, `ObjectExists`) dan resolusi storage dipusatkan di
      `resource.ResolveStorage`, supaya jalur CLI (`formspec seed`) memakai
      **key, validasi, dan service yang sama** dengan jalur HTTP — bukan
      implementasi kedua yang bisa melenceng.
- [x] 7.17.2 `storage` spec enforcement — `allowed_types`, `max_size_mb`, `max_count`, `visibility` (public|private|signed) — ✅ 2026-08-25 (changelog 007)
- [⏸️] 7.17.3 Transform — server-side resize/thumbnail per `transform` spec — ⏸️ belum dikerjakan (dinyatakan eksplisit di changelog 2026-08-25-007)
- [x] 7.17.4 Download link — `POST .../{field}/link` issue (presigned MinIO untuk `visibility: signed` — menggantikan `501`; app-token link untuk jalur default), `GET .../storage/link/{token}` consume; capability `Linker` (Starlark + sidecar); `visibility: signed` kini token-gated via `?link_token=` — ✅ 2026-09-04 (changelog 2026-09-04-003, plan storage-links-plan.md)
- [x] 7.17.5 Chunked upload — routes `upload/init` / `upload/{uid}/part/{n}` / `upload/{uid}/complete`; capability `ChunkUploader`; MinIO via S3 multipart (`minio.Core`), fs/memory via parts-dir + concat; `ctx.storage().init_upload/put_chunk/complete_upload` — ✅ 2026-09-04 (changelog 2026-09-04-003)
- [x] 7.17.6 1x download + TTL — `StorageSpec.one_time` (delete-after-download, atomic budget via tabel `formspec_storage_link`), `StorageSpec.ttl` + sweeper worker (`internal/api/storage_sweeper.go`); `ctx.storage().delete` — ✅ 2026-09-04 (changelog 2026-09-04-003)
- [x] 7.17.7 Size limit — global `FORMSPEC_UPLOAD_MAX_MB` (100) / `FORMSPEC_DOWNLOAD_MAX_MB` (200) + per-field `max_size_mb` / `max_download_mb`; download over-limit → `413 FILE_TOO_LARGE` via `Stat` sebelum memuat objek; `ctx.storage().stat` — ✅ 2026-09-04 (changelog 2026-09-04-003)
- [x] 7.17.8 Driver object storage `garage` (default) — MinIO digantikan Garage sebagai object store bawaan dev container (`.devcontainer/garage.toml` + service `garage` di compose, port S3 3900); driver `garage` jadi default (`spec.DefaultStorageDriver`), `minio`/`s3` tetap didukung; client S3-compatible diekstrak ke `datastore/s3store` (dipakai bersama oleh wrapper `garage`/`minio`), `Stat`/`Delete`/`Link`/`ChunkUploader` ikut pindah; `validateDatastoreServes` + regenerasi schema/kind-docs memuat `garage`; `go.mod` tetap `minio-go` (SDK client S3, bukan server). ✅ 2026-09-16 (changelog 2026-09-16-013, plan garage-object-storage-driver.md)

### 7.18 `kind: KindDefinition` runtime

- [ ] 7.18.1 Kind registry — daftarkan kind baru dari `KindDefinition` manifest; validasi `group` + `version` + `schema` — loader masih hanya menerima built-in kinds (`internal/manifest/loader.go`)
- [ ] 7.18.2 Handler resolution — `impl.native`/`impl.script`/`impl.compiled`/`impl.sidecar`; eksekusi di bawah `uses` module yang mendeklarasikan
- [ ] 7.18.3 Group-scoped naming — instance pakai `apiVersion: {group}/v1`; tabrakan namespace mustahil secara struktural

### 7.19 `kind: Mockup` runtime

- [ ] 7.19.1 Mock connector — simulated third-party API response; `for` menunjuk Service/Webhook yang di-mock; `config_ref` ke Config — manifest diterima loader + contoh ada (`examples/Midtrans-Payment-Gateway`), tapi tidak ada yang mengonsumsinya di runtime
- [⏸️] 7.19.2 Dev-only gate — Mockup hanya aktif di `formspec dev`; production → `formspec apply` menolak atau warning

---

## Fase 8: Production Self-Hosting Single Server 🚧 (2026-08-27)

**Goal**: `formspec serve --mode=production` — production-grade, single-server, no Control Plane.

**Progress (2026-08-27)**: 8.1.1–8.1.5 ✅ · 8.2.1–8.2.6 ✅ · 8.1.6 ⏸️ · 8.2.7 ⏸️ · 8.3 ⏸️ — plan: `docs_internal/plan/fase8-production-serve.md`, changelog `2026-08-27-004`.

### 8.1 Production mode

- [x] 8.1.1 `formspec serve --mode=production` — disable dev shortcuts: no auto-approve, no self-signed, no dev auth — `cmd/formspec/serve.go`: ProdMode=true (no dev validator, no seeding, strict uses), gate validasi production (Postgres wajib, JWT wajib, CORS allow-list wajib). ✅ 2026-08-27
- [x] 8.1.2 Production JWT — RS256/ES256 (test + wire to config) — `internal/auth/jwt_asym_test.go`: RS256/ES256 end-to-end + penolakan algorithm confusion (HMAC token ditolak validator asimetris & sebaliknya); wiring `--jwt-public-key` → `Config.JWTPublicKeyPath` (ECDSA/RSA PEM, sudah ada). ✅ 2026-08-27
- [x] 8.1.3 HTTPS — TLS configuration — `--tls-cert/--tls-key` → `tls.Config` (MinVersion TLS 1.2), `ListenAndServeTLS`. ✅ 2026-08-27
- [x] 8.1.4 Production datastore — Postgres (not SQLite) — gate di serve: DSN `sqlite:` ditolak di production mode. ✅ 2026-08-27
- [x] 8.1.5 CORS origin allow-list — production TIDAK boleh `Access-Control-Allow-Origin: *` — `NewCORSMiddleware(allowList)` di `internal/api/middleware.go`; `*` hanya dev default / opt-in eksplisit; serve menolak `--cors-origin '*'`; unit test `TestCORSMiddleware_AllowList`. ✅ 2026-08-27
- [⏸️] 8.1.6 Peran DB least-privilege — `formspec_ops_backup` (REPLICATION-only), `formspec_ops_ddl` (DDL-only, NOSUPERUSER), tanpa superuser manusia (`platform/06-datastore.md` §8) — ⏸️ deferred: butuh keputusan provisioning (engine membuat role sendiri vs operator). Terkait: `docs/architecture/10-database-topology.md` §7 (backup per tier) — least-privilege adalah prasyarat bagi restore terarah yang aman.
- [⏸️] 8.1.7 **Peringatan workspace aktif hanya dipasang di jalur dev — `formspec serve`/`resource` belum.** Ditulis sebagai "**Sisa (dicatat)**" di kafe TODO 2.8 dan tidak punya item pelacak sampai audit 2026-09-22. Beda kelas dengan 8.1.6: yang ini bukan menunggu keputusan arsitektur, hanya belum dipindahkan. **Teramati:** pada dev server, spec kafe mencetak `workspace: kafe (the only one declared…)`; pada jalur non-dev (serve/native binary) deklarasi workspace tunggal **tidak** memicu peringatan apa pun, sehingga workspace yang salah tetap dipakai seperti di `default` — jalur sunyi yang membuat pengguna tidak tahu.
  Effort: small (pindahkan `DevConfig.WorkspaceIDExplicit` + pesan ke jalur boot bersama + 3 test).

### 8.2 Observability

- [x] 8.2.1 Structured JSON-lines logging — 12 mandatory fields: timestamp, level, request_id, workspace, module, entity, action, actor, duration_ms, error_code, trace_id, environment — `internal/observability/logger.go` (field kosong → `null`, bukan dihilangkan); `LoggingMiddleware` JSON-lines; dev tanpa logger tetap legacy text. ✅ 2026-08-27
- [x] 8.2.2 PII discipline — info/warn/error MUST NOT contain business data; debug gated by operator control — `SetDebugEnabled` (off default; `--log-debug` = kontrol operator yang wajib dicatat); `RedactError` helper. ✅ 2026-08-27
- [x] 8.2.3 Request ID — issue at boundary, propagate to Starlark (`ctx.request_id`), sidecar (header), ctx.\* calls — context key dipindah ke `internal/observability/requestid.go` (tanpa import cycle); `RequestIDMiddleware` meneruskan upstream `X-Request-ID`; `ctx.request_id` via `CtxAPI.RequestID` (executor thread dari Go context); sidecar header propagation ⏸️ menyusul bersama 8.2.7 (wire contract SDK). ✅ 2026-08-27 (sebagian: sidecar header)
- [x] 8.2.4 Prometheus `/metrics` endpoint (separate admin listener) — 12 mandatory metrics (`09-observability.md` §3.1): http_requests_total, http_request_errors_total, http_request_duration_seconds, action_duration_seconds, action_errors_total, outbox_pending, outbox_lag_seconds, ws_connections, db_pool_open/idle/wait_total, snapshot_age_seconds — `internal/observability/metrics.go` + `NewAdminMux` (listener `--metrics-addr`, default `:9102`); db_pool gauge di-poll tiap 10s; outbox/ws/snapshot gauge tersedia (di-set oleh worker terkait saat di-wire). ✅ 2026-08-27
- [x] 8.2.5 Cardinality discipline — labels limited to bounded dimensions; no entity_id, request_id, actor, raw URL as labels — label hanya route_class/method/status_class/module/action/error_code; `ClassifyRoute` bounded; unit test `TestMetricsMiddleware` membuktikan raw path tidak bocor. ✅ 2026-08-27
- [x] 8.2.6 Health endpoint `GET /health` — `{status, reasons, checked_at}` with controlled vocabulary: healthy/degraded/unhealthy; reasons: snapshot_stale, datastore_unreachable, db_pool_exhausted, outbox_backlog, control_plane_unreachable (`09-observability.md` §5); endpoint sama melayani liveness+readiness (ready hanya saat healthy/degraded) — `internal/observability/health.go`; probe datastore (hard → unhealthy) + db pool (degraded) auto-registered; HTTP 503 saat unhealthy. ✅ 2026-08-27
- [⏸️] 8.2.7 OpenTelemetry tracing — W3C Trace Context propagation to sidecar (wire contract) — ⏸️ deferred: butuh perubahan wire contract di semua SDK sidecar (PHP/Python/Node/Java/Ruby/.NET/TS) — kerja lintas SDK, layak fase sendiri

### 8.3 Backup automation

- [⏸️] 8.3.1 Scheduled backup — periodic full + incremental — ⏸️ deferred: butuh keputusan scheduler (in-process tick vs cron eksternal)
- [⏸️] 8.3.2 Restore procedure — documented, tested — `formspec backup create/inspect` + `formspec restore` sudah ada (4.8.x, termasuk conflict remap); dokumentasi restore procedure + incremental ⏸️ deferred
- [⏸️] 8.3.3 Routing koneksi per-workspace di Resource Plane — hari ini satu DSN per proses dan satu database melayani semua workspace (dipisah `tenant_id`), jadi "dedicated per workspace" berarti SATU deployment per workspace (D-ARCH-31). Satu proses melayani banyak database tenant adalah **fase cloud** — sejalan dengan baris "Scale-to-zero" dan "Control Plane" di tabel Deferred (Cloud Phase). Dinyatakan sebagai batas di `docs/architecture/10-database-topology.md` §8; item ini ada supaya batas itu tidak terbaca sebagai backlog tersembunyi. Termasuk konsekuensi turunannya: worker per-store / scheduler terdistribusi (selama worker wajib in-process, §3 dokumen yang sama).
  Effort: large.

---

## Fase 9: Final Audit & Cleanup

### 9.1 Full audit — code vs `docs/spec/`

- [ ] 9.1.1 Systematic comparison of every spec file against implementation; catalog all deviations

### 9.2 Integration test

- [x] 9.2.1 Order-to-Cash end-to-end — order → invoice → payment → general ledger; all flows automated — ✅ 2026-09-22 (`resource/o2c_e2e_test.go`). **Sebelumnya seluruh rantai ini hanya terverifikasi MANUAL** (walkthrough skenario 8 di `examples/kafe/gaps_found/TODO.md`) — tidak ada test yang gagal bila rantai putus. Harness baru mem-_boot_ app kafe **in-process** dengan `New(Config{…})` yang sama seperti dev server/native binary (manifest loading, schema sync, outbox worker, subscription dispatch, Starlark), lalu mendorong rantai lewat **transport produksi**: event durable `on_paid` (nama pendek + envelope `EventMessage`, persis seperti jalur emit di `internal/api/handler.go:2184`) diletakkan di **outbox**, worker yang berjalan mengantarkannya ke subscription `gl`, `journalize.star` menyusun baris double-entry, dan jurnal ter-posting.
      **6 test**: boot in-process + route/registry/subscription ada · subscription `gl/journalize` benar mem-_own_ `cafe-order.order.on_paid` · transisi `confirm-payment → paid` meng-`emit: on_paid` & event-nya `publish.durable` · **jurnal seimbang secara numerik** (debit **143750** = kredit **125000 + 12500 + 6250**, dan kas = total pesanan — bukan sekadar "jurnal ada") · **idempoten** (redeliver event yang sama tidak menggandakan jurnal) · pesanan tanpa nilai **ditolak** (`FORMSPEC.GL.ACCOUNT_NOT_FOUND`/`NO_AMOUNT`), tidak membuat jurnal.
      **Dua pelajaran konkret dari harness ini** (keduanya sebelumnya tak terlihat): (1) outbox menerima **nama event pendek** (`on_paid`) + envelope — meng-enqueue nama _fully-qualified_ membuat runtime melaporkan `no channels resolved for cafe-order/order`, jadi bentuk luarnya bagian dari kontrak, bukan detail; (2) child `lines` hanya ter-_hydrate_ lewat `GetByID`, **tidak** lewat `FindByField` — pembacaan naif mengembalikan 0 baris padahal jurnal sudah ter-posting (nilai ter-hydrate bertipe `[]map[string]any`, bukan `[]any`).
      **Bukti**: `go test ./resource/ -run TestKafe` → **6 PASS**; `go test ./...` hijau. Changelog `2026-09-22-008`.
      **Sisa (jujur)**: yang otomatis sekarang adalah **rantai akuntansi** (paid → jurnal). Skenario kafe lain yang masih manual: kasir/KDS (1–4), shift & kas (5), void/approval (6), stok/HPP (7), dan bagian UI/browser (1, 9) — tercatat sebagai `examples/kafe/gaps_found/TODO.md` 9.4.

### 9.3 Code generation

- [x] 9.4.1a **Kafe belum punya seeder bagan akun (`kind: Seed`)** — ✅
      **2026-09-26: SUDAH ADA (stale).**
      `examples/kafe/spec/modules/gl/seeds/chart-of-accounts.yaml` (kind `Seed`,
      `metadata.name: gl-chart-of-accounts`) men-seed 10 akun lewat
      `spec.entities[].records`, dan komentarnya menyebut item ini sebagai alasan
      keberadaannya. **Cakupan diverifikasi terhadap konfigurasi, bukan terhadap
      ingatan:** kesembilan kode yang di-resolve `journalize.star` semuanya ada —
      `1-1000` kas, `4-1000` omzet, `2-2000` pajak, `2-1000` service charge,
      `5-1000` diskon (semua dari `modules/gl/config/gl.yaml`
      `gl_journal_account_*`), plus `1-2000` persediaan, `2-3000` utang dagang
      (landed-cost/10.7), `3-1000` modal, `5-2000` HPP, `5-3000` beban operasional.
      Jadi jalur dev/produksi tidak lagi bergantung pada pembuatan akun manual.
      **Bukti:** `ls examples/kafe/spec/modules/gl/seeds/chart-of-accounts.yaml` →
      ada; setiap `default:` di `gl.yaml:24–54` punya pasangan `code:` di seed.
- [⏸️] 9.3.1 `make generate` — generate TypeScript types from `pkg/spec/` → `renderers/web/src/generated/types.ts`
  **Status 2026-09-22**: `make generate` **masih stub** (`Makefile` mencetak pesan "not implemented yet"), dan target path-nya (`renderers/web/`) **sudah tidak ada** — frontend sekarang `renderers/react-shadcn/`. Yang nyata berjalan hari ini adalah `formspec generate` (per-entitas client dari manifest, `cmd/formspec/generate.go` — terverifikasi: `cafe` → 12 interface), tetapi itu **client codegen**, bukan "TS types dari `pkg/spec`" yang diminta item ini. Handler `formspec generate` juga menolak `--lang` selain `typescript`. Sebagian tujuan item ini (mirror TS tidak boleh drift dari Go) sudah ditutup oleh guard parity 9.3.2. **Sisa**: generator TS dari struct `pkg/spec` (atau hapus item ini bila mirror hand-written dianggap final) — perlu keputusan apakah mirror dipertahankan atau digantikan generated types. Effort: medium.
- [x] 9.3.2 Validate generated types against manual `types/manifest.ts` — ✅ **2026-09-22 (sebagai parity guard, bukan migrasi).** `formspec generate` (9.3.1) **sudah ada** dan menghasilkan client per-entity (`cafe` → 12 interface, terverifikasi), tetapi ia men-generate `formspec-client.ts` yang **berbeda artefak** dari mirror hand-written `renderers/react-shadcn/src/types/manifest.ts` — jadi "validate generated vs manual" tidak bisa dijalankan apa-adanya. Yang dikerjakan: guard lintas-bahasa `pkg/spec/renderer_parity_test.go` yang mempin mirror TS ke sumber Go — `API_VERSION` vs `spec.APIVersion`, dan himpunan `KIND_*` vs `spec.AllKinds()` (dua arah). **Menemukan & menutup drift nyata:** `API_VERSION` masih `"formspec.dev/v1alpha1"` (nilai pra-stabil yang **ditolak** `schemaregistry.ParseVersion`, sudah diperbaiki jadi `formspec.dev/v1` di 8.2) → kini `formspec.dev/v1`; phantom `KIND_MIGRATION` (buat `kind: Migration` yang tidak pernah ada di katalog engine) dihapus; 9 kind hilang ditambahkan (Integrator, KindDefinition, Mockup, Renderer, VisualSpecKind, PersistBackend, Workspace, Calendar, ApprovalInbox, NotificationCenter); `ResourceKind` union ikut diselaraskan. Ikut ditemukan: `spec.IsValidKind` **tidak pernah mengecek `KindWorkspace`** padahal engine menerimanya (`internal/manifest.KnownKinds`) — diperbaiki + dipin oleh `TestIsValidKind_MatchesKnownKinds`. **Bukti**: `TestTSManifest_APIVersionMatchesGo` & `TestTSManifest_KindsCoverGoCatalog` (gagal lebih dulu, lalu hijau), `TestIsValidKind_MatchesKnownKinds`, `TestKnownKinds_ContainsWorkspace`; `go test ./...` hijau; `tsc` bersih; `vitest` 288. **Sisa** (bukan bagian item ini): mengganti mirror hand-written dengan tipe yang di-generate dari `pkg/spec` → item **9.3.1 ⏸️**.

### 9.4 Developer guide

- [x] 9.4.1 "Buat App Pertama Anda dengan FormSpec" — getting started guide — ✅ 2026-09-22: `docs/guides/getting-started.md` (terdaftar di `docs/guides/README.md` + sidebar VitePress; `docs-site/docs` adalah symlink ke `../docs`, jadi ikut otomatis). Isinya: prasyarat install → `formspec init` → **bentuk App** (`access` × `app_renderer`, dua sumbu design-time) → model domain (characteristic + `formspec new entity`) → run → jalur logic bisnis → daftar "berikutnya". **Setiap klaim dijalankan dulu, tidak ditulis dari hafalan** — dan itu menemukan **dua bug nyata**: (1) `formspec init` **tidak** menghasilkan `kind: Module` padahal App yang di-scaffold me-mount module itu, sehingga project baru **gagal gate-nya sendiri** (`App mounts module(s) tokoku, which no kind: Module declares`); diperbaiki dengan template `spec/modules/_module_/module.yaml` (placeholder direktori → `spec/modules/{module}/module.yaml`); (2) draft pertama contoh Entity di guide **tidak valid** (`spec.version` wajib; `states[i]` butuh `name` **dan** `label`; `permissions:` bukan bagian spec Entity) — diperbaiki sampai validator hijau. Ditambahkan juga §"expose → kalau endpoint 404 cek blok ini". Test baru: `TestRunInit_ScaffoldsProject` mempin `spec/modules/testapp/module.yaml` ada + ter-render.
      ⏸️ **Sisa**: dua Entity dengan `metadata.name` sama di satu module **tidak** dilaporkan `formspec validate` (diuji: dua manifest OK; perilaku runtime belum dipastikan) — guide menyatakannya apa adanya, dan gap-nya perlu ditutup di validator. Lihat changelog `2026-09-22-005`.

### 9.5 Retirement

- [x] 9.5.1 Final audit `docs_old/` — verify all content migrated; archive or delete — ✅ **2026-09-22: audit + pensiun tuntas** (keputusan pemilik: "perbaiki lalu hapus sekarang"). Laporan: `docs_internal/audit/docs-old-pensiun-2026-09-22.md`. **Audit**: 49 file diperiksa terhadap peta `MIGRATION.md` §2 — setiap file ber-penerus punya penerus **yang ada di disk** (32 path diverifikasi; satu-satunya "MISS" adalah rename `forma-*` → `formspec-*`), invariant `docs/` tidak mereferensikan `docs_old/` **bersih** (grep → 0), dan ledger D1–D50 tercatat tuntas ter-sweep (`MIGRATION.md` §5 baris 136). **Satu klaim di draf pertama laporan saya keliru dan sudah dikoreksi di dokumen itu**: "sweep L4–L6 belum ditutup" — `L4` di `11-reference.md:165` adalah **level persona** (Cloud Owner + admin), bukan level validasi. **Perbaikan sebelum hapus**: 16 rujukan `docs_old/` di luar arsip diarahkan ke penerus nyata — 8 di kode (`pkg/spec/spec.go`, `cmd/formspec-ctl/main.go`, `internal/ui/registry.go`, `internal/manifest/examples_roundtrip_test.go`, `sdk/browser/src/{error,types,client}.ts`), 2 di README SDK, 2 di `AGENTS.md` + `ai_skills/formspec-spec-structure/SKILL.md` (+3 salinan di `examples/*/.agents/skills/`), 4 filter arsip di `docs-site/.vitepress/config.mts`, 2 di plan internal, 1 di dokumen kafe. **Penghapusan**: `git rm -r docs_old` (49 file, semua tracked → pulih lewat git history). Changelog `2026-09-22-009` + `-010`. **Bukti**: `grep -rn docs_old` (excl. `reff_docs`/node_modules/dist) → hanya menyisakan catatan "sudah dipensiunkan" + entri changelog historis yang tidak boleh diubah; `go build ./...` ok; `go test ./...` hijau; `tsc` bersih (renderer + sdk); `vitest` 295; `vitepress build` 18,6s sukses.

---

## Fase 10: `formspec consult` — AI Business Consultant & Spec Author

**Goal**: AI membantu discovery kebutuhan bisnis + menulis spec FormSpec yang valid, lewat grounding
(tool nyata) dan validasi wajib server-side — bukan bergantung kedisiplinan LLM.
**Depends on**: Module Vendoring §6/§2.1 (`vendors/`, `formspec.lock`, alias — untuk
`list_installed_modules()` yang akurat) dan Marketplace (`ai_index`, trust tier) — keduanya masih
di tabel Deferred di bawah, jadi Fase ini realistis baru mulai setelah salah satunya landing.
**Sumber**: `docs/ai/` (README + 01–06), `docs/cli-tools/05-formspec-consult.md` (referensi verb).

### 10.0 Jalur tanpa-MCP — agent-assisted app development ✅

> Jalur yang berjalan hari ini **tanpa** MCP — lihat
> `docs_internal/plan/agent-assisted-app-development.md`. MCP (10.1–10.7 di bawah) tetap
> di-defer.

- [x] 10.0.1 Skill `formspec-app-workflow`: section Phase Detection + No-MCP Tool Map
- [x] 10.0.2 `formspec init`: copilot-instructions mereferensikan workflow 4 fase + `formspec validate` gate; `init_test.go`
- [x] 10.0.3 Guide `docs/guides/agent-assisted-app-development.md` + index
- [x] 10.0.4 Contoh `examples/cafe/` (16 manifest, 0 problem) — dibangun lewat alur ini

### 10.0.5 Cafe Order — child items UX (auto-fill, read_only, dropdown) ✅

> Lanjutan `examples/cafe` order form (`order-create`, widget `child-grid`).
> Plan: `docs_internal/plan/cafe-order-child-autofill-readonly-dropdown.md`; changelog
> `docs_internal/changelog/2026-08-24-001`.

- [x] 10.0.5.1 `auto_fill` client-side — child field `unit_price` auto-fill dari
      `menu_item_id → price` (record lengkap dari RelationPicker via `onSelectRecord`);
      clear relation → target ikut kosong. `pkg/spec` `AutoFillDecl` + `Field.AutoFill`.
- [x] 10.0.5.2 `read_only` pada child field — `Field.ReadOnly` (Go+TS),
      `ChildTable.isCellReadonly` menghormatinya; `unit_price` selalu read-only
      (tanpa `readonly_when`).
- [x] 10.0.5.3 Dropdown RelationPicker tidak terpotong — portal ke `<body>`
      (`position: fixed`, anchor bounding rect) + flip ke atas saat ruang bawah
      kurang; reposisi pada scroll/resize.
- [x] 10.0.5.4 Schema — `AutoFillDecl` masuk `sharedTypes` generator;
      `make generate-schema` (125 shared defs); `formspec validate` tetap hijau.
- [x] 10.0.5.5 Fix integer input menerima pecahan — `ChildTable` + `NumberInput`
      blokir `.`/`,`/`e`/`E` (keydown) + `step={1}` + strip desimal saat paste;
      `quantity` (integer) tidak bisa diisi pecahan lagi.
- [x] 10.0.5.6 `NumberInput` bedakan integer vs decimal via prop eksplisit
      `integer` (bukan inferensi dari `step` yang tidak konsisten) — fix decimal
      ter-truncate ke integer; `precision` kini dipakai untuk step decimal.
- [x] 10.0.5.7 Decimal `scale` membatasi input pecahan — `Field.Scale`/`Precision`
      (Go+TS+schema, 05-field-types.md §1.2); `NumberInput` prop `precision`→`scale`,
      round ke `scale` desimal saat change/paste + blokir digit berlebih di keydown;
      `FormRenderer` kirim `scale={entityField.scale}`.
- [x] 10.0.5.8 `ChildTable` pakai `NumberInput` untuk child integer/decimal —
      `scale` kini berlaku di child field (mis. `quantity: decimal, scale: 2` →
      step 0.01, input dibatasi 2 desimal); logika duplikat dihapus.
- [x] 10.0.5.9 Fix select-all + ketik di `NumberInput` (scale) terblokir —
      `type="number"` tidak expose `selectionStart` (null), jadi blokir keydown
      berbasis scale salah memblokir replace; blokir scale dihapus, pembatasan
      ditangani sanitize-on-change (`toFixed`).
- [x] 10.0.5.10 `NumberInput` spinner hormati `min`/`max` + flag merah
      out-of-range — prop `positive` (rule spec `> 0`); boundary spinner =
      langkah positif terkecil; nilai di luar range di-flag merah (border+teks+
      tooltip), bukan di-ignore/clamp; `ChildTable` kirim `min`/`max`/`positive`
      dari rules.

### 10.1 `formspec-local-mcp` — Tool Server (`formspec mcp-serve`) 🚧 (2026-08-27)

> `list_kind_schemas`/`read_workspace_manifest`/`list_installed_modules`/`read_module_spec`
> (10.1.1–10.1.2) sudah tercakup jalur tanpa-MCP di atas (§10.0); sisanya
> (10.1.3–10.1.7) menunggu MCP server (`docs/ai/03-formspec-local-mcp.md` §1).
> **Progress (2026-08-27)**: server MCP stdio jalan via SDK resmi
> (`modelcontextprotocol/go-sdk` v1.7.0) — plan `docs_internal/plan/fase10-local-mcp.md`,
> changelog `2026-08-27-005`. Tool grounding (10.1.1–10.1.2 versi MCP) ikut
> diimplementasikan karena server tak berguna tanpanya.

- [x] 10.1.3 Tool `propose_spec_file(path,content)` — tulis draft ke sesi + jalankan `validate_spec` otomatis (03 §2) — `cmd/formspec/mcp_consult.go`: draft ke `.formspec/consult/{session}/draft/`, validasi otomatis dengan overlay draft ke salinan spec tree (referensi lintas-manifest tercek); caller tidak bisa skip gate. ✅ 2026-08-27
- [x] 10.1.4 Tool `apply_draft(session,file)` — pindahkan draft ke lokasi asli, guard read-only `vendors/` (03 §4) — guard ditegakkan di kode (pesan → Entity Extension/shadow copy), auto-backup ke `undo/`, re-validate tree setelah apply. ✅ 2026-08-27
- [x] 10.1.5 Tool `validate_spec(yaml)` / `check_naming_conflict(name)` — reuse package sama dengan `formspec apply --dry-run`/boot `formspec-server`, bukan reimplementasi (03 §3) — `validateSpecTree` memakai `internal/manifest` + `kindSchemaCompiler` + honesty scan (layer sama dengan `formspec validate`); scope structural. ✅ 2026-08-27
- [x] 10.1.6 Tool `restart_server()` / `get_server_status()` / `stop_server()` — kontrol proses `formspec dev` lokal (03 §5) — PID file `.formspec/dev.pid`; restart = validate dulu (tolak kalau invalid) → stop → spawn detached (log → `.formspec/consult/server.log`) → poll `/health`; boot gagal = tail log. ✅ 2026-08-27
- [x] 10.1.7 Tool `list_skills()` / `read_skill(name)` — index dan isi FormSpec Skill (03 §1, 06) — dari `AISkillsFS` embed; `read_skill` return Markdown mentah (06 §2). ✅ 2026-08-27

### 10.2 `formspec consult` client (Go — deviasi dari spec TS, dicatat) ✅ (2026-08-27)

> **Keputusan desain 2026-08-27**: client diimplementasi dalam **Go** sebagai
> subcommand `formspec consult` — bukan TypeScript + Vercel AI SDK seperti
> tertulis di `docs/ai/01` §2 / `05` §1 (kedua docs sudah direvisi). Alasan:
> satu binary tanpa toolchain bun/node, MCP Go SDK resmi punya client+server,
> `go-keyring` adalah library Go, tim satu bahasa. LLM SDK: **`openai-go`**
> (resmi) di balik interface internal `llm.Provider` — target provider
> DeepSeek v4 Flash & GLM 5.3 Flash via gateway OpenAI-compatible. Tool loop
> ditulis sendiri (~100 baris). Plan: `docs_internal/plan/fase10-consult-client.md`,
> changelog `2026-08-27-006`.
> Keputusan tambahan: **option picker langsung di REPL** (model menyajikan
> A/B/C → user pilih satu huruf → seleksi di-inject eksplisit ke riwayat,
> fallback teks bebas) dan **history konsultasi wajib tersimpan ke file**
> (`transcript.md` incremental per turn, untuk review).

- [x] 10.2.1 ~~Scaffold project TypeScript~~ → Subcommand `formspec consult` di binary Go yang ada (`cmd/formspec/consult.go`) + package `internal/consult/` — tanpa build pipeline kedua. ✅ 2026-08-27
- [x] 10.2.2 Tool-use loop (ditulis sendiri, bukan `ToolLoopAgent`) — spawn `formspec mcp-serve` sebagai child process stdio via `modelcontextprotocol/go-sdk` `CommandTransport` (01 §3) — boundary identik dengan client MCP eksternal. ✅ 2026-08-27
- [x] 10.2.3 LLM Provider Layer — interface `llm.Provider` + adapter `openai-go` (base URL override untuk DeepSeek/GLM/OpenCode gateway), BYOK, validated-provider list (deepseek/glm/openai) + `--allow-unvalidated` warning (05 §2). ✅ 2026-08-27
- [x] 10.2.4 Credential storage — `zalando/go-keyring` tiered ke environment variable (05 §3): keyring → env (`FORMSPEC_LLM_API_KEY`, fallback `OPENAI_API_KEY`) → error jelas. ✅ 2026-08-27
- [x] 10.2.5 REPL — kelola sesi, **option picker A/B/C langsung** (deteksi blok opsi, pilih satu huruf, inject seleksi + teks lengkap ke riwayat, fallback teks bebas), perintah `/diff` `/apply` `/status` `/quit`, render diff unified (02 §4). ✅ 2026-08-27
- [x] 10.2.6 Auto-invoke deterministik saat sesi mulai (`read_workspace_manifest`+`list_installed_modules`+`list_skills`) — bukan bergantung inisiatif LLM (01 §5). ✅ 2026-08-27
- [x] 10.2.7 Kompresi riwayat sesi panjang — ⏸️ **deferred ke iterasi berikutnya**: struktur riwayat + pasangan tool_use/tool_result sudah dijaga di loop; distilasi otomatis menyusul setelah ada data pemakaian sesi nyata (transcript penuh tetap di disk sejak awal).
- [x] 10.2.8 **Transcript ke file untuk review** — `.formspec/consult/{session}/transcript.md` ditulis incremental per turn (Markdown human-readable: header metadata + per-turn user/assistant/tool ringkas); `--resume <session>` rebuild riwayat dari transcript (10.4.1 sebagian). ✅ 2026-08-27

### 10.3 Validation Gate ✅ (2026-08-27 — 10.3.1/10.3.2 terimplementasi di 10.1)

- [x] 10.3.1 `propose_spec_file` composite tool — validasi wajib server-side, proteksi sama untuk client built-in maupun eksternal (03 §2) — `cmd/formspec/mcp_consult.go`: validasi otomatis dengan overlay draft ke salinan spec tree; client built-in (`formspec consult`) memakai jalur MCP yang sama dengan client eksternal. ✅ 2026-08-27
- [x] 10.3.2 Scope structural-only (schema, referensi `depends`/Entity Extension/shadow-copy, bentrok nama) — bukan validasi data runtime (03 §3) — `validateSpecTree` = engine loader + JSON Schema + integrator + honesty scan; tanpa DB/runtime. ✅ 2026-08-27
- [⏸️] 10.3.3 Jalur online eksplisit terpisah untuk verifikasi signature/trust-tier vendor module (03 §3) — ⏸️ **deferred ke Fase 13** (Module Vendoring/Marketplace belum ada — tidak ada signature yang bisa diverifikasi)

### 10.4 Session storage & diff 🚧 (2026-08-27)

- [x] 10.4.1 `.formspec/consult/{session}/` — `transcript.md`, `discovery-summary.md`, `draft/`, `undo/` (02 §3) — transcript incremental per turn + `--resume` (10.2.8); `discovery-summary.md` via perintah REPL `/summary` (model merangkum discovery dalam bahasa awam → ditulis ke file untuk konfirmasi owner); `draft/`+`undo/` dari 10.1. ✅ 2026-08-27
- [x] 10.4.2 `formspec consult diff` — unified diff `draft/` vs `modules/`/`vendors/` project asli (02 §4) — `internal/consult/diff.go` (LCS line diff) + subcommand `formspec consult diff --session <id>` + `/diff` di REPL. ✅ 2026-08-27
- [x] 10.4.3 Accept/reject per file → `apply_draft` (02 §4) — `/apply <path>` (satu file) atau `/apply` (semua), `/reject <path>` (buang draft, spec tree tak tersentuh). ✅ 2026-08-27

### 10.5 `formspec-remote-mcp` (FormSpec Cloud, hosted)

- [ ] 10.5.1 Streamable HTTP server — `list_business_templates()`, `search_modules_registry(query)`, `get_module_detail(name)` (`docs/ai/04-formspec-remote-mcp.md`)
- [ ] 10.5.2 Katalog industry template awal, 100% FormSpec-authored — pattern YAML + probing questions (04 §1)
- [ ] 10.5.3 pgvector embedding untuk `search_modules_registry` — model multilingual (Voyage AI/BGE-M3), hybrid dengan `aliases:` eksplisit (04 §2)
- [ ] 10.5.4 `ai_index`/`skills_for_ai` untrusted-input handling — wajib selesai sebelum trust tier `community` dibuka untuk field ini (04 §3.1)

### 10.6 FormSpec Skill ✅ (2026-08-27)

- [x] 10.6.1 Format YAML frontmatter + Markdown bodPy — `name`, `description`, `applies_to_kind`, `min_core_spec_version` (`docs/ai/06-formspec-skill.md` §2) — parser `skillMeta` di `cmd/formspec/mcpserve.go` membaca keempat field; skill baru memakai format ini. ✅ 2026-08-27
- [x] 10.6.2 Skill pertama: entity-authoring, form-layout, entity-extension-authoring, module-vendoring (06 §2, §4) — 4 SKILL.md baru di `ai_skills/` dengan frontmatter `applies_to_kind` + `min_core_spec_version`, isi diground ke docs/spec. ✅ 2026-08-27
- [x] 10.6.3 Bundling bersama instalasi `formspec` (ikut siklus rilis, dicek vs Core Spec lokal), dibaca lewat `list_skills()`/`read_skill()` (06 §2–§3) — `AISkillsFS` embed (sudah ada sejak 10.0) + tool MCP dari 10.1.7. ✅ 2026-08-27
- [x] 10.6.4 Re-cek skill relevan sebagai bagian composite `propose_spec_file` — pemicu deterministik, bukan inisiatif LLM (06 §3) — `relevantSkillsFor`: parse kind draft → match `applies_to_kind` (kosong = selalu relevan) → `relevant_skills` dikembalikan dalam hasil propose; model membaca via `read_skill`. ✅ 2026-08-27

### 10.7 Operational safety ✅ (2026-08-27 — terimplementasi di 10.1)

- [x] 10.7.1 Snapshot & undo — auto-backup file-level di `.formspec/consult/{session}/undo/` sebelum `apply_draft` menimpa (02 §4) — `cmd/formspec/mcp_consult.go` applyDraft. ✅ 2026-08-27
- [x] 10.7.2 Guard read-only `vendors/` ditegakkan di semua tool tulis, bukan konvensi dokumentasi (03 §4) — `guardVendors` di `propose_spec_file` + `apply_draft`, pesan mengarahkan ke Entity Extension/shadow copy; unit test. ✅ 2026-08-27

---

## Fase 11: Resolusi Review Schema ↔ Docs ✅ COMPLETE (2026-07-31)

Menutup kontradiksi `pkg/spec`/JSON Schema/`renderers/web` vs `docs/spec/`.
Referensi: `docs_internal/changelog/2026-07-31-001-resolusi-review-schema-docs.md`.

| Item                                                                                                                                                           | Status | Catatan                                                                                     |
| -------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ | ------------------------------------------------------------------------------------------- |
| A1: `Config.spec.keys` → `$ref ConfigKey` (generator map-of-struct)                                                                                            | ✅     | `internal/genjsonschema` + regen schema                                                     |
| A2: `Entity` canonical (revert `Document`), konsolidasi validator                                                                                              | ✅     | `spec.go`/`entity.go`/`registry.go`/`kinds.go`/`manifest.ts`; `Document` = deprecated alias |
| B1: `ModuleSpec` +`vendor/datastore/config/ai_index`; `AppSpec` structured `publishes`/`consumes` +`app_renderer/theme_ref/auth_config_ref`                    | ✅     | binding `datastore` runtime tetap defer (2.9.4)                                             |
| B2: `EnvironmentSpec`/`PolicySpec` (`pkg/spec/control.go`) — schema tak lagi bare                                                                              | ✅     | eksekusi control-plane tetap defer                                                          |
| B3: `attachment` alias `file` + docs `spec.auth` §1.4                                                                                                          | ✅     | normalisasi di `ValidateEntitySpec`                                                         |
| C1: Form `sections`/`read_only`/`render:{mode}` + Table `filters`/`default_sort` — docs→kode                                                                   | ✅     | perbaiki fixture `internal/ui` (test sebelumnya merah)                                      |
| C2: Kanban docs → schema (`status_field` wajib, `columns{status,label}`, `card_template`) + Open zero-config/`group_by`/`drag_guard`/`wip_limit`/`card_fields` | ✅     | defer → `docs_internal/plan/kanban-full-implementation.md`                                  |
| C3: Dashboard/Widget docs → ref-based + Open rendering widget                                                                                                  | ✅     | defer → Fase 5.7                                                                            |
| C4: Report docs → kode (`parameters`/`groups`/`export`, objek) + Open `source.filter`; TS/ renderer fix                                                        | ✅     | contoh manifest Report diperbarui                                                           |
| C5: Print hapus `formats` redundan                                                                                                                             | ✅     | satu format per manifest = `output.format`                                                  |
| C6: Page `binds`/`mode:custom` + BlockRef `needs:` ditandai Open                                                                                               | ✅     | defer → Fase 5                                                                              |

## Fase 12: Domain Infrastruktur formspec.dev 🚧 (2026-08-12)

Setup domain, landing, docs site, dan schema hosting. Referensi:
`docs/architecture/09-domain-map.md`, `docs_internal/plan/rename-formspec.md`.

| Item                                                                                            | Status | Catatan                                                                            |
| ----------------------------------------------------------------------------------------------- | ------ | ---------------------------------------------------------------------------------- |
| 12.1 DNS Cloudflare: nameserver, records, SSL Full (strict), redirect rules                     | 🔲     | Manual — butuh akses akun Cloudflare/registrar                                     |
| 12.2 Landing page `formspec.dev` (`site/`, Vite+React)                                          | ✅     | Build hijau; deploy Pages belum (butuh akun)                                       |
| 12.3 Docs site `docs.formspec.dev` (`docs-site/`, VitePress)                                    | ✅     | Build hijau (123 halaman); changelog/plan/presentations/technical-notes di-exclude |
| 12.4 Schema hosting `schemas.formspec.dev` (`scripts/publish-schemas.sh`)                       | ✅     | Stage v1 + alias latest; upload R2 belum                                           |
| 12.5 Email Resend `send.formspec.dev` (SPF/DKIM/DMARC)                                          | 🔲     | Manual — butuh akun Resend + set DNS                                               |
| 12.6 Reserve subdomain backend (registry/mcp/api/ops/status/try/control.\*)                     | 🔲     | Manual — Cloudflare DNS                                                            |
| 12.7 `security.txt`, `.well-known`, `robots.txt`, `_redirects`/`_headers`                       | ✅     | Di `site/public/` & `docs-site/public/`                                            |
| 12.8 Deploy CI: Pages project (site/, docs-site/, schemas)                                      | 🔲     | Manual — pakai Build watch paths (changelog 2026-08-14-001)                        |
| 12.9 Schema registry online: versi di `apiVersion`, `formspec schema`, cache lokal, hapus embed | ✅     | `docs_internal/plan/schema-registry-online.md` + changelog 2026-08-14-004          |
| 12.10 Subcommand `formspec version` + stamping ldflags di build release                         | ✅     | `cmd/formspec/main.go` + `Makefile` (docs_internal/plan/install-page-plan.md)      |
| 12.11 `make release` — cross-compile binary multi-OS (6 target) + tar.gz/zip + SHA256SUMS       | ✅     | Target `release` + `release-upload` (GitHub Releases, manual via `gh`)             |
| 12.12 Installer script `site/public/install.sh` + `install.ps1` (user-local, no sudo)           | ✅     | URL `formspec.dev/install.sh` / `/install.ps1`                                     |
| 12.13 Landing section `#install` (3 metode × tab OS) + link di Nav                              | ✅     | `site/src/components/Install.tsx`                                                  |
| 12.14 Guide `docs/guides/install.md` + sidebar VitePress + update `01-formspec-dev.md` §8       | ✅     | Placeholder wget diganti installer; tautan dari how-to-run.md                      |
| 12.15 First real release: tag + `make release` + upload ke GitHub Releases                      | 🔲     | Manual — butuh push akses; jalankan `make release-upload VERSION=vX.Y.Z`           |
| 12.16 Uji installer end-to-end di macOS + Windows (ARM64 included)                              | 🔲     | Setelah 12.15 — validasi PATH/checksum per OS                                      |
| 12.17 Autodetect tab OS di section `#install` (via userAgent) + merge tab Linux → 2 tab         | ✅     | `site/src/components/Install.tsx` (changelog 2026-09-11-003)                       |

> **Catatan 2026-09-13** — `make release-upload` sekarang idempotent: draft
> release yang upload-nya terputus di tengah bisa dilanjutkan dengan menjalankan
> perintah yang sama (asset yang sudah lengkap dilewati berdasarkan size, sisanya
> `gh release upload --clobber`). Release yang sudah _published_ tetap ditolak
> ("satu tag = satu release"). Implementasi: `Makefile` target `release-upload` +
> `docs/guides/releasing.md` §4 — changelog
> `docs_internal/changelog/2026-09-13-003-release-upload-resume.md`.
>
> Draft `v0.0.7` sudah dibuat dan lengkap (8 asset) — sisa 12.15: review lalu
> klik **Publish** di GitHub Releases, kemudian lanjut 12.16.

> **Catatan 2026-09-18** — langkah tag di alur rilis dibuat terbaca. `make release`
> sukses (auto-bump `v0.0.9`) tapi `make release-upload` gagal "Tag v0.0.9 belum ada
> lokal": guard-nya benar (tag tidak pernah dibuat otomatis — keputusan di
> `docs_internal/plan/release-version-auto.md` §Keputusan), tapi `make release`
> tidak menyebut langkah tag sama sekali sehingga kebutuhannya baru terasa setelah
> SPA build + 6 cross-compile selesai. Perbaikan (opsi A — guard tidak diubah):
> target `check-release-tag` direprequisite **sebelum** `build-spa` + dicetak lagi
> di ringkasan akhir via `scripts/release-tag-status.sh` (baru; info-only, sekaligus
> memperingatkan bila tag menunjuk commit selain HEAD), dan §2
> `docs/guides/releasing.md` merekonsiliasi urutan tag ↔ build (dua urutan sah,
> syaratnya tag di commit yang dibangun). Changelog
> `docs_internal/changelog/2026-09-18-002-visibilitas-langkah-tag-rilis.md`.
>
> 📌 Rilis `v0.0.9` sedang berjalan: tag sudah di-push ke `origin` (commit
> `71bf2fa`), artifact 8 file siap di `dist/release/` — sisa 12.15: `make
release-upload`, review draft, lalu **Publish**.

> **Catatan 2026-09-17** — versi rilis tidak lagi diambil dari `git describe`:
> `make release` tanpa `VERSION=` memakai patch-bump tag semver tertinggi
> (`scripts/next-version.sh`), `make release-upload` memakai versi artifact di
> `dist/release/`, dan guard `scripts/check-semver.sh` menolak string
> git-describe (mis. `v0.0.8-4-gceaaf2a`) yang membuat `formspec upgrade`
> menampilkan prompt rollback palsu. Komparator semver `cmd/formspec/semver.go`
> kini mengenali suffix describe sebagai post-release. Lihat
> `docs_internal/plan/release-version-auto.md` + changelog
> `docs_internal/changelog/2026-09-17-002-versi-rilis-otomatis-anti-git-describe.md`.
>
> ⚠️ **Temuan menyertai**: `go test ./cmd/formspec` **gagal build** sejak commit
> `2d816b4` ("update cafe2", 2026-09-16) — `cmd/formspec/migrate_dialect_test.go`
> memanggil `loadCustomMigrations` dan field `spec.MigrationSpec.DDLByDialect` /
> `.DML` yang tidak ada (produksi menyimpannya di `pkg/spec/entity.go` dengan
> bentuk lain; terkait changelog `2026-09-16-008-migration-dialect-and-dml.md`).
> Artinya `make test` (dan gate test di `scripts/git-push-and-tag.sh`) merah
> untuk paket itu; perlu diselaraskan sebelum rilis berikutnya.

## Fase 13: Module Registry & Vendoring (npm-like) 🚧 (2026-08-20, planned; 13.1–13.2 ✅, 13.3 sebagian ✅ 2026-08-28)

**Goal**: Ekosistem module registry — `formspec module install/publish/list/uninstall`,
`formspec override adopt/diff`, `vendors/` + `overrides/` + `formspec.lock`, aktivasi
berbasis marker, dan registry server sebagai **FormSpec app (dogfooding)** untuk
`registry.formspec.dev` — trial & POC bahwa service internal FormSpec dibangun dengan
FormSpec sendiri.
**Model**: read-only vendoring + shadow copy (sesuai `docs/spec/platform/08-project-layout.md` §6).
**Sumber**: `docs/spec/platform/07-marketplace.md`, `08-project-layout.md` §6,
`02-workspace-app-module.md` §2.1, `docs/technical-notes/Forma-Technical-Note-Module-Vendoring-Aktivasi.md`,
`docs/cli-tools/02-formspec-cli.md` §9, `docs/architecture/09-domain-map.md`.
**Depends on**: Fase 6 (auth — 6.2 permission model + 6.4 API keys) untuk bagian
auth-dependent; Fase 8 (production serve) untuk deploy nyata ke `registry.formspec.dev`.

### 13.1 Local vendoring & activation (offline-first CLI) ✅ (2026-08-28)

- [x] 13.1.1 `formspec.lock` schema + layout `vendors/` — paket baru `internal/vendor/` (types lockfile, marker parser, alias resolver). Entri per module: `source`, `version`, `checksum` tree, `signature`, `trust_tier`, `alias`, `installed_at` (YAML). — `internal/vendor/lock.go` (+`name` untuk normalisasi nama efektif). ✅ 2026-08-28
- [x] 13.1.2 `formspec module install <source>` (git/folder/tarball dulu, offline) — `cmd/formspec/module.go` + `module_install.go`. Fetch → stage → validate (`module.yaml`) → copy ke `vendors/{module}/` → checksum → lock → tulis marker block ter-comment di `App.spec.modules`. Flag `--use` (aktif langsung), `--from` (registry, 13.3), `--yes` (skip consent). — `internal/vendor/install.go` + `cmd/formspec/module.go`; `--from`/`--yes` menyusul dengan 13.3 (registry). ✅ 2026-08-28
- [x] 13.1.3 Alias saat konflik nama — dihitung saat install (Opsi B: terhadap semua yang pernah di-install + module lokal), dicatat di lock + marker. — `internal/vendor/alias.go`: derivasi `{org}-{name}` (prefix match); `module.yaml` vendor dinormalisasi ke nama efektif sehingga boot mendaftar entity di bawah nama tersebut. ✅ 2026-08-28
- [x] 13.1.4 Boot-time enforcement — `AddManifestRoot("vendors/")` di `internal/entity/registry.go`; resolusi nama efektif dari lock; bentrok di set aktif → refuse boot; hanya module aktif (uncommented) yang diregister. — `resource/formspec.go`: `vendor.ActiveModules` → root per module aktif di entity registry DAN loader resolusi App/Module; normalisasi nama di install-time menggantikan alias-rewrite saat boot; E2E terverifikasi (aktif → API reachable, inactive → 404). ✅ 2026-08-28
- [x] 13.1.5 `formspec module list` / `uninstall` — list: status aktif/nonaktif + trust tier; uninstall: hapus `vendors/` + lock + marker (jaga status aktivasi). — `cmd/formspec/module.go`. ✅ 2026-08-28
- [x] 13.1.6 `formspec verify` — checksum tree `vendors/` vs lock; tolak build kalau ada modifikasi manual. — `cmd/formspec/verify.go` + `vendor.Verify`; E2E tamper detection terverifikasi. ✅ 2026-08-28

### 13.2 Shadow copy (`overrides/`) ✅ (2026-08-28)

- [x] 13.2.1 `formspec override adopt <module> <kind> <name>` — copy ke `overrides/{module}/{kind}.{name}.yaml`, catat checksum sumber di lock ("asal fork"). — `internal/vendor/override.go`: upstream dilokasi via loader (bukan tebak path); `LockEntry.Overrides` menyimpan kind/name/origin/rel_path/base_checksum. ✅ 2026-08-28
- [x] 13.2.2 Boot-time replace-total + whitelist — `AddManifestRoot("overrides/")`; override menang atas `modules/`/`vendors/`; whitelist per kind (Form/Menu/VisualSpecKind instance boleh; Entity/Service/Workflow diblokir). — overrides root ditambahkan TERAKHIR (later roots win); whitelist §5.4 = Form + VisualSpecKind (Menu bukan kind standalone — navigasi di App/Module spec); `ValidateOverridesDir` menolak boot pada pelanggaran. ✅ 2026-08-28
- [x] 13.2.3 `formspec override diff <module> <kind> <name>` — bandingkan shadow copy vs upstream. — `internal/textdiff` (package bersama, dipakai consult + override); CLI `formspec override diff`. ✅ 2026-08-28
- [x] 13.2.4 Drift detection — saat install/update, bandingkan checksum base baru vs "asal fork" → warning. — `vendor.CheckDrift` dipanggil saat boot (warning stderr, bukan hard-fail §5.3); record Overrides selamat across re-install. ✅ 2026-08-28

### 13.3 Registry sebagai FormSpec app (dogfooding POC)

> Registry TIDAK dibangun sebagai Go binary hand-written — melainkan sebagai **FormSpec app**.
> Native embedding sudah ada (`examples/reference-app/main.go` + `docs/runtimes/02-formspec-resource.md`);
> `cmd/formspec-registry` = wrapper tipis yang meng-embed engine + spec registry
> (`verticals/registry/spec/` via `//go:embed` atau `--spec`). Tidak perlu ubah spec —
> hanya tambah docs: "native app binary" sebagai deployment mode first-class.

- [x] 13.3.1 Scaffold `verticals/registry/` — App + Module manifests, `formspec generate auth` (API key untuk publish). POC jalan via `formspec dev`. — App `registry` + Module `registry` + 3 entities; validate hijau engine+schema; POC boot via `formspec dev` terverifikasi. `generate auth` menyusul (auth module dogfooding terpisah). ✅ 2026-08-28
- [x] 13.3.2 Entities `Module` / `ModuleVersion` / `Vendor` — state machine (draft→published→deprecated), events, permissions; tarball → `ctx.storage` (ref di Entity, blob di storage). — entity manifests lengkap (ModuleVersion: semver unique, checksum, signature ed25519, tarball file field, state machine); storage blob via file field + upload route (7.17.1). Events/permissions detail menyusul. ✅ 2026-08-28
- [⏸️] 13.3.3 Services `signature-verify` (ed25519), `checksum`, `search` (list filters dulu; pgvector 10.5.3 nanti). — ⏸️ deferred: verifikasi signature saat ini client-side di `module install --from` (terverifikasi E2E); server-side verify butuh `cmd/formspec-registry` wrapper + native handler.
- [x] 13.3.4 `spec.expose` — public read (search/detail/download), authenticated publish (API key, 2.5.1). — expose list/find/create/submit di ketiga entity; publish flow E2E lewat REST surface workspace-scoped. API key enforcement menyusul (dev mode POC tanpa auth). ✅ 2026-08-28
- [⏸️] 13.3.5 Workflow review trust tier `verified` (approval). — ⏸️ deferred (butuh 13.3.3 + keputusan reviewer flow).
- [x] 13.3.6 `formspec sign` — ed25519 keypair, sign checksum tree module; registry verifikasi saat publish; trust tier (official/verified/community). — `cmd/formspec/sign.go` (keygen/sign/verify) + `internal/vendor/sign.go`; tamper detection E2E; trust tier community default, verified/official menyusul dengan 13.3.5. ✅ 2026-08-28
- [x] 13.3.7 `formspec module publish` — sign + upload; `--registry`/env `FORMSPEC_MODULE_REGISTRY` (default `https://registry.formspec.dev`). — `cmd/formspec/publish.go` + `internal/vendor/registry.go`; versi immutable (re-publish semver sama content beda → ditolak); E2E terverifikasi. ✅ 2026-08-28
- [x] 13.3.8 `formspec module install --from registry.formspec.dev` — download tarball → verifikasi signature → alur 13.1. — verifikasi signature terhadap public key vendor terdaftar SEBELUM tarball dipercaya (checksum mismatch → REFUSED); lock mencatat registry ref + signature; E2E penuh (publish → install → boot → module ter-register). ✅ 2026-08-28
- [⏸️] 13.3.9 Marketplace layer — pricing/metering/licensing per `07-marketplace.md` §4–§9 — out of scope fase ini.

### 13.4 Docs & tests

- [x] 13.4.1 Update `docs/spec/platform/08-project-layout.md` §6 (target desain → implemented) + resolve open questions §6.5.
- [x] 13.4.2 Update `docs/cli-tools/02-formspec-cli.md` §9.
- [x] 13.4.3 Tests: unit (lock/marker/alias/checksum), integration (install→boot), e2e (publish→install). — verified via `go test ./internal/vendor/...` on 2026-09-15.
- [x] 13.4.4 Changelog per hari + update todo. — recorded in `docs_internal/changelog/2026-09-15-005-module-vendoring-tests-todo.md`.

**Dependensi**: Fase 6 (6.2 permission model + 6.4 API keys) wajib selesai untuk bagian
auth-dependent (13.3.4–13.3.8); Fase 8 (production serve) untuk deploy nyata ke
`registry.formspec.dev`. Data model/API/storage (13.3.1–13.3.3) bisa dibangun paralel
tanpa auth.

### 13.5 Registry docs terpusat + portal publik + register ✅ (2026-08-28)

- [x] 13.5.1 Docs terpusat `docs/registry/` (7 dokumen: concepts, quickstart, cli-reference, rest-api, self-hosting, trust-tier) + koreksi docs usang (CLI §9 = 13.4.2, project-layout §6 = 13.4.1) + link dari `docs/README.md`. — changelog `2026-08-28-004`. ✅ 2026-08-28
- [x] 13.5.2 Folder `registry/` (pindah dari `verticals/registry/`); App `no-nav` + `access: public`, root `/`. ✅ 2026-08-28
- [x] 13.5.3 Module `portal` — landing page (hero/feature_grid/cara pakai/cta) + Listing katalog module publik (search + filter trust_tier). ✅ 2026-08-28
- [x] 13.5.4 `POST /{ws}/_ui/auth/register` — register publik (bcrypt, rate limit 3/30s per IP); E2E smoke hijau (boot → register → meta UI). ✅ 2026-08-28
- [x] 13.5.5 Onboarding Vendor ter-link ke user register — halaman `/portal/vendor-signup` (Form create `registry.vendor` + field `owner_username`); alur register → create vendor E2E terverifikasi. ✅ 2026-08-28
- [x] 13.5.6 Native binary `cmd/formspec-registry` + server-side signature verify (13.3.3) + Redis cache + deploy K8s 3 replica (Plan C). — **batch 1 ✅ 2026-08-29**: binary native (embed spec via `registry/embed.go`, extract temp saat boot) + service `signature-verify` (impl native `registry.SignatureVerify`) + publish CLI verify server-side sebelum upload (best-effort di registry dev); E2E: valid→lulus, tampered→ditolak, publish ke native registry sukses. **batch 2 ✅ 2026-08-29**: driver Redis/Valkey `ctx.cache` (`renderers/jsonb-persist/datastore/rediskv/` — resolve di `resource/datastoreregistry.go`, test integrasi vs Valkey dev container) + deploy artifacts `registry/deploy/` (Dockerfile distroless, K8s 3 replica + probes + Ingress TLS, Datastore valkey manifest). Sisa (deferred): cache-aside wiring di module registry, shared rate limiter antar-pod.
- [⏸️] 13.5.7 **cache-aside wiring registry + shared rate limiter antar-pod belum ada** — 13.5.6 menyebutnya sebagai `Sisa (deferred)` tanpa item bernomor. grep `cache-aside` / `rate limiter` di todo → 0. Effort: medium (butuh backend KV bersama + pembagian kuota antar-replica).
  - 备注 2026-08-31: registry binary 新增内嵌 SPA fallback — `--web-dir` 为空时用 `registry/web` 内嵌 dist（`make build-registry` 同步），见 changelog 2026-08-31-004。

## Fase 14: Framework-Level Entity Cache (Read-Through) ✅ (2026-08-29)

**Goal**: Read-through cache di `HandleFind` (GetByID) — opt-in eksplisit per
entity (`spec.cache.ttl`), invalidasi in-process + Redis pub/sub broadcast
untuk multi-instance. List caching skip. Berlaku umum untuk semua app
(registry = adopter pertama).
**Plan**: `docs_internal/plan/fase14-entity-cache.md` · **Keputusan**: opt-in eksplisit,
list skip, v1+v2 invalidasi, Fase terpisah (bukan 13.6).

- [x] 14.1 Spec — `CacheSpec{ttl}` di `pkg/spec/entity.go` + validasi (parse duration, 1s–1h) + JSON Schema (`CacheSpec` shared def). ✅ 2026-08-29
- [x] 14.2 Cache layer — `internal/api/entitycache.go`: key `{ws}:{module}:{entity}:id:{id}`, read-through (record MENTAH via struct `cachedRecord` — EntityRecord MarshalJSON flat tanpa unmarshal kebalikan), invalidasi di update/delete/submit/deactivate/workflow-transition. ✅ 2026-08-29
- [x] 14.3 Backend resolver — `resource/formspec.go`: module bound ke datastore `serves:[cache]` → backend itu; else shared in-memory. `SetEntityCache` di RouterBuilder + HandlerFactory. ✅ 2026-08-29
- [x] 14.4 Invalidasi multi-instance (v2) — `rediskv.KV` subscription `formspec:cache:invalidate` (delete lokal tanpa re-broadcast) + `BroadcastInvalidate`; `api.CacheInvalidator` optional interface. ✅ 2026-08-29
- [x] 14.5 Tests — 6 api (hit/miss, invalidasi, non-opted, tenant key, TTL) + 2 rediskv (termasuk broadcast dua-instance vs Valkey nyata); E2E smoke hit→stale→invalidasi. ✅ 2026-08-29
- [x] 14.6 Adoptasi registry (`cache: {ttl: 300s}` di module/vendor/module-version) + docs (`backend/01` §10 baru + `registry/01-concepts`). ✅ 2026-08-29

**Catatan**: `formspec validate` default memakai schema dari cache registry
online — schema lokal dengan field `cache` perlu `make publish-schemas` dulu;
sampai itu, validasi pakai `--schema schemas`.

### 14.a Portal UX & App identity (2026-08-29, commit a486267)

- [x] AppSpec.title (display name, spasi boleh) + AppSpec.logo (lucide icon) — brand bar shell + document.title. ✅ 2026-08-29
- [x] NoNavShell: brand logo+title, nav link active state, auth area (Sign in/Sign up saat anonim, Log out saat signed-in). ✅ 2026-08-29 — _auth area kini dikontrol `App.spec.chrome` (14.b), bukan hardcode shell_
- [x] LoginScreen mode register (display_name + POST /{ws}/\_ui/auth/register + auto-login) + route /register; public surface boot memakai session tersimpan (signed-in user dipertahankan di portal). ✅ 2026-08-29
- [⏸️] Row-level ownership (update/delete module milik sendiri) — butuh fitur **record-level authorization** di framework (permission check per-record via relasi vendor.owner_username). Deferred: kerja framework, bukan registry-specific.

### 14.b Chrome Composition Spec (2026-08-29)

> Plan: `docs_internal/plan/chrome-composition-spec.md` · changelog `2026-08-29-007`

- [x] `App.spec.chrome` — sub-spec `brand`/`nav`/`auth`/`footer`/`breadcrumbs`/`theme_switcher` (default `auto`), ortogonal terhadap `app_renderer` & `access`; validasi enum di `ValidateAppSpec`; schemas regenerated. ✅ 2026-08-29
- [x] Resolusi default di backend meta — `internal/ui.resolveChrome` → `bundle.app.chrome` (no-nav: nav=none, auth=none; sidebar/topnav: nav=menu, auth=links). ✅ 2026-08-29
- [x] Frontend — komponen bersama `AuthArea`; `NoNavShell` tanpa hardcode auth/nav/footer; `SideNavShell`/`TopNavShell` hormati override breadcrumbs/theme_switcher/auth. ✅ 2026-08-29
- [x] `registry.yaml` — `chrome: {nav: menu, auth: links}` eksplisit (perilaku portal tetap). ✅ 2026-08-29
- [x] Docs — `05-app-kinds.md` §4 rewrite + §5 Chrome Composition (renumber §5→§6, §6→§7 + cross-ref), `03-kind-renderers.md`, glossary. ✅ 2026-08-29

### 14.c Chrome Regions + landing & auth-exit (2026-09-29)

> Plan: `docs_internal/plan/chrome-regions.md` · changelog `2026-09-29-002`

- [x] Fase A — `DefaultRedirect` sadar permission (`src/shell/landing.ts`); `NoAccessState` menggantikan "No entities found"; `AuthArea` merender user menu bila ada token (sebelum cek `mode`). ✅ 2026-09-29
- [x] Fase B — `App.spec.chrome.regions` (`topbar/sidebar/rightbar/bottombar/footer` → `none|auto|<ref>`) di `pkg/spec` + validasi; `internal/ui.resolveChrome` preset per-archetype + gula boolean; `tier: component` boleh `implements_slot: <region>`. ✅ 2026-09-29
- [x] Fase B — renderer: satu `RegionShell` menggantikan `SideNavShell`/`TopNavShell`/`NoNavShell` (dihapus); `OverlayHost` di semua komposisi; `types/manifest.ts` union literal. ✅ 2026-09-29
- [x] Fase C — `kafe-qr` `chrome.regions` (topbar/footer `auto`); docs §4–§5 + `02-visual-spec-kind` §4 + renderer doc + glossary; `make generate-schema`/`generate-kind-docs`. ✅ 2026-09-29
- [x] 14.c.1 Invarian validator "App privat wajib punya jalan masuk" — **DITUTUP 2026-10-02** dengan bentuk yang berbeda dari rencana: invariannya ditegakkan saat **resolve App** (`internal/app/resolve.go` → `ui.ChromeAcceptsLogin`), bukan di loader validator. App `private` yang chrome-nya tak punya entry point auth (mis. `access: private` + `app_renderer: no-nav` tanpa `chrome.auth`) ditolak dengan pesan yang menyebut jalan keluarnya. Satu implementasi dipakai bersama oleh `/_meta/apps`, endpoint login, dan invarian ini. Plan `docs_internal/plan/app-scoped-login.md`. ✅ 2026-10-02 · changelog `2026-10-02-006`
- [⏸️] 14.c.2 `useAutoLogout` masih mati di permukaan publik (`!isPublic` di `App.tsx`), jadi sesi di App publik tidak kedaluwarsa. Effort: small.
- [⏸️] 14.c.3 `auth_action` belum punya `logout`; Page/Form belum bisa menyatakan aksi auth (kafe 10.18b). Effort: medium.
- [x] 14.c.4 Gerbang App backend — **DITUTUP 2026-10-02 sebagai digantikan, bukan dikerjakan.** Field `access_permission` (`app-entry-gate.md`) tidak dipakai; gerbangnya adalah **validasi saat login**: `app` tak dikenal → 400 `UNKNOWN_APP`, App tanpa entry point auth → 400 `APP_PUBLIC_NO_LOGIN`, kredensial benar tapi 0 permission di App itu → 403 `NO_APP_ACCESS` (kafe 10.22), plus sesi mengikat `_meta/ui` (403 `APP_MISMATCH`). Efek yang dituju `app-entry-gate.md` (kafe 10.21/10.22) tertutup tanpa skema baru. Plan `docs_internal/plan/app-scoped-login.md` D6/D7. ✅ 2026-10-02 · changelog `2026-10-02-006`
- [x] 14.c.5 Change Password di permukaan App — route `change-password` hanya ada top-level untuk `_admin`, sedangkan permukaan App (`root_url` bebas) tak bisa dideklarasikan statis → item user menu menabrak catch-all "Page not found" (juga return `/{ws}/_admin` hardcoded setelah sukses). Didaftarkan di `<Routes>` bersarang `SurfaceShell` sebagai `surfacePath − mountPrefix + "/change-password"`; sukses kembali ke root permukaan aktif. ✅ 2026-10-02 · changelog `2026-10-02-005` · plan `docs_internal/plan/auth-screens-app-surface.md`

## Fase 15: Migrasi Otomatis + Gerbang Perubahan Destruktif ✅ (2026-09-16)

**Goal**: `kind: Migration`/`DataMigration` dicabut; migrasi struktural murni
otomatis dari diff Entity, tapi setiap perubahan **dinilai** — lossy ditolak
sampai manifest menyatakannya.
**Plan**: `docs_internal/plan/migration-destruktif-otomatis.md` · **Changelog**:
`docs_internal/changelog/2026-09-16-012-migrasi-otomatis-dan-gerbang-destruktif.md`
**Keputusan (2026-09-16)**: flag `removed` (bukan `deleted`); tabel **tanpa**
jalur otomatis (backup → drop manual → hapus manifest); hapus index otomatis +
notice; perbaikan data di luar spec sekali jalan.

- [x] 15.1 Klasifikasi diff + snapshot — `ChangeClass` (`additive`/`derived`/`lossy`/`never`), `DiffShapes`, `DesiredSnapshot`, tabel sistem `formspec_schema_snapshot`, `entityChecksum` (bentuk + deklarasi), bootstrap adopsi baseline. ✅ 2026-09-16
- [x] 15.2 Deklarasi destruktif — `Field.removed` + `reason`, `Field.accept_data_loss` + `reason`, validasi (alasan wajib, mutually exclusive dengan `renamed_from`, tidak boleh `required`); runtime membersihkan tombstone pada baca+tulis. ✅ 2026-09-16
- [x] 15.3 `persist.raw_ddl` — `RawDDLDecl` (`ddl` | `ddl_by`, `reason`), DDL-only, checksum-recorded, forward-only, ikut jalur sync otomatis. ✅ 2026-09-16
- [x] 15.4 Gerbang + preflight — refus**a**l sebelum pernyataan pertama (dev & prod sama), hitungan baris/grup duplikat/baris gagal cast, `never` untuk DROP TABLE, prune snapshot `ForgetOnly`. ✅ 2026-09-16
- [x] 15.5 Cabut kind — `MigrationSpec`, `DataMigrationSpec`, `ValidateMigrationSpec`, `MigrationDialects`, loader/schema/kind-doc/genjsonschema/genkinddocs, verb `migrate data`; tambah `formspec repl -f`. ✅ 2026-09-16
- [x] 15.6 Dokumen & artefak — `01-core-basic.md` §4 ditulis ulang (§4.1–§4.4), `04-persist-backend.md`, `03-kind-system.md` (11 → 10), cli-tools, glossary, `03-migration-engine.md` (Outline → Draft), `ai_skills` + vendored, `.github/skills/backend`, example kafe, schema diregenerasi. ✅ 2026-09-16
- [x] 15.7 **Gap ditemukan saat 15.4 — DIPERBAIKI 2026-09-20 (kafe TODO 3.11).** Kolom turunan yang ditambahkan **setelah** tabel dibuat di SQLite adalah kolom biasa yang tidak pernah terisi: modernc tidak bisa `ALTER TABLE ADD COLUMN ... GENERATED ALWAYS`, jadi `diffExistingTable` menambah kolom polos. Akibatnya index atas kolom itu tidak menegakkan apa pun sampai baris ditulis ulang — dan unique index yang baru dibuat bisa lolos dari duplikat lama. Preflight **tidak** terpengaruh (ia menghitung payload lewat `json_extract`), jadi penolakannya benar; yang belum benar adalah penegakan sesudahnya.
      _Accept:_ salah satu — (a) isi kolom turunan dari payload saat kolom ditambahkan (`UPDATE ... SET _f = json_extract(data,'$.f')`), atau (b) buat ulang tabel bila perlu, atau (c) nyatakan batasannya di validate/docs dan tolak index unik atas kolom yang belum materialized. Bukti: `TestMigrate_UniqueIndexBlockedByDuplicates` (komentar "Not asserted here").
      **Bukti E2E tambahan (2026-09-20, walkthrough kafe 9.4 → kafe TODO 3.11):**
      pada DB kafe yang sudah ada, tabel `cafe_order_shifts` berisi
      `_branch_id text GENERATED ALWAYS AS (json_extract(data,'$.branch_id')) STORED, _cashier_id text`
      — `_cashier_id` ditambahkan lewat ALTER (kolom biasa) dan **tidak pernah
      terisi** (NULL). Akibatnya partial unique index aturan bisnis #10
      `(branch_id, cashier_id) WHERE status='open'` **tidak menendang**: dua shift
      `open` untuk (cabang, kasir) yang sama diterima lewat API (**201**, harusnya
      ditolak). Test DDL yang ada lolos karena membuat tabel dari nol (kedua
      kolom GENERATED), jadi jalur ALTER tidak pernah tersentuh — itu sebabnya
      bug ini baru terlihat saat walkthrough di DB nyata.
      **✅ Perbaikan (2026-09-20):** SQLite menolak `ADD COLUMN ... STORED` tetapi
      **menerima** varian **VIRTUAL** — kolom yang dihitung saat baca, jadi benar
      untuk baris lama maupun baru. `addDerivedColumnSQL` kini memakai
      `GENERATED ALWAYS AS (...) VIRTUAL` di SQLite (PostgreSQL tetap STORED);
      ekspresinya dibagi satu sumber dengan jalur CREATE TABLE
      (`generatedColumnExpr`), jadi kolom ALTER dan kolom CREATE tidak mungkin
      berbeda. Kolom polos peninggalan bentuk lama dideteksi lewat introspeksi
      (`generatedColumns`: SQLite `pragma_table_xinfo.hidden IN (2,3)`,
      PostgreSQL `information_schema.columns.is_generated='ALWAYS'`) dan
      **dibangun ulang** oleh `diffExistingTable` — DROP index dependen dulu
      (SQLite menolak DROP COLUMN yang masih dirujuk index), DROP COLUMN, ADD
      COLUMN generated, CREATE index kembali. Rekonseilasi storage ini juga
      berjalan di jalur "checksum sama" (manifest tak berubah), karena checksum
      mem-fingerprint manifest, bukan storage. Dua test pengunci:
      `TestMigrationRunner_AlteredDerivedColumnEnforcesUnique` (ALTER path
      menegakkan unique) dan `TestMigrationRunner_StaleDerivedColumnIsRepaired`
      (DB lama dengan kolom polos direncanakan diperbaiki lalu konvergen).
      **Bukti E2E pada DB kafe lama:** `migrate apply` → `Applied 8 migration(s)`
      (semua `storage_drift`); `migrate plan` → `No pending migrations`; kolom
      `_cashier_id`/`_branch_id` kini `hidden=2` (VIRTUAL) dan **terisi**;
      INSERT shift `open` kedua untuk (cabang, kasir) sama → **REJECTED**
      `UNIQUE constraint failed` (aturan bisnis #10 ditegakkan); shift kasir
      lain dan shift `closed` tetap diterima (partial index benar).
- [x] 15.8 **Verifikasi PostgreSQL jalur baru — DIJALANKAN NYATA 2026-09-21 (PG 17, instance user), TUNTAS.** Preflight (`jsonb_exists(data,'x')`), strip (`data - 'x'`), `DROP COLUMN`, `DROP INDEX <schema>.<name>`, schema kategori, **dan `ddl_by: postgres`** terverifikasi di DB sungguhan; `formspec migrate apply` kafe penuh (24 entitas) → konvergen; jalur destruktif round-trip lengkap. **`ddl_by: postgres` (sisa terakhir, 2026-09-21):** dideklarasikan di spec copy kafe (`raw_ddl` dengan `ddl_by` dua dialect, PG memakai `upper(data->>'code')`) → `[additive] raw_ddl_added` → apply → index `idx_branch_probe_pg` ada di PG dengan definisi `upper((data ->> 'code'::text))` yang benar; **varian SQLite tidak pernah jalan di PG** (`count(*)=0`) dan sebaliknya varian PG tidak jalan di SQLite; jalur SQLite → varian sqlite-nya dibuat. Semantik **forward-only** terverifikasi: deklarasi dihapus → plan mencatat `[derived] raw_ddl_removed … the DDL stays applied (forward-only)` → index tetap ada di PG, konvergen. **9 bug ditemukan, semuanya kelas "diam-diam salah" atau "jalur mati" — semuanya diperbaiki:** 1. **DDL tabel sistem memakai nama TIPE sebagai DEFAULT** (`applied_at timestamptz NOT NULL DEFAULT timestamptz`): PG menolak ("column reference in DEFAULT"); SQLite menerima quirk dan menyimpan literal string `"text"` sebagai timestamp — **DB SQLite yang sudah ada menyimpan `applied_at='text'` dan `updated_at='text'`** (terbukti di kafe.db). Fix: `currentTimestampFn` (SQLite `datetime('now')`, PG `now()`). 2. **`gen_uuid_v7()` tidak ada di PG ≤17** → `gen_random_uuid()` (built-in PG 13+), selaras dengan aturan ddl.go. 3. **`existingColumns` dengan schema kosong** match nol baris di PG → cek kolom selalu "tidak ada" → ALTER escalated_steps gagal "already exists". Fix: `COALESCE(NULLIF($1,''), current_schema())`. 4. **Schema kategori tidak pernah dibuat** → fix: `CREATE SCHEMA IF NOT EXISTS` semua `CategorySchema` di `EnsureSystemTables`. 5. **CHECK enum memakai `json_extract`** (SQLite-only) unconditional → `payloadExpr(driver, …)` yang driver-aware. 6. **pgx stdlib tidak mendukung placeholder `?`** — seluruh kode persist memakai `?`: statement pertama yang parameterized gagal. Fix: rewriter `pgRewritePlaceholders` (`?` → `$n`, string-literal-safe, positional stabil) di `PostgresDB` + `pgTx`; operator jsonb `data ? 'x'` diganti bentuk fungsinya `jsonb_exists(data,'x')` agar tidak tertukar dengan placeholder. 7. **Ekspresi kolom generated PG bertipe text** (`data->>'f'`) untuk kolom `timestamptz`/`numeric` → PG menolak ("type … default expression is of type text"); `::timestamptz` inline juga ditolak ("generation expression is not immutable" — cast text→timestamp/date bergantung DateStyle GUC). Fix: cast non-text via cast inline, date/time lewat fungsi IMMUTABLE baru `formspec_to_timestamptz`/`formspec_to_date` (dibuat idempoten di EnsureSystemTables; numeric/boolean/uuid cast sudah immutable). 8. **`tenant_id`/`created_by`/`updated_by` bertipe `uuid`** di PG — app layer menyimpan slug workspace (`kafe`) dan `"anonymous"` → setiap INSERT gagal "invalid input syntax for type uuid". Fix: tiga kolom itu `text` di kedua dialect. 9. **Inline partial UNIQUE** (`UNIQUE (…) WHERE deleted_at IS NULL` dalam CREATE TABLE) bukan sintaks PG → partial kini lewat `CREATE UNIQUE INDEX … WHERE` (sama dengan jalur S8); index tanpa predikat tetap inline constraint. 10. **Normalisasi indexdef PG tidak lengkap** — PG menulis `((_status)::text = 'open'::text)`; pembanding bentuk memotong `::` sebelum tanda kutip sehingga membandingkan `'open'` dengan `'open` → index enum-predicate dilaporkan drift selamanya. Fix: strip token cast yang diketahui (bukan potong sejak `::`) + buang semua parens pada kedua sisi. Test pengunci: `TestIndexShapeOf_PostgreSQLIndexDef`.
      **Bukti E2E:** fresh DB PG → `Applied 24 migration(s)` → insert baris dengan `tenant_id='kafe'` + `created_by='anonymous'` berhasil → plan re-add/drop oscillation pada field tombstone ber-index ditemukan & diperbaiki (skip `Removed` di `derivedColumnFields` + generator DDL) → round-trip destruktif: field_added → field_removed **ditolak** `(1 row(s) affected)` → dideklarasikan → strip `data - 'x'` → `DROP COLUMN` + `DROP INDEX` → konvergen (PG & kedua DB SQLite tetap `No pending migrations`).
      **Sisa:** DB SQLite yang sudah ada masih menyimpan timestamp `"text"` di kolom sistem (perbaikan butuh rebuild tabel; nilai itu tidak dibaca logika); runtime penuh `formspec dev` di PG belum diuji (query runtime sudah driver-aware; rewriter `?`→`$n` menjangkau semuanya).
      **Setup PG verifikasi:** instance user-sendiri (PG 17, port 55432, `initdb -U vscode --auth=trust`) — cluster milik root tidak bisa dikendalikan (sudo interaktif tidak tersedia).
- [x] 15.9 **Adopsi di aplikasi nyata** — pada spec kafe (69 manifest, tanpa manifest Migration): `formspec validate --schema schemas` **0 problem**; `migrate plan` → 24 perubahan aditif; `migrate apply` → 24 diterapkan; `migrate plan` lagi → **No pending migrations** (konvergen). Gerbangnya diuji pada salinan spec di `/tmp`: hapus field `sort_order` tanpa deklarasi → plan **exit 1** dengan pesan `field_removed`; setelah `removed: true` + `reason` → apply mencetak `(1 row(s) affected)` dan `data` benar-benar menjadi `{"name":"Kopi"}`. Sisa (walkthrough 9.4 penuh 9 skenario) tetap di Fase 9 kafe. ✅ 2026-09-16
- [x] 15.10 **Gap baru (temuan walkthrough skenario 6 kafe, 2026-09-21 — DIPERBAIKI):** workflow approval interception TIDAK pernah ada di jalur PATCH. `wfEngine.RequiresApproval` hanya dipanggil di `HandleCustomAction`; `HandleUpdate` langsung `store.Update`. Transisi **tanpa `impl`** hanya bisa dicapai lewat PATCH (route `/{id}/{action}` hanya untuk action ber-`impl`), jadi workflow yang mengawal transisi semacam itu selalu bypass — void pesanan `paid` langsung `cancelled` tanpa approval (dibuktikan di spec kafe: 200, bukan 202). Bug kedua yang menyembunyikan akar: `merged := current.Data` bukan salinan — merge loop menulis ke current.Data juga, sehingga state asal yang dibaca setelah merge selalu == state tujuan; deteksi "state crossed" mustahil menyala dan diagnosis pertama salah mengira lookup-nya salah.
      **Perbaikan:** `StateMachineEngine.FindTransitionByStates(from, to)` (reverse lookup — PATCH membawa state tujuan, bukan nama transisi; transisi multi-asal seperti `void-order` dengan 4 state asal tak bisa diidentifikasi dari satu state); `HandleUpdate` memeriksa transisi yang dilintasi lewat `preUpdateState` yang diambil SEBELUM merge dan merutekan ke `handleWorkflowApproval` (handler yang sama dengan jalur custom action); `decision` dibuang dari payload record (verb approval, bukan field entity).
      **Bukti:** kasir ajukan void → **202 approval_required**; non-holder → **403 WORKFLOW_DENIED**; supervisor role `cafe-order.supervisor` approve → **transition_completed**, order `cancelled` + `void_reason` tersimpan; baris `formspec_workflow_approval` tercatat. Test pengunci: `TestFindTransitionByStates` (termasuk transisi multi-asal). `go test ./...` hijau · `make lint` 0 issues. Changelog `2026-09-21-001`.
      **Catatan (pre-existing, bukan dari fix ini):** setelah quorum dan transisi dieksekusi, baris approval tetap `status: pending` meski tanda tangan approver tercatat — sama ada di jalur custom action maupun PATCH (handler yang sama). Sisa kecil untuk fase workflow berikutnya.
- [x] 15.11 **Sisa 15.10 — status baris approval setelah quorum (DIPERBAIKI 2026-09-21).** Baris `formspec_workflow_approval` tetap `status: pending` setelah semua step disetujui dan transisi dieksekusi, padahal `Reject` mengeset `rejected` — baris `approved` yang hanggung itu juga yang di-scan escalation worker, jadi approval yang sudah selesai bisa meng-eskalasi selamanya. Fix: set `approval.Status = ApprovalApproved` ketika `AllStepsApproved` (sebelum persist), di `handleWorkflowApproval` yang dipakai kedua jalur.
      **Bukti E2E (spec kafe, skenario 6):** approve → `transition_completed` + baris `status = approved` (sebelumnya `pending`); jalur reject juga diverifikasi — reject → `rejected`, order **tetap `paid`** (semantik `on_reject.to`: transisi tidak pernah dieksekusi, state asal tidak pernah berubah). `go test ./...` hijau · `make lint` 0 issues. Changelog `2026-09-21-001`.
- [x] 15.12 **Skenario 8 kafe — jurnal GL otomatis dari event `on_paid` (TUNTAS 2026-09-21).** Desain pemilik: order hanya memancarkan event durable; module `gl` mendengarkan lewat `kind: Subscription` dan membangun + posting jurnal. Jurnal tidak seimbang = setting akun GL belum lengkap = tanggung jawab `gl` (error `FORMSPEC.GL.*`). Module `cafe-gl-integrator` + app-nya dihapus (akan double-post). **11 bug mesin ditemukan & diperbaiki** — dua terparah: (a) **`fail()` tidak menghentikan script**, hanya mengembalikan nilai yang dibuang, jadi setiap guard `if bad: fail(...)` di seluruh ekosistem adalah no-op (script lanjut, record setengah jadi dibuat, `ok()` dilaporkan); (b) **`valuesEqual` memakai `==` pada slice → panic proses**, mematikan outbox worker saat update jurnal. Sisanya: `emit` dibuang `UnmarshalYAML`; `resource.call` tak bisa menargetkan satu record; `sum_line(field)` (bentuk terdokumentasi) tidak ada; env guard menolak `starlark.Value`; guard hanya menerima `lines` `[]any`; `event.payload` tak pernah memuat `id`; `ctx.config` menutupi nilai Config manifest dengan store KV kosong; `ChildrenExtract` hanya menerima `[]any`; `emits` tak pernah diresolusi di jalur `resource.call`. **Bukti E2E:** `ORD-2026-00021` → 1 jurnal `JRN-2026-000055` (`source_id` terisi, idempoten) · 4 baris di tabel child · Kas debit 143750 = Omzet 125000 + Pajak 12500 + Service charge 6250 · `status = posted` · `journal-posted` terbit. `go test ./...` hijau · `make lint` 0 issues · `formspec validate` 78 manifest 0 problem. Changelog `2026-09-21-003`.
      **Sisa dicatat — DITUTUP 2026-09-21 (changelog `2026-09-21-004`):** `deliver: target: {resource, action}` kini benar-benar memanggil action target (channel `reliable_event` = sync call + retry outbox, sesuai kontrak §12.2), dan proyeksi `gl-balance` hidup: POST → 4 baris saldo benar (Kas 143750, Omzet 125000, Pajak 12500, Service charge 6250), REVERSE → kembali 0. **5 bug ditemukan di jalur consequence-nya:** (a) `reliable_event` target hanya enqueue, tidak pernah call; (b) `payload.fields: [id]` menghasilkan id null (id = kolom tabel, bukan field) → konsumen yang meng-address record mati; (c) `journal-reversed` menargetkan action `gl.gl-balance.reverse` yang tidak ada → retry ke dead-letter, proyeksi tak pernah ter-update, **validate hijau**; (d) `gl_balance_update.star` salah tanda closing untuk akun kredit-normal (Omzet tercatat −125000); (e) `condition: resource.status == 'posted'` gagal (_"dict has no .status field"_) karena `resource` disuntik sebagai map mentah, bukan `FieldMap` — action `reverse` tidak pernah bisa jalan. Validasi cross-manifest baru (`validate_events.go`) menolak target action yang tidak ada / tidak idempotent, menutup kelas bug (c). Item 6.3 tetap deferred tetapi skenario 8 tidak bergantung padanya.
      **Sisa baru — dipindahkan ke item terlacak (2026-09-21):** idempotency retry `deliver.target` → **7.7.5 ✅ (ditutup 2026-10-05, changelog `2026-10-05-004`)** (terbukti: requeue event yang sama menggandakan pergerakan, 143750 → 287500); `webhook`/`notification` channel → **7.7.6 ✅ (ditutup 2026-10-06, changelog `2026-10-06-003`)** — keduanya kini terkirim, sisa turunannya (signature `webhook`, blok Tier-2 inert) menjadi **7.7.7 ⏸️** / **7.7.8 ⏸️**. Keduanya item bernomor yang bisa di-grep, bukan prosa di bawah item `[x]`.

## Fase 16: DX Dev Container — Lint & Cache Go ✅ (2026-09-17)

**Goal**: target yang harus me-load seluruh package graph (`lint`, dan nanti `test`)
tidak lagi tampak freeze setelah Rebuild Container.
**Plan**: `docs_internal/plan/devcontainer-go-cache-lint.md` · **Changelog**:
`docs_internal/changelog/2026-09-17-001-lint-warmup-dan-cache-go-persisten.md`
**Akar**: `golangci-lint` me-load package graph lewat `go list` sebelum menganalisis dan
**tidak mencetak apa pun** selama itu, sementara cache modul container cold (~200 MB,
≈1 GB setelah ekstrak) dan unduhannya bisa stall tanpa timeout — tampak seperti hang.

- [x] 16.1 Warm-up + batas waktu — target `deps-warm` (`go mod download`, progress `go: downloading …` terlihat) sebagai prerequisite `lint`; `golangci-lint run --timeout 10m ./...` (v2: `--timeout` disabled by default). ✅ 2026-09-17
- [x] 16.2 Cache Go persisten — named volume `go-mod-cache` → `/go/pkg/mod` + `go-build-cache` → `/home/vscode/.cache/go-build` di `.devcontainer/compose.yaml`; aktif setelah _Rebuild and Reopen in Container_. ✅ 2026-09-17
- [x] 16.3 **Working tree tidak bisa di-build — WIP di `pkg/spec` menghapus API yang masih dipakai (bukan soal cache).** ✅ **2026-09-22 — TIDAK BERLAKU LAGI (stale).** WIP yang dilaporkan sudah tidak ada di tree: `git diff --numstat pkg/spec` **kosong** (bukan `resources.go 5 233`), `go build ./...` **exit 0**, `go test ./pkg/spec/` **ok**, dan API yang dilaporkan hilang ada kembali — `spec.ValidateWorkflowSpec`/`spec.ValidateModuleSpec` (`pkg/spec/resources.go`) serta `WorkflowTransitionRef.ByName()` (`pkg/spec/workflow_test.go`), `ModuleSpec.Runtime` (`pkg/spec/resources.go:26`). Artinya item ini menggambarkan kondisi uncommitted-working-tree sesaat (kemungkinan besar `pkg/spec` sedang setengah-edit saat catatan ditulis) yang teratasi saat perubahan itu di-commit/di-revert pada `06f1284` ("fixings cafe gaps"). `go test ./...` untuk seluruh repo juga hijau, jadi temuan menyertai "`go test ./cmd/formspec` gagal build sejak `2d816b4`" di Fase 12 **juga sudah tidak berlaku** (file `migrate_dialect_test.go` sudah tidak ada; `go vet ./cmd/formspec/` exit 0). Tidak ada aksi lanjutan.

      <!-- Riwayat temuan asli (2026-09-17) dipertahankan di bawah untuk konteks audit. -->
      _Accept (asli):_ tentukan sadar — (a) kembalikan API yang hilang ke `pkg/spec` (WIP-nya lanjut apa adanya), atau (b) migrasikan pemakainya (`internal/manifest`, `internal/workflow`, plus test) bila penghapusan itu memang disengaja. Bukti (asli): `go build ./...` exit 1; `git diff --numstat pkg/spec` → `resources.go 5 233`.

## Fase 17: Auth Form Autofill Compliance (Chromium guidance) ✅ (2026-09-18)

**Goal**: seluruh form auth FormSpec patuh "Create Amazing Password Forms" agar
password manager bisa memasangkan, mengisi, menyimpan, dan memperbarui
kredensial dengan benar.
**Plan**: `docs_internal/plan/auth-form-autofill-chromium.md` · **Changelog**:
`docs_internal/changelog/2026-09-18-009-form-auth-autofill-chromium.md`
**Akar**: form register memakai token `current-password`; auth form spec-driven
tidak punya `autocomplete` sama sekali dan `<label htmlFor>`-nya menunjuk id yang
tidak pernah dirender; sisa token tidak valid `"nope"` dari hack lama
(2026-09-09-007 hanya membersihkan `Input`, bukan `textarea`/wizard).

- [x] 17.1 `LoginScreen` — `autocomplete` kondisional login/register, `name` pada semua input, `<form key={mode}>`, hidden `workspace`/`app` saat berasal dari URL. ✅ 2026-09-18
- [x] 17.2 `AuthFormRenderer` — tabel konvensional `AUTOCOMPLETE_BY_ACTION[auth_action][field]`, `autoComplete="on"`, teruskan `id`/`name`/`autoComplete` (sekaligus memperbaiki label↔input yang tidak terhubung). ✅ 2026-09-18
- [x] 17.3 Widget — `PasswordInput`/`TextInput` meneruskan `name`+`autoComplete`; tombol reveal password dapat `aria-label`/`aria-pressed`. ✅ 2026-09-18
- [x] 17.4 Bersihkan sisa "fool the browser" — `components/ui/textarea.tsx` dan 2 komponen wizard bebas `"nope"`; field password entity dapat `new-password`. ✅ 2026-09-18
- [x] 17.5 Layar auth lain mengikuti pola yang sama — `SetupScreen`, `ResetPasswordScreen`, `ChangePasswordPage`, `ChangePasswordDialog` (`name` + hidden `workspace`; reset juga hidden `token` dari `?reset_token`). ✅ 2026-09-18
- [x] 17.6 Test regresi + docs — `shell/LoginScreen.test.tsx` (5 test) + `shell/auth-screens.autofill.test.tsx` (3 test), `docs/kind/ui/Form.md` §Auth Forms, `.github/skills/formspec-frontend/SKILL.md`. ✅ 2026-09-18
- [ ] 17.7 **Belum diverifikasi di browser nyata.** Klaim "Chrome memasangkan username+password lalu menawarkan simpan" baru diuji lewat DOM assertion (vitest/jsdom); autofill Chrome sesungguhnya, `?mode=register`, dan login app-scoped (`/{ws}/app/{app}`) belum dijalankan manual.
      _Accept:_ walkthrough manual di Chrome dengan password manager aktif untuk `/login`, `/register`, `?mode=register`, dan `/{ws}/app/{app}/login`.

## Deferred (Cloud Phase)

| Area                                                                                                                                                                                                                                               | Reason                                                                                                                                                                                                                                                                                                                                                                                              |
| -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `formspec-ctl` (all modes: region, cluster, standalone)                                                                                                                                                                                            | Control Plane — cloud deployment phase                                                                                                                                                                                                                                                                                                                                                              |
| K8s Operator (`formspec-operator`)                                                                                                                                                                                                                 | Production infrastructure                                                                                                                                                                                                                                                                                                                                                                           |
| Marketplace (pricing, metering, licensing)                                                                                                                                                                                                         | Business features — registry foundation (upload/download/listing) ada di **Fase 13.3**; lapisan pricing/metering/licensing tetap deferred (13.3.9)                                                                                                                                                                                                                                                  |
| Control Plane (Environment, Policy/OPA, transparency log, key model, contracts)                                                                                                                                                                    | Governance                                                                                                                                                                                                                                                                                                                                                                                          |
| Two-stage deployment pipeline (register→deploy, snapshot, evidence)                                                                                                                                                                                | Requires Control Plane                                                                                                                                                                                                                                                                                                                                                                              |
| `formspec promote/archive/saga/script/freeze/rollback/lock/workspace/suspend`                                                                                                                                                                      | CLI — depend on Control Plane. **`module`/`sign` sudah direncanakan di Fase 13**                                                                                                                                                                                                                                                                                                                    |
| Module vendoring & activation — `vendors/`/`external/` folders, `formspec.lock`, install-time alias on name conflict, marker-based activation (`--use`), shadow copy (`formspec override adopt/diff`) for `Form`/`Menu`/`VisualSpecKind` instances | **Dipindah ke Fase 13** (13.1–13.2) — planned 2026-08-20. Design agreed (`docs/spec/platform/08-project-layout.md` §6, `docs/spec/platform/02-workspace-app-module.md` §2.1). **Sebagian sudah landing untuk auth**: `external/` (module user-kustom, di-commit) di-load loader + menang atas `formspec.core` defaults; `formspec generate auth` meng-scaffold auth module ke `external/auth` (6.1) |
| `formspec consult` task breakdown — see **Fase 10** above                                                                                                                                                                                          | Zero implementation; realistically starts after Module vendoring or Marketplace (below) lands                                                                                                                                                                                                                                                                                                       |
| Conformance test-suite VisualSpecKind/Renderer/PersistBackend (fixture, trust tier `verified`/`official`)                                                                                                                                          | Terkait Marketplace/distribusi — `frontend/02` §6, `frontend/03` §5, `backend/04` §7                                                                                                                                                                                                                                                                                                                |
| Print: thermal/dotmatrix                                                                                                                                                                                                                           | Niche — PDF sufficient                                                                                                                                                                                                                                                                                                                                                                              |
| gRPC + mTLS transport                                                                                                                                                                                                                              | Cloud deployment                                                                                                                                                                                                                                                                                                                                                                                    |
| Platform signing (HSM/KMS)                                                                                                                                                                                                                         | Cloud deployment                                                                                                                                                                                                                                                                                                                                                                                    |
| Generic Docker image (`formahub/formspec-resource`)                                                                                                                                                                                                | Cloud deployment                                                                                                                                                                                                                                                                                                                                                                                    |
| Scale-to-zero                                                                                                                                                                                                                                      | Cloud deployment                                                                                                                                                                                                                                                                                                                                                                                    |
| Unmanaged client codegen (Dart, Flutter)                                                                                                                                                                                                           | Future SDK                                                                                                                                                                                                                                                                                                                                                                                          |
