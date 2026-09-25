package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/primadi/formspec/internal/auth"
)

// WebSocket handshake auth via single-use ticket (todo 5.8.4; plan
// docs_internal/plan/ws-ticket-auth-plan.md).
//
// Browsers cannot set an Authorization header on a WebSocket handshake, so the
// realtime client historically passed its full-lifetime JWT as ?token= in the
// URL. Query strings leak into proxy/server access logs, browser history, and
// Referer headers, so the JWT — valid for far longer than the connection
// handshake — ends up recorded in places it should not be.
//
// The ticket flow removes the JWT from the URL entirely:
//
//	1. POST /{ws}/_ui/_ws/ticket   (Authorization: Bearer — never in a URL)
//	2. → {"ticket": "<opaque>", "expires_in": 30}
//	3. WS   /{ws}/_ui/_ws?ticket=…  (handshake)
//	4. server consumes the ticket once → resolves identity → deletes it
//
// An access log now only ever sees an opaque, single-use value that is dead 30
// seconds after issuance; replaying it is rejected. ?token= remains supported
// as a fallback for older clients (deprecated; see the plan's follow-up to
// retire it once all consumers migrate).

const (
	// wsTicketTTL is how long an issued ticket stays valid. Deliberately
	// short: the client requests one immediately before connecting, so the
	// window only has to cover the handshake round-trip.
	wsTicketTTL = 30 * time.Second
	// wsTicketMaxPerMinute bounds issuance per (workspace, user) so a
	// compromised session cannot mint tickets without limit.
	wsTicketMaxPerMinute = 60
	// wsTicketRateWindow is the sliding window for the issuance limiter.
	wsTicketRateWindow = time.Minute
)

// wsTicket is a single-use, opaque handshake credential bound to the identity
// and workspace that requested it.
type wsTicket struct {
	workspaceID string
	identity    *auth.Identity
	expiresAt   time.Time
}

// wsTicketStore holds outstanding tickets and the issuance rate limiter.
// In-memory, matching the oauthStates pattern in oauth_handler.go: tickets are
// short-lived and single-server (the WS connection is also server-local).
type wsTicketStore struct {
	mu      sync.Mutex
	tickets map[string]wsTicket
	issued  map[string][]time.Time // key: workspaceID + "\x00" + userID
}

func newWSTicketStore() *wsTicketStore {
	return &wsTicketStore{
		tickets: make(map[string]wsTicket),
		issued:  make(map[string][]time.Time),
	}
}

// issue mints a new ticket for identity/workspace. It returns ("", false) when
// the issuance rate limit for this identity is exhausted.
func (s *wsTicketStore) issue(workspaceID string, identity *auth.Identity) (string, bool) {
	now := time.Now()

	// Random 32-byte opaque value. crypto/rand.Read never returns an error
	// on any supported platform (and panics inside the stdlib otherwise), so
	// there is no partial-value fallback to write here.
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	ticket := hex.EncodeToString(raw)

	userID := ""
	if identity != nil {
		userID = identity.UserID
	}
	key := workspaceID + "\x00" + userID

	s.mu.Lock()
	defer s.mu.Unlock()

	// Sliding-window rate limit (prune, then count).
	window := s.issued[key]
	cutoff := now.Add(-wsTicketRateWindow)
	kept := window[:0]
	for _, ts := range window {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= wsTicketMaxPerMinute {
		s.issued[key] = kept
		return "", false
	}
	s.issued[key] = append(kept, now)

	// Opportunistic sweep of expired tickets so the map cannot grow without
	// bound if issuance outpaces consumption.
	for k, t := range s.tickets {
		if now.After(t.expiresAt) {
			delete(s.tickets, k)
		}
	}

	s.tickets[ticket] = wsTicket{
		workspaceID: workspaceID,
		identity:    identity,
		expiresAt:   now.Add(wsTicketTTL),
	}
	return ticket, true
}

// consume returns the ticket bound to value and removes it — single use. An
// unknown or expired ticket returns ok=false.
func (s *wsTicketStore) consume(value string) (wsTicket, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tickets[value]
	if !ok {
		return wsTicket{}, false
	}
	delete(s.tickets, value)
	if time.Now().After(t.expiresAt) {
		return wsTicket{}, false
	}
	return t, true
}

// HandleWSTicket issues a single-use ticket for the authenticated caller.
//
// POST /{ws}/_ui/_ws/ticket — no request body; identity comes from
// AuthMiddleware (Authorization header or session cookie, never a query
// param), which is what keeps the JWT out of the WS URL.
func (b *RouterBuilder) HandleWSTicket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity := IdentityFromContext(r.Context())
		if identity == nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}
		workspaceID := workspaceFromContext(r.Context())

		ticket, ok := b.wsTickets.issue(workspaceID, identity)
		if !ok {
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many ticket requests")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ticket":     ticket,
			"expires_in": int(wsTicketTTL / time.Second),
		})
	}
}
