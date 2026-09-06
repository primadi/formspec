package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// HandleConfig serves GET /{ws}/_ui/config/{name} — the UI-exposable keys of
// one Config manifest (plan custom-screens-spec-driven Phase 3). Only keys
// that are BOTH non-secret and explicitly marked `public: true` are served
// (Security by Default — nothing is exposed unless opted in). Consumed by
// the render-context resolver (`spec.context` `source: config`).
//
// Public by design: the keys are opt-in presentation values (branding,
// feature flags). Secrets and non-public keys never leave the server.
func (b *RouterBuilder) HandleConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if b.cfgReg == nil {
			writeError(w, http.StatusServiceUnavailable, "CONFIG_NOT_CONFIGURED",
				"config registry is not configured")
			return
		}
		name := chi.URLParam(r, "name")
		if name == "" {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST",
				"config name is required")
			return
		}
		keys, ok := b.cfgReg.PublicFor(name)
		if !ok {
			writeError(w, http.StatusNotFound, "CONFIG_NOT_FOUND",
				"config "+name+" not found")
			return
		}
		writeJSON(w, http.StatusOK, SingleResponse{
			Data: keys,
			Meta: MetaSingle{
				RequestID: requestIDFromContext(r.Context()),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}
