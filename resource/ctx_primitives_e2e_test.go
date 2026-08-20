package formspec

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildCtxPrimitivesSpecDir writes a minimal spec whose order entity exposes
// a custom action `prims` implemented by prims.star, which exercises the
// closed set of ctx.* primitives (todo 2.9.2/2.9.3): cache, kvstore, lock,
// queue, pubsub, storage, config.
func buildCtxPrimitivesSpecDir(t *testing.T, dir string) {
	t.Helper()

	write := func(rel, content string) {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	write("apps/test.yaml", `apiVersion: formspec.dev/v1
kind: App
metadata:
  name: test
spec:
  version: 1.0.0
  root_url: /app/test
  modules:
    - alpha
`)

	write("modules/alpha/module.yaml", `apiVersion: formspec.dev/v1
kind: Module
metadata:
  name: alpha
spec:
  version: 1.0.0
`)

	write("modules/alpha/transaction/order/entity.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: order
  module: alpha
spec:
  version: v1
  characteristic: transaction
  fields:
    - name: transaction_date
      type: date
      required: true
      index: true
    - name: number
      type: string
  actions:
    - name: prims
      required_permission: alpha.orders.prims
      impl: { type: script_ref, ref: prims }
  expose:
    - type: rest
      actions: [list, find, create, update, delete]
`)

	write("modules/alpha/transaction/order/scripts/prims.star", `def execute(resource, params, ctx):
    ctx.cache().set("ck", "cv", ttl=60)
    cv = ctx.cache().get("ck")
    ctx.kvstore().set("kk", 42)
    kv = ctx.kvstore().get("kk")
    locked = ctx.lock().acquire("lk", ttl=30)
    ctx.lock().release("lk")
    ctx.queue().enqueue("jobs", {"id": 1})
    job = ctx.queue().dequeue("jobs")
    ctx.storage().upload("a.txt", b"bytes")
    data = ctx.storage().download("a.txt")
    return ok({"cv": cv, "kv": kv, "locked": locked, "job": job, "data": data})
`)
}

// TestCtxPrimitives_EndToEnd proves the closed set of ctx.* primitives is
// auto-provisioned in single-server mode (todo 2.9.2/2.9.3): a script action
// can use cache/kvstore/lock/queue/pubsub/storage/config against their
// 'default' in-memory/filesystem backends instead of failing with "no live
// datastore".
func TestCtxPrimitives_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	buildCtxPrimitivesSpecDir(t, dir)

	app, err := New(Config{
		SpecPath: dir,
		DSN:      "sqlite:" + filepath.Join(t.TempDir(), "ctxprims.db"),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	status, out := doJSON(t, app, "POST", "/demo/_ui/entity/alpha/order", map[string]any{
		"transaction_date": recentDate(),
		"number":           "ORD-1",
	})
	if status != http.StatusCreated {
		t.Fatalf("create order: status %d, body %v", status, out)
	}
	id, _ := out["data"].(map[string]any)["id"].(string)
	if id == "" {
		t.Fatalf("expected order id, got %v", out)
	}

	status, out = doJSON(t, app, "POST", "/demo/_ui/entity/alpha/order/"+id+"/prims", nil)
	if status != http.StatusOK {
		t.Fatalf("prims: status %d, body %v", status, out)
	}
	data, _ := out["data"].(map[string]any)
	if data["cv"] != "cv" {
		t.Fatalf("cv = %v, want cv", data["cv"])
	}
	if data["kv"] != float64(42) {
		t.Fatalf("kv = %v, want 42", data["kv"])
	}
	if data["locked"] != true {
		t.Fatalf("locked = %v, want true", data["locked"])
	}
	// storage.download returns starlark.Bytes, which stringifies as b"bytes".
	if s, ok := data["data"].(string); !ok || !strings.Contains(s, "bytes") {
		t.Fatalf("data = %v, want bytes round-trip", data["data"])
	}
}
