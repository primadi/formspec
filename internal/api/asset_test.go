package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestHandleAsset verifies the module asset-serving endpoint (todo 5.9.1):
// serves {root}/modules/{module}/assets/{path}, 404 on missing, 400 on
// path traversal.
func TestHandleAsset(t *testing.T) {
	dir := t.TempDir()
	assetDir := filepath.Join(dir, "modules", "billing", "assets")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "export default { mount() {} }"
	if err := os.WriteFile(filepath.Join(assetDir, "hello.js"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	factory := NewHandlerFactory(nil)
	factory.SetAssetRoots([]string{dir})

	// Existing asset → 200 with content.
	req := httptest.NewRequest("GET", "/demo/_ui/assets/billing/assets/hello.js", nil)
	req.SetPathValue("module", "billing")
	req.SetPathValue("path", "assets/hello.js")
	rec := httptest.NewRecorder()
	factory.HandleAsset()(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != content {
		t.Fatalf("unexpected body %q", rec.Body.String())
	}

	// Missing asset → 404.
	req2 := httptest.NewRequest("GET", "/demo/_ui/assets/billing/assets/nope.js", nil)
	req2.SetPathValue("module", "billing")
	req2.SetPathValue("path", "assets/nope.js")
	rec2 := httptest.NewRecorder()
	factory.HandleAsset()(rec2, req2)
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec2.Code)
	}

	// Path traversal → 400.
	req3 := httptest.NewRequest("GET", "/demo/_ui/assets/billing/../../etc/passwd", nil)
	req3.SetPathValue("module", "billing")
	req3.SetPathValue("path", "../../etc/passwd")
	rec3 := httptest.NewRecorder()
	factory.HandleAsset()(rec3, req3)
	if rec3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for traversal, got %d", rec3.Code)
	}
}
