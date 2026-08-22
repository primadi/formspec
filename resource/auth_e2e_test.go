package formspec

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/api"
	"github.com/primadi/formspec/internal/auth"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// buildAuthSpecDir writes a minimal spec with one exposed entity (customer)
// that has a masked field and standard CRUD, so the e2e test can exercise the
// full auth + authorization flow through the HTTP API.
func buildAuthSpecDir(t *testing.T, dir string) {
	t.Helper()
	files := map[string]string{
		"apps/acme.yaml": `apiVersion: formspec.dev/v1
kind: App
metadata:
  name: acme-app
spec:
  version: 1.0.0
  root_url: /app/acme
  modules: [acme]
`,
		"modules/acme/module.yaml": `apiVersion: formspec.dev/v1
kind: Module
metadata:
  name: acme
spec:
  version: 1.0.0
`,
		"modules/acme/master/customer/entity.yaml": `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: customer
  module: acme
spec:
  version: v1
  plural: customers
  characteristic: master
  expose:
    - type: rest
  fields:
    - name: name
      type: string
      required: true
    - name: secret
      type: string
      masked: true
    - name: salary
      type: integer
      required_permission: acme.customers.salary.view
`,
		"modules/acme/master/customer/tables/list.yaml": `apiVersion: formspec.dev/v1
kind: Table
metadata:
  name: customer-table
  module: acme
spec:
  entity: customer
`,
		"modules/acme/master/customer/pages/list.yaml": `apiVersion: formspec.dev/v1
kind: Page
metadata:
  name: customer-list
  module: acme
spec:
  route: /customers
  blocks:
    - table:
        ref: customer-table
`,
	}
	for rel, content := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

// login returns the access token for a username/password.
func login(t *testing.T, app *App, username, password string) string {
	t.Helper()
	access, _ := loginPair(t, app, username, password)
	return access
}

// loginPair returns the access + refresh token pair for a username/password.
func loginPair(t *testing.T, app *App, username, password string) (string, string) {
	t.Helper()
	status, out := doJSON(t, app, "POST", "/demo/api/v1/auth/login", map[string]any{
		"username": username, "password": password,
	})
	if status != 200 {
		t.Fatalf("login %s: status %d, body %v", username, status, out)
	}
	data, _ := out["data"].(map[string]any)
	access, _ := data["access_token"].(string)
	refresh, _ := data["refresh_token"].(string)
	if access == "" || refresh == "" {
		t.Fatalf("login %s: missing tokens", username)
	}
	return access, refresh
}

// doAuthed performs a request with a Bearer token and returns status + body.
func doAuthed(t *testing.T, app *App, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return rr.Code, out
}

// seedUser inserts a user with the given permissions directly into the user
// entity (ProdMode does not auto-seed).
func seedUser(t *testing.T, app *App, username, password string, perms []string) {
	t.Helper()
	reg := app.Registry()
	userStore, err := reg.GetEntityStore("formspec.core", "user")
	if err != nil {
		t.Fatalf("user store: %v", err)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if _, err := userStore.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "demo", CreatedBy: "test",
		Data: map[string]any{
			"username": username, "password_hash": hash,
			"roles": []string{}, "permissions": perms, "active": true,
		},
	}); err != nil {
		t.Fatalf("insert user %s: %v", username, err)
	}
}

