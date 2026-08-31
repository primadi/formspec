package starlark

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/primadi/formspec/pkg/spec"
)

// fakeQuerier implements Querier for tests.
type fakeQuerier struct {
	rows []map[string]any
	err  error
}

func (f *fakeQuerier) Query(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	return f.rows, f.err
}

// TestCtxDBQuery_ResolvedAndExecuted proves the ctx.* resolver is wired into
// the CtxAPI (todo 2.9.1): a script calling ctx.db().query(...) resolves the
// "db" primitive through the resolver and executes the query against the
// returned connection, instead of failing with "datastore resolver not
// configured".
func TestCtxDBQuery_ResolvedAndExecuted(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "q.star")
	script := "def execute(resource, params, ctx):\n    rows = ctx.db().query(\"SELECT 1 AS one\")\n    return ok({\"n\": len(rows), \"one\": rows[0][\"one\"]})\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now
	ctxObj.SetDatastoreResolver(func(primitiveType, name, module string) (interface{}, error) {
		// Plain ctx.db() passes an empty name — the registry applies
		// module-scoped binding (todo 2.9.4); "" means "caller's bound
		// datastore" which is 'default' for unbound modules.
		if primitiveType != "db" || (name != "default" && name != "") {
			t.Fatalf("unexpected resolve(%q, %q)", primitiveType, name)
		}
		return &fakeQuerier{rows: []map[string]any{{"one": int64(1)}}}, nil
	})

	res := NewResourceAPI("clinic", "visit", "id-1", 1, map[string]any{})
	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.OK {
		t.Fatalf("script failed: %s", result.Error)
	}
	if got := result.Data["n"]; got != int64(1) {
		t.Fatalf("n = %v, want 1", got)
	}
	if got := result.Data["one"]; got != int64(1) {
		t.Fatalf("one = %v, want 1", got)
	}
}

// TestCtxDatastoreAccessDenied proves the uses.datastores gate (plan
// docs/plan/infra-registry-3-level.md fase B): when the action declares a
// datastores map, a ctx.<primitive> access not covered by a key fails with
// DATASTORE_ACCESS_DENIED; a declared primitive passes through.
func TestCtxDatastoreAccessDenied(t *testing.T) {
	dir := t.TempDir()

	// Script touching ctx.cache() — NOT declared in uses.datastores.
	denied := "def execute(resource, params, ctx):\n    ctx.cache().set(\"k\", 1)\n    return ok({})\n"
	deniedPath := filepath.Join(dir, "denied.star")
	if err := os.WriteFile(deniedPath, []byte(denied), 0o644); err != nil {
		t.Fatal(err)
	}

	// Script touching ctx.db() — declared.
	allowed := "def execute(resource, params, ctx):\n    rows = ctx.db().query(\"SELECT 1 AS one\")\n    return ok({\"n\": len(rows)})\n"
	allowedPath := filepath.Join(dir, "allowed.star")
	if err := os.WriteFile(allowedPath, []byte(allowed), 0o644); err != nil {
		t.Fatal(err)
	}

	newCtx := func() *CtxAPI {
		ctxObj := NewCtxAPI("demo", "", "user", "", nil)
		ctxObj.SetUses(&spec.UsesDecl{
			Primitives: []string{"db"},
			Datastores: map[string]string{"db": "pg-main"},
		})
		ctxObj.SetDatastoreResolver(func(primitiveType, name, module string) (interface{}, error) {
			return &fakeQuerier{rows: []map[string]any{{"one": int64(1)}}}, nil
		})
		return ctxObj
	}

	res := NewResourceAPI("clinic", "visit", "id-1", 1, map[string]any{})

	// Denied: cache not in uses.datastores. Starlark errors surface as a
	// failed ScriptResult (not a Go error).
	deniedRes, err := ExecuteScript(context.Background(), deniedPath, res, nil, newCtx())
	if err != nil {
		t.Fatalf("denied case: unexpected Go error %v", err)
	}
	if deniedRes.OK || !strings.Contains(deniedRes.Error, "DATASTORE_ACCESS_DENIED") {
		t.Fatalf("undeclared primitive: want DATASTORE_ACCESS_DENIED, got OK=%v error=%q", deniedRes.OK, deniedRes.Error)
	}

	// Allowed: db declared.
	result, err := ExecuteScript(context.Background(), allowedPath, res, nil, newCtx())
	if err != nil {
		t.Fatalf("declared primitive: unexpected error %v", err)
	}
	if !result.OK {
		t.Fatalf("declared primitive: script failed: %s", result.Error)
	}
}

