package sidecar

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Health status values reported by the sidecar /health endpoint
// (docs/runtimes/04-forma-sidecar.md §7).
const (
	StatusHealthy = "healthy"
	// StatusDegraded means the app process stopped answering pings but the
	// sidecar itself can still serve requests that don't need app handlers
	// (pure CRUD without custom sidecar actions).
	StatusDegraded = "degraded"
)

// AppMonitor pings the app process's lib-forma listener periodically and
// aggregates the result into the sidecar's own health status.
type AppMonitor struct {
	client       *http.Client
	pingURL      string
	interval     time.Duration
	maxFailures  int
	mu           sync.RWMutex
	failures     int
	lastOK       time.Time
	everAnswered bool
}

// NewAppMonitor creates a monitor for the app endpoint (same forms as the
// sidecar executor: unix:///path.sock or http://localhost:PORT). The app is
// reported degraded after maxFailures consecutive missed pings.
func NewAppMonitor(appEndpoint string, interval time.Duration, maxFailures int) (*AppMonitor, error) {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if maxFailures <= 0 {
		maxFailures = 3
	}

	client, base, err := newLocalHTTPClient(appEndpoint)
	if err != nil {
		return nil, err
	}
	client.Timeout = 5 * time.Second

	return &AppMonitor{
		client:      client,
		pingURL:     base + "/health",
		interval:    interval,
		maxFailures: maxFailures,
	}, nil
}

// Run pings the app until ctx is cancelled.
func (m *AppMonitor) Run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	m.pingOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.pingOnce(ctx)
		}
	}
}

func (m *AppMonitor) pingOnce(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.pingURL, nil)
	if err != nil {
		m.record(false)
		return
	}
	resp, err := m.client.Do(req)
	if err != nil {
		m.record(false)
		return
	}
	resp.Body.Close()
	m.record(resp.StatusCode < 500)
}

func (m *AppMonitor) record(ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ok {
		m.failures = 0
		m.lastOK = time.Now()
		m.everAnswered = true
		return
	}
	m.failures++
}

// AppStatus reports the app-process side of health aggregation.
func (m *AppMonitor) AppStatus() (status string, lastOK time.Time) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.failures >= m.maxFailures {
		return StatusDegraded, m.lastOK
	}
	return StatusHealthy, m.lastOK
}

// newLocalHTTPClient builds an HTTP client + base URL for a local endpoint,
// either unix:///path.sock or http://localhost:PORT.
func newLocalHTTPClient(endpoint string) (*http.Client, string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, "", err
	}
	switch u.Scheme {
	case "unix":
		socketPath := u.Path
		if u.Host != "" {
			socketPath = u.Host + u.Path
		}
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		}
		return &http.Client{Transport: transport}, "http://forma-app", nil
	case "http":
		return &http.Client{}, strings.TrimRight(endpoint, "/"), nil
	default:
		return nil, "", fmt.Errorf("endpoint %q: unsupported scheme %q (want unix:// or http://)", endpoint, u.Scheme)
	}
}
