package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/auth/oauth"
	"github.com/primadi/formspec/internal/entity"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// setupOAuthEnv builds a router with a mock OAuth provider wired to the auth
// service. Returns the handler and the mock provider's base URL.
func setupOAuthEnv(t *testing.T) (http.Handler, string) {
	t.Helper()
	ResetAuthRateLimiters()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "oauth_test.db"), nil)
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
	SetAuthService(svc)

	// Mock OAuth provider.
	mux := http.NewServeMux()
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		redirect := r.URL.Query().Get("redirect_uri")
		http.Redirect(w, r, redirect+"?code=mock-code&state="+state, http.StatusFound)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"mock-token","token_type":"bearer"}`))
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Explicit email_verified:false — the provider says the email is NOT
		// verified, so OAuth login cannot claim an unverified account.
		_, _ = w.Write([]byte(`{"id":"mock-1","email":"oauth.user@example.com","name":"OAuth User","email_verified":false}`))
	})
	mockSrv := httptest.NewServer(mux)
	t.Cleanup(mockSrv.Close)

	// Wire a custom oauth2 provider pointing at the mock.
	prov, err := oauth.New(oauth.Config{
		Name:         "mock",
		Type:         "oauth2",
		ClientID:     "client",
		ClientSecret: "secret",
		AuthorizeURL: mockSrv.URL + "/authorize",
		TokenURL:     mockSrv.URL + "/token",
		UserInfoURL:  mockSrv.URL + "/userinfo",
		RedirectURL:  "/demo/_ui/auth/oauth/mock/callback",
	})
	if err != nil {
		t.Fatalf("oauth.New: %v", err)
	}
	svc.SetOAuthProviders(map[string]oauth.Provider{"mock": prov})

	rb := NewRouterBuilder(reg)
	rb.BuildRoutes()
	return rb.BuildHTTP(), mockSrv.URL
}

func TestOAuth_AuthorizeRedirects(t *testing.T) {
	handler, _ := setupOAuthEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/demo/_ui/auth/oauth/mock/authorize", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/authorize?") || !strings.Contains(loc, "client_id=client") {
		t.Errorf("expected provider authorize URL with client_id, got %q", loc)
	}
	if !strings.Contains(loc, "state=") {
		t.Errorf("expected CSRF state in authorize URL, got %q", loc)
	}
}

func TestOAuth_Callback_CreatesUserAndIssuesToken(t *testing.T) {
	handler, _ := setupOAuthEnv(t)

	// 1. Authorize → capture state.
	req := httptest.NewRequest(http.MethodGet, "/demo/_ui/auth/oauth/mock/authorize", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse authorize location: %v", err)
	}
	state := loc.Query().Get("state")
	if state == "" {
		t.Fatal("no state in authorize URL")
	}

	// 2. Callback with the code (the mock redirects to the callback).
	cbReq := httptest.NewRequest(http.MethodGet,
		"/demo/_ui/auth/oauth/mock/callback?code=mock-code&state="+state, nil)
	cbRec := httptest.NewRecorder()
	handler.ServeHTTP(cbRec, cbReq)

	if cbRec.Code != http.StatusFound {
		t.Fatalf("expected 302 from callback, got %d: %s", cbRec.Code, cbRec.Body.String())
	}
	cbLoc := cbRec.Header().Get("Location")
	if !strings.Contains(cbLoc, "/demo/_admin/oauth/callback#token=") {
		t.Errorf("expected SPA callback with token fragment, got %q", cbLoc)
	}

	// 3. Verify the user was created with the OAuth email.
	svc := GetAuthService()
	user, err := svc.GetUserByEmail(context.Background(), "demo", "oauth.user@example.com")
	if err != nil {
		t.Fatalf("expected OAuth user created by email: %v", err)
	}
	if user.Username == "" {
		t.Error("expected derived username")
	}
	if user.Status != auth.UserStatusActive {
		t.Errorf("expected active status, got %q", user.Status)
	}
}

func TestOAuth_Callback_InvalidState(t *testing.T) {
	handler, _ := setupOAuthEnv(t)

	req := httptest.NewRequest(http.MethodGet,
		"/demo/_ui/auth/oauth/mock/callback?code=mock-code&state=invalid", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid state, got %d", rec.Code)
	}
	var errResp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if errResp.Error.Code != "INVALID_STATE" {
		t.Errorf("expected INVALID_STATE, got %s", errResp.Error.Code)
	}
}

// TestOAuth_Callback_LinkMode_PassesCodeThrough verifies the explicit
// account-linking flow (todo 5.2.21): an authorize started with ?mode=link
// must NOT run OAuthLogin — the callback redirects to the SPA link callback
// with the code in the fragment, and no user is created.
func TestOAuth_Callback_LinkMode_PassesCodeThrough(t *testing.T) {
	handler, _ := setupOAuthEnv(t)

	// 1. Authorize with ?mode=link → capture state.
	req := httptest.NewRequest(http.MethodGet,
		"/demo/_ui/auth/oauth/mock/authorize?mode=link", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 from authorize, got %d", rec.Code)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse authorize location: %v", err)
	}
	state := loc.Query().Get("state")
	if state == "" {
		t.Fatal("no state in authorize URL")
	}

	// 2. Callback with the code → must redirect to the SPA link callback
	// (NOT the token callback), carrying the code + provider in the fragment.
	cbReq := httptest.NewRequest(http.MethodGet,
		"/demo/_ui/auth/oauth/mock/callback?code=mock-code&state="+state, nil)
	cbRec := httptest.NewRecorder()
	handler.ServeHTTP(cbRec, cbReq)

	if cbRec.Code != http.StatusFound {
		t.Fatalf("expected 302 from callback, got %d: %s", cbRec.Code, cbRec.Body.String())
	}
	cbLoc := cbRec.Header().Get("Location")
	if !strings.Contains(cbLoc, "/demo/_admin/oauth/link-callback#code=mock-code&provider=mock") {
		t.Errorf("expected link-callback redirect with code+provider, got %q", cbLoc)
	}
	if strings.Contains(cbLoc, "#token=") {
		t.Errorf("link mode must not issue a token pair, got %q", cbLoc)
	}

	// 3. OAuthLogin must NOT have run — no user created from the code.
	svc := GetAuthService()
	if _, err := svc.GetUserByEmail(context.Background(), "demo", "oauth.user@example.com"); err == nil {
		t.Error("link mode must not create a user (OAuthLogin must not run)")
	}
}

// apiFakeMailer records sent messages so tests can extract verification
// tokens from the emailed links.
type apiFakeMailer struct {
	sent    []string
	baseURL string
}

func (f *apiFakeMailer) Send(to, subject, text, html string) error {
	f.sent = append(f.sent, to+"|"+subject+"|"+text)
	return nil
}
func (f *apiFakeMailer) BaseURL() string { return f.baseURL }

// runOAuthCallback drives the authorize → callback flow and returns the
// callback's redirect Location.
func runOAuthCallback(t *testing.T, handler http.Handler) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/demo/_ui/auth/oauth/mock/authorize", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse authorize location: %v", err)
	}
	state := loc.Query().Get("state")
	if state == "" {
		t.Fatal("no state in authorize URL")
	}
	cbReq := httptest.NewRequest(http.MethodGet,
		"/demo/_ui/auth/oauth/mock/callback?code=mock-code&state="+state, nil)
	cbRec := httptest.NewRecorder()
	handler.ServeHTTP(cbRec, cbReq)
	return cbRec.Header().Get("Location")
}

func TestOAuth_Callback_UnverifiedEmail_Redirects(t *testing.T) {
	handler, _ := setupOAuthEnv(t)
	svc := GetAuthService()

	// Seed an unverified account with the mock provider's email (a hacker's
	// claim). No mailer → no verification email → stays unverified.
	if err := svc.Register(context.Background(), "demo", "claimed", "oauth.user@example.com", "password123"); err != nil {
		t.Fatalf("Register: %v", err)
	}

	loc := runOAuthCallback(t, handler)
	if !strings.Contains(loc, "#oauth=email_unverified") {
		t.Errorf("expected email_unverified redirect, got %q", loc)
	}
}

func TestOAuth_Callback_LinkRequired_Redirects(t *testing.T) {
	handler, _ := setupOAuthEnv(t)
	svc := GetAuthService()

	// A verified password account with the mock provider's email. Register
	// with a mailer, then consume the emailed verification token.
	m := &apiFakeMailer{baseURL: "http://localhost:18080"}
	svc.SetMailer(m)
	if err := svc.Register(context.Background(), "demo", "owner", "oauth.user@example.com", "password123"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if len(m.sent) != 1 {
		t.Fatalf("expected 1 verification email, got %d", len(m.sent))
	}
	token := extractVerifyToken(t, m.sent[0])
	if err := svc.VerifyEmail(context.Background(), "demo", token); err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}

	loc := runOAuthCallback(t, handler)
	if !strings.Contains(loc, "#oauth=link_required") {
		t.Errorf("expected link_required redirect, got %q", loc)
	}
}

// extractVerifyToken pulls the verify_token query param out of an emailed
// link (the fake mailer records "to|subject|text").
func extractVerifyToken(t *testing.T, mail string) string {
	t.Helper()
	idx := strings.Index(mail, "verify_token=")
	if idx < 0 {
		t.Fatalf("no verify_token in mail: %q", mail)
	}
	rest := mail[idx+len("verify_token="):]
	if sp := strings.IndexAny(rest, " \n\r"); sp >= 0 {
		rest = rest[:sp]
	}
	return rest
}