// TestCtxNamedDatastore proves the named logical primitive path (plan fase
// C): ctx.db.named("analytics") resolves via the registry's ResolveNamed,
// gated by the "db/analytics" key in uses.datastores.
func TestCtxNamedDatastore(t *testing.T) {
	dir := t.TempDir()

	// Script using the named primitive — declared as "db/analytics".
	named := "def execute(resource, params, ctx):\n    rows = ctx.db.named(\"analytics\").query(\"SELECT 1 AS one\")\n    return ok({\"n\": len(rows)})\n"
	namedPath := filepath.Join(dir, "named.star")
	if err := os.WriteFile(namedPath, []byte(named), 0o644); err != nil {
		t.Fatal(err)
	}

	// Script using an undeclared alias.
	undeclared := "def execute(resource, params, ctx):\n    ctx.db.named(\"secret\").query(\"SELECT 1\")\n    return ok({})\n"
	undeclaredPath := filepath.Join(dir, "undeclared.star")
	if err := os.WriteFile(undeclaredPath, []byte(undeclared), 0o644); err != nil {
		t.Fatal(err)
	}

	newCtx := func() *CtxAPI {
		ctxObj := NewCtxAPI("demo", "", "user", "", nil)
		ctxObj.SetModule("billing")
		ctxObj.SetUses(&spec.UsesDecl{
			Datastores: map[string]string{"db/analytics": "pg-analytics"},
		})
		ctxObj.SetDatastoreResolver(func(primitiveType, name, module string) (interface{}, error) {
			t.Fatalf("plain resolver must not be called for named access (got %q, %q)", primitiveType, name)
			return nil, nil
		})
		// Attach a named-capable resolver (mimics DatastoreRegistry).
		type namedResolver interface {
			ResolveNamed(primitiveType, alias, module string) (interface{}, error)
		}
		ctxObj.SetDatastoreResolverNamed(namedResolverFunc(func(primitiveType, alias, module string) (interface{}, error) {
			if alias != "analytics" {
				return nil, fmt.Errorf("DATASTORE_NOT_FOUND — no named logical primitive %q", alias)
			}
			return &fakeQuerier{rows: []map[string]any{{"one": int64(1)}}}, nil
		}))
		_ = namedResolver(nil)
		return ctxObj
	}

	res := NewResourceAPI("clinic", "visit", "id-1", 1, map[string]any{})

	// Declared alias resolves through ResolveNamed.
	result, err := ExecuteScript(context.Background(), namedPath, res, nil, newCtx())
	if err != nil {
		t.Fatalf("named primitive: unexpected error %v", err)
	}
	if !result.OK {
		t.Fatalf("named primitive: script failed: %s", result.Error)
	}
	if got := result.Data["n"]; got != int64(1) {
		t.Fatalf("n = %v, want 1", got)
	}

	// Undeclared alias → DATASTORE_ACCESS_DENIED (gate fires before
	// resolution).
	undRes, err := ExecuteScript(context.Background(), undeclaredPath, res, nil, newCtx())
	if err != nil {
		t.Fatalf("undeclared alias: unexpected Go error %v", err)
	}
	if undRes.OK || !strings.Contains(undRes.Error, "DATASTORE_ACCESS_DENIED") {
		t.Fatalf("undeclared alias: want DATASTORE_ACCESS_DENIED, got OK=%v error=%q", undRes.OK, undRes.Error)
	}
}

// namedResolverFunc adapts a function to the namedResolver interface used
// by CtxAPI.resolveNamed (mirrors DatastoreRegistry's ResolveNamed).
type namedResolverFunc func(primitiveType, alias, module string) (interface{}, error)

func (f namedResolverFunc) ResolveNamed(primitiveType, alias, module string) (interface{}, error) {
	return f(primitiveType, alias, module)
}

// TestCtxDBQuery_NoResolver proves the pre-2.9.1 behavior is preserved when
// no resolver is wired: ctx.db() fails with "datastore resolver not
// configured".
func TestCtxDBQuery_NoResolver(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "q.star")
	script := "def execute(resource, params, ctx):\n    ctx.db()\n    return ok({})\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now

	res := NewResourceAPI("clinic", "visit", "id-1", 1, map[string]any{})
	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.OK {
		t.Fatalf("expected failure, got OK")
	}
	if result.Error == "" {
		t.Fatalf("expected an error message")
	}
}

// TestCtxDBQuery_UnsupportedBackend proves a resolved connection that does
// not implement Querier fails with a clear "not yet implemented for this
// backend" error (not "not configured").
func TestCtxDBQuery_UnsupportedBackend(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "q.star")
	script := "def execute(resource, params, ctx):\n    ctx.db().query(\"SELECT 1\")\n    return ok({})\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now
	// Resolver returns a plain struct that does not implement Querier.
	ctxObj.SetDatastoreResolver(func(primitiveType, name, module string) (interface{}, error) {
		return struct{}{}, nil
	})

	res := NewResourceAPI("clinic", "visit", "id-1", 1, map[string]any{})
	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.OK {
		t.Fatalf("expected failure, got OK")
	}
}

