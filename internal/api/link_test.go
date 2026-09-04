package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
	"github.com/primadi/formspec/renderers/jsonb-persist/datastore/memory"
)

// linkTestHarness wires a factory with fs storage, a link store, and one
// billing/document entity with the given StorageSpec; returns the factory,
// store, fs storage, seeded record id, and a view/update identity.
type linkTestHarness struct {
	factory *HandlerFactory
	store   *db.EntityStore
	fs      *memory.Storage
	id      string
	viewer  *auth.Identity
}

func newLinkTestHarness(t *testing.T, storageSpec *spec.StorageSpec) *linkTestHarness {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "link.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)
	docSpec := spec.EntitySpec{
		Version: "v1",
		Plural:  "documents",
		Fields: []spec.Field{
			{Name: "title", Type: spec.FieldString},
			{Name: "attachment", Type: spec.FieldFile, Storage: storageSpec},
		},
	}
	registerTestEntity(t, d, reg, "billing", "document", docSpec)

	fsStore, err := memory.NewStorage(filepath.Join(dir, "storage"))
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	factory := NewHandlerFactory(reg)
	factory.SetSpecLookup(func(module, name string) (*spec.EntitySpec, bool) {
		if module == "billing" && name == "document" {
			return &docSpec, true
		}
		return nil, false
	})
	factory.SetStorageResolver(func() (Storage, error) { return fsStore, nil })
	factory.SetLinkStore(db.NewStorageLinkStore(d, db.DriverSQLite))

	ctx := context.Background()
	store, err := reg.GetEntityStore("billing", "document")
	if err != nil {
		t.Fatalf("GetEntityStore: %v", err)
	}
	id, err := store.Insert(ctx, db.InsertParams{
		WorkspaceID: "t1",
		CreatedBy:   "tester",
		Data:        map[string]any{"title": "doc"},
	})
	if err != nil {
		t.Fatalf("seed insert: %v", err)
	}
	return &linkTestHarness{
		factory: factory,
		store:   store,
		fs:      fsStore,
		id:      id,
		viewer: &auth.Identity{
			UserID:      "u1",
			WorkspaceID: "t1",
			Permissions: []string{"billing.document.update", "billing.document.view"},
		},
	}
}

// seedObject uploads an object directly and attaches the key to the record.
func (h *linkTestHarness) seedObject(t *testing.T, key string, data []byte) {
	t.Helper()
	ctx := context.Background()
	if err := h.fs.Upload(ctx, key, data); err != nil {
		t.Fatalf("seed upload: %v", err)
	}
	if _, err := h.store.Insert(ctx, db.InsertParams{
		WorkspaceID: "t1", CreatedBy: "tester",
		Data: map[string]any{"title": "unused"},
	}); err != nil {
		t.Fatalf("seed extra: %v", err)
	}
	if err := h.store.UpdateFields(ctx, "t1", h.id, map[string]any{"attachment": key}); err != nil {
		t.Fatalf("seed attach: %v", err)
	}
}

func (h *linkTestHarness) issueLink(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/t1/_ui/entity/billing/document/"+h.id+"/attachment/link", nil)
	req.SetPathValue("module", "billing")
	req.SetPathValue("entity", "document")
	req.SetPathValue("id", h.id)
	req.SetPathValue("field", "attachment")
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	req = req.WithContext(WithIdentity(req.Context(), h.viewer))
	rec := httptest.NewRecorder()
	h.factory.HandleLinkIssue()(rec, req)
	return rec
}

func (h *linkTestHarness) consumeLink(token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/t1/_ui/storage/link/"+token, nil)
	req.SetPathValue("token", token)
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	rec := httptest.NewRecorder()
	h.factory.HandleLinkConsume()(rec, req)
	return rec
}

