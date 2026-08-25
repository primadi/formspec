package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// HandleAsset returns a GET /_ui/assets/{module}/{path...} handler that
// serves a module's asset file (custom UI component, todo 5.9.1). Assets
// live at {root}/modules/{module}/assets/{path} under the manifest roots.
// Path traversal is rejected.
func (f *HandlerFactory) HandleAsset() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		module := r.PathValue("module")
		path := r.PathValue("path")
		if module == "" || path == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST",
				"missing module or path")
			return
		}

		// Reject traversal: any ".." segment is rejected outright, then the
		// path is normalized and anchored under the module dir.
		if strings.Contains(path, "..") {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid path")
			return
		}
		clean := strings.TrimPrefix(filepath.Clean("/"+path), "/")

		for _, root := range f.assetRoots {
			// `path` already includes the assets/ prefix (the manifest asset
			// path is "module/assets/x.js"), so join directly under the
			// module dir.
			candidate := filepath.Join(root, "modules", module, clean)
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
