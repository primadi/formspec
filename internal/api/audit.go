package api

import (
	"fmt"
	"sync"
	"time"
)

// AuthAuditEntry records one authentication attempt (todo 6.6.4).
type AuthAuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Method    string    `json:"method"` // "login" | "refresh" | "apikey" | "jwt"
	Username  string    `json:"username,omitempty"`
	IP        string    `json:"ip,omitempty"`
	Result    string    `json:"result"` // "success" | "failure"
	Reason    string    `json:"reason,omitempty"`
}

// authAudit is an in-memory recorder of authentication attempts. It keeps a
// bounded ring buffer and logs each attempt to stderr. A durable audit-log
// entity (owned by 4.7/7.x) can be wired here later.
type authAudit struct {
	mu      sync.Mutex
	entries []AuthAuditEntry
	max     int
}

// authAuditLog is the process-wide auth audit recorder.
var authAuditLog = &authAudit{max: 1000}

// record appends an auth attempt to the audit log.
func (a *authAudit) record(e AuthAuditEntry) {
	a.mu.Lock()
	a.entries = append(a.entries, e)
	if len(a.entries) > a.max {
		a.entries = a.entries[len(a.entries)-a.max:]
	}
	a.mu.Unlock()

	fmt.Printf("auth.audit method=%s user=%q ip=%q result=%s reason=%q\n",
		e.Method, e.Username, e.IP, e.Result, e.Reason)
}

// recent returns the most recent n audit entries (for diagnostics).
func (a *authAudit) recent(n int) []AuthAuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	if n > len(a.entries) {
		n = len(a.entries)
	}
	out := make([]AuthAuditEntry, n)
	copy(out, a.entries[len(a.entries)-n:])
	return out
}

// RecentAuthAudit returns the most recent n auth audit entries (todo 6.6.4).
// Exported for diagnostics and e2e verification.
func RecentAuthAudit(n int) []AuthAuditEntry {
	return authAuditLog.recent(n)
}
