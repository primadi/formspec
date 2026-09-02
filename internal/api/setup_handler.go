package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/primadi/formspec/internal/auth"
)

// setupRequest is the POST /_ui/setup body — creates the first admin.
type setupRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

// HandleSetupStatus serves GET /{ws}/_ui/setup — reports whether first-run
// setup is required (the workspace has no users yet). Public endpoint, no
// auth required. The SPA checks this on boot and redirects to the setup
// wizard when true (self-hosted prod bootstrap without formspec-ctl).
func (b *RouterBuilder) HandleSetupStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authService == nil {
			writeError(w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED",
				"auth service is not configured")
			return
		}
		workspaceID := workspaceFromContext(r.Context())
		required, err := authService.SetupRequired(r.Context(), workspaceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, SingleResponse{
			Data: map[string]any{"setup_required": required},
			Meta: MetaSingle{
				RequestID: requestIDFromContext(r.Context()),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}

// HandleSetup serves POST /{ws}/_ui/setup — creates the first admin user
// (roles ["admin"], permissions ["*"]) and seeds the owner roles. One-time
// bootstrap: only allowed while the workspace has no users. After the first
// admin exists, normal auth applies. Public endpoint, rate-limited per IP.
func (b *RouterBuilder) HandleSetup() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authService == nil {
			writeError(w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED",
				"auth service is not configured")
			return
		}

		ip := clientIP(r)
		if !registerLimiter.Allow("setup:" + ip) {
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED",
				"too many setup attempts, try again later")
			return
		}

		var req setupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST",
				"invalid request body: "+err.Error())
			return
		}
		if req.Username == "" || req.Password == "" {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST",
				"username and password are required")
			return
		}
		if len(req.Password) < 8 {
			writeError(w, http.StatusBadRequest, "WEAK_PASSWORD",
				"password must be at least 8 characters")
			return
		}

		workspaceID := workspaceFromContext(r.Context())
		if err := authService.SetupFirstAdmin(r.Context(), workspaceID, req.Username, req.Password, req.DisplayName); err != nil {
			status := http.StatusInternalServerError
			code := "INTERNAL"
			switch {
			case errors.Is(err, auth.ErrSetupComplete):
				status, code = http.StatusConflict, "SETUP_COMPLETE"
			case errors.Is(err, auth.ErrInvalidUsername):
				status, code = http.StatusBadRequest, "INVALID_USERNAME"
			case errors.Is(err, auth.ErrInvalidCredentials):
				status, code = http.StatusBadRequest, "WEAK_PASSWORD"
			}
			writeError(w, status, code, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, SingleResponse{
			Data: map[string]any{"username": req.Username, "created": true},
			Meta: MetaSingle{
				RequestID: requestIDFromContext(r.Context()),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}
