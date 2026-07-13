package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeConn implements Querier and Locker but not the KV interfaces.
type fakeConn struct {
	lastSQL string
	locked  map[string]bool
}

func (f *fakeConn) Query(_ context.Context, sql string, _ ...any) ([]map[string]any, error) {
	f.lastSQL = sql
	return []map[string]any{{"n": 1}}, nil
}

func (f *fakeConn) Acquire(_ context.Context, key string, _ time.Duration) (bool, error) {
	if f.locked[key] {
		return false, nil
	}
	f.locked[key] = true
	return true, nil
}

func (f *fakeConn) Release(_ context.Context, key string) error {
	delete(f.locked, key)
	return nil
}

func postCtx(t *testing.T, h http.Handler, path, body string) (*httptest.ResponseRecorder, ctxResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var resp ctxResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	return rec, resp
}

func TestCtxHandler_QueryAndLock(t *testing.T) {
	conn := &fakeConn{locked: map[string]bool{}}
	h := NewCtxHandler(func(prim, name string) (any, error) {
		if name != "default" {
			return nil, fmt.Errorf("datastore %q not found", name)
		}
		return conn, nil
	}, "demo")

	rec, resp := postCtx(t, h, "/ctx/db/query", `{"sql":"SELECT 1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("query status = %d body=%s", rec.Code, rec.Body)
	}
	if conn.lastSQL != "SELECT 1" {
		t.Errorf("sql = %q", conn.lastSQL)
	}
	if resp.Data == nil {
		t.Error("query returned no data")
	}

	rec, resp = postCtx(t, h, "/ctx/lock/acquire", `{"key":"workspace:X","ttl_seconds":30}`)
	if rec.Code != http.StatusOK || resp.OK == nil || !*resp.OK {
		t.Fatalf("acquire failed: %d %s", rec.Code, rec.Body)
	}
	rec, resp = postCtx(t, h, "/ctx/lock/acquire", `{"key":"workspace:X"}`)
	if resp.OK == nil || *resp.OK {
		t.Error("second acquire should report ok=false")
	}
	rec, _ = postCtx(t, h, "/ctx/lock/release", `{"key":"workspace:X"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("release status = %d", rec.Code)
	}
}

func TestCtxHandler_UnsupportedOpIs501(t *testing.T) {
	h := NewCtxHandler(func(prim, name string) (any, error) {
		return &fakeConn{locked: map[string]bool{}}, nil // no KV interfaces
	}, "demo")
	rec, resp := postCtx(t, h, "/ctx/cache/get", `{"key":"k"}`)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", rec.Code)
	}
	if !strings.Contains(resp.Error, "not yet implemented") {
		t.Errorf("error = %q", resp.Error)
	}
}

func TestCtxHandler_NoResolver(t *testing.T) {
	h := NewCtxHandler(nil, "demo")
	rec, resp := postCtx(t, h, "/ctx/db/query", `{"sql":"SELECT 1"}`)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", rec.Code)
	}
	if !strings.Contains(resp.Error, "resolver not configured") {
		t.Errorf("error = %q", resp.Error)
	}
}

func TestCtxHandler_UnknownPrimitive(t *testing.T) {
	h := NewCtxHandler(nil, "demo")
	rec, _ := postCtx(t, h, "/ctx/gpu/query", `{}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestCtxHandler_NamedResolution(t *testing.T) {
	var gotName string
	h := NewCtxHandler(func(prim, name string) (any, error) {
		gotName = name
		return &fakeConn{locked: map[string]bool{}}, nil
	}, "demo")
	postCtx(t, h, "/ctx/db/query", `{"named":"analytics-db","sql":"SELECT 1"}`)
	if gotName != "analytics-db" {
		t.Errorf("resolved name = %q, want analytics-db", gotName)
	}
}
