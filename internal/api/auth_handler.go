package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/primadi/formspec/internal/auth"
)

// authService is the active auth service, configured at startup.
// It backs the /auth/login and /auth/refresh endpoints.
var authService *auth.Service

// SetAuthService configures the global auth service.
// Called once at server startup from main() / resource.New().
func SetAuthService(svc *auth.Service) {
	authService = svc
}

// loginRequest is the POST /auth/login body.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// refreshRequest is the POST /auth/refresh body.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// HandleLogin serves POST /{ws}/api/v1/auth/login.
//
// Verifies credentials (bcrypt) and issues an access + refresh token pair
// (todo 6.1.1). Public endpoint — no auth required.
func (b *RouterBuilder) HandleLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authService == nil {
			writeError(w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED",
				"auth service is not configured")
			return
		}

		var req loginRequest
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

		workspaceID := workspaceFromContext(r.Context())
		pair, err := authService.Login(r.Context(), workspaceID, req.Username, req.Password)
		if err != nil {
			// Do not leak whether the user exists — same 401 for both.
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED",
				"invalid username or password")
			return
		}

		writeJSON(w, http.StatusOK, SingleResponse{
			Data: pair,
			Meta: MetaSingle{
				RequestID: requestIDFromContext(r.Context()),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}

// HandleRefresh serves POST /{ws}/api/v1/auth/refresh.
//
// Validates the refresh token, rotates it (invalidates the old jti), and
// issues a fresh pair (todo 6.1.3). Public endpoint — no auth required.
func (b *RouterBuilder) HandleRefresh() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authService == nil {
			writeError(w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED",
				"auth service is not configured")
			return
		}

		var req refreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST",
				"invalid request body: "+err.Error())
			return
		}
		if req.RefreshToken == "" {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST",
				"refresh_token is required")
			return
		}

		pair, err := authService.Refresh(r.Context(), req.RefreshToken)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED",
				"invalid or expired refresh token")
			return
		}

		writeJSON(w, http.StatusOK, SingleResponse{
			Data: pair,
			Meta: MetaSingle{
				RequestID: requestIDFromContext(r.Context()),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}
