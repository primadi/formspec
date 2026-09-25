package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/entity"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// newTicketTestServer builds a router with a recording auth validator wired,
// so ticket issuance/consumption runs through the real middleware stack. The
// validator accepts "ws-token" and resolves it to a user in `workspace`.
func newTicketTestServer(t *testing.T, workspace string) (*httptest.Server, func()) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.OpenSQLite(filepath.Join(dir, "wsticket_test.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	reg := entity.NewRegistry(database, db.DriverSQLite, dir)
	rb := NewRouterBuilder(reg)
	rb.BuildRoutes()
	srv := httptest.NewServer(rb.BuildHTTP())
	t.Cleanup(srv.Close)

	prev := GetAuthValidator()
	SetAuthValidator(ticketValidator{workspace: workspace})
	return srv, func() { SetAuthValidator(prev) }
}

// ticketValidator resolves the literal token "ws-token" to an identity scoped
// to a fixed workspace; anything else is unauthenticated.
type ticketValidator struct{ workspace string }

func (v ticketValidator) Validate(_ context.Context, token string) (*auth.Identity, error) {
	if token != "ws-token" {
		return nil, nil
	}
	return &auth.Identity{
		UserID:      "user-1",
		WorkspaceID: v.workspace,
		Permissions: []string{"*"},
	}, nil
}

// issueTicket POSTs to the ticket endpoint with a Bearer credential and
// returns the decoded response.
func issueTicket(t *testing.T, srv *httptest.Server, workspace, token string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest("POST", srv.URL+"/"+workspace+"/_ui/_ws/ticket", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

func TestWSTicket_IssueAndConnect(t *testing.T) {
	srv, restore := newTicketTestServer(t, "acme")
	defer restore()

	code, body := issueTicket(t, srv, "acme", "ws-token")
	if code != http.StatusOK {
		t.Fatalf("issue: expected 200, got %d (%v)", code, body)
	}
	ticket, _ := body["ticket"].(string)
	if ticket == "" {
		t.Fatalf("issue: expected non-empty ticket, got %v", body)
	}
	if exp, _ := body["expires_in"].(float64); exp != 30 {
		t.Errorf("issue: expected expires_in=30, got %v", body["expires_in"])
	}

	// Connect with the ticket instead of ?token=.
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/acme/_ui/_ws?ticket=" + ticket
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial with ticket: %v", err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })

	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"op":"subscribe","resource":"*"}`)); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
}

func TestWSTicket_UnauthenticatedIssueRejected(t *testing.T) {
	srv, restore := newTicketTestServer(t, "acme")
	defer restore()

	code, _ := issueTicket(t, srv, "acme", "")
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a credential, got %d", code)
	}
}

func TestWSTicket_SingleUse(t *testing.T) {
	srv, restore := newTicketTestServer(t, "acme")
	defer restore()

	_, body := issueTicket(t, srv, "acme", "ws-token")
	ticket, _ := body["ticket"].(string)
	if ticket == "" {
		t.Fatal("no ticket issued")
	}

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/acme/_ui/_ws?ticket=" + ticket
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("first dial should succeed: %v", err)
	}
	_ = conn.CloseNow()

	// Replaying the same ticket must be rejected with 401, not upgraded.
	_, resp, err := websocket.Dial(ctx, url, nil)
	if err == nil {
		t.Fatal("second dial with a consumed ticket must not succeed")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 on replay, got resp=%v err=%v", resp, err)
	}
}

func TestWSTicket_Expired(t *testing.T) {
	store := newWSTicketStore()
	identity := &auth.Identity{UserID: "user-1", WorkspaceID: "acme"}

	ticket, ok := store.issue("acme", identity)
	if !ok {
		t.Fatal("issue failed")
	}
	// Backdate the expiry rather than sleeping the TTL.
	store.mu.Lock()
	entry := store.tickets[ticket]
	entry.expiresAt = time.Now().Add(-time.Second)
	store.tickets[ticket] = entry
	store.mu.Unlock()

	if _, ok := store.consume(ticket); ok {
		t.Fatal("expired ticket must not be consumable")
	}
}

func TestWSTicket_CrossWorkspaceRejected(t *testing.T) {
	srv, restore := newTicketTestServer(t, "acme")
	defer restore()

	_, body := issueTicket(t, srv, "acme", "ws-token")
	ticket, _ := body["ticket"].(string)
	if ticket == "" {
		t.Fatal("no ticket issued")
	}

	// Ticket was issued for "acme"; presenting it on "other" must be refused
	// before the upgrade.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/other/_ui/_ws?ticket=" + ticket
	_, resp, err := websocket.Dial(ctx, url, nil)
	if err == nil {
		t.Fatal("cross-workspace ticket must not connect")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for cross-workspace ticket, got resp=%v err=%v", resp, err)
	}
}

// TestWSTicket_LegacyTokenFallbackStillWorks guards the deprecated ?token=
// path: existing clients must keep connecting while consumers migrate.
func TestWSTicket_LegacyTokenFallbackStillWorks(t *testing.T) {
	srv, restore := newTicketTestServer(t, "acme")
	defer restore()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/acme/_ui/_ws?token=ws-token"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("legacy ?token= dial should still work: %v", err)
	}
	_ = conn.CloseNow()
}

func TestWSTicketStore_RateLimited(t *testing.T) {
	store := newWSTicketStore()
	identity := &auth.Identity{UserID: "user-1", WorkspaceID: "acme"}

	for i := 0; i < wsTicketMaxPerMinute; i++ {
		if _, ok := store.issue("acme", identity); !ok {
			t.Fatalf("issue #%d should succeed", i+1)
		}
	}
	if _, ok := store.issue("acme", identity); ok {
		t.Fatalf("issue #%d must be rate limited", wsTicketMaxPerMinute+1)
	}
}
