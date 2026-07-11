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
)

// ctxRequest is the union request body for all /ctx/{prim}/{op} calls
// (docs/runtimes/04-forma-sidecar.md §4.3). Named selects a named datastore;
// empty means the default one.
type ctxRequest struct {
	Named      string         `json:"named,omitempty"`
	SQL        string         `json:"sql,omitempty"`
	Args       []any          `json:"args,omitempty"`
	Filter     map[string]any `json:"filter,omitempty"`
	Key        string         `json:"key,omitempty"`
	Value      any            `json:"value,omitempty"`
	TTLSeconds int            `json:"ttl_seconds,omitempty"`
}

type ctxResponse struct {
	Data  any    `json:"data,omitempty"`
	OK    *bool  `json:"ok,omitempty"`
	Error string `json:"error,omitempty"`
}

var knownPrimitives = map[string]bool{
	"db": true, "cache": true, "lock": true, "queue": true,
	"pubsub": true, "storage": true, "kvstore": true,
}

// CtxHandler serves the App → Sidecar direction: /ctx/{prim}/{op}.
type CtxHandler struct {
	resolver PrimitiveResolver
}

// NewCtxHandler creates the ctx.* proxy handler. A nil resolver makes every
// call fail with "datastore resolver not configured" — matching the Starlark
// behavior when SetDatastoreResolver was never called.
func NewCtxHandler(resolver PrimitiveResolver) *CtxHandler {
	return &CtxHandler{resolver: resolver}
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

	default:
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
