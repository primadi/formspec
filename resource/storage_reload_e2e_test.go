package formspec

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// TestKafe_PublicMenuPhotoIsReadableAnonymously pins the fix for a gap found
// while verifying the catalog in a browser (kafe item 10.8).
//
// The public menu catalog (`kafe-qr`, `access: public`) could list menu items
// anonymously but every photo answered **403**: `menu-item.photo` declared no
// `visibility`, and the default is `private`, which demands `view` permission on
// the entity — something a guest does not have. A menu page whose images 403 is
// exactly the "menu bergambar" failure the kafe ledger tracked as gap #4.
//
// Menu photos are public data: they appear on the menu board, in the QR flow and
// in the anonymous catalog. Requiring a session to render them is not a security
// posture, it is a broken page.
func TestKafe_PublicMenuPhotoIsReadableAnonymously(t *testing.T) {
	app := bootKafe(t)
	token := seedKafeAdminToken(t, app)

	// The object key is written directly: the subject here is the READ
	// authorization on the file field, and a real multipart upload would add
	// bytes-on-disk setup without changing which check is exercised.
	catID := insertRecord(t, app, "cafe-master", "menu-category", map[string]any{
		"name": "Kategori Publik",
	})
	photoID := insertRecord(t, app, "cafe-master", "menu-item", map[string]any{
		"code": "MKN-PUB-01", "name": "Menu Publik", "menu_category_id": catID,
		"photo": "seed/menu/nasi-goreng.jpg",
	})
	_ = token
	var body map[string]any
	var status int

	// A guest (no token) reads the catalog and the image.
	status, body = doJSON(t, app, http.MethodGet,
		"/kafe/_ui/entity/cafe-master/menu-item?per_page=5", nil)
	if status != http.StatusOK {
		t.Fatalf("anonymous menu-item list: %d %v", status, body)
	}

	status, _ = doJSON(t, app, http.MethodGet,
		"/kafe/_ui/entity/cafe-master/menu-item/"+photoID+"/photo", nil)
	if status == http.StatusForbidden {
		t.Fatal("anonymous photo download is 403: `menu-item.photo` must declare " +
			"`visibility: public` — the catalog is public, so its images are too. " +
			"Without it the menu renders without pictures for every guest.")
	}
	// 404 is acceptable in this harness (the storage object may not exist in a
	// temp state dir) — the assertion is about AUTHORIZATION, not bytes.
	if status != http.StatusOK && status != http.StatusNotFound {
		t.Errorf("anonymous photo download: unexpected status %d", status)
	}
}

// TestReloadSpecKeepsFileStorage pins the second gap found in the same session,
// and the more damaging one.
//
// ReloadSpec builds a FRESH RouterBuilder. The object-store resolver and
// download-link store live on the App (they are not spec-derived, so a reload
// has nothing to re-resolve) — but the new builder started empty. The result:
// after the FIRST hot-reload, every file upload and download answered
// `STORAGE_UNAVAILABLE: storage not configured`.
//
// Measured on a live dev server: photo GET → 200, then touch a spec file
// (triggering the watcher), then photo GET → 500. Nothing about the edit was
// related to storage. That is what makes this class of bug expensive: the
// symptom is disconnected from the cause, and it only appears in the workflow
// the dev server exists to support (edit a file, keep working).
func TestReloadSpecKeepsFileStorage(t *testing.T) {
	app := bootKafe(t)
	token := seedKafeAdminToken(t, app)

	catID := insertRecord(t, app, "cafe-master", "menu-category", map[string]any{
		"name": "Kategori Reload",
	})
	recID := insertRecord(t, app, "cafe-master", "menu-item", map[string]any{
		"code": "MKN-RELOAD", "name": "Menu Reload", "menu_category_id": catID,
	})
	photoURL := "/kafe/_ui/entity/cafe-master/menu-item/" + recID + "/photo"

	before, _ := doJSON(t, app, http.MethodGet, photoURL, nil)
	if before == http.StatusInternalServerError {
		t.Fatalf("photo before reload: 500 — storage was never wired at boot")
	}

	if err := app.ReloadSpec(); err != nil {
		t.Fatalf("ReloadSpec: %v", err)
	}

	after, body := doJSON(t, app, http.MethodGet, photoURL, nil)
	if after == http.StatusInternalServerError {
		t.Fatalf("photo after ReloadSpec: 500 — the reload dropped the storage "+
			"resolver, so every file operation fails until the process restarts\nbody=%v", body)
	}
	if before != after {
		t.Errorf("photo status changed across reload: %d → %d (should be identical — "+
			"a reload re-resolves the SPEC, not the object store)", before, after)
	}

	// And uploads keep working too, not just downloads.
	status, body := doAuthed(t, app, http.MethodPost, photoURL, token,
		map[string]any{"key": "seed/menu/es-teh.jpg"})
	if status == http.StatusInternalServerError {
		t.Fatalf("upload after ReloadSpec: 500 — storage resolver missing\nbody=%v", body)
	}
}

// writeSpecFile is a tiny helper for tests that need to mutate a spec file.
func writeSpecFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
