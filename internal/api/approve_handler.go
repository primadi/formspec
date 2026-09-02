package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/primadi/formspec/internal/auth"
)

// approveRequest is the POST /_ui/auth/approve body — approves a pending
// user (approval registration policy) and assigns roles.
type approveRequest struct {
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
}

// HandleApproveUser serves POST /{ws}/_ui/auth/approve — approves a pending
// user (sets status active) and assigns the given roles. Admin-only: requires
// the formspec.core.users.update permission. Returns 409 when the user is not
// pending.
func (b *RouterBuilder) HandleApproveUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authService == nil {
			writeError(w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED",
				"auth service is not configured")
			return
		}

		var req approveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST",
				"invalid request body: "+err.Error())
			return
		}
		if req.Username == "" {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST",
				"username is required")
			return
		}

		workspaceID := workspaceFromContext(r.Context())
		if err := authService.ApproveUser(r.Context(), workspaceID, req.Username, req.Roles); err != nil {
			status := http.StatusInternalServerError
			code := "INTERNAL"
			switch {
			case err == auth.ErrUserNotFound:
				status, code = http.StatusNotFound, "USER_NOT_FOUND"
			case err == auth.ErrNotPending:
				status, code = http.StatusConflict, "NOT_PENDING"
			}
			writeError(w, status, code, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, SingleResponse{
			Data: map[string]any{"username": req.Username, "approved": true},
			Meta: MetaSingle{
				RequestID: requestIDFromContext(r.Context()),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}
