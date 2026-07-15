# Forma API Layer — `internal/api`

**Category:** Implementation Documentation  
**Status:** Partially implemented — REST routing, D49 expose gating, and standard/custom handlers are live; workspace-slug resolution, versioned paths, smart internal dispatch, gRPC, and WebSocket are still planned  
**Package:** `github.com/primadi/forma/internal/api`  
**Implements:** Core §11.1, §16 / D49, D50

---

## 1. Overview

`internal/api` generates and serves API endpoints from registered entity manifests. It implements the deny-by-default exposure model (D49) with multi-protocol routing (D50).

### Architecture

```
EntitySpec.expose → RouteDescriptor → Router → Protocol Adapter → Handler
                          │                ├── REST (chi)
                          │                ├── gRPC (grpc-go)
                          │                └── WebSocket (gorilla/ws)
                          │
                          └── Smart dispatch:
                              same-process → direct function call (bypass net)
                              cross-process → gRPC/HTTP to remote Resource Plane
```

---

## 2. Implementation Decisions

### 2.1 Router: `go-chi/chi`

| Criteria | Choice | Rationale |
|---|---|---|
| Library | `github.com/go-chi/chi/v5` | Radix-tree router, O(path segments) lookup |
| Performance | ~10K routes = same latency as 1 route | Radix tree, not linear scan |
| Compatibility | 100% `net/http` compatible | Standard middleware + handler signature |
| Middleware | Native chaining | `r.Use()` for tenant isolation, auth, CORS |
| Adoption | Widely used (18K+ GitHub stars) | Mature, well-documented |

Alternatives considered: `httprouter` (faster but no native middleware chaining), `gin` (opinionated framework, heavier), `net/http` stdlib (linear scan, unacceptable at scale).

### 2.2 Workspace Prefix

Every route is namespaced under a workspace identifier:

```
/{workspace_slug}/api/{version}/{module}/{plural}/:id/:action
```

- `workspace_slug` — human-readable alias set per Workspace; falls back to UUID
- The router resolves slug → workspace UUID via the registry before dispatching
- Middleware injects `workspace_id` + `tenant_id` into request context

> **Status:** planned. The current implementation captures `{workspace}` but mounts routes at a hardcoded `/api/v1` prefix (`router.go`); slug→UUID resolution is not yet implemented.

### 2.3 Smart Internal Dispatch

Tidak semua call harus lewat network. Router mendeteksi kalau caller dan target entity ada dalam satu process:

```go
// Router checks registry locality
if registry.IsLocal(callerModule, targetEntity) {
    // Direct call: skip HTTP, zero serialization overhead
    store := registry.GetEntityStore(targetModule, targetEntity)
    return store.GetByID(ctx, params)
}
// Cross-process: use protocol adapter
return restAdapter.Dispatch(ctx, req)
```

> **Status:** planned. `registry.IsLocal` and the direct-dispatch path do not exist yet.

---

## 3. Route Descriptor

Protocol-agnostic intermediate representation:

```go
type ProtocolType string

const (
    ProtocolREST       ProtocolType = "rest"
    ProtocolGRPC       ProtocolType = "grpc"
    ProtocolWebSocket  ProtocolType = "ws"
)

type RouteDescriptor struct {
    Module             string            // module name
    Entity             string            // entity name
    Plural             string            // table plural (for path building)
    Action             string            // list, find, create, update, delete, or custom
    Method             string            // GET, POST, PATCH, DELETE
    Path               string            // REST path template relative to workspace prefix
    Protocol           spec.ProtocolType // rest, grpc, ws
    Handler            string            // "auto" for CRUD, or custom action name
    RequiredPermission string            // e.g. "billing.customers.list"; empty = no check
}
```

`ProtocolType` and `ExposeConfig` live in `pkg/spec/entity.go` (they are part of the manifest vocabulary, D49); `descriptor.go` re-exports the protocol constants. Workspace resolution is a middleware concern, not a descriptor field.