// TestAuthAuthz_E2E exercises the full authentication + authorization flow
// through the HTTP API: login, authorized CRUD, unauthorized 401, field-level
// masking, permission-based 403, and API key auth on the external surface.
func TestAuthAuthz_E2E(t *testing.T) {
	dir := t.TempDir()
	buildAuthSpecDir(t, dir)
	api.ResetAuthRateLimiters()

	app, err := New(Config{
		SpecPath:  dir,
		DSN:       "sqlite:" + filepath.Join(t.TempDir(), "auth_e2e.db"),
		ProdMode:  true,
		JWTSecret: "test-secret",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// ProdMode does not auto-seed a dev user — seed admin + limited manually.
	seedUser(t, app, "admin", "admin", []string{"*"})
	seedUser(t, app, "limited", "limited", []string{"acme.customers.list", "acme.customers.view"})

	// ── 1. Authentication: login success + wrong password ──
	adminTok := login(t, app, "admin", "admin")
	if status, _ := doJSON(t, app, "POST", "/demo/api/v1/auth/login", map[string]any{
		"username": "admin", "password": "wrong",
	}); status != 401 {
		t.Fatalf("expected 401 for wrong password, got %d", status)
	}

	// ── 2. Authorization: create customer with admin token ──
	status, out := doAuthed(t, app, "POST", "/demo/_ui/entity/acme/customer", adminTok, map[string]any{
		"name": "Alice", "secret": "topsecret", "salary": 100000,
	})
	if status != 201 {
		t.Fatalf("create customer: status %d, body %v", status, out)
	}
	custID, _ := out["data"].(map[string]any)["id"].(string)
	if custID == "" {
		t.Fatal("create customer: no id")
	}

	// ── 3. Field-level security: masked field in response ──
	status, out = doAuthed(t, app, "GET", "/demo/_ui/entity/acme/customer/"+custID, adminTok, nil)
	if status != 200 {
		t.Fatalf("get customer: status %d", status)
	}
	recData, _ := out["data"].(map[string]any)
	secret, _ := recData["secret"].(string)
	if secret == "topsecret" || secret == "" {
		t.Fatalf("expected secret masked, got %q", secret)
	}
	// Field-level required_permission: admin (has *) sees salary.
	if salary, _ := recData["salary"].(float64); salary != 100000 {
		t.Fatalf("expected admin to see salary, got %v", recData["salary"])
	}

	// ── 4. Authentication: no token → 401 ──
	if status, _ := doJSON(t, app, "GET", "/demo/_ui/entity/acme/customer/"+custID, nil); status != 401 {
		t.Fatalf("expected 401 without token, got %d", status)
	}

	// ── 5. Authorization: limited user without delete permission → 403 ──
	limitedTok := login(t, app, "limited", "limited")

	// view allowed (has acme.customers.view)
	status, out = doAuthed(t, app, "GET", "/demo/_ui/entity/acme/customer/"+custID, limitedTok, nil)
	if status != 200 {
		t.Fatalf("expected 200 for view (has permission), got %d", status)
	}
	// Field-level required_permission: limited user (no acme.customers.salary.view)
	// must NOT see salary in the response.
	if _, ok := out["data"].(map[string]any)["salary"]; ok {
		t.Fatal("expected salary excluded for user without required_permission")
	}
	// delete forbidden (no acme.customers.delete)
	if status, _ := doAuthed(t, app, "DELETE", "/demo/_ui/entity/acme/customer/"+custID, limitedTok, nil); status != 403 {
		t.Fatalf("expected 403 for delete (no permission), got %d", status)
	}

	// ── 6. API key auth on external surface ──
	reg := app.Registry()
	apiKeyStore, err := reg.GetEntityStore("formspec.core", "api-key")
	if err != nil {
		t.Fatalf("api-key store: %v", err)
	}
	keyStore := auth.NewApiKeyStore(apiKeyStore)
	plaintext, err := keyStore.Create(context.Background(), "demo", &auth.ApiKey{
		Name: "svc", Scope: "workspace", Permissions: []string{"acme.customers.list"},
	})
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}
	req := httptest.NewRequest("GET", "/demo/api/v1/acme/customers", nil)
	req.Header.Set("X-FormSpec-Key", plaintext)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("expected 200 for api key list, got %d", rr.Code)
	}

	// ── 7. API key revoked → 401 ──
	// Resolve the key to get its ID, then revoke it.
	key, err := keyStore.GetByKey(context.Background(), "demo", plaintext)
	if err != nil {
		t.Fatalf("get api key: %v", err)
	}
	if err := keyStore.Revoke(context.Background(), "demo", key.ID); err != nil {
		t.Fatalf("revoke api key: %v", err)
	}
	req2 := httptest.NewRequest("GET", "/demo/api/v1/acme/customers", nil)
	req2.Header.Set("X-FormSpec-Key", plaintext)
	rr2 := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr2, req2)
	if rr2.Code != 401 {
		t.Fatalf("expected 401 for revoked api key, got %d", rr2.Code)
	}
}

