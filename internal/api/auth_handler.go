package api

import (
	"encoding/json"
	"errors"
	"net"
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

// GetAuthService returns the configured global auth service (may be nil).
// Used by native handlers (e.g. vendor approval) to grant roles.
func GetAuthService() *auth.Service {
	return authService
}

// Auth endpoint rate limiters (todo 6.6.3) — token bucket per client IP.
var (
	loginLimiter    = newRateLimiter(0.5, 5) // 5 burst, refill 0.5/s (5 per 10s)
	refreshLimiter  = newRateLimiter(1, 10)  // 10 burst, refill 1/s
	registerLimiter = newRateLimiter(0.1, 3) // 3 burst, refill 0.1/s (3 per 30s)
)

// clientIP extracts the client IP from the request (RemoteAddr host part).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// loginRequest is the POST /auth/login body.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	// App scopes the session to one App (role management is per-App). Empty =
	// workspace-level session (e.g. the _admin surface).
	App string `json:"app,omitempty"`
}

// registerRequest is the POST /auth/register body. Email is optional but
// recommended — when provided, the account starts unverified and a
// verification email is sent (account pre-hijacking protection).
type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// refreshRequest is the POST /auth/refresh body.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// HandleLogin serves POST /{ws}/_ui/auth/login (and, when EnableAPIAuth is
// set, POST /{ws}/api/v1/auth/login).
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

		// Rate limit per client IP (todo 6.6.3).
		ip := clientIP(r)
		if !loginLimiter.Allow("login:" + ip) {
			authAuditLog.record(AuthAuditEntry{
				Timestamp: time.Now().UTC(), Method: "login", IP: ip,
				Result: "failure", Reason: "rate_limited",
			})
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED",
				"too many login attempts, try again later")
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
		pair, err := authService.Login(r.Context(), workspaceID, req.App, req.Username, req.Password)
		if err != nil {
			// Do not leak whether the user exists — same 401 for both.
			authAuditLog.record(AuthAuditEntry{
				Timestamp: time.Now().UTC(), Method: "login", Username: req.Username,
				IP: ip, Result: "failure", Reason: "invalid_credentials",
			})
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED",
				"invalid username or password")
			return
		}

		authAuditLog.record(AuthAuditEntry{
			Timestamp: time.Now().UTC(), Method: "login", Username: req.Username,
			IP: ip, Result: "success",
		})
		writeJSON(w, http.StatusOK, SingleResponse{
			Data: pair,
			Meta: MetaSingle{
				RequestID: requestIDFromContext(r.Context()),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}

// HandleRegister serves POST /{ws}/_ui/auth/register (registry portal B.3 —
// self-service vendor sign-up). Creates an active user with NO roles —
// least privilege by default; role assignment is an admin concern.
// Public endpoint, rate-limited per IP.
func (b *RouterBuilder) HandleRegister() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authService == nil {
			writeError(w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED",
				"auth service is not configured")
			return
		}

		ip := clientIP(r)
		if !registerLimiter.Allow("register:" + ip) {
			authAuditLog.record(AuthAuditEntry{
				Timestamp: time.Now().UTC(), Method: "register", IP: ip,
				Result: "failure", Reason: "rate_limited",
			})
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED",
				"too many registration attempts, try again later")
			return
		}

		var req registerRequest
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
		if err := authService.Register(r.Context(), workspaceID, req.Username, req.Email, req.Password); err != nil {
			reason := "error"
			status := http.StatusInternalServerError
			code := "INTERNAL"
			switch {
			case errors.Is(err, auth.ErrUsernameTaken):
				status, code, reason = http.StatusConflict, "USERNAME_TAKEN", "username_taken"
			case errors.Is(err, auth.ErrEmailTaken):
				status, code, reason = http.StatusConflict, "EMAIL_TAKEN", "email_taken"
			case errors.Is(err, auth.ErrInvalidUsername):
				status, code, reason = http.StatusBadRequest, "INVALID_USERNAME", "invalid_username"
			case errors.Is(err, auth.ErrInvalidCredentials):
				status, code, reason = http.StatusBadRequest, "INVALID_REQUEST", "invalid_input"
			case errors.Is(err, auth.ErrRegistrationClosed):
				status, code, reason = http.StatusForbidden, "REGISTRATION_CLOSED", "registration_closed"
			}
			authAuditLog.record(AuthAuditEntry{
				Timestamp: time.Now().UTC(), Method: "register", Username: req.Username,
				IP: ip, Result: "failure", Reason: reason,
			})
			writeError(w, status, code, err.Error())
			return
		}

		authAuditLog.record(AuthAuditEntry{
			Timestamp: time.Now().UTC(), Method: "register", Username: req.Username,
			IP: ip, Result: "success",
		})
		writeJSON(w, http.StatusCreated, SingleResponse{
			Data: map[string]any{"username": req.Username, "created": true},
			Meta: MetaSingle{
				RequestID: requestIDFromContext(r.Context()),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}

// verifyEmailRequest is the POST /auth/verify-email body.
type verifyEmailRequest struct {
	Token string `json:"token"`
}

// HandleVerifyEmail serves POST /{ws}/_ui/auth/verify-email — consumes the
// emailed verification token and marks the user's email as verified. Public +
// rate-limited. A verified email is required before OAuth login will link to
// the account (account pre-hijacking protection).
func (b *RouterBuilder) HandleVerifyEmail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authService == nil {
			writeError(w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED",
				"auth service is not configured")
			return
		}
		ip := clientIP(r)
		if !registerLimiter.Allow("verify:" + ip) {
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED",
				"too many requests, try again later")
			return
		}

		var req verifyEmailRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST",
				"invalid request body: "+err.Error())
			return
		}
		if req.Token == "" {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST",
				"token is required")
			return
		}

		workspaceID := workspaceFromContext(r.Context())
		if err := authService.VerifyEmail(r.Context(), workspaceID, req.Token); err != nil {
			switch {
			case errors.Is(err, auth.ErrInvalidVerifyToken):
				writeError(w, http.StatusBadRequest, "INVALID_VERIFY_TOKEN",
					"invalid or expired verification token")
			default:
				writeError(w, http.StatusInternalServerError, "INTERNAL",
					"failed to verify email")
			}
			return
		}

		authAuditLog.record(AuthAuditEntry{
			Timestamp: time.Now().UTC(), Method: "verify-email",
			IP: ip, Result: "success",
		})
		writeJSON(w, http.StatusOK, SingleResponse{
			Data: map[string]any{"verified": true},
			Meta: MetaSingle{
				RequestID: requestIDFromContext(r.Context()),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}

// HandleResendVerification serves POST /{ws}/_ui/auth/resend-verification —
// re-sends the email-verification link to the signed-in user's address.
// Authenticated (the caller's user ID comes from the session identity).
func (b *RouterBuilder) HandleResendVerification() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authService == nil {
			writeError(w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED",
				"auth service is not configured")
			return
		}
		id := IdentityFromContext(r.Context())
		if id == nil || id.UserID == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED",
				"authentication required")
			return
		}

		workspaceID := workspaceFromContext(r.Context())
		if err := authService.RequestEmailVerification(r.Context(), workspaceID, id.Username); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL",
				"failed to send verification email")
			return
		}

		authAuditLog.record(AuthAuditEntry{
			Timestamp: time.Now().UTC(), Method: "resend-verification",
			Username: id.Username, IP: clientIP(r), Result: "success",
		})
		writeJSON(w, http.StatusOK, SingleResponse{
			Data: map[string]any{"sent": true},
			Meta: MetaSingle{
				RequestID: requestIDFromContext(r.Context()),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}

// HandleRefresh serves POST /{ws}/_ui/auth/refresh (and, when EnableAPIAuth
// is set, POST /{ws}/api/v1/auth/refresh).
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

		// Rate limit per client IP (todo 6.6.3).
		ip := clientIP(r)
		if !refreshLimiter.Allow("refresh:" + ip) {
			authAuditLog.record(AuthAuditEntry{
				Timestamp: time.Now().UTC(), Method: "refresh", IP: ip,
				Result: "failure", Reason: "rate_limited",
			})
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED",
				"too many refresh attempts, try again later")
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
			authAuditLog.record(AuthAuditEntry{
				Timestamp: time.Now().UTC(), Method: "refresh", IP: ip,
				Result: "failure", Reason: "invalid_refresh_token",
			})
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED",
				"invalid or expired refresh token")
			return
		}

		authAuditLog.record(AuthAuditEntry{
			Timestamp: time.Now().UTC(), Method: "refresh", IP: ip, Result: "success",
		})
		writeJSON(w, http.StatusOK, SingleResponse{
			Data: pair,
			Meta: MetaSingle{
				RequestID: requestIDFromContext(r.Context()),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}

// changePasswordRequest is the POST /auth/change-password body.
type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// HandleChangePassword serves POST /{ws}/_ui/auth/change-password — the
// self-service "change my password" flow from the profile/user menu.
//
// Authenticated: the caller's user ID comes from the session identity (401
// when anonymous). Verifies the current password (when the user has one) and
// sets the new one.
func (b *RouterBuilder) HandleChangePassword() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authService == nil {
			writeError(w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED",
				"auth service is not configured")
			return
		}
		id := IdentityFromContext(r.Context())
		if id == nil || id.UserID == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED",
				"authentication required")
			return
		}

		var req changePasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST",
				"invalid request body: "+err.Error())
			return
		}
		if req.NewPassword == "" {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST",
				"new_password is required")
			return
		}

		workspaceID := workspaceFromContext(r.Context())
		err := authService.ChangePassword(r.Context(), workspaceID, id.UserID,
			req.CurrentPassword, req.NewPassword)
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrWeakPassword):
				writeError(w, http.StatusBadRequest, "WEAK_PASSWORD",
					"password must be at least 8 characters")
			case errors.Is(err, auth.ErrInvalidCredentials):
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED",
					"current password is incorrect")
			default:
				writeError(w, http.StatusInternalServerError, "INTERNAL",
					"failed to change password")
			}
			return
		}

		authAuditLog.record(AuthAuditEntry{
			Timestamp: time.Now().UTC(), Method: "change-password",
			Username: id.Username, IP: clientIP(r), Result: "success",
		})
		writeJSON(w, http.StatusOK, SingleResponse{
			Data: map[string]any{"changed": true},
			Meta: MetaSingle{
				RequestID: requestIDFromContext(r.Context()),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}

// forgotPasswordRequest is the POST /auth/forgot-password body.
type forgotPasswordRequest struct {
	Email string `json:"email"`
}

// HandleForgotPassword serves POST /{ws}/_ui/auth/forgot-password — the
// "forgot password" flow from the login screen. Public + rate-limited.
//
// Always returns 200 (even for unknown emails) so the endpoint does not leak
// whether an address is registered. When the user exists and a mailer is
// configured, an email with a reset link is sent.
func (b *RouterBuilder) HandleForgotPassword() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authService == nil {
			writeError(w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED",
				"auth service is not configured")
			return
		}
		ip := clientIP(r)
		if !registerLimiter.Allow("forgot:" + ip) {
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED",
				"too many requests, try again later")
			return
		}

		var req forgotPasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST",
				"invalid request body: "+err.Error())
			return
		}
		if req.Email == "" {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST",
				"email is required")
			return
		}

		workspaceID := workspaceFromContext(r.Context())
		if err := authService.RequestPasswordReset(r.Context(), workspaceID, req.Email); err != nil {
			// Log server-side; still return 200 to the client (no leak).
			authAuditLog.record(AuthAuditEntry{
				Timestamp: time.Now().UTC(), Method: "forgot-password",
				IP: ip, Result: "failure", Reason: "mail_error",
			})
		}

		writeJSON(w, http.StatusOK, SingleResponse{
			Data: map[string]any{"sent": true},
			Meta: MetaSingle{
				RequestID: requestIDFromContext(r.Context()),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}

// resetPasswordRequest is the POST /auth/reset-password body.
type resetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// HandleResetPassword serves POST /{ws}/_ui/auth/reset-password — consumes
// the emailed token and sets a new password. Public + rate-limited.
func (b *RouterBuilder) HandleResetPassword() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authService == nil {
			writeError(w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED",
				"auth service is not configured")
			return
		}
		ip := clientIP(r)
		if !registerLimiter.Allow("reset:" + ip) {
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED",
				"too many requests, try again later")
			return
		}

		var req resetPasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST",
				"invalid request body: "+err.Error())
			return
		}
		if req.Token == "" || req.Password == "" {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST",
				"token and password are required")
			return
		}

		workspaceID := workspaceFromContext(r.Context())
		err := authService.ResetPassword(r.Context(), workspaceID, req.Token, req.Password)
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrWeakPassword):
				writeError(w, http.StatusBadRequest, "WEAK_PASSWORD",
					"password must be at least 8 characters")
			case errors.Is(err, auth.ErrInvalidResetToken):
				writeError(w, http.StatusBadRequest, "INVALID_RESET_TOKEN",
					"invalid or expired reset token")
			default:
				writeError(w, http.StatusInternalServerError, "INTERNAL",
					"failed to reset password")
			}
			return
		}

		authAuditLog.record(AuthAuditEntry{
			Timestamp: time.Now().UTC(), Method: "reset-password",
			IP: ip, Result: "success",
		})
		writeJSON(w, http.StatusOK, SingleResponse{
			Data: map[string]any{"reset": true},
			Meta: MetaSingle{
				RequestID: requestIDFromContext(r.Context()),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}