```go
```

### Generation

```go
func GenerateRoutes(registry *entity.Registry) []RouteDescriptor
```

Iterates all registered entities, checks `spec.expose`, generates `RouteDescriptor` per protocol per action.

---

## 4. REST Adapter (`chi`)

### Route Registration

```go
func (a *RESTAdapter) RegisterRoutes(r chi.Router, routes []RouteDescriptor) {
    for _, rd := range routes {
        if rd.Protocol != ProtocolREST {
            continue
        }
        pattern := fmt.Sprintf("/{workspace}/api/{version}/%s/%s", rd.Module, entityPlural)

        switch rd.Action {
        case "list":
            r.Get(pattern, a.handleList)
        case "find":
            r.Get(pattern+"/{id}", a.handleFind)
        case "create":
            r.Post(pattern, a.handleCreate)
        case "update":
            r.Patch(pattern+"/{id}", a.handleUpdate)
        case "delete":
            r.Delete(pattern+"/{id}", a.handleDelete)
        }
    }
}
```

### Response Envelope (Core §16)

```go
// List
{ "data": [...], "meta": { "page": 1, "per_page": 20, "total": 150, "total_pages": 8 }, "links": {...} }

// Single
{ "data": {...}, "meta": { "request_id": "req-abc", "timestamp": "2026-07-06T..." } }

// Error
{ "error": { "code": "NOT_FOUND", "message": "...", "details": [...] }, "meta": {...} }
```

Each `data` record is the entity's own fields flattened alongside the reserved framework columns (`id, tenant_id, version, created_at, updated_at, created_by, updated_by`) at the same level, snake_case — enforced by `EntityRecord.MarshalJSON` (`internal/db/crud.go`). Until that method was added, `EntityRecord`'s lack of JSON tags meant Go's default marshaling leaked PascalCase field names with entity data nested under a `"Data"` key — a real bug (no test exercised the full HTTP path to catch it), fixed while building the browser client SDK (`sdk/browser`, `docs/cli-tools/03-forma-generate.md`).

---

## 5. Handler Functions

### Standard handlers (auto-generated)

Setiap handler:
1. Parse workspace slug → inject `tenant_id`
2. Verify auth token → resolve identity + permissions
3. Validate input (JSON body / query params)
4. Call `EntityStore` method
5. Format response envelope
6. Record audit log (if `audit: true`)

### Custom handlers

Ketika `Action.Impl` di-set, handler mendispatch ke implementation yang sesuai (Starlark, native Go, sidecar) alih-alih auto-CRUD.

---

## 6. File Reference

| File | Purpose |
|---|---|
| `internal/api/descriptor.go` | `RouteDescriptor`, `StandardRESTActions`, protocol constant re-exports |
| `internal/api/generator.go` | `GenerateRoutes` / `GenerateCustomActionRoutes` — manifest → route descriptors |
| `internal/api/router.go` | chi router setup, workspace prefix, REST route binding |
| `internal/api/handler.go` | Auto-generated CRUD + custom-action handlers, response envelopes |
| `internal/api/middleware.go` | Tenant isolation, auth, CORS, uses-enforcement middleware |
| `internal/api/*_test.go` | Tests |
| `internal/api/adapter_grpc.go` | gRPC adapter (future — not yet created) |
| `internal/api/adapter_ws.go` | WebSocket adapter (future — not yet created) |

---

## 7. Dependencies

| Package | Version | Purpose |
|---|---|---|
| `github.com/go-chi/chi/v5` | v5 | Radix-tree HTTP router |
| `github.com/primadi/forma/internal/entity` | — | Entity registry |
| `github.com/primadi/forma/internal/db` | — | EntityStore CRUD |

---

> **Related Spec:** Core §11.1 (Standard actions), §16 (API Delivery), D49 (deny-by-default), D50 (multi-protocol router)
