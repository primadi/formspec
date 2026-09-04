package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/entity"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// setupAuthAPIEnv builds a router with core entities registered and the auth
// service wired, backed by an in-memory SQLite database.
func setupAuthAPIEnv(t *testing.T) http.Handler {
	t.Helper()
	ResetAuthRateLimiters()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "auth_api_test.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, "")
	if err := auth.RegisterCoreEntities(reg); err != nil {
		t.Fatalf("RegisterCoreEntities: %v", err)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		t.Fatalf("SyncSchema: %v", err)
	}

	roles := auth.NewRoleResolver(reg)
	svc, err := auth.NewService(roles, auth.NewTokenIssuer("test-secret", "formspec", "", 0, 0))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := svc.SeedDevUser(context.Background(), "demo", "admin", "admin"); err != nil {
		t.Fatalf("SeedDevUser: %v", err)
	}
	SetAuthService(svc)

	// Prod-mode JWT validator so authenticated requests (e.g. change-password)
	// resolve a real identity from the token. Restore the previous validator on
	// cleanup so other tests relying on the dev fallback are unaffected.
	prev := GetAuthValidator()
	SetAuthValidator(auth.NewJWTValidator("test-secret", "formspec", ""))
	t.Cleanup(func() { SetAuthValidator(prev) })

	rb := NewRouterBuilder(reg)
	rb.BuildRoutes()
	return rb.BuildHTTP()
}

func TestAuthLogin_Success(t *testing.T) {
	handler := setupAuthAPIEnv(t)

	body := bytes.NewBufferString(`{"username":"admin","password":"admin"}`)
	req := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int64  `json:"expires_in"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Data.AccessToken == "" || resp.Data.RefreshToken == "" {
		t.Fatal("expected non-empty tokens")
	}
}

func TestAuthLogin_WrongPassword(t *testing.T) {
	handler := setupAuthAPIEnv(t)

	body := bytes.NewBufferString(`{"username":"admin","password":"wrong"}`)
	req := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAuthRefresh_Rotation(t *testing.T) {
	handler := setupAuthAPIEnv(t)

	// Login first.
	loginBody := bytes.NewBufferString(`{"username":"admin","password":"admin"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)

	var loginResp struct {
		Data struct {
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("unmarshal login: %v", err)
	}

	// Refresh with the token.
	refreshBody := bytes.NewBufferString(`{"refresh_token":"` + loginResp.Data.RefreshToken + `"}`)
	refreshReq := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/refresh", refreshBody)
	refreshReq.Header.Set("Content-Type", "application/json")
	refreshRec := httptest.NewRecorder()
	handler.ServeHTTP(refreshRec, refreshReq)

	if refreshRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on refresh, got %d: %s", refreshRec.Code, refreshRec.Body.String())
	}

	// Replaying the old refresh token must now fail (rotated).
	replayBody := bytes.NewBufferString(`{"refresh_token":"` + loginResp.Data.RefreshToken + `"}`)
	replayReq := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/refresh", replayBody)
	replayReq.Header.Set("Content-Type", "application/json")
	replayRec := httptest.NewRecorder()
	handler.ServeHTTP(replayRec, replayReq)
	if replayRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on replayed token, got %d: %s", replayRec.Code, replayRec.Body.String())
	}
}

func TestAuthLogin_MissingFields(t *testing.T) {
	handler := setupAuthAPIEnv(t)

	body := bytes.NewBufferString(`{"username":"admin"}`)
	req := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAuthRegister_WithEmail(t *testing.T) {
	handler := setupAuthAPIEnv(t)

	body := bytes.NewBufferString(`{"username":"newuser","email":"new@example.com","password":"password123"}`)
	req := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// The account exists and is unverified (no mailer in this env → no
	// verification email, but the flag stays false).
	svc := GetAuthService()
	user, err := svc.GetUserByEmail(context.Background(), "demo", "new@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if user.EmailVerified {
		t.Error("expected new account to start unverified")
	}
}

func TestAuthRegister_EmailTaken(t *testing.T) {
	handler := setupAuthAPIEnv(t)

	reg := func(username string) int {
		body := bytes.NewBufferString(`{"username":"` + username + `","email":"dup@example.com","password":"password123"}`)
		req := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/register", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := reg("first"); code != http.StatusCreated {
		t.Fatalf("expected 201 for first register, got %d", code)
	}
	if code := reg("second"); code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate email, got %d", code)
	}
}

func TestAuthVerifyEmail_Endpoint(t *testing.T) {
	handler := setupAuthAPIEnv(t)
	svc := GetAuthService()

	// Register with a mailer so a verification token is generated.
	m := &apiFakeMailer{baseURL: "http://localhost:18080"}
	svc.SetMailer(m)
	if err := svc.Register(context.Background(), "demo", "newuser", "new@example.com", "password123"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	token := extractVerifyToken(t, m.sent[0])

	body := bytes.NewBufferString(`{"token":"` + token + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/verify-email", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	user, err := svc.GetUserByEmail(context.Background(), "demo", "new@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if !user.EmailVerified {
		t.Error("expected email verified after endpoint call")
	}
}

func TestAuthVerifyEmail_InvalidToken(t *testing.T) {
	handler := setupAuthAPIEnv(t)

	body := bytes.NewBufferString(`{"token":"bogus"}`)
	req := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/verify-email", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