// TestAuthRefresh_Rotation_E2E verifies refresh token rotation (todo 6.1.3):
// a refresh token issues a new pair, and the old refresh token is invalidated
// (replay → 401).
func TestAuthRefresh_Rotation_E2E(t *testing.T) {
	dir := t.TempDir()
	buildAuthSpecDir(t, dir)
	api.ResetAuthRateLimiters()

	app, err := New(Config{
		SpecPath:  dir,
		DSN:       "sqlite:" + filepath.Join(t.TempDir(), "auth_refresh_e2e.db"),
		ProdMode:  true,
		JWTSecret: "test-secret",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	seedUser(t, app, "admin", "admin", []string{"*"})

	// Login → access + refresh.
	_, refresh := loginPair(t, app, "admin", "admin")

	// Refresh → new pair (200).
	status, out := doJSON(t, app, "POST", "/demo/api/v1/auth/refresh", map[string]any{
		"refresh_token": refresh,
	})
	if status != 200 {
		t.Fatalf("refresh: status %d, body %v", status, out)
	}
	data, _ := out["data"].(map[string]any)
	newRefresh, _ := data["refresh_token"].(string)
	if newRefresh == "" {
		t.Fatal("refresh: no new refresh_token")
	}

	// Replay the OLD refresh token → 401 (rotated/invalidated).
	if status, _ := doJSON(t, app, "POST", "/demo/api/v1/auth/refresh", map[string]any{
		"refresh_token": refresh,
	}); status != 401 {
		t.Fatalf("expected 401 for replayed old refresh token, got %d", status)
	}

	// The NEW refresh token still works.
	if status, _ := doJSON(t, app, "POST", "/demo/api/v1/auth/refresh", map[string]any{
		"refresh_token": newRefresh,
	}); status != 200 {
		t.Fatalf("expected 200 for new refresh token, got %d", status)
	}
}

// TestAuthRoleGrants_E2E verifies role-based authorization via grant
// materialization (todo 6.3.1): a role's page/tab/action grants are expanded
// into concrete {module}.{entity}.{action} permissions, and a user holding
// that role gets exactly those permissions.
func TestAuthRoleGrants_E2E(t *testing.T) {
	dir := t.TempDir()
	buildAuthSpecDir(t, dir)
	api.ResetAuthRateLimiters()

	app, err := New(Config{
		SpecPath:  dir,
		DSN:       "sqlite:" + filepath.Join(t.TempDir(), "auth_role_e2e.db"),
		ProdMode:  true,
		JWTSecret: "test-secret",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	seedUser(t, app, "admin", "admin", []string{"*"})

	// Create a customer as admin (for the viewer to read).
	adminTok := login(t, app, "admin", "admin")
	status, out := doAuthed(t, app, "POST", "/demo/_ui/entity/acme/customer", adminTok, map[string]any{
		"name": "Bob",
	})
	if status != 201 {
		t.Fatalf("create customer: status %d, body %v", status, out)
	}
	custID, _ := out["data"].(map[string]any)["id"].(string)

	// Create a role "viewer" with a grant on the customer-list page → view.
	reg := app.Registry()
	roleStore, err := reg.GetEntityStore("formspec.core", "role")
	if err != nil {
		t.Fatalf("role store: %v", err)
	}
	if _, err := roleStore.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "demo", CreatedBy: "test",
		Data: map[string]any{
			"name": "viewer", "app": "", "module": "",
			"grants": []map[string]any{
				{"page": "customer-list", "actions": []map[string]any{{"name": "view"}}},
			},
		},
	}); err != nil {
		t.Fatalf("insert role: %v", err)
	}

	// Create a user holding the viewer role.
	userStore, err := reg.GetEntityStore("formspec.core", "user")
	if err != nil {
		t.Fatalf("user store: %v", err)
	}
	hash, err := auth.HashPassword("viewer")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if _, err := userStore.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "demo", CreatedBy: "test",
		Data: map[string]any{
			"username": "viewer", "password_hash": hash,
			"roles": []string{"viewer"}, "permissions": []string{}, "active": true,
		},
	}); err != nil {
		t.Fatalf("insert viewer user: %v", err)
	}

	// Login as viewer → the role grant materializes to acme.customers.view.
	viewerTok := login(t, app, "viewer", "viewer")

	// view allowed (materialized acme.customers.view).
	if status, _ := doAuthed(t, app, "GET", "/demo/_ui/entity/acme/customer/"+custID, viewerTok, nil); status != 200 {
		t.Fatalf("expected 200 for view (materialized grant), got %d", status)
	}
	// delete forbidden (no acme.customers.delete in the role grant).
	if status, _ := doAuthed(t, app, "DELETE", "/demo/_ui/entity/acme/customer/"+custID, viewerTok, nil); status != 403 {
		t.Fatalf("expected 403 for delete (no grant), got %d", status)
	}
}

