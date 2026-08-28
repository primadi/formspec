package observability

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Health status vocabulary (spec §5) — the single vocabulary shared with
// the failover doc (architecture/05-failover.md §7) and Plane Protocol
// health evidence.
const (
	StatusHealthy   = "healthy"
	StatusDegraded  = "degraded"
	StatusUnhealthy = "unhealthy"
)

// Controlled reason codes (spec §5). Only these may appear in reasons[].
const (
	ReasonSnapshotStale           = "snapshot_stale"
	ReasonDatastoreUnreachable    = "datastore_unreachable"
	ReasonDBPoolExhausted         = "db_pool_exhausted"
	ReasonOutboxBacklog           = "outbox_backlog"
	ReasonControlPlaneUnreachable = "control_plane_unreachable"
)

// Probe is a named health check. It returns a reason code when unhealthy,
// "" when fine. A probe may also report degraded via a second return
// convention: return (reason, true) where the bool marks hard failure
// (unhealthy); without it, a reason means degraded.
type Probe func() (reason string, unhealthy bool)

// Health aggregates probes and computes the machine-readable status.
type Health struct {
	mu     sync.RWMutex
	probes map[string]Probe
}

// NewHealth creates an empty health registry.
func NewHealth() *Health {
	return &Health{probes: make(map[string]Probe)}
}

// Register adds or replaces a probe under a reason code.
func (h *Health) Register(reason string, p Probe) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.probes[reason] = p
}

// Report is the /health response body (spec §5).
type Report struct {
	Status    string   `json:"status"`
	Reasons   []string `json:"reasons"`
	CheckedAt string   `json:"checked_at"`
}

// Check runs all probes and derives the status:
//   - any unhealthy probe → unhealthy
//   - any degraded probe  → degraded
//   - otherwise           → healthy
func (h *Health) Check() Report {
	h.mu.RLock()
	probes := make(map[string]Probe, len(h.probes))
	for k, v := range h.probes {
		probes[k] = v
	}
	h.mu.RUnlock()

	var reasons []string
	status := StatusHealthy
	for code, p := range probes {
		reason, unhealthy := p()
		if reason == "" {
			continue
		}
		reasons = append(reasons, code)
		if unhealthy {
			status = StatusUnhealthy
		} else if status == StatusHealthy {
			status = StatusDegraded
		}
	}
	if reasons == nil {
		reasons = []string{}
	}
	return Report{
		Status:    status,
		Reasons:   reasons,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// writeHealthJSON marshals the report; on marshal failure falls back to a
// minimal unhealthy body (never fail the probe silently).
func writeHealthJSON(w http.ResponseWriter, rep Report) {
	b, err := json.Marshal(rep)
	if err != nil {
		b = []byte(`{"status":"unhealthy","reasons":["datastore_unreachable"],"checked_at":""}`)
	}
	w.Write(b)
}

// Handler serves GET /health with the machine-readable report. The same
// endpoint serves liveness and readiness (spec §5): readiness passes when
// status ∈ {healthy, degraded}; liveness passes while the process answers.
func (h *Health) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rep := h.Check()
		code := http.StatusOK
		if rep.Status == StatusUnhealthy {
			code = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		writeHealthJSON(w, rep)
	})
}