// TestLinkIssueConsumeOneTime verifies the 1x-download flow (todo 7.17.6):
// issue → consume serves the bytes → the object is deleted → a second
// consume is gone (410).
func TestLinkIssueConsumeOneTime(t *testing.T) {
	h := newLinkTestHarness(t, &spec.StorageSpec{OneTime: true})
	key := "t1/billing/document/" + h.id + "/attachment/secret.pdf"
	h.seedObject(t, key, []byte("one-time-bytes"))

	rec := h.issueLink(t)
	if rec.Code != http.StatusOK {
		t.Fatalf("issue: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var issueBody map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&issueBody); err != nil {
		t.Fatalf("issue decode: %v", err)
	}
	if issueBody["mode"] != linkModeToken {
		t.Fatalf("issue: expected mode=token, got %v", issueBody["mode"])
	}
	url, _ := issueBody["url"].(string)
	token := filepath.Base(url)
	if token == "" || token == "." {
		t.Fatalf("issue: bad url %q", url)
	}

	// First consume: 200 + body.
	consumeRec := h.consumeLink(token)
	if consumeRec.Code != http.StatusOK {
		t.Fatalf("consume 1: expected 200, got %d: %s", consumeRec.Code, consumeRec.Body.String())
	}
	if consumeRec.Body.String() != "one-time-bytes" {
		t.Fatalf("consume 1: unexpected body %q", consumeRec.Body.String())
	}

	// The object is deleted after the one-time download.
	if _, err := h.fs.Download(context.Background(), key); err == nil {
		t.Fatalf("consume 1: expected object deleted after download")
	}

	// Second consume: 410 GONE.
	if rec := h.consumeLink(token); rec.Code != http.StatusGone {
		t.Fatalf("consume 2: expected 410, got %d", rec.Code)
	}
}

// TestLinkIssueDownloadLimit verifies the size limit gates (todo 7.17.7):
// link issue and direct download reject over-limit objects with 413 before
// loading them.
func TestLinkIssueDownloadLimit(t *testing.T) {
	// max_download_mb=1 → 1MB effective limit; object is 2MB.
	h := newLinkTestHarness(t, &spec.StorageSpec{MaxDownloadMB: 1})
	key := "t1/billing/document/" + h.id + "/attachment/big.bin"
	h.seedObject(t, key, bytes.Repeat([]byte("x"), 2<<20))

	if rec := h.issueLink(t); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("issue: expected 413, got %d: %s", rec.Code, rec.Body.String())
	}

	req := httptest.NewRequest("GET", "/t1/_ui/entity/billing/document/"+h.id+"/attachment", nil)
	req.SetPathValue("module", "billing")
	req.SetPathValue("entity", "document")
	req.SetPathValue("id", h.id)
	req.SetPathValue("field", "attachment")
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	req = req.WithContext(WithIdentity(req.Context(), h.viewer))
	dlRec := httptest.NewRecorder()
	h.factory.HandleFileDownload()(dlRec, req)
	if dlRec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("download: expected 413, got %d", dlRec.Code)
	}
}

// TestChunkUploadFlow verifies the chunked upload routes (todo 7.17.5):
// init → parts → complete assembles the object and attaches the key; an
// over-limit assembly is rejected 413 and deleted.
func TestChunkUploadFlow(t *testing.T) {
	h := newLinkTestHarness(t, &spec.StorageSpec{})
	viewer := h.viewer

	// ── Init ──
	initBody, _ := json.Marshal(map[string]string{"filename": "big.bin"})
	initReq := httptest.NewRequest("POST", "/t1/_ui/entity/billing/document/"+h.id+"/attachment/upload/init", bytes.NewReader(initBody))
	initReq.SetPathValue("module", "billing")
	initReq.SetPathValue("entity", "document")
	initReq.SetPathValue("id", h.id)
	initReq.SetPathValue("field", "attachment")
	initReq = initReq.WithContext(WithWorkspace(initReq.Context(), "t1"))
	initReq = initReq.WithContext(WithIdentity(initReq.Context(), viewer))
	initRec := httptest.NewRecorder()
	h.factory.HandleChunkInit()(initRec, initReq)
	if initRec.Code != http.StatusOK {
		t.Fatalf("init: expected 200, got %d: %s", initRec.Code, initRec.Body.String())
	}
	var initResp map[string]any
	if err := json.NewDecoder(initRec.Body).Decode(&initResp); err != nil {
		t.Fatalf("init decode: %v", err)
	}
	uploadID, _ := initResp["upload_id"].(string)
	if uploadID == "" {
		t.Fatalf("init: empty upload_id")
	}

	// ── Parts ──
	for i, part := range []string{"AAAA", "BBBB"} {
		partReq := httptest.NewRequest("POST", "/t1/_ui/entity/billing/document/"+h.id+"/attachment/upload/"+uploadID+"/part/"+strconv.Itoa(i), bytes.NewReader([]byte(part)))
		partReq.SetPathValue("module", "billing")
		partReq.SetPathValue("entity", "document")
		partReq.SetPathValue("uid", uploadID)
		partReq.SetPathValue("part", strconv.Itoa(i))
		partReq = partReq.WithContext(WithWorkspace(partReq.Context(), "t1"))
		partReq = partReq.WithContext(WithIdentity(partReq.Context(), viewer))
		partRec := httptest.NewRecorder()
		h.factory.HandleChunkPart()(partRec, partReq)
		if partRec.Code != http.StatusOK {
			t.Fatalf("part %d: expected 200, got %d: %s", i, partRec.Code, partRec.Body.String())
		}
	}

	// ── Complete ──
	compReq := httptest.NewRequest("POST", "/t1/_ui/entity/billing/document/"+h.id+"/attachment/upload/"+uploadID+"/complete", nil)
	compReq.SetPathValue("module", "billing")
	compReq.SetPathValue("entity", "document")
	compReq.SetPathValue("id", h.id)
	compReq.SetPathValue("field", "attachment")
	compReq.SetPathValue("uid", uploadID)
	compReq = compReq.WithContext(WithWorkspace(compReq.Context(), "t1"))
	compReq = compReq.WithContext(WithIdentity(compReq.Context(), viewer))
	compRec := httptest.NewRecorder()
	h.factory.HandleChunkComplete()(compRec, compReq)
	if compRec.Code != http.StatusOK {
		t.Fatalf("complete: expected 200, got %d: %s", compRec.Code, compRec.Body.String())
	}
	var compResp map[string]any
	if err := json.NewDecoder(compRec.Body).Decode(&compResp); err != nil {
		t.Fatalf("complete decode: %v", err)
	}
	key, _ := compResp["key"].(string)
	if key == "" {
		t.Fatalf("complete: empty key")
	}

	// The record carries the key and the object holds the assembled bytes.
	rec, err := h.store.GetByID(context.Background(), db.GetByIDParams{WorkspaceID: "t1", ID: h.id})
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if rec.Data["attachment"] != key {
		t.Fatalf("expected attachment=%q, got %v", key, rec.Data["attachment"])
	}
	got, err := h.fs.Download(context.Background(), key)
	if err != nil || !bytes.Equal(got, []byte("AAAABBBB")) {
		t.Fatalf("assembled: got %q err=%v, want %q", got, err, "AAAABBBB")
	}
}