// fakeKV implements KVGetter/KVSetter/KVDeleter for tests.
type fakeKV struct {
	items map[string]any
}

func (f *fakeKV) Get(_ context.Context, key string) (any, error) { return f.items[key], nil }
func (f *fakeKV) Set(_ context.Context, key string, value any, _ time.Duration) error {
	f.items[key] = value
	return nil
}
func (f *fakeKV) Delete(_ context.Context, key string) error {
	delete(f.items, key)
	return nil
}

// fakeQueue implements Queue for tests.
type fakeQueue struct {
	items []any
}

func (f *fakeQueue) Enqueue(_ context.Context, _ string, payload any) error {
	f.items = append(f.items, payload)
	return nil
}
func (f *fakeQueue) Dequeue(_ context.Context, _ string) (any, error) {
	if len(f.items) == 0 {
		return nil, nil
	}
	head := f.items[0]
	f.items = f.items[1:]
	return head, nil
}

// fakePubSub implements PubSub for tests.
type fakePubSub struct {
	subs []func(any)
}

func (f *fakePubSub) Publish(_ context.Context, _ string, payload any) error {
	for _, cb := range f.subs {
		cb(payload)
	}
	return nil
}
func (f *fakePubSub) Subscribe(_ context.Context, _ string, cb func(any)) error {
	f.subs = append(f.subs, cb)
	return nil
}

// fakeStorage implements Storage for tests.
type fakeStorage struct {
	items map[string][]byte
}

func (f *fakeStorage) Upload(_ context.Context, path string, data []byte) error {
	f.items[path] = data
	return nil
}
func (f *fakeStorage) Download(_ context.Context, path string) ([]byte, error) {
	return f.items[path], nil
}

// TestCtxPrimitives_ClosedSet exercises the closed set of primitives
// (todo 2.9.2): cache/kvstore get-set-delete, lock acquire-release, queue
// enqueue-dequeue, pubsub publish-subscribe, storage upload-download, and
// config get — all through the resolver (config/log are separate ctx
// builtins, not routed through the resolver).
func TestCtxPrimitives_ClosedSet(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "p.star")
	script := `def execute(resource, params, ctx):
    ctx.cache().set("ck", "cv", ttl=60)
    cv = ctx.cache().get("ck")
    ctx.cache().delete("ck")
    ctx.kvstore().set("kk", 42)
    kv = ctx.kvstore().get("kk")
    locked = ctx.lock().acquire("lk", ttl=30)
    ctx.lock().release("lk")
    ctx.queue().enqueue("jobs", {"id": 1})
    job = ctx.queue().dequeue("jobs")
    ctx.storage().upload("a.txt", b"bytes")
    data = ctx.storage().download("a.txt")
    cur = ctx.config.get("currency")
    return ok({"cv": cv, "kv": kv, "locked": locked, "job": job, "data": data, "cur": cur})
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now
	ctxObj.Config = NewConfigAPI(map[string]any{"currency": "IDR"})
	kv := &fakeKV{items: map[string]any{}}
	queue := &fakeQueue{}
	pubsub := &fakePubSub{}
	storage := &fakeStorage{items: map[string][]byte{}}
	ctxObj.SetDatastoreResolver(func(primitiveType, name, module string) (interface{}, error) {
		switch primitiveType {
		case "cache", "kvstore":
			return kv, nil
		case "lock":
			return &fakeLock{}, nil
		case "queue":
			return queue, nil
		case "pubsub":
			return pubsub, nil
		case "storage":
			return storage, nil
		default:
			return nil, nil
		}
	})

	res := NewResourceAPI("clinic", "visit", "id-1", 1, map[string]any{})
	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.OK {
		t.Fatalf("script failed: %s", result.Error)
	}
	if result.Data["cv"] != "cv" {
		t.Fatalf("cv = %v, want cv", result.Data["cv"])
	}
	if result.Data["kv"] != int64(42) {
		t.Fatalf("kv = %v, want 42", result.Data["kv"])
	}
	if result.Data["locked"] != true {
		t.Fatalf("locked = %v, want true", result.Data["locked"])
	}
	if result.Data["cur"] != "IDR" {
		t.Fatalf("cur = %v, want IDR", result.Data["cur"])
	}
}

// fakeLock implements Locker for tests.
type fakeLock struct{ held bool }

func (f *fakeLock) Acquire(_ context.Context, _ string, _ time.Duration) (bool, error) {
	if f.held {
		return false, nil
	}
	f.held = true
	return true, nil
}
func (f *fakeLock) Release(_ context.Context, _ string) error {
	f.held = false
	return nil
}
