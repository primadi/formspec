// Package sidecar implements the forma-sidecar side of the sidecar
// protocol (docs/runtimes/04-forma-sidecar.md): a local HTTP listener
// (unix socket or localhost TCP) that lets a non-Go app process call
// ctx.* primitives back into the engine, plus health aggregation over
// the app process.
//
// The wire contract mirrors internal/starlark's primitive contract
// (query/get/set/delete/acquire/release over db/cache/lock/... handles)
// so Starlark scripts and non-Go apps share the same backend.
package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// PrimitiveResolver resolves a primitive type ("db", "cache", "lock", ...)
// and datastore name ("default" or a named datastore) to a live connection.
// This is the exact contract of internal/starlark's CtxAPI.SetDatastoreResolver,
// so both execution paths resolve through the same registry.
type PrimitiveResolver func(primitiveType, name string) (any, error)

// Capability interfaces a resolved connection may implement. Operations on
// connections that do not implement the matching interface fail with 501 —
// the same "not yet implemented" surface the Starlark primitives expose today.
type (
	// Querier serves POST /ctx/{prim}/query.
	Querier interface {
		Query(ctx context.Context, sql string, args ...any) ([]map[string]any, error)
	}
	// KVGetter serves POST /ctx/{prim}/get.
	KVGetter interface {
		Get(ctx context.Context, key string) (any, error)
	}
	// KVSetter serves POST /ctx/{prim}/set.
	KVSetter interface {
		Set(ctx context.Context, key string, value any, ttl time.Duration) error
	}
	// KVDeleter serves POST /ctx/{prim}/delete.
	KVDeleter interface {
		Delete(ctx context.Context, key string) error
	}
	// Locker serves POST /ctx/{prim}/acquire and /ctx/{prim}/release.
	Locker interface {
		Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error)
		Release(ctx context.Context, key string) error
	}
	// EntityLoader serves POST /ctx/entity/get — fetch full record by ID.
	EntityLoader interface {
		Fetch(ctx context.Context, workspaceID, id string) (map[string]any, error)
	}
	// EntityFullSaver serves POST /ctx/entity/set — full data replace.
	EntityFullSaver interface {
		Save(ctx context.Context, workspaceID, id string, data map[string]any) error
	}
	// EntityFieldUpdater serves POST /ctx/entity/update — atomic per-field jsonb_set.
	EntityFieldUpdater interface {
		UpdateFields(ctx context.Context, workspaceID, id string, fields map[string]any) error
	}
	// EntityFieldCounter serves POST /ctx/entity/increment and /ctx/entity/decrement
	// — atomic arithmetic on a numeric JSONB field, single SQL statement.
	EntityFieldCounter interface {
		IncrementField(ctx context.Context, workspaceID, id, field string, amount float64) error
		DecrementField(ctx context.Context, workspaceID, id, field string, amount float64) (float64, error)
	}
)

// ctxRequest is the union request body for all /ctx/{prim}/{op} calls
// (docs/runtimes/04-forma-sidecar.md §4.3). Named selects a named datastore;
// empty means the default one. Entity-specific fields (Field, Amount,
// Fields) are used by the entity primitive for atomic field operations.
// Tenant isolation is NOT a request parameter — it is bound at connection
// time via CtxHandler.defaultWorkspaceID.
type ctxRequest struct {
	Named      string         `json:"named,omitempty"`
	SQL        string         `json:"sql,omitempty"`
	Args       []any          `json:"args,omitempty"`
	Filter     map[string]any `json:"filter,omitempty"`
	Key        string         `json:"key,omitempty"`
	Value      any            `json:"value,omitempty"`
	TTLSeconds int            `json:"ttl_seconds,omitempty"`
	Field      string         `json:"field,omitempty"`
	Amount     float64        `json:"amount,omitempty"`
	Fields     map[string]any `json:"fields,omitempty"`
}

type ctxResponse struct {
	Data  any    `json:"data,omitempty"`
	OK    *bool  `json:"ok,omitempty"`
	Error string `json:"error,omitempty"`
}

var knownPrimitives = map[string]bool{
	"db": true, "cache": true, "lock": true, "queue": true,
	"pubsub": true, "storage": true, "kvstore": true, "entity": true,
}

// CtxHandler serves the App → Sidecar direction: /ctx/{prim}/{op}.
type CtxHandler struct {
	resolver           PrimitiveResolver
	defaultWorkspaceID string // from auth context, immutable per connection
}

// NewCtxHandler creates the ctx.* proxy handler with the given workspace scope.
// A nil resolver makes every call fail with "datastore resolver not configured" —
// matching the Starlark behavior when SetDatastoreResolver was never called.
// defaultWorkspaceID is the workspace's internal identifier, derived from
// the auth token at connection time — it MUST NOT be overridable per-request.
func NewCtxHandler(resolver PrimitiveResolver, defaultWorkspaceID string) *CtxHandler {
	return &CtxHandler{resolver: resolver, defaultWorkspaceID: defaultWorkspaceID}
}

// ServeHTTP handles POST /ctx/{prim}/{op}.
func (h *CtxHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCtxError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Path: /ctx/{prim}/{op}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/ctx"), "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		writeCtxError(w, http.StatusNotFound, "expected /ctx/{primitive}/{operation}")
		return
	}
	prim, op := parts[0], parts[1]

	if !knownPrimitives[prim] {
		writeCtxError(w, http.StatusNotFound, fmt.Sprintf("unknown primitive %q", prim))
		return
	}

	var req ctxRequest
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		writeCtxError(w, http.StatusBadRequest, fmt.Sprintf("read body: %v", err))
		return
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			writeCtxError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
			return
		}
	}

	if h.resolver == nil {
		writeCtxError(w, http.StatusNotImplemented,
			fmt.Sprintf("ctx.%s.%s: datastore resolver not configured", prim, op))
		return
	}

	name := req.Named
	if name == "" {
		name = "default"
	}
	conn, err := h.resolver(prim, name)
	if err != nil {
		writeCtxError(w, http.StatusBadGateway, fmt.Sprintf("ctx.%s: %v", prim, err))
		return
	}

	h.dispatch(w, r.Context(), prim, op, conn, req)
}

