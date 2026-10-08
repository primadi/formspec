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

// downloadFixture wires a minimal entity + storage and returns everything a
// download test needs. visibility selects the field's storage policy.
func downloadFixture(t *testing.T, visibility string) (*HandlerFactory, string, *auth.Identity) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "dl.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)
	entSpec := spec.EntitySpec{
		Version: "v1",
		Plural:  "items",
		Fields: []spec.Field{
			{Name: "title", Type: spec.FieldString},
			{Name: "photo", Type: spec.FieldFile, Storage: &spec.StorageSpec{
				AllowedTypes: []string{"png"},
				MaxSizeMB:    5,
				Visibility:   visibility,
			}},
		},
	}
	registerTestEntity(t, d, reg, "shop", "item", entSpec)

	fsStore, err := memory.NewStorage(filepath.Join(dir, "storage"))
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	factory := NewHandlerFactory(reg)
	factory.SetSpecLookup(func(module, name string) (*spec.EntitySpec, bool) {
		if module == "shop" && name == "item" {
			return &entSpec, true
		}
		return nil, false
	})
	factory.SetStorageResolver(func() (Storage, error) { return fsStore, nil })
	// A link store so `visibility: signed` reaches its token gate instead of
	// stopping at LINK_STORE_UNAVAILABLE (501) — the difference matters here,
	// because 501 would hide the fact that the token gate returns 401.
	factory.SetLinkStore(db.NewStorageLinkStore(d, db.DriverSQLite))

	ctx := context.Background()
	store, err := reg.GetEntityStore("shop", "item")
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

	identity := &auth.Identity{
		UserID:      "u1",
		WorkspaceID: "t1",
		Permissions: []string{"shop.items.update", "shop.items.view"},
	}
	return factory, id, identity
}

// uploadPNG puts a file on the record and returns the resulting object key.
func uploadPNG(t *testing.T, factory *HandlerFactory, id string, identity *auth.Identity, body string) string {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "shot.png")
	_, _ = fw.Write([]byte(body))
	_ = mw.Close()

	req := httptest.NewRequest("POST", "/shop/items/"+id+"/photo", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.SetPathValue("module", "shop")
	req.SetPathValue("entity", "item")
	req.SetPathValue("id", id)
	req.SetPathValue("field", "photo")
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	req = req.WithContext(WithIdentity(req.Context(), identity))
	rec := httptest.NewRecorder()
	factory.HandleFileUpload()(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload: %d: %s", rec.Code, rec.Body.String())
	}

	store, err := factory.registry.GetEntityStore("shop", "item")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	rec2, err := store.GetByID(context.Background(), db.GetByIDParams{WorkspaceID: "t1", ID: id})
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	key, _ := rec2.Data["photo"].(string)
	if key == "" {
		t.Fatal("upload did not set the field key")
	}
	return key
}

// downloadWith serves the file field with the given request headers.
func downloadWith(t *testing.T, factory *HandlerFactory, id string, identity *auth.Identity, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/shop/items/"+id+"/photo", nil)
	req.SetPathValue("module", "shop")
	req.SetPathValue("entity", "item")
	req.SetPathValue("id", id)
	req.SetPathValue("field", "photo")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	ctx := WithWorkspace(req.Context(), "t1")
	if identity != nil {
		ctx = WithIdentity(ctx, identity)
	}
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	factory.HandleFileDownload()(rec, req)
	return rec
}

// A downloaded file must carry a validator and a bounded, cache-private
// lifetime — before this, the response had no cache headers at all, so every
// view re-downloaded the whole object.
func TestFileDownloadCaching(t *testing.T) {
	factory, id, identity := downloadFixture(t, "private")
	const body = "PNGDATA-0123456789"
	uploadPNG(t, factory, id, identity, body)

	first := downloadWith(t, factory, id, identity, nil)
	if first.Code != http.StatusOK {
		t.Fatalf("first download: %d: %s", first.Code, first.Body.String())
	}
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag on download")
	}
	if got := first.Header().Get("Cache-Control"); got != "private, max-age=300" {
		t.Errorf("Cache-Control %q, want %q", got, "private, max-age=300")
	}
	if got := first.Header().Get("Content-Length"); got != "18" {
		t.Errorf("Content-Length %q, want 18", got)
	}
	if first.Body.String() != body {
		t.Errorf("body %q, want %q", first.Body.String(), body)
	}

	// A matching validator is answered 304 with no body — the whole point of
	// deriving the ETag from the key.
	revalidated := downloadWith(t, factory, id, identity, map[string]string{"If-None-Match": etag})
	if revalidated.Code != http.StatusNotModified {
		t.Fatalf("conditional download: %d, want 304", revalidated.Code)
	}
	if revalidated.Body.Len() != 0 {
		t.Errorf("304 carried %d body bytes", revalidated.Body.Len())
	}

	// A stale validator still gets the bytes.
	stale := downloadWith(t, factory, id, identity, map[string]string{"If-None-Match": `"deadbeef"`})
	if stale.Code != http.StatusOK {
		t.Errorf("stale validator: %d, want 200", stale.Code)
	}
	if stale.Body.String() != body {
		t.Errorf("stale validator body %q, want %q", stale.Body.String(), body)
	}

	// `*` matches any current representation (RFC 9110 §13.1.2).
	if star := downloadWith(t, factory, id, identity, map[string]string{"If-None-Match": "*"}); star.Code != http.StatusNotModified {
		t.Errorf("If-None-Match * : %d, want 304", star.Code)
	}
	// A weak-prefixed candidate must still match its strong counterpart.
	if weak := downloadWith(t, factory, id, identity, map[string]string{"If-None-Match": "W/" + etag}); weak.Code != http.StatusNotModified {
		t.Errorf("weak validator: %d, want 304", weak.Code)
	}
}

