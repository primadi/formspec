package api

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
	"github.com/primadi/formspec/renderers/jsonb-persist/datastore/memory"
)

// TestFileUploadDownload verifies the file upload/download routes (todo
// 7.17.1): upload stores the object key on the record's file field, download
// streams it back, and permission is enforced.
func TestFileUploadDownload(t *testing.T) {
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "file.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)
	docSpec := spec.EntitySpec{
		Version: "v1",
		Plural:  "documents",
		Fields: []spec.Field{
			{Name: "title", Type: spec.FieldString},
			{Name: "attachment", Type: spec.FieldFile, Storage: &spec.StorageSpec{
				AllowedTypes: []string{".pdf", "image/*"},
				MaxSizeMB:    5,
			}},
		},
	}
	registerTestEntity(t, d, reg, "billing", "document", docSpec)

	// Wire a filesystem-backed storage resolver.
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

	// Seed a record.
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

	identity := &auth.Identity{
		UserID:      "u1",
		WorkspaceID: "t1",
		Permissions: []string{"billing.document.update", "billing.document.view"},
	}

	// ── Upload ──
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "report.pdf")
	_, _ = fw.Write([]byte("%PDF-1.4 fake"))
	_ = mw.Close()

	upReq := httptest.NewRequest("POST", "/billing/documents/"+id+"/attachment", &buf)
	upReq.Header.Set("Content-Type", mw.FormDataContentType())
	upReq.SetPathValue("module", "billing")
	upReq.SetPathValue("entity", "document")
	upReq.SetPathValue("id", id)
	upReq.SetPathValue("field", "attachment")
	upReq = upReq.WithContext(WithWorkspace(upReq.Context(), "t1"))
	upReq = upReq.WithContext(WithIdentity(upReq.Context(), identity))
	upRec := httptest.NewRecorder()
	factory.HandleFileUpload()(upRec, upReq)

	if upRec.Code != http.StatusOK {
		t.Fatalf("upload: expected 200, got %d: %s", upRec.Code, upRec.Body.String())
	}
	var upBody map[string]any
	if err := json.NewDecoder(upRec.Body).Decode(&upBody); err != nil {
		t.Fatalf("upload: decode: %v", err)
	}
	key, _ := upBody["key"].(string)
	if key == "" {
		t.Fatalf("upload: empty key")
	}

	// The record's file field now carries the object key.
	rec, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: "t1", ID: id})
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if rec.Data["attachment"] != key {
		t.Fatalf("expected attachment=%q, got %v", key, rec.Data["attachment"])
	}

	// ── Download ──
	dlReq := httptest.NewRequest("GET", "/billing/documents/"+id+"/attachment", nil)
	dlReq.SetPathValue("module", "billing")
	dlReq.SetPathValue("entity", "document")
	dlReq.SetPathValue("id", id)
	dlReq.SetPathValue("field", "attachment")
	dlReq = dlReq.WithContext(WithWorkspace(dlReq.Context(), "t1"))
	dlReq = dlReq.WithContext(WithIdentity(dlReq.Context(), identity))
	dlRec := httptest.NewRecorder()
	factory.HandleFileDownload()(dlRec, dlReq)

	if dlRec.Code != http.StatusOK {
		t.Fatalf("download: expected 200, got %d: %s", dlRec.Code, dlRec.Body.String())
	}
	if dlRec.Body.String() != "%PDF-1.4 fake" {
		t.Fatalf("download: unexpected body %q", dlRec.Body.String())
	}
	if ct := dlRec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("download: expected application/pdf, got %q", ct)
	}

	// ── Permission denied (no update permission) ──
	noPerm := &auth.Identity{
		UserID:      "u2",
		WorkspaceID: "t1",
		Permissions: []string{"billing.document.view"},
	}
	denyReq := httptest.NewRequest("POST", "/billing/documents/"+id+"/attachment", &buf)
	denyReq.Header.Set("Content-Type", mw.FormDataContentType())
	denyReq.SetPathValue("module", "billing")
	denyReq.SetPathValue("entity", "document")
	denyReq.SetPathValue("id", id)
	denyReq.SetPathValue("field", "attachment")
	denyReq = denyReq.WithContext(WithWorkspace(denyReq.Context(), "t1"))
	denyReq = denyReq.WithContext(WithIdentity(denyReq.Context(), noPerm))
	denyRec := httptest.NewRecorder()
	factory.HandleFileUpload()(denyRec, denyReq)
	if denyRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without update permission, got %d", denyRec.Code)
	}
}