func (h *CtxHandler) dispatch(w http.ResponseWriter, ctx context.Context, prim, op string, conn any, req ctxRequest) {
	notImplemented := func() {
		writeCtxError(w, http.StatusNotImplemented,
			fmt.Sprintf("ctx.%s.%s: not yet implemented for this datastore backend", prim, op))
	}

	switch op {
	case "query":
		q, ok := conn.(Querier)
		if !ok {
			notImplemented()
			return
		}
		rows, err := q.Query(ctx, req.SQL, req.Args...)
		if err != nil {
			writeCtxError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeCtxJSON(w, ctxResponse{Data: rows})

	case "get":
		// Entity get = fetch full record by ID
		if prim == "entity" {
			loader, ok := conn.(EntityLoader)
			if !ok {
				notImplemented()
				return
			}
			record, err := loader.Fetch(ctx, h.defaultWorkspaceID, req.Key)
			if err != nil {
				writeCtxError(w, http.StatusBadGateway, err.Error())
				return
			}
			writeCtxJSON(w, ctxResponse{Data: record})
			return
		}
		g, ok := conn.(KVGetter)
		if !ok {
			notImplemented()
			return
		}
		val, err := g.Get(ctx, req.Key)
		if err != nil {
			writeCtxError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeCtxJSON(w, ctxResponse{Data: val})

	case "set":
		// Entity set = full data replace
		if prim == "entity" {
			saver, ok := conn.(EntityFullSaver)
			if !ok {
				notImplemented()
				return
			}
			data, _ := req.Value.(map[string]any)
			if err := saver.Save(ctx, h.defaultWorkspaceID, req.Key, data); err != nil {
				writeCtxError(w, http.StatusBadGateway, err.Error())
				return
			}
			writeCtxJSON(w, ctxResponse{OK: boolPtr(true)})
			return
		}
		s, ok := conn.(KVSetter)
		if !ok {
			notImplemented()
			return
		}
		if err := s.Set(ctx, req.Key, req.Value, time.Duration(req.TTLSeconds)*time.Second); err != nil {
			writeCtxError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeCtxJSON(w, ctxResponse{OK: boolPtr(true)})

	case "delete":
		d, ok := conn.(KVDeleter)
		if !ok {
			notImplemented()
			return
		}
		if err := d.Delete(ctx, req.Key); err != nil {
			writeCtxError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeCtxJSON(w, ctxResponse{OK: boolPtr(true)})

	case "acquire":
		l, ok := conn.(Locker)
		if !ok {
			notImplemented()
			return
		}
		acquired, err := l.Acquire(ctx, req.Key, time.Duration(req.TTLSeconds)*time.Second)
		if err != nil {
			writeCtxError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeCtxJSON(w, ctxResponse{OK: boolPtr(acquired)})

	case "release":
		l, ok := conn.(Locker)
		if !ok {
			notImplemented()
			return
		}
		if err := l.Release(ctx, req.Key); err != nil {
			writeCtxError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeCtxJSON(w, ctxResponse{OK: boolPtr(true)})

	case "update":
		// Entity update = atomic per-field jsonb_set (entity primitive only)
		if prim != "entity" {
			writeCtxError(w, http.StatusNotFound, fmt.Sprintf("unknown operation %q for primitive %q", op, prim))
			return
		}
		updater, ok := conn.(EntityFieldUpdater)
		if !ok {
			notImplemented()
			return
		}
		if err := updater.UpdateFields(ctx, h.defaultWorkspaceID, req.Key, req.Fields); err != nil {
			writeCtxError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeCtxJSON(w, ctxResponse{OK: boolPtr(true)})

	case "increment":
		if prim != "entity" {
			writeCtxError(w, http.StatusNotFound, fmt.Sprintf("unknown operation %q for primitive %q", op, prim))
			return
		}
		counter, ok := conn.(EntityFieldCounter)
		if !ok {
			notImplemented()
			return
		}
		if err := counter.IncrementField(ctx, h.defaultWorkspaceID, req.Key, req.Field, req.Amount); err != nil {
			writeCtxError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeCtxJSON(w, ctxResponse{OK: boolPtr(true)})

	case "decrement":
		if prim != "entity" {
			writeCtxError(w, http.StatusNotFound, fmt.Sprintf("unknown operation %q for primitive %q", op, prim))
			return
		}
		counter, ok := conn.(EntityFieldCounter)
		if !ok {
			notImplemented()
			return
		}
		newVal, err := counter.DecrementField(ctx, h.defaultWorkspaceID, req.Key, req.Field, req.Amount)
		if err != nil {
			writeCtxError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeCtxJSON(w, ctxResponse{Data: newVal, OK: boolPtr(true)})

	default:
		if prim == "entity" {
			writeCtxError(w, http.StatusNotFound, fmt.Sprintf("unknown operation %q for entity primitive (want get/set/update/increment/decrement)", op))
			return
		}
		writeCtxError(w, http.StatusNotFound, fmt.Sprintf("unknown operation %q (want query/get/set/delete/acquire/release)", op))
	}
}

func boolPtr(b bool) *bool { return &b }

func writeCtxJSON(w http.ResponseWriter, resp ctxResponse) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeCtxError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ctxResponse{Error: msg})
}