// public fields are readable anonymously; the response still must not be stored
// by a shared cache (it is `private`, not `public`).
func TestFileDownloadCaching_PublicVisibility(t *testing.T) {
	factory, id, identity := downloadFixture(t, "public")
	uploadPNG(t, factory, id, identity, "PUBLICBYTES")

	anonymous := downloadWith(t, factory, id, nil, nil)
	if anonymous.Code != http.StatusOK {
		t.Fatalf("anonymous download of a public field: %d: %s", anonymous.Code, anonymous.Body.String())
	}
	if anonymous.Header().Get("ETag") == "" {
		t.Error("public download has no ETag")
	}
	if got := anonymous.Header().Get("Cache-Control"); got != "private, max-age=300" {
		t.Errorf("Cache-Control %q, want %q (private, never a shared-cache directive)", got, "private, max-age=300")
	}
}

// Re-uploading a file repoints the SAME url at a NEW object key. The validator
// must change with it, or a browser would keep showing the previous photo.
func TestFileDownloadCaching_ReuploadChangesETag(t *testing.T) {
	factory, id, identity := downloadFixture(t, "public")

	key1 := uploadPNG(t, factory, id, identity, "FIRSTPHOTO")
	first := downloadWith(t, factory, id, identity, nil)
	etag1 := first.Header().Get("ETag")
	if etag1 == "" {
		t.Fatal("no ETag after first upload")
	}

	key2 := uploadPNG(t, factory, id, identity, "SECONDPHOTO")
	if key2 == key1 {
		t.Fatal("re-upload reused the object key — the fixture cannot exercise the change")
	}

	// The old validator must NOT be satisfied by the new content.
	again := downloadWith(t, factory, id, identity, map[string]string{"If-None-Match": etag1})
	if again.Code != http.StatusOK {
		t.Fatalf("stale ETag after re-upload: %d, want 200", again.Code)
	}
	if again.Body.String() != "SECONDPHOTO" {
		t.Errorf("body %q, want the new photo", again.Body.String())
	}
	if again.Header().Get("ETag") == etag1 {
		t.Error("ETag did not change after re-upload")
	}
}

// signed visibility is token-scoped and may be one-time: the response must not
// be stored anywhere, and it carries no validator (a validator would invite a
// conditional request that can only fail the next Consume).
func TestFileDownloadCaching_SignedIsNoStore(t *testing.T) {
	// Without a link token the request is refused before the body is read.
	factory, id, identity := downloadFixture(t, "signed")
	uploadPNG(t, factory, id, identity, "SIGNEDBYTES")
	if noToken := downloadWith(t, factory, id, identity, nil); noToken.Code != http.StatusUnauthorized {
		t.Fatalf("signed without link_token: %d, want 401", noToken.Code)
	}

	// The consume path is the one that actually serves a token-scoped body.
	h := newLinkTestHarness(t, &spec.StorageSpec{Visibility: "signed", OneTime: true})
	key := "t1/billing/document/" + h.id + "/attachment/secret.pdf"
	h.seedObject(t, key, []byte("signed-bytes"))

	issue := h.issueLink(t)
	if issue.Code != http.StatusOK {
		t.Fatalf("issue: %d: %s", issue.Code, issue.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(issue.Body).Decode(&body); err != nil {
		t.Fatalf("issue decode: %v", err)
	}
	url, _ := body["url"].(string)
	token := filepath.Base(url)
	if token == "" || token == "." {
		t.Fatalf("issue: bad url %q", url)
	}

	consume := h.consumeLink(token)
	if consume.Code != http.StatusOK {
		t.Fatalf("consume: %d: %s", consume.Code, consume.Body.String())
	}
	if got := consume.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control %q, want no-store", got)
	}
	if got := consume.Header().Get("ETag"); got != "" {
		t.Errorf("token-scoped response emitted ETag %q, want none", got)
	}
	if consume.Body.String() != "signed-bytes" {
		t.Errorf("body %q, want signed-bytes", consume.Body.String())
	}
}

func TestETagMatches(t *testing.T) {
	cases := []struct {
		header string
		want   bool
	}{
		{``, false},
		{`"x"`, true},
		{`"y"`, false},
		{`*`, true},
		{`"y", "x"`, true},
		{`W/"x"`, true},
		{` "x" `, true},
	}
	for _, tc := range cases {
		if got := etagMatches(tc.header, `"x"`); got != tc.want {
			t.Errorf("etagMatches(%q) = %v, want %v", tc.header, got, tc.want)
		}
	}
}
