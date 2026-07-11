package control

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/primadi/forma/internal/artifact"
)

// Server is the Control Plane HTTP server.
type Server struct {
	httpServer *http.Server
	store      artifact.Store
	signer     *artifact.Signer
	port       int
	devMode    bool
}

// NewServer creates a new Control Plane server.
func NewServer(store artifact.Store, signer *artifact.Signer, port int, devMode bool) *Server {
	return &Server{
		store:   store,
		signer:  signer,
		port:    port,
		devMode: devMode,
	}
}

// Start starts the Control Plane HTTP server.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Initialize handlers
	registerHandler := NewRegisterHandler(s.store, s.signer)
	snapshotHandler := NewSnapshotHandler(s.store, s.signer)
	evidenceHandler := NewEvidenceHandler(s.store)

	// Register routes
	mux.HandleFunc("/v1/artifacts", registerHandler.HandleRegister)
	mux.HandleFunc("/v1/artifacts/", s.handleGetArtifact)
	mux.HandleFunc("/v1/snapshot", snapshotHandler.HandleSnapshot)
	mux.HandleFunc("/v1/evidence", evidenceHandler.HandleEvidence)

	// Dev-only routes
	if s.devMode {
		pollHandler := NewPollHandler(snapshotHandler)
		mux.HandleFunc("/v1/poll", pollHandler.HandlePoll)
	}

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// CORS middleware for dev
	handler := corsMiddleware(mux)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	mode := "PRODUCTION"
	if s.devMode {
		mode = "DEVELOPMENT"
	}
	log.Printf("[control] Server starting in %s mode on :%d", mode, s.port)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("control server: %w", err)
	}
	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// handleGetArtifact serves GET /v1/artifacts/{id}.
func (s *Server) handleGetArtifact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	// Extract ID from path: /v1/artifacts/{id}
	id := r.URL.Path[len("/v1/artifacts/"):]
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing artifact id"})
		return
	}

	artifact, err := s.store.GetArtifact(r.Context(), artifact.ArtifactID(id))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("artifact %s not found", id),
		})
		return
	}

	if artifact.Envelope == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("artifact %s has no envelope", id),
		})
		return
	}

	writeJSON(w, http.StatusOK, artifact.Envelope)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, If-None-Match, ETag")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
