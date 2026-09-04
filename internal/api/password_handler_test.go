package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// loginAsAdmin logs in via the API and returns the access token.
func loginAsAdmin(t *testing.T, handler http.Handler) string {
	t.Helper()
	body := bytes.NewBufferString(`{"username":"admin","password":"admin"}`)
	req := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal login: %v", err)
	}
	return resp.Data.AccessToken
}

func TestChangePassword_Success(t *testing.T) {
	handler := setupAuthAPIEnv(t)
	token := loginAsAdmin(t, handler)

	body := bytes.NewBufferString(`{"current_password":"admin","new_password":"newpass123"}`)
	req := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/change-password", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Old password must no longer work.
	body2 := bytes.NewBufferString(`{"username":"admin","password":"admin"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/login", body2)
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("old password should fail: expected 401, got %d", rec2.Code)
	}
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	handler := setupAuthAPIEnv(t)
	token := loginAsAdmin(t, handler)

	body := bytes.NewBufferString(`{"current_password":"wrong","new_password":"newpass123"}`)
	req := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/change-password", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong current password, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangePassword_Weak(t *testing.T) {
	handler := setupAuthAPIEnv(t)
	token := loginAsAdmin(t, handler)

	body := bytes.NewBufferString(`{"current_password":"admin","new_password":"short"}`)
	req := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/change-password", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for weak password, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangePassword_Anonymous(t *testing.T) {
	handler := setupAuthAPIEnv(t)

	body := bytes.NewBufferString(`{"current_password":"admin","new_password":"newpass123"}`)
	req := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/change-password", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for anonymous, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestForgotPassword_Always200(t *testing.T) {
	handler := setupAuthAPIEnv(t)

	// Unknown email → still 200 (no leak).
	body := bytes.NewBufferString(`{"email":"nobody@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/forgot-password", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestResetPassword_InvalidToken(t *testing.T) {
	handler := setupAuthAPIEnv(t)

	body := bytes.NewBufferString(`{"token":"bogus","password":"newpass123"}`)
	req := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/reset-password", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid token, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestResetPassword_MissingFields(t *testing.T) {
	handler := setupAuthAPIEnv(t)

	body := bytes.NewBufferString(`{"token":"","password":""}`)
	req := httptest.NewRequest(http.MethodPost, "/demo/_ui/auth/reset-password", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing fields, got %d: %s", rec.Code, rec.Body.String())
	}
}