// TestChunkUploadOverLimit verifies complete enforces the effective upload
// limit on the assembled object and removes it (todo 7.17.7).
func TestChunkUploadOverLimit(t *testing.T) {
	h := newLinkTestHarness(t, &spec.StorageSpec{MaxSizeMB: 1})
	h.factory.SetUploadLimitMB(1) // 1MB global (matches per-field)
	viewer := h.viewer

	initBody, _ := json.Marshal(map[string]string{"filename": "big.bin"})
	initReq := httptest.NewRequest("POST", "/x", bytes.NewReader(initBody))
	initReq.SetPathValue("module", "billing")
	initReq.SetPathValue("entity", "document")
	initReq.SetPathValue("id", h.id)
	initReq.SetPathValue("field", "attachment")
	initReq = initReq.WithContext(WithWorkspace(initReq.Context(), "t1"))
	initReq = initReq.WithContext(WithIdentity(initReq.Context(), viewer))
	initRec := httptest.NewRecorder()
	h.factory.HandleChunkInit()(initRec, initReq)
	if initRec.Code != http.StatusOK {
		t.Fatalf("init: expected 200, got %d", initRec.Code)
	}
	var initResp map[string]any
	_ = json.NewDecoder(initRec.Body).Decode(&initResp)
	uploadID, _ := initResp["upload_id"].(string)

	// Two 600KB parts = 1.2MB > 1MB limit.
	for i := 0; i < 2; i++ {
		partReq := httptest.NewRequest("POST", "/x", bytes.NewReader(bytes.Repeat([]byte("x"), 600<<10)))
		partReq.SetPathValue("module", "billing")
		partReq.SetPathValue("entity", "document")
		partReq.SetPathValue("uid", uploadID)
		partReq.SetPathValue("part", strconv.Itoa(i))
		partReq = partReq.WithContext(WithWorkspace(partReq.Context(), "t1"))
		partReq = partReq.WithContext(WithIdentity(partReq.Context(), viewer))
		partRec := httptest.NewRecorder()
		h.factory.HandleChunkPart()(partRec, partReq)
		if partRec.Code != http.StatusOK {
			t.Fatalf("part %d: expected 200, got %d", i, partRec.Code)
		}
	}

	compReq := httptest.NewRequest("POST", "/x", nil)
	compReq.SetPathValue("module", "billing")
	compReq.SetPathValue("entity", "document")
	compReq.SetPathValue("id", h.id)
	compReq.SetPathValue("field", "attachment")
	compReq.SetPathValue("uid", uploadID)
	compReq = compReq.WithContext(WithWorkspace(compReq.Context(), "t1"))
	compReq = compReq.WithContext(WithIdentity(compReq.Context(), viewer))
	compRec := httptest.NewRecorder()
	h.factory.HandleChunkComplete()(compRec, compReq)
	if compRec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("complete: expected 413, got %d: %s", compRec.Code, compRec.Body.String())
	}
	// The over-limit object must not persist.
	key, _ := initResp["key"].(string)
	if key != "" {
		if _, err := h.fs.Download(context.Background(), key); err == nil {
			t.Fatalf("over-limit object still exists")
		}
	}
}