// TestAuthSessionRevoke_E2E verifies session revocation (todo 6.5.4): after a
// session is deleted (logout), its refresh token is rejected.
func TestAuthSessionRevoke_E2E(t *testing.T) {
	dir := t.TempDir()
	buildAuthSpecDir(t, dir)
	api.ResetAuthRateLimiters()

	app, err := New(Config{
		SpecPath:  dir,
		DSN:       "sqlite:" + filepath.Join(t.TempDir(), "auth_session_e2e.db"),
		ProdMode:  true,
		JWTSecret: "test-secret",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	seedUser(t, app, "admin", "admin", []string{"*"})

	// Login → refresh token.
	_, refresh := loginPair(t, app, "admin", "admin")

	// Revoke all sessions for the user (logout all devices).
	reg := app.Registry()
	userStore, err := reg.GetEntityStore("formspec.core", "user")
	if err != nil {
		t.Fatalf("user store: %v", err)
	}
	userRec, err := userStore.FindByField(context.Background(), "demo", "username", "admin")
	if err != nil || userRec == nil {
		t.Fatalf("find admin user: %v", err)
	}
	sessionStore, err := reg.GetEntityStore("formspec.core", "session")
	if err != nil {
		t.Fatalf("session store: %v", err)
	}
	// Delete sessions for the admin user ID.
	res, err := sessionStore.List(context.Background(), db.ListParams{
		WorkspaceID: "demo", PerPage: 100,
		Filters: map[string]db.FilterOp{"user_id": {Op: "eq", Value: userRec.ID}},
	})
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	for _, rec := range res.Data {
		if err := sessionStore.SoftDelete(context.Background(), "demo", rec.ID); err != nil {
			t.Fatalf("delete session: %v", err)
		}
	}

	// Refresh with the revoked session's token → 401.
	if status, _ := doJSON(t, app, "POST", "/demo/api/v1/auth/refresh", map[string]any{
		"refresh_token": refresh,
	}); status != 401 {
		t.Fatalf("expected 401 for revoked session refresh, got %d", status)
	}
}

// TestAuthConcurrentSessionLimit_E2E verifies the concurrent session limit
// (todo 6.5.3): when a user exceeds the cap, the oldest session is evicted and
// its refresh token is rejected.
func TestAuthConcurrentSessionLimit_E2E(t *testing.T) {
	dir := t.TempDir()
	buildAuthSpecDir(t, dir)
	api.ResetAuthRateLimiters()

	app, err := New(Config{
		SpecPath:           dir,
		DSN:                "sqlite:" + filepath.Join(t.TempDir(), "auth_limit_e2e.db"),
		ProdMode:           true,
		JWTSecret:          "test-secret",
		MaxSessionsPerUser: 1,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	seedUser(t, app, "admin", "admin", []string{"*"})

	// Login #1 → session A.
	_, refreshA := loginPair(t, app, "admin", "admin")
	// Login #2 → session B; session A is evicted (cap = 1).
	_, refreshB := loginPair(t, app, "admin", "admin")

	// Refresh with the evicted session A's token → 401.
	if status, _ := doJSON(t, app, "POST", "/demo/api/v1/auth/refresh", map[string]any{
		"refresh_token": refreshA,
	}); status != 401 {
		t.Fatalf("expected 401 for evicted session A, got %d", status)
	}
	// Refresh with the current session B's token → 200.
	if status, _ := doJSON(t, app, "POST", "/demo/api/v1/auth/refresh", map[string]any{
		"refresh_token": refreshB,
	}); status != 200 {
		t.Fatalf("expected 200 for current session B, got %d", status)
	}
}

// TestAuthRateLimit_E2E verifies auth rate limiting (todo 6.6.3): after the
// burst is exhausted, further login attempts are rejected with 429.
func TestAuthRateLimit_E2E(t *testing.T) {
	dir := t.TempDir()
	buildAuthSpecDir(t, dir)
	api.ResetAuthRateLimiters()

	app, err := New(Config{
		SpecPath:  dir,
		DSN:       "sqlite:" + filepath.Join(t.TempDir(), "auth_rate_e2e.db"),
		ProdMode:  true,
		JWTSecret: "test-secret",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	seedUser(t, app, "admin", "admin", []string{"*"})

	// Burst is 5 — the first 5 logins succeed.
	for i := 0; i < 5; i++ {
		if status, _ := doJSON(t, app, "POST", "/demo/api/v1/auth/login", map[string]any{
			"username": "admin", "password": "admin",
		}); status != 200 {
			t.Fatalf("login %d: expected 200, got %d", i, status)
		}
	}
	// The 6th login is rate-limited → 429.
	if status, _ := doJSON(t, app, "POST", "/demo/api/v1/auth/login", map[string]any{
		"username": "admin", "password": "admin",
	}); status != 429 {
		t.Fatalf("expected 429 after burst exhausted, got %d", status)
	}
}

// TestAuthWildcardPermission_E2E verifies wildcard permission matching
// (todo 6.2.2): a user with `acme.customers.*` can perform any action on the
// entity (including delete).
func TestAuthWildcardPermission_E2E(t *testing.T) {
	dir := t.TempDir()
	buildAuthSpecDir(t, dir)
	api.ResetAuthRateLimiters()

	app, err := New(Config{
		SpecPath:  dir,
		DSN:       "sqlite:" + filepath.Join(t.TempDir(), "auth_wildcard_e2e.db"),
		ProdMode:  true,
		JWTSecret: "test-secret",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	seedUser(t, app, "admin", "admin", []string{"*"})
	seedUser(t, app, "manager", "manager", []string{"acme.customers.*"})

	// Create a customer as admin.
	adminTok := login(t, app, "admin", "admin")
	status, out := doAuthed(t, app, "POST", "/demo/_ui/entity/acme/customer", adminTok, map[string]any{
		"name": "Carol",
	})
	if status != 201 {
		t.Fatalf("create customer: status %d", status)
	}
	custID, _ := out["data"].(map[string]any)["id"].(string)

	// Manager (acme.customers.*) can delete → 204 (wildcard matches delete).
	managerTok := login(t, app, "manager", "manager")
	if status, _ := doAuthed(t, app, "DELETE", "/demo/_ui/entity/acme/customer/"+custID, managerTok, nil); status != 204 && status != 200 {
		t.Fatalf("expected 204 for wildcard delete, got %d", status)
	}
}

// TestAuthOwnerRole_E2E verifies the workspace-owner role (todo 6.3.4): a user
// holding it resolves to the `*` super-wildcard and can perform any action.
func TestAuthOwnerRole_E2E(t *testing.T) {
	dir := t.TempDir()
	buildAuthSpecDir(t, dir)
	api.ResetAuthRateLimiters()

	app, err := New(Config{
		SpecPath:  dir,
		DSN:       "sqlite:" + filepath.Join(t.TempDir(), "auth_owner_e2e.db"),
		ProdMode:  true,
		JWTSecret: "test-secret",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	seedUser(t, app, "admin", "admin", []string{"*"})

	// Create the workspace-owner role + a user holding it.
	reg := app.Registry()
	roleStore, err := reg.GetEntityStore("formspec.core", "role")
	if err != nil {
		t.Fatalf("role store: %v", err)
	}
	if _, err := roleStore.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "demo", CreatedBy: "test",
		Data: map[string]any{"name": "workspace-owner", "app": "", "module": "", "grants": []any{}},
	}); err != nil {
		t.Fatalf("insert owner role: %v", err)
	}
	userStore, err := reg.GetEntityStore("formspec.core", "user")
	if err != nil {
		t.Fatalf("user store: %v", err)
	}
	hash, err := auth.HashPassword("owner")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if _, err := userStore.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "demo", CreatedBy: "test",
		Data: map[string]any{
			"username": "owner", "password_hash": hash,
			"roles": []string{"workspace-owner"}, "permissions": []string{}, "active": true,
		},
	}); err != nil {
		t.Fatalf("insert owner user: %v", err)
	}

	// Create a customer as admin.
	adminTok := login(t, app, "admin", "admin")
	status, out := doAuthed(t, app, "POST", "/demo/_ui/entity/acme/customer", adminTok, map[string]any{
		"name": "Dave",
	})
	if status != 201 {
		t.Fatalf("create customer: status %d", status)
	}
	custID, _ := out["data"].(map[string]any)["id"].(string)

	// Owner (workspace-owner → *) can delete → 204.
	ownerTok := login(t, app, "owner", "owner")
	if status, _ := doAuthed(t, app, "DELETE", "/demo/_ui/entity/acme/customer/"+custID, ownerTok, nil); status != 204 && status != 200 {
		t.Fatalf("expected 204 for owner delete, got %d", status)
	}
}

// TestAuthAuditLog_E2E verifies every auth attempt is audited (todo 6.6.4):
// a successful and a failed login both appear in the audit log.
func TestAuthAuditLog_E2E(t *testing.T) {
	dir := t.TempDir()
	buildAuthSpecDir(t, dir)
	api.ResetAuthRateLimiters()

	app, err := New(Config{
		SpecPath:  dir,
		DSN:       "sqlite:" + filepath.Join(t.TempDir(), "auth_audit_e2e.db"),
		ProdMode:  true,
		JWTSecret: "test-secret",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	seedUser(t, app, "admin", "admin", []string{"*"})

	// Successful login.
	loginPair(t, app, "admin", "admin")
	// Failed login.
	doJSON(t, app, "POST", "/demo/api/v1/auth/login", map[string]any{
		"username": "admin", "password": "wrong",
	})

	// Verify both appear in the audit log.
	entries := api.RecentAuthAudit(10)
	var sawSuccess, sawFailure bool
	for _, e := range entries {
		if e.Method == "login" && e.Result == "success" {
			sawSuccess = true
		}
		if e.Method == "login" && e.Result == "failure" {
			sawFailure = true
		}
	}
	if !sawSuccess {
		t.Error("expected a login success audit entry")
	}
	if !sawFailure {
		t.Error("expected a login failure audit entry")
	}
}
