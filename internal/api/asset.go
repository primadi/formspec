package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

// HandleAsset returns a GET /_ui/assets/* handler that serves a module's
// asset file (custom UI component, todo 5.9.1). The wildcard is the
// spec-root-relative asset path ("modules/{module}/assets/{path}") —
// resolved directly under the manifest roots. Path traversal is rejected.
func (f *HandlerFactory) HandleAsset() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// chi wildcard "*" — the full spec-relative asset path
		// ("modules/portal/assets/profile.js").
		path := chi.URLParam(r, "*")
		if path == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST",
				"missing asset path")
			return
		}

		// Reject traversal: any ".." segment is rejected outright, then the
		// path is normalized and anchored under the spec root.
		if strings.Contains(path, "..") {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid path")
			return
		}
		clean := strings.TrimPrefix(filepath.Clean("/"+path), "/")

		for _, root := range f.assetRoots {
			candidate := filepath.Join(root, clean)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				// CSP sandbox (todo 5.9.7): the asset module may only connect
				// to the App origin itself — no external endpoints.
				w.Header().Set("Content-Security-Policy",
					"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; img-src 'self' data:")
				http.ServeFile(w, r, candidate)
				return
			}
		}
		writeError(w, http.StatusNotFound, "NOT_FOUND", "asset not found")
	}
}
