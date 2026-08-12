package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// Server is the sidecar's local listener (unix socket or localhost TCP)
// serving the App → Sidecar direction of the protocol:
//
//	POST /ctx/{primitive}/{operation}   — ctx.* primitive proxy (§4.3)
//	GET  /health                        — sidecar + app aggregated health (§7)
type Server struct {
	listenURL string
	ctx       *CtxHandler
	monitor   *AppMonitor // nil = no app monitoring (health reports sidecar only)
	engineOK  func() error
	httpSrv   *http.Server
	listener  net.Listener
}

// NewServer creates the local listener server. listenURL is
// "unix:///tmp/formspec/sidecar.sock" or "http://localhost:PORT".
// engineOK reports engine health (nil error = healthy); it may be nil.
func NewServer(listenURL string, ctxHandler *CtxHandler, monitor *AppMonitor, engineOK func() error) *Server {
	return &Server{listenURL: listenURL, ctx: ctxHandler, monitor: monitor, engineOK: engineOK}
}

// Listen binds the socket/port without serving yet, so callers can fail
// fast on bad configuration.
func (s *Server) Listen() error {
	u, err := url.Parse(s.listenURL)
	if err != nil {
		return fmt.Errorf("listen URL %q: %w", s.listenURL, err)
	}

	switch u.Scheme {
	case "unix":
		socketPath := u.Path
		if u.Host != "" {
			socketPath = u.Host + u.Path
		}
		if socketPath == "" {
			return fmt.Errorf("listen URL %q: missing socket path", s.listenURL)
		}
		if err := os.MkdirAll(filepath.Dir(socketPath), 0755); err != nil {
			return fmt.Errorf("create socket dir: %w", err)
		}
		// Remove a stale socket left by a previous run; refuse to clobber
		// anything that is not a socket.
		if fi, err := os.Lstat(socketPath); err == nil {
			if fi.Mode()&os.ModeSocket == 0 {
				return fmt.Errorf("listen path %q exists and is not a socket", socketPath)
			}
			if err := os.Remove(socketPath); err != nil {
				return fmt.Errorf("remove stale socket: %w", err)
			}
		}
		ln, err := net.Listen("unix", socketPath)
		if err != nil {
			return fmt.Errorf("listen on %s: %w", socketPath, err)
		}
		// The app container runs as a different user; the shared emptyDir
		// mount is the access boundary, not socket permissions.
		if err := os.Chmod(socketPath, 0666); err != nil {
			ln.Close()
			return fmt.Errorf("chmod socket: %w", err)
		}
		s.listener = ln
	case "http":
		addr := u.Host
		if addr == "" {
			return fmt.Errorf("listen URL %q: missing host:port", s.listenURL)
		}
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("listen on %s: %w", addr, err)
		}
		s.listener = ln
	default:
		return fmt.Errorf("listen URL %q: unsupported scheme %q (want unix:// or http://)", s.listenURL, u.Scheme)
	}
	return nil
}

// Serve blocks serving the listener. Call Listen first.
func (s *Server) Serve() error {
	if s.listener == nil {
		if err := s.Listen(); err != nil {
			return err
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/ctx/", s.ctx)
	mux.HandleFunc("/health", s.handleHealth)

	s.httpSrv = &http.Server{Handler: mux}
	err := s.httpSrv.Serve(s.listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpSrv == nil {
		return nil
	}
	return s.httpSrv.Shutdown(ctx)
}

// HealthPayload is the aggregated health report (§7): the sidecar engine's
// own health plus the app process's ping status.
type HealthPayload struct {
	Status string     `json:"status"` // healthy | degraded
	Engine string     `json:"engine"` // healthy | degraded
	App    *AppHealth `json:"app,omitempty"`
}

// AppHealth is the app-process part of the health report.
type AppHealth struct {
	Status string    `json:"status"`
	LastOK time.Time `json:"last_ok,omitempty"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	payload := HealthPayload{Status: StatusHealthy, Engine: StatusHealthy}

	if s.engineOK != nil {
		if err := s.engineOK(); err != nil {
			payload.Engine = StatusDegraded
			payload.Status = StatusDegraded
		}
	}
	if s.monitor != nil {
		appStatus, lastOK := s.monitor.AppStatus()
		payload.App = &AppHealth{Status: appStatus, LastOK: lastOK}
		if appStatus != StatusHealthy {
			payload.Status = StatusDegraded
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}
