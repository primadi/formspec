# 2026-08-31-003 — Cleanup unused symbols (staticcheck U1000)

## Apa yang diubah

Menghapus / menandai 11 simbol yang terdeteksi unused oleh `staticcheck -checks U1000`:

**Dihapus (benar-benar tidak dipakai):**

- `internal/api/api_test.go` — `setupAPIEnv` (superseded oleh `setupTestRegistryWithExpose`)
- `internal/api/middleware.go` — `generateRequestID` (superseded oleh `observability.NewRequestID`, todo 8.2.3)
- `internal/auth/user.go` — `ensureEntityUserStore` (wrapper redundan; `NewEntityUserStore` dipakai langsung)
- `internal/workflow/engine.go` — `splitEntityRef` (+ import `strings`)
- `internal/workflow/escalation.go` — type alias `specWorkflowStep` (+ import `spec`)
- `renderers/jsonb-persist/datastore/memory/memory.go` — field `PubSub.seq`
- `renderers/jsonb-persist/sqlite_db.go` — field `SQLiteDB.mu` (+ import `sync`; koneksi dibatasi `MaxOpenConns(1)` sehingga tidak perlu mutex)

**Dipertahankan sebagai deferred (dibutuhkan di masa depan), diberi catatan + assignment `var _ =` agar tidak warning:**

- `internal/manifest/loader.go` — `Loader.applyAliases`: bagian fitur module aliasing (todo 13.1.4), belum di-wire ke pipeline `Load()`
- `internal/api/middleware.go` — `writeUsesViolation`, `writeConfigAccessDenied`, `writeKvstoreAccessDenied`: helper error code untuk uses enforcement Fase 2 (§16)

## Kenapa

Membersihkan warning unused agar codebase tetap bersih tanpa menghilangkan kontrak yang masih ditunggu fitur deferred.

## File terdampak

- `internal/api/api_test.go`, `internal/api/middleware.go`
- `internal/auth/user.go`
- `internal/manifest/loader.go`
- `internal/workflow/engine.go`, `internal/workflow/escalation.go`
- `renderers/jsonb-persist/datastore/memory/memory.go`, `renderers/jsonb-persist/sqlite_db.go`

## Verifikasi

`go build ./...`, `go vet ./...`, `staticcheck -checks U1000 ./...` bersih; 480 test di 9 paket terdampak lulus.

---

## Lanjutan: unused parameter (gopls unusedparam)

Tahap kedua — parameter fungsi/method yang tidak pernah dipakai di body (warning gopls `unusedparam` di Problems panel). Diverifikasi manual: yang merupakan implementasi interface / func value (gopls tidak flag) dibiarkan; yang dipanggil langsung diperbaiki.

**Dihapus dari signature:**

- `internal/api/handler.go` — `handleWorkflowApproval`: param `actionName` (tidak pernah dipakai; audit memakai nama event hardcoded) + call site di `HandleCustomAction` diperbarui

**Direname menjadi `_` (deferred — bagian dari kontrak signature, catatan di doc comment):**

- `internal/api/handler.go` — `executeWorkflowTransition` (`r`), `HandleJobStatus` (`module`, `serviceName`), `HandleWebhook` (`module`, `name`), `HandlePrepare` (`module`, `entity`, `actionName`)
- `internal/action/deliver.go` — `runBeforeDeliver` (`resource`)
- `internal/job/tracker.go` — `publish` (`ctx`, `jobID`)
- `internal/auth/materialize.go` — `blockFootprint` (`component`)
- `resource/formspec.go` — `newDispatcher` (`database`, `sharedPubSub`)
- `cmd/formspec/check.go` — `checkExpr` (`owner`)
- `cmd/formspec/honesty.go` — `applyHonestyFix` (`manifests`)
- `internal/api/router.go` — `serveFileFS` (`r`)
- `renderers/jsonb-persist/ddl.go` — `GenerateExtensionDDL` (`meta`)
- `internal/subscription/stream.go` — `deadLetter` (`group`)
- `internal/workflow/engine.go` — `CanApprove` (`resourceData`)

**Dibiarkan (implementasi interface / func value / build-tag — bukan temuan gopls):**

- `internal/artifact/store.go` (Store iface), `internal/action/dispatcher.go` Info/Warn/Error (Logger iface), `internal/starlark/primitive.go` CallInternal (Callable), `cmd/formspec/repl.go` builtin (func value), `renderers/jsonb-persist/sqlite_db.go` HasTable (DB iface), `internal/stream/memory.go` Read (Stream iface), `internal/auth/jwt.go` Validate, `internal/sidecar/server.go` handleHealth (http.HandlerFunc), `cmd/formspec/procattr_windows.go` (build tag windows)

### Verifikasi lanjutan

`go build ./...` OK; 929 test di 47 paket lulus; scan AST unused-param bersih untuk semua fungsi yang dipanggil langsung.
