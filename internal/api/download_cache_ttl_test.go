package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
	"github.com/primadi/formspec/renderers/jsonb-persist/datastore/memory"

	"context"
	"net/http/httptest"
	"path/filepath"
)

// ttlFixture wires a single file field carrying the given `download_cache_ttl`
// declaration on the field, with the given global default.
func ttlFixture(t *testing.T, fieldTTL string, globalTTL time.Duration) (*HandlerFactory, string, *auth.Identity) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "ttl.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)
	entSpec := spec.EntitySpec{
		Version: "v1",
		Plural:  "assets",
		Fields: []spec.Field{
			{Name: "title", Type: spec.FieldString},
			{Name: "blob", Type: spec.FieldFile, Storage: &spec.StorageSpec{
				AllowedTypes:     []string{"png"},
				MaxSizeMB:        5,
				Visibility:       "public",
				DownloadCacheTTL: fieldTTL,
			}},
		},
	}
	registerTestEntity(t, d, reg, "shop", "asset", entSpec)

	fsStore, err := memory.NewStorage(filepath.Join(dir, "storage"))
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	factory := NewHandlerFactory(reg)
	factory.SetSpecLookup(func(module, name string) (*spec.EntitySpec, bool) {
		if module == "shop" && name == "asset" {
			return &entSpec, true
		}
		return nil, false
	})
	factory.SetStorageResolver(func() (Storage, error) { return fsStore, nil })
	factory.SetDownloadCacheTTL(globalTTL)

	ctx := context.Background()
	store, err := reg.GetEntityStore("shop", "asset")
	if err != nil {
		t.Fatalf("GetEntityStore: %v", err)
	}
	id, err := store.Insert(ctx, db.InsertParams{
		WorkspaceID: "t1",
		CreatedBy:   "tester",
		Data:        map[string]any{"title": "thing"},
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := fsStore.Upload(ctx, "t1/shop/asset/"+id+"/blob/x.png", []byte("PNGDATA")); err != nil {
		t.Fatalf("seed upload: %v", err)
	}
	if _, err := store.Insert(ctx, db.InsertParams{WorkspaceID: "t1", CreatedBy: "tester", Data: map[string]any{"title": "p"}}); err != nil {
		t.Fatalf("seed extra: %v", err)
	}
	if err := store.UpdateFields(ctx, "t1", id, map[string]any{"blob": "t1/shop/asset/" + id + "/blob/x.png"}); err != nil {
		t.Fatalf("attach: %v", err)
	}

	return factory, id, &auth.Identity{
		UserID:      "u1",
		WorkspaceID: "t1",
		Permissions: []string{"shop.assets.view"},
	}
}

func fetchBlob(t *testing.T, factory *HandlerFactory, id string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/shop/assets/"+id+"/blob", nil)
	req.SetPathValue("module", "shop")
	req.SetPathValue("entity", "asset")
	req.SetPathValue("id", id)
	req.SetPathValue("field", "blob")
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	rec := httptest.NewRecorder()
	factory.HandleFileDownload()(rec, req)
	return rec
}

// The window is configurable, and each layer overrides the one above it.
// Before this, 300 s was a single constant with no way to change it.
func TestDownloadCacheTTL_Resolution(t *testing.T) {
	cases := []struct {
		name     string
		fieldTTL string
		global   time.Duration
		want     string
	}{
		{"global default", "", 7 * time.Minute, "private, max-age=420"},
		{"field wins over global", "60s", 7 * time.Minute, "private, max-age=60"},
		{"field zero means no-cache", "0s", 7 * time.Minute, "no-cache"},
		{"global zero means no-cache", "", 0, "no-cache"},
		{"field one hour", "1h", 5 * time.Minute, "private, max-age=3600"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			factory, id, _ := ttlFixture(t, tc.fieldTTL, tc.global)
			rec := fetchBlob(t, factory, id)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Cache-Control"); got != tc.want {
				t.Errorf("Cache-Control %q, want %q", got, tc.want)
			}
			// `no-cache` still carries the validator, so the revalidation it
			// forces is a 304 rather than a re-download.
			if rec.Header().Get("ETag") == "" {
				t.Error("no ETag — revalidation would cost a full download")
			}
		})
	}
}

// A malformed duration must not silently fall back to the default at runtime;
// `formspec validate` refuses it instead. This pins both halves.
func TestValidateStorageSpec_DurationGates(t *testing.T) {
	bad := []struct {
		name string
		s    *spec.StorageSpec
	}{
		{"bad download_cache_ttl", &spec.StorageSpec{DownloadCacheTTL: "15min"}},
		{"bad signed_url_ttl", &spec.StorageSpec{SignedURLTTL: "15min"}},
		{"bad ttl", &spec.StorageSpec{TTL: "7d"}},
		{"negative cache ttl", &spec.StorageSpec{DownloadCacheTTL: "-5s"}},
	}
	for _, tc := range bad {
		if err := spec.ValidateStorageSpec("photo", tc.s); err == nil {
			t.Errorf("%s: want a validation error, got nil", tc.name)
		}
	}
	good := []*spec.StorageSpec{
		{DownloadCacheTTL: "0s"},
		{DownloadCacheTTL: "60s", SignedURLTTL: "15m", TTL: "168h"},
		{},
	}
	for i, s := range good {
		if err := spec.ValidateStorageSpec("photo", s); err != nil {
			t.Errorf("good[%d]: unexpected error: %v", i, err)
		}
	}
}
